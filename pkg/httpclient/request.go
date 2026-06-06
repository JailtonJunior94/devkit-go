package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	DefaultMaxResponseSize int64 = 10 * 1024 * 1024
)

var (
	ErrResponseTooLarge = errors.New("response body exceeds maximum allowed size")
)

func MakeRequest[TSuccess any, TError any](ctx context.Context, client HTTPClient, method, url string, headers map[string]string, payload io.Reader) (*TSuccess, *TError, error) {
	return MakeRequestWithLimit[TSuccess, TError](ctx, client, method, url, headers, payload, DefaultMaxResponseSize)
}

func MakeRequestWithLimit[TSuccess any, TError any](ctx context.Context, client HTTPClient, method, url string, headers map[string]string, payload io.Reader, maxBodySize int64) (success *TSuccess, errResp *TError, err error) {
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

	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	bodyReader, err := readLimited(response.Body, maxBodySize)
	if err != nil {
		return nil, nil, err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		var errorResponse *TError
		if decodeErr := json.NewDecoder(bodyReader).Decode(&errorResponse); decodeErr != nil {
			return nil, nil, decodeErr
		}
		return nil, errorResponse, nil
	}

	var successResponse *TSuccess
	if decodeErr := json.NewDecoder(bodyReader).Decode(&successResponse); decodeErr != nil {
		return nil, nil, decodeErr
	}
	return successResponse, nil, nil
}

func readLimited(r io.Reader, maxBodySize int64) (io.Reader, error) {
	if maxBodySize <= 0 {
		return r, nil
	}

	data, err := io.ReadAll(io.LimitReader(r, maxBodySize+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maxBodySize {
		return nil, ErrResponseTooLarge
	}

	return bytes.NewReader(data), nil
}
