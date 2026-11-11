package myhttp

import (
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"
)

// TestRetryOn5xx — memastikan retry bekerja
func TestRetryOn5xx(t *testing.T) {
    var attempts int32
    
    // Server yang selalu balas 500 (kecuali attempt terakhir)
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        n := atomic.AddInt32(&attempts, 1)
        if n < 3 {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"ok": true}`))
    }))
    defer server.Close()

    client := New(
        WithRetry(3, 10*time.Millisecond),
        WithUserAgent("test/1.0"),
    )

    resp, err := client.Get(server.URL)
    if err != nil {
        t.Fatalf("harusnya sukses setelah retry, dapat error: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        t.Errorf("status = %d, mau 200", resp.StatusCode)
    }
    if attempts != 3 {
        t.Errorf("attempts = %d, mau 3", attempts)
    }
}

// TestUserAgent — memastikan UA ter-inject
func TestUserAgent(t *testing.T) {
    var gotUA string
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotUA = r.Header.Get("User-Agent")
        w.WriteHeader(200)
    }))
    defer server.Close()

    client := New(WithUserAgent("jenderal/1.0"))
    resp, _ := client.Get(server.URL)
    defer resp.Body.Close()

    if gotUA != "jenderal/1.0" {
        t.Errorf("UA = %q, mau %q", gotUA, "jenderal/1.0")
    }
}

// TestRateLimit — memastikan tidak lebih cepat dari limit
func TestRateLimit(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))
    defer server.Close()

    client := New(WithRateLimit(5)) // 5 req/detik = 200ms/req
    
    start := time.Now()
    for i := 0; i < 3; i++ {
        resp, _ := client.Get(server.URL)
        resp.Body.Close()
    }
    elapsed := time.Since(start)

    // 3 request dengan limit 5/detik minimal butuh ~400ms
    if elapsed < 350*time.Millisecond {
        t.Errorf("terlalu cepat: %v (harusnya ≥400ms)", elapsed)
    }
}
