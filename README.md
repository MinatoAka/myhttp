[![Go Reference](https://pkg.go.dev/badge/github.com/MinatoAka/myhttp.svg)](https://pkg.go.dev/github.com/MinatoAka/myhttp)
[![Auto Release](https://github.com/MinatoAka/myhttp/actions/workflows/release.yml/badge.svg?branch=main&event=release)](https://github.com/MinatoAka/myhttp/actions/workflows/release.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Support via Saweria](https://img.shields.io/badge/Support-Saweria-orange)](https://saweria.co/AKaAturDompet)

# myhttp

A lightweight Go HTTP client with configurable retries, rate limiting, timeouts, and User-Agent. Uses only the standard library.


## Install

```bash
go get github.com/MinatoAka/myhttp
```


## Quick start

```go
client := myhttp.New(
    myhttp.WithRetry(3, time.Second),
    myhttp.WithRateLimit(5),
    myhttp.WithTimeout(10*time.Second),
    myhttp.WithUserAgent("myapp/1.0"),
)

resp, err := client.Get("https://example.com")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```
Import `github.com/MinatoAka/myhttp` and `time` in your Go file.


## Options

| Option | Description | Default |
|---|---|---|
| `WithRetry(maxRetries int, baseWait time.Duration)` | Retry network errors and HTTP 5xx. `0` disables retries | `3`, `1s` |
| `WithRateLimit(reqPerSec int)` | Max requests per second per client. `0` disables | `0` (disabled) |
| `WithTimeout(d time.Duration)` | Timeout for the whole call, including retries | `15s` |
| `WithUserAgent(ua string)` | Default `User-Agent` header | `myhttp/1.0` |

`myhttp.New()` with no options is valid — all defaults apply.


## API

| Method | Signature |
|---|---|
| `New` | `New(opts ...Option) *Client` |
| `Get` | `Get(url string) (*http.Response, error)` |
| `Do` | `Do(req *http.Request) (*http.Response, error)` |

`Get` is a shortcut. For POST, PUT, DELETE, or any request with custom headers, build an `*http.Request` and send it with `Do`.


## Behavior notes

**Retries**
- Triggered by network errors, HTTP `5xx`, and HTTP `429` (rate limited).
- Other `4xx` (400/401/403/404) are returned immediately — check `resp.StatusCode` yourself.
- If the server sends `Retry-After`, that value is used instead of backoff.
  Both formats are supported: delta-seconds (`5`) and HTTP-date.
- Otherwise backoff doubles from `baseWait` (`1s → 2s → 4s`) plus 0–500ms jitter, capped at 60s.
- Sleep between attempts is context-aware: cancelling the request stops retries immediately.
- When retries are exhausted, `Get`/`Do` return a **nil response** and an error wrapping the last failure.
- Request bodies are buffered once and replayed on each attempt, so POST/PUT retries are safe.
- `WithRetry(0, ...)` disables the retry layer entirely.

**Rate limiting**
- Applied once per `Get`/`Do` call, before the retry layer. Retries inside a call are not gated again.
- Blocks (sleeps) until the next slot is free; it never returns an error.
- Shared across all goroutines using the same client.

**Timeout**
- `WithTimeout` covers the **entire call**, including all retries and backoff sleeps — not each individual attempt.
- Example: `WithRetry(3, time.Second)` can sleep ~7s in backoff alone, so keep the timeout comfortably above that.

**Response body**
- The caller closes `resp.Body` on success.
- Bodies from failed attempts are closed automatically before retrying.


## Test

```bash
go test ./...
```
