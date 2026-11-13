package myhttp

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryOn429 — 429 harus di-retry, bukan dianggap gagal final
func TestRetryOn429(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(WithRetry(3, 10*time.Millisecond))

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("harusnya sukses setelah retry 429, err: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, mau 200", resp.StatusCode)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, mau 3", attempts)
	}
}

// TestRetryAfterSeconds — header Retry-After harus mengalahkan backoff
func TestRetryAfterSeconds(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// baseWait kecil: kalau Retry-After diabaikan, jeda cuma ~10ms
	client := New(
		WithRetry(3, 10*time.Millisecond),
		WithTimeout(10*time.Second),
	)

	start := time.Now()
	resp, err := client.Get(server.URL)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("harusnya sukses, err: %v", err)
	}
	defer resp.Body.Close()

	if elapsed < 900*time.Millisecond {
		t.Errorf("Retry-After diabaikan: %v (harusnya ≥1s)", elapsed)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, mau 2", attempts)
	}
}

// TestNoRetryOn4xx — 4xx selain 429 tidak boleh di-retry
func TestNoRetryOn4xx(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 422} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var attempts int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(code)
			}))
			defer server.Close()

			client := New(WithRetry(3, 10*time.Millisecond))

			resp, err := client.Get(server.URL)
			if err != nil {
				t.Fatalf("4xx harusnya langsung balik, bukan error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != code {
				t.Errorf("status = %d, mau %d", resp.StatusCode, code)
			}
			if attempts != 1 {
				t.Errorf("attempts = %d, mau 1 (4xx tidak boleh di-retry)", attempts)
			}
		})
	}
}

// TestParseRetryAfter — unit test parser header
func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"kosong", "", 0},
		{"detik", "5", 5 * time.Second},
		{"detik nol", "0", 0},
		{"negatif", "-3", 0},
		{"sampah", "abc", 0},
		{"tanggal lewat", "Wed, 21 Oct 2015 07:28:00 GMT", 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseRetryAfter(c.in); got != c.want {
				t.Errorf("parseRetryAfter(%q) = %v, mau %v", c.in, got, c.want)
			}
		})
	}

	// HTTP-date di masa depan
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got < 25*time.Second || got > 31*time.Second {
		t.Errorf("parseRetryAfter(future) = %v, mau ~30s", got)
	}
}
