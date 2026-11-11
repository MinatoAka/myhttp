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
	return &rateLimitTransport{
		base:     base,
		interval: time.Second / time.Duration(reqPerSec),
	}
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Kunci agar request lain menunggu giliran
	t.mu.Lock()
	
	// Hitung waktu yang berlalu sejak request terakhir
	elapsed := time.Since(t.lastReq)
	if elapsed < t.interval {
		// Jika terlalu cepat, tidur sejenak
		time.Sleep(t.interval - elapsed)
	}
	
	// Catat waktu request ini
	t.lastReq = time.Now()
	
	// Lepas kunci
	t.mu.Unlock()

	return t.base.RoundTrip(req)
}
