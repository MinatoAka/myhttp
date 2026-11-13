package myhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

		// Kalau penyebab gagalnya context mati → berhenti, jangan retry
		if ctx.Err() != nil {
			break
		}

		// Attempt terakhir → jangan tidur
		if attempt == t.retries {
			break
		}

		// Jitter: tambah waktu random 0-500ms (hindari thundering herd)
		jitter := time.Duration(rand.Intn(500)) * time.Millisecond

		// Tidur yang bisa dibangunkan oleh context
		if !sleepCtx(ctx, wait+jitter) {
			break
		}

		wait *= 2 // Exponential backoff
	}

	return nil, fmt.Errorf("gagal setelah %d attempt: %w", attempts, lastErr)
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
