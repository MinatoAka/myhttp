package myhttp

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// newDNSTransport membuat http.Transport dengan custom DNS resolution via HTTPS (DoH).
// Nama dibuat umum agar implementasi internal bisa diubah tanpa memengaruhi API publik.
func newDNSTransport(dnsURL string) *http.Transport {
	// Client khusus untuk request DNS (menggunakan DefaultTransport agar tidak recursive/infinite loop)
	dnsClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   5 * time.Second,
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			// PENTING: Jika host sudah berupa IP address (misal: 127.0.0.1 atau ::1), 
			// lewati resolusi DNS dan langsung connect. Ini mencegah error saat 
			// testing dengan httptest.NewServer atau request ke IP langsung.
			if net.ParseIP(host) != nil {
				dialer := &net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}
				return dialer.DialContext(ctx, network, addr)
			}

			// 1. Build DNS Query (Type A untuk IPv4)
			msg := dnsmessage.Message{
				Header: dnsmessage.Header{
					ID:               1,
					RecursionDesired: true,
				},
				Questions: []dnsmessage.Question{
					{
						Name:  dnsmessage.MustNewName(host + "."), // FQDN requires trailing dot
						Type:  dnsmessage.TypeA,
						Class: dnsmessage.ClassINET,
					},
				},
			}

			packed, err := msg.Pack()
			if err != nil {
				return nil, fmt.Errorf("gagal pack DNS message: %w", err)
			}

			// 2. Kirim request POST ke endpoint DNS
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, dnsURL, bytes.NewReader(packed))
			if err != nil {
				return nil, fmt.Errorf("gagal buat request DNS: %w", err)
			}
			req.Header.Set("Content-Type", "application/dns-message")
			req.Header.Set("Accept", "application/dns-message")

			resp, err := dnsClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("request DNS gagal: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("DNS mengembalikan status bukan 200: %d", resp.StatusCode)
			}

			// 3. Baca dan Parse Response DNS
			body := make([]byte, 512)
			n, _ := resp.Body.Read(body)
			body = body[:n]

			var responseMsg dnsmessage.Message
			if err := responseMsg.Unpack(body); err != nil {
				return nil, fmt.Errorf("gagal unpack respons DNS: %w", err)
			}

			// 4. Ekstrak Record A (IPv4) pertama yang ditemukan
			for _, ans := range responseMsg.Answers {
				if ans.Header.Type == dnsmessage.TypeA {
					if a, ok := ans.Body.(*dnsmessage.AResource); ok {
						ipStr := net.IP(a.A[:]).String()

						// 5. Connect ke IP yang berhasil di-resolve
						dialer := &net.Dialer{
							Timeout:   10 * time.Second,
							KeepAlive: 30 * time.Second,
						}
						return dialer.DialContext(ctx, network, net.JoinHostPort(ipStr, port))
					}
				}
			}

			return nil, fmt.Errorf("tidak ada record A (IPv4) ditemukan untuk host: %s", host)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
}
