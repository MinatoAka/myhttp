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
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(WithRetry(3, 10*time.Millisecond))

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

// ==========================================
// TEST BARU UNTUK FITUR DNS
// ==========================================

// TestWithDNSOption — memastikan option WithDNS bekerja dengan benar
func TestWithDNSOption(t *testing.T) {
	client1 := New(WithDNS(""))
	if client1 == nil {
		t.Fatal("client tidak boleh nil")
	}

	client2 := New(WithDNS("https://cloudflare-dns.com/dns-query"))
	if client2 == nil {
		t.Fatal("client tidak boleh nil")
	}

	client3 := New(WithRetry(3, time.Second))
	if client3 == nil {
		t.Fatal("client tidak boleh nil")
	}
}

// TestDNSWithLocalServer — memastikan client dengan DNS custom tetap bisa request ke server lokal (IP bypass)
func TestDNSWithLocalServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := New(
		WithDNS(""),
		WithTimeout(5*time.Second),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request ke local server gagal (IP bypass tidak bekerja): %v", err)
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

	client := New(
		WithDNS(""),
		WithRetry(3, 10*time.Millisecond),
		WithRateLimit(10),
		WithTimeout(5*time.Second),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
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
