package postgres

import "errors"

var (
	ErrAlreadyConnected    = errors.New("database is already connected")
	ErrNotConnected        = errors.New("database is not connected")
	ErrConnectionFailed    = errors.New("failed to open database connection")
	ErrPingFailed          = errors.New("failed to ping database")
	ErrHealthCheckFailed   = errors.New("health check failed")
	ErrCloseFailed         = errors.New("failed to close database connection")
	ErrInvalidHost         = errors.New("invalid host")
	ErrInvalidPort         = errors.New("invalid port")
	ErrInvalidUser         = errors.New("invalid user")
	ErrInvalidDatabase     = errors.New("invalid database name")
	ErrInvalidMaxOpenConns = errors.New("max open connections must be greater than 0")
	ErrInvalidMaxIdleConns = errors.New("max idle connections must be greater than 0")
)
