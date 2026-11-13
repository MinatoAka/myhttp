package myhttp

import (
	"bytes"
	"fmt"
	"io"
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxRetryWait — batas atas jeda antar attempt.
// Mencegah Retry-After besar (misal 3600) bikin request menggantung.
const maxRetryWait = 60 * time.Second

// retryTransport — RoundTripper yang retry otomatis
type retryTransport struct {
	base    http.RoundTripper
	retries int
	wait    time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	var lastErr error
	wait := t.wait
	attempts := 0

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

	for attempt := 0; attempt <= t.retries; attempt++ {
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
		if attempt == t.retries {
			break
		}

		// Tentukan jeda
		var delay time.Duration
		if retryAfter > 0 {
			// Server kasih tahu kapan boleh balik → pakai itu, backoff tidak dinaikkan
			delay = retryAfter
		} else {
			// Jitter: random 0-500ms (hindari thundering herd)
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
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
// 5xx = server bermasalah, 429 = rate limit (sementara).
// 4xx lain (400/401/403/404) tidak di-retry karena retry tidak akan menolong.
func retryableStatus(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests
}

// parseRetryAfter — baca header Retry-After.
// Support dua format: delta-seconds ("5") dan HTTP-date ("Wed, 21 Oct 2026 07:28:00 GMT").
// Return 0 kalau header kosong, tidak valid, atau waktunya sudah lewat.
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
// Return true kalau tidur selesai normal, false kalau dipotong context.
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
