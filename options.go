package myhttp

import "time"

// options — struct internal untuk menyimpan config
type options struct {
    retries       int
    retryBaseWait time.Duration
    rateLimit     int           // request per detik (0 = unlimited)
    timeout       time.Duration
    userAgent     string
}

// Option — tipe fungsi yang memodifikasi options
type Option func(*options)

// WithRetry — konfigurasi retry
func WithRetry(maxRetries int, baseWait time.Duration) Option {
    return func(o *options) {
        o.retries = maxRetries
        o.retryBaseWait = baseWait
    }
}

// WithRateLimit — batasi request per detik
func WithRateLimit(reqPerSec int) Option {
    return func(o *options) {
        o.rateLimit = reqPerSec
    }
}

// WithTimeout — timeout per request
func WithTimeout(d time.Duration) Option {
    return func(o *options) {
        o.timeout = d
    }
}

// WithUserAgent — set User-Agent
func WithUserAgent(ua string) Option {
    return func(o *options) {
        o.userAgent = ua
    }
}
