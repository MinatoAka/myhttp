package myhttp

import (
    "fmt"
    "math/rand"
    "net/http"
    "time"
)

// retryTransport — RoundTripper yang retry otomatis
type retryTransport struct {
    base    http.RoundTripper
    retries int
    wait    time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    var lastErr error
    wait := t.wait

    for attempt := 0; attempt <= t.retries; attempt++ {
        // Clone request (body tidak bisa dibaca 2x)
        r := req.Clone(req.Context())
        
        resp, err := t.base.RoundTrip(r)
        
        // Sukses → langsung return
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }
        
        // Error 5xx → retry (server sedang bermasalah)
        if err == nil && resp.StatusCode >= 500 {
            resp.Body.Close()
            lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
        } else if err != nil {
            lastErr = err
        }

        // Attempt terakhir → jangan tidur
        if attempt == t.retries {
            break
        }

        // Jitter: tambah waktu random 0-500ms (hindari thundering herd)
        jitter := time.Duration(rand.Intn(500)) * time.Millisecond
        time.Sleep(wait + jitter)
        
        wait *= 2 // Exponential backoff
    }

    return nil, fmt.Errorf("setelah %d retry: %w", t.retries+1, lastErr)
}
