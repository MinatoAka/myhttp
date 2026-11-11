# myhttp

[![Go Reference](https://pkg.go.dev/badge/github.com/MinatoAka/myhttp.svg)](https://pkg.go.dev/github.com/MinatoAka/myhttp)
[![Go Report Card](https://goreportcard.com/badge/github.com/MinatoAka/myhttp)](https://goreportcard.com/report/github.com/MinatoAka/myhttp)
[![GitHub stars](https://img.shields.io/github/stars/MinatoAka/myhttp)](https://github.com/MinatoAka/myhttp/stargazers)
[![Support via Saweria](https://img.shields.io/badge/Support%20via-Saweria-orange)](https://saweria.co/AKaAturDompet)

A lightweight, production-ready HTTP client for Go with built-in Retry, Rate Limiting, and User-Agent injection.

## ✨ Features

- 🔄 **Exponential Backoff with Jitter**: Retries failed requests gracefully without overwhelming the server (Thundering Herd).
- 🛡️ **Safe Body Retry**: Automatically buffers request bodies so POST/PUT requests don't fail on retries.
- ⏱️ **Rate Limiter**: Prevents 429 Too Many Requests by strictly limiting requests per second.
- 🤖 **Custom User-Agent**: Automatically injects User-Agent into every request.
- 🪶 **Zero Dependencies**: Built entirely on the Go standard library.
- ✅ **Fully Tested**: Comprehensive unit tests using `httptest`.

## 📦 Installation

```bash
go get github.com/MinatoAka/myhttp
```

🚀 Quick Start

```go
package main

import (
	"fmt"
	"io"
	"time"

	"github.com/MinatoAka/myhttp"
)

func main() {
	// Initialize client with custom configurations
	client := myhttp.New(
		myhttp.WithRetry(3, 1*time.Second),     // Retry 3x with 1s base wait
		myhttp.WithRateLimit(5),                // Max 5 requests per second
		myhttp.WithTimeout(10*time.Second),     // 10s timeout per request
		myhttp.WithUserAgent("myapp/1.0"),      // Custom User-Agent
	)

	// Make a request
	resp, err := client.Get("https://indodax.com/api/ticker/ethidr")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

📖 API Reference

Options

Option Description
WithRetry(retries, wait) Set max retries and base wait time (uses exponential backoff + jitter).
WithRateLimit(reqPerSec) Limit requests to N per second.
WithTimeout(duration) Set global timeout for requests.
WithUserAgent(ua) Set custom User-Agent header.

Methods

Method Description
New(opts ...Option) *Client Create a new client with optional configurations.
client.Get(url) Shortcut for GET requests.
client.Do(req) Execute raw http.Request.

🧪 Testing

```bash
go test -v ./...
```
