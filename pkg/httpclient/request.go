package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	// DefaultMaxResponseSize is the default maximum response body size (10MB).
	DefaultMaxResponseSize int64 = 10 * 1024 * 1024
)

var (
	// ErrResponseTooLarge is returned when the response body exceeds the maximum size.
	ErrResponseTooLarge = errors.New("response body exceeds maximum allowed size")
)

// MakeRequest performs an HTTP request and decodes the response.
// The response body is limited to DefaultMaxResponseSize (10MB) to prevent memory exhaustion.
func MakeRequest[TSuccess any, TError any](ctx context.Context, client HTTPClient, method, url string, headers map[string]string, payload io.Reader) (*TSuccess, *TError, error) {
	return MakeRequestWithLimit[TSuccess, TError](ctx, client, method, url, headers, payload, DefaultMaxResponseSize)
}

// MakeRequestWithLimit performs an HTTP request with a custom response body size limit.
// Set maxBodySize to 0 or negative for no limit (not recommended).
func MakeRequestWithLimit[TSuccess any, TError any](ctx context.Context, client HTTPClient, method, url string, headers map[string]string, payload io.Reader, maxBodySize int64) (*TSuccess, *TError, error) {
	request, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return nil, nil, err
	}

	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if response != nil {
		defer func() {
			_ = response.Body.Close()
		}()
	}

	// Limit the response body size to prevent memory exhaustion attacks
	var bodyReader io.Reader = response.Body
	if maxBodySize > 0 {
		bodyReader = io.LimitReader(response.Body, maxBodySize+1)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		var errorResponse *TError
		if err := json.NewDecoder(bodyReader).Decode(&errorResponse); err != nil {
			return nil, nil, err
		}
		return nil, errorResponse, nil
	}

	var successResponse *TSuccess
	if err := json.NewDecoder(bodyReader).Decode(&successResponse); err != nil {
		return nil, nil, err
	}
	return successResponse, nil, nil
}
