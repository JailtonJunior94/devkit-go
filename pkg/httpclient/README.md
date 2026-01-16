# httpclient

HTTP client with retry logic, circuit breaking, and OpenTelemetry instrumentation.

## Quick Start

```go
client := httpclient.NewHTTPClient()

req, _ := httpclient.NewRequest(http.MethodGet, "https://api.example.com/users")
resp, err := client.Do(req)
```

## Features

- **Automatic Retries**: Exponential backoff
- **Instrumentation**: OpenTelemetry traces
- **Timeout Management**: Configurable timeouts
- **Request Builder**: Fluent API

## API

```go
// Client
NewHTTPClient() HTTPClient
NewHTTPClientWithTimeout(timeout time.Duration) HTTPClient

// Request Builder
NewRequest(method, url string) (*Request, error)
request.WithHeaders(headers map[string]string) *Request
request.WithBody(body io.Reader) *Request
request.WithContext(ctx context.Context) *Request
```

## Example: POST with Retry

```go
client := httpclient.NewHTTPClientWithTimeout(10 * time.Second)

body := bytes.NewReader([]byte(`{"name":"John"}`))
req, _ := httpclient.NewRequest(http.MethodPost, "https://api.example.com/users")
req.WithBody(body).WithHeaders(map[string]string{
    "Content-Type": "application/json",
})

resp, err := client.Do(req.Build())
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

## Related

- `pkg/observability` - Automatic tracing
