package myhttp

import (
	"net/http"
	"sync"
	"time"
)

// rateLimitTransport — RoundTripper yang rate limit
type rateLimitTransport struct {
	base     http.RoundTripper
	interval time.Duration
	mu       sync.Mutex
	lastReq  time.Time
}

func newRateLimitTransport(base http.RoundTripper, reqPerSec int) *rateLimitTransport {
	// Guard: cegah panic divide by zero
	if reqPerSec <= 0 {
		reqPerSec = 1
	}
	return &rateLimitTransport{
		base:     base,
		interval: time.Second / time.Duration(reqPerSec),
	}
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	t.mu.Lock()
	elapsed := time.Since(t.lastReq)
	if elapsed < t.interval {
		// Gunakan sleepCtx agar bisa dibangunkan oleh context
		if !sleepCtx(ctx, t.interval-elapsed) {
			t.mu.Unlock()
			return nil, ctx.Err()
		}
	}
	t.lastReq = time.Now()
	t.mu.Unlock()

	return t.base.RoundTrip(req)
}
