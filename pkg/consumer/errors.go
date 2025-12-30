package consumer

import "fmt"

// ConsumerError represents an error that occurred in the consumer.
// It wraps the underlying error with additional context.
type ConsumerError struct {
	Op      string // Operation that failed (e.g., "start", "shutdown", "process")
	Topic   string // Topic being processed (if applicable)
	Message string // Human-readable error message
	Err     error  // Underlying error
}

// Error implements the error interface.
func (e *ConsumerError) Error() string {
	if e.Topic != "" {
		return fmt.Sprintf("consumer error [%s] on topic %s: %s: %v",
			e.Op, e.Topic, e.Message, e.Err)
	}
	return fmt.Sprintf("consumer error [%s]: %s: %v", e.Op, e.Message, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *ConsumerError) Unwrap() error {
	return e.Err
}

// HandlerError represents an error that occurred in a message handler.
type HandlerError struct {
	Handler string // Handler name or type
	Topic   string // Topic being processed
	Message string // Human-readable error message
	Err     error  // Underlying error
	Retry   bool   // Whether the message should be retried
}

// Error implements the error interface.
func (e *HandlerError) Error() string {
	retryStatus := "no retry"
	if e.Retry {
		retryStatus = "will retry"
	}
	return fmt.Sprintf("handler error [%s] on topic %s (%s): %s: %v",
		e.Handler, e.Topic, retryStatus, e.Message, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *HandlerError) Unwrap() error {
	return e.Err
}

// ProcessingError represents an error during message processing.
type ProcessingError struct {
	Topic      string // Topic being processed
	Partition  int32  // Partition (if applicable)
	Offset     int64  // Offset (if applicable)
	Attempt    int    // Current retry attempt
	MaxRetries int    // Maximum retry attempts
	Err        error  // Underlying error
}

// Error implements the error interface.
func (e *ProcessingError) Error() string {
	return fmt.Sprintf("processing error on topic %s (partition %d, offset %d) attempt %d/%d: %v",
		e.Topic, e.Partition, e.Offset, e.Attempt, e.MaxRetries, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *ProcessingError) Unwrap() error {
	return e.Err
}

// ShutdownError represents an error during graceful shutdown.
type ShutdownError struct {
	Message string // Human-readable error message
	Err     error  // Underlying error
}

// Error implements the error interface.
func (e *ShutdownError) Error() string {
	return fmt.Sprintf("shutdown error: %s: %v", e.Message, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *ShutdownError) Unwrap() error {
	return e.Err
}
