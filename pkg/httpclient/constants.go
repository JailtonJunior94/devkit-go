package httpclient

import (
	"errors"
	"time"
)

const (
	DefaultTimeout            = 30 * time.Second
	DefaultMaxRequestBodySize = 10 * 1024 * 1024
	DefaultMaxDrainSize       = 1 * 1024 * 1024
)

var ErrRequestBodyTooLarge = errors.New("request body exceeds maximum allowed size for retry buffering")
