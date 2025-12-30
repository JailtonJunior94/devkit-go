package vos

import "errors"

var (
	// ErrInvalidCurrency is returned when an invalid currency code is provided.
	ErrInvalidCurrency = errors.New("invalid currency code")

	// ErrCurrencyMismatch is returned when attempting operations between different currencies.
	ErrCurrencyMismatch = errors.New("currency mismatch: cannot operate on different currencies")

	// ErrDivisionByZero is returned when attempting to divide by zero.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrInvalidValue is returned when an invalid value is provided.
	ErrInvalidValue = errors.New("invalid value")

	// ErrOverflow is returned when an operation would cause numeric overflow.
	ErrOverflow = errors.New("numeric overflow")

	// ErrNegativeValue is returned when a negative value is provided where only positive values are allowed.
	ErrNegativeValue = errors.New("negative value not allowed")

	// ErrInvalidScale is returned when an invalid scale is provided.
	ErrInvalidScale = errors.New("invalid scale")

	// ErrInvalidFormat is returned when parsing fails due to invalid format.
	ErrInvalidFormat = errors.New("invalid format")

	// ErrNullValue is returned when scanning a NULL database value into a non-nullable type.
	ErrNullValue = errors.New("cannot scan NULL into non-nullable type")
)
