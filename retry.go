package myhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxRetryWait — batas atas jeda antar attempt.
const maxRetryWait = 60 * time.Second

// retryTransport — RoundTripper yang retry otomatis
type retryTransport struct {
	base      http.RoundTripper
	retries   int
	wait      time.Duration
	retryPOST bool // TAMBAHAN: Izinkan retry POST
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	var lastErr error
	wait := t.wait
	attempts := 0

	// PENTING: Jika method POST dan retryPOST tidak diaktifkan, 
	// paksa maxAttempts = 0 agar hanya dijalankan 1 kali (tanpa loop).
	maxAttempts := t.retries
	if req.Method == http.MethodPost && !t.retryPOST {
		maxAttempts = 0
	}

	// Baca Body sekali di awal, simpan di memori (biar bisa di-replay tiap attempt)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca body: %w", err)
		}
		req.Body.Close() // Tutup body asli
	}

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		// Context sudah mati? Jangan buang waktu untuk attempt yang pasti gagal
		if err := ctx.Err(); err != nil {
			if lastErr == nil {
				lastErr = err
			}
			break
		}

		attempts++

		// Clone request
		r := req.Clone(ctx)

		// Selalu buat reader baru dari bodyBytes untuk setiap percobaan
		if bodyBytes != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := t.base.RoundTrip(r)

		// Sukses (bukan 5xx, bukan 429) → langsung return
		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Gagal → catat penyebab, ambil Retry-After kalau ada
		var retryAfter time.Duration
		if err == nil {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		// Kalau penyebab gagalnya context mati → berhenti, jangan retry
		if ctx.Err() != nil {
			break
		}

		// Attempt terakhir → jangan tidur
		if attempt == maxAttempts {
			break
		}

		// Tentukan jeda
		var delay time.Duration
		if retryAfter > 0 {
			// Server kasih tahu kapan boleh balik → pakai itu, backoff tidak dinaikkan
			delay = retryAfter
		} else {
			// Jitter: random 0-500ms (hindari thundering herd)
			jitter := time.Duration(rand.Intn(2000)) * time.Millisecond
			delay = wait + jitter
			wait *= 2 // Exponential backoff
			if wait > maxRetryWait {
				wait = maxRetryWait
			}
		}

		if delay > maxRetryWait {
			delay = maxRetryWait
		}

		// Tidur yang bisa dibangunkan oleh context
		if !sleepCtx(ctx, delay) {
			break
		}
	}

	return nil, fmt.Errorf("gagal setelah %d attempt: %w", attempts, lastErr)
}

// retryableStatus — status yang layak dicoba lagi.
func retryableStatus(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests
}

// parseRetryAfter — baca header Retry-After.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// Format 1: detik
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	// Format 2: HTTP-date
	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// sleepCtx — tidur selama d, tapi langsung bangun kalau context dibatalkan.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
