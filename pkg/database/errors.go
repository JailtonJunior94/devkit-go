package database

import "errors"

var (
	ErrManagerClosed     = errors.New("database: manager closed")
	ErrShutdownTimeout   = errors.New("database: shutdown timeout exceeded")
	ErrNestedTransaction = errors.New("database: nested transaction not supported")
	ErrInvalidConfig     = errors.New("database: invalid configuration")
	ErrMigrationFailed   = errors.New("database: migration failed")
)
