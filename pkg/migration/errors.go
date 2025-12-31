package migration

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidDriver is returned when an unsupported database driver is specified.
	ErrInvalidDriver = errors.New("invalid or unsupported database driver")

	// ErrMissingDSN is returned when the database DSN is not provided.
	ErrMissingDSN = errors.New("database DSN is required")

	// ErrMissingSource is returned when the migration source is not provided.
	ErrMissingSource = errors.New("migration source is required")

	// ErrInvalidTimeout is returned when timeout is zero or negative.
	ErrInvalidTimeout = errors.New("timeout must be positive")

	// ErrInvalidLockTimeout is returned when lock timeout is negative.
	ErrInvalidLockTimeout = errors.New("lock timeout must be non-negative")

	// ErrMigratorNotInitialized is returned when operations are called on an uninitialized migrator.
	ErrMigratorNotInitialized = errors.New("migrator not properly initialized")

	// ErrNoChanges indicates that the database is already at the target version.
	// This is not treated as a fatal error - it's an informational state.
	ErrNoChanges = errors.New("no migrations to apply - database is up to date")

	// ErrDirtyDatabase indicates the database is in an inconsistent state.
	// This typically happens when a migration was partially applied and failed.
	ErrDirtyDatabase = errors.New("database is in a dirty state - manual intervention required")

	// ErrMigrationLocked indicates that another process is currently running migrations.
	ErrMigrationLocked = errors.New("migration lock is held by another process")

	// ErrAlreadyClosed is returned when operations are called on a closed migrator.
	ErrAlreadyClosed = errors.New("migrator has already been closed")
)

// MigrationError wraps migration errors with additional context.
type MigrationError struct {
	Operation string
	Driver    Driver
	Version   uint
	Err       error
}

// Error implements the error interface.
func (e *MigrationError) Error() string {
	if e.Version > 0 {
		return fmt.Sprintf("migration error during %s (driver=%s, version=%d): %v",
			e.Operation, e.Driver, e.Version, e.Err)
	}
	return fmt.Sprintf("migration error during %s (driver=%s): %v",
		e.Operation, e.Driver, e.Err)
}

// Unwrap returns the underlying error.
func (e *MigrationError) Unwrap() error {
	return e.Err
}

// NewMigrationError creates a new migration error with context.
func NewMigrationError(operation string, driver Driver, version uint, err error) error {
	if err == nil {
		return nil
	}
	return &MigrationError{
		Operation: operation,
		Driver:    driver,
		Version:   version,
		Err:       err,
	}
}

// IsNoChangeError checks if the error indicates no migrations were needed.
func IsNoChangeError(err error) bool {
	return errors.Is(err, ErrNoChanges)
}

// IsDirtyError checks if the error indicates a dirty database state.
func IsDirtyError(err error) bool {
	return errors.Is(err, ErrDirtyDatabase)
}

// IsLockError checks if the error indicates a migration lock conflict.
func IsLockError(err error) bool {
	return errors.Is(err, ErrMigrationLocked)
}
