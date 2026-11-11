package myhttp

import (
    "fmt"
    "net/http"
    "time"
)

// Client — wrapper http.Client dengan retry & rate limit
type Client struct {
    http *http.Client
}

// New — bikin client baru dengan options
func New(opts ...Option) *Client {
    // Default config
    cfg := options{
        retries:       3,
        retryBaseWait: 1 * time.Second,
        timeout:       15 * time.Second,
        userAgent:     "myhttp/1.0",
    }

    // Apply options
    for _, opt := range opts {
        opt(&cfg)
    }

    // Build RoundTripper chain:
    // base → retry → ratelimit
    var transport http.RoundTripper = http.DefaultTransport
    
    if cfg.retries > 0 {
        transport = &retryTransport{
            base:    transport,
            retries: cfg.retries,
            wait:    cfg.retryBaseWait,
        }
    }
    
    if cfg.rateLimit > 0 {
        transport = newRateLimitTransport(transport, cfg.rateLimit)
    }

    // Wrap untuk inject User-Agent otomatis
    transport = &uaTransport{base: transport, userAgent: cfg.userAgent}

    return &Client{
        http: &http.Client{
            Transport: transport,
            Timeout:   cfg.timeout,
        },
    }
}

// uaTransport — inject User-Agent
type uaTransport struct {
    base      http.RoundTripper
    userAgent string
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    r := req.Clone(req.Context())
    if r.Header.Get("User-Agent") == "" {
        r.Header.Set("User-Agent", t.userAgent)
    }
    return t.base.RoundTrip(r)
}

// Get — shortcut untuk GET request
func (c *Client) Get(url string) (*http.Response, error) {
    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("bikin request: %w", err)
    }
    return c.http.Do(req)
}

// Do — raw request
func (c *Client) Do(req *http.Request) (*http.Response, error) {
    return c.http.Do(req)
}
