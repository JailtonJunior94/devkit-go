package httpclientfx

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"go.uber.org/fx"
)

// Module provides a default HTTP client.
// Usage:
//
//	fx.New(
//	    httpclientfx.Module,
//	)
var Module = fx.Module("httpclient",
	fx.Provide(ProvideHTTPClient),
)

// ModuleWithRetry provides a retryable HTTP client.
// Usage:
//
//	fx.New(
//	    httpclientfx.ModuleWithRetry,
//	    fx.Supply(httpclientfx.Config{
//	        MaxRetries: 3,
//	        BackoffTime: time.Second,
//	    }),
//	)
var ModuleWithRetry = fx.Module("httpclient-retry",
	fx.Provide(ProvideRetryableHTTPClient),
)

// HTTPClientParams contains dependencies for creating an HTTP client.
type HTTPClientParams struct {
	fx.In

	Config Config `optional:"true"`
}

// HTTPClientResult contains the HTTP client output.
type HTTPClientResult struct {
	fx.Out

	HTTPClient httpclient.HTTPClient
}

// ProvideHTTPClient creates a simple HTTP client.
func ProvideHTTPClient(p HTTPClientParams) HTTPClientResult {
	if p.Config.Timeout > 0 {
		return HTTPClientResult{
			HTTPClient: httpclient.NewHTTPClientWithTimeout(p.Config.Timeout),
		}
	}
	return HTTPClientResult{
		HTTPClient: httpclient.NewHTTPClient(),
	}
}

// ProvideRetryableHTTPClient creates a retryable HTTP client with configuration.
func ProvideRetryableHTTPClient(p HTTPClientParams) HTTPClientResult {
	opts := []httpclient.Option{}

	if p.Config.MaxRetries > 0 {
		opts = append(opts, httpclient.WithMaxRetries(p.Config.MaxRetries))
	}
	if p.Config.BackoffTime > 0 {
		opts = append(opts, httpclient.WithBackoff(p.Config.BackoffTime))
	}
	if p.Config.Timeout > 0 {
		opts = append(opts, httpclient.WithTimeout(p.Config.Timeout))
	}

	return HTTPClientResult{
		HTTPClient: httpclient.NewHTTPClientRetryable(opts...),
	}
}

// ProvideNamedHTTPClient creates a named HTTP client for multiple instance support.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    httpclientfx.ProvideNamedHTTPClient(httpclientfx.Config{
//	        Timeout:     10*time.Second,
//	        MaxRetries:  3,
//	        BackoffTime: time.Second,
//	    }),
//	    fx.ResultTags(`name:"payment-client"`),
//	))
func ProvideNamedHTTPClient(cfg Config) func() httpclient.HTTPClient {
	return func() httpclient.HTTPClient {
		opts := []httpclient.Option{}

		if cfg.MaxRetries > 0 {
			opts = append(opts, httpclient.WithMaxRetries(cfg.MaxRetries))
		}
		if cfg.BackoffTime > 0 {
			opts = append(opts, httpclient.WithBackoff(cfg.BackoffTime))
		}
		if cfg.Timeout > 0 {
			opts = append(opts, httpclient.WithTimeout(cfg.Timeout))
		}

		if len(opts) == 0 {
			return httpclient.NewHTTPClient()
		}
		return httpclient.NewHTTPClientRetryable(opts...)
	}
}

// ProvideSimpleHTTPClient creates a simple HTTP client with custom timeout.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    httpclientfx.ProvideSimpleHTTPClient(10*time.Second),
//	    fx.ResultTags(`name:"fast-client"`),
//	))
func ProvideSimpleHTTPClient(timeout time.Duration) func() httpclient.HTTPClient {
	return func() httpclient.HTTPClient {
		if timeout > 0 {
			return httpclient.NewHTTPClientWithTimeout(timeout)
		}
		return httpclient.NewHTTPClient()
	}
}
