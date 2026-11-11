package myhttp

import (
    "net/http"
    "sync"
    "time"
)

// rateLimitTransport — RoundTripper yang rate limit
type rateLimitTransport struct {
    base    http.RoundTripper
    ticker  *time.Ticker
    mu      sync.Mutex
    lastReq time.Time
}

func newRateLimitTransport(base http.RoundTripper, reqPerSec int) *rateLimitTransport {
    interval := time.Second / time.Duration(reqPerSec)
    return &rateLimitTransport{
        base:   base,
        ticker: time.NewTicker(interval),
    }
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    t.mu.Lock()
    <-t.ticker.C // tunggu sampai "jatah" berikutnya tersedia
    t.mu.Unlock()

    return t.base.RoundTrip(req)
}
