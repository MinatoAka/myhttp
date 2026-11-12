# myhttp

[![Go Reference](https://pkg.go.dev/badge/github.com/MinatoAka/myhttp.svg)](https://pkg.go.dev/github.com/MinatoAka/myhttp)
[![Go Report Card](https://goreportcard.com/badge/github.com/MinatoAka/myhttp)](https://goreportcard.com/report/github.com/MinatoAka/myhttp)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Support via Saweria](https://img.shields.io/badge/Support-Saweria-orange)](https://saweria.co/AKaAturDompet)

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

Import `github.com/MinatoAka/myhttp` and `time` in your Go file. Use `client.Do(req)` to send a custom `http.Request`.

## Options

- `WithRetry(maxRetries, baseWait)` — retries network errors and HTTP 5xx responses with exponential backoff and jitter.
- `WithRateLimit(reqPerSec)` — limits requests per second; `0` disables the limit.
- `WithTimeout(duration)` — sets the request timeout.
- `WithUserAgent(ua)` — sets the default User-Agent.

Defaults: 3 retries, 1-second base wait, 15-second timeout, and `myhttp/1.0` User-Agent. Rate limiting is disabled by default.

## Test

```bash
go test ./...
```
