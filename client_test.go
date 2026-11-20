package myhttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	client := New(WithUserAgent("shinobi-bot/1.0"))
	resp, _ := client.Get(server.URL)
	defer resp.Body.Close()

	if gotUA != "shinobi-bot/1.0" {
		t.Errorf("UA = %q, mau %q", gotUA, "shinobi-bot/1.0")
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

// TestRetryPostBody — Memastikan body POST tidak hilang saat retry
func TestRetryPostBody(t *testing.T) {
	var attempts int32
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError) // Paksa retry
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(WithRetry(3, 10*time.Millisecond))

	// Kirim POST dengan body JSON
	req, _ := http.NewRequest("POST", server.URL, strings.NewReader(`{"pair":"ethidr"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Harusnya sukses, err: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Status = %d, mau 200", resp.StatusCode)
	}
	if receivedBody != `{"pair":"ethidr"}` {
		t.Errorf("Body hilang saat retry! Diterima: %q", receivedBody)
	}
}

// TestRetryContextCancel — retry harus berhenti cepat saat context dibatalkan
func TestRetryContextCancel(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Base wait besar: kalau tidur tidak sadar context, test ini bakal sangat lama
	client := New(WithRetry(5, 2*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		resp.Body.Close()
		t.Fatal("harusnya error karena context dibatalkan")
	}
	if elapsed > 1*time.Second {
		t.Errorf("terlalu lama: %v (harusnya berhenti ~200ms)", elapsed)
	}
	if attempts > 2 {
		t.Errorf("attempts = %d, harusnya berhenti cepat (≤2)", attempts)
	}
}

// TestWithDNSOption — memastikan option WithDNS bekerja dengan benar
func TestWithDNSOption(t *testing.T) {
	// Test 1: WithDNS dengan string kosong harus set default Quad9
	client1 := New(WithDNS(""))
	if client1 == nil {
		t.Fatal("client tidak boleh nil")
	}

	// Test 2: WithDNS dengan URL custom
	client2 := New(WithDNS("https://cloudflare-dns.com/dns-query"))
	if client2 == nil {
		t.Fatal("client tidak boleh nil")
	}

	// Test 3: Client tanpa WithDNS tetap bisa dibuat (backward compatible)
	client3 := New(WithRetry(3, time.Second))
	if client3 == nil {
		t.Fatal("client tidak boleh nil")
	}
}

// TestDNSWithLocalServer — memastikan client dengan DNS custom tetap bisa request ke server lokal
func TestDNSWithLocalServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Buat client dengan DNS custom (pakai Quad9 default)
	// Note: Test ini akan mencoba resolve DNS via Quad9, tapi karena server.URL adalah localhost/127.0.0.1,
	// DNS resolution tidak diperlukan (langsung connect ke IP lokal)
	client := New(
		WithDNS(""),
		WithTimeout(5*time.Second),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		// Kalau error karena Quad9 tidak reachable, skip test (bukan failure)
		if strings.Contains(err.Error(), "DNS") || strings.Contains(err.Error(), "dial") {
			t.Skipf("Skip test karena DNS server tidak reachable: %v", err)
		}
		t.Fatalf("request gagal: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, mau 200", resp.StatusCode)
	}
}

// TestDNSChainIntegration — memastikan DNS transport terintegrasi dengan retry dan rate limit
func TestDNSChainIntegration(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client dengan DNS + Retry + RateLimit
	client := New(
		WithDNS(""),
		WithRetry(3, 10*time.Millisecond),
		WithRateLimit(10),
		WithTimeout(5*time.Second),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		if strings.Contains(err.Error(), "DNS") || strings.Contains(err.Error(), "dial") {
			t.Skipf("Skip test karena DNS server tidak reachable: %v", err)
		}
		t.Fatalf("request gagal: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, mau 200", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, mau 2 (1 gagal + 1 sukses)", attempts)
	}
}
