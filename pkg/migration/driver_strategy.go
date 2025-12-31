package migration

import (
	"fmt"
	"net/url"
	"time"
)

// DriverStrategy defines the interface for database driver strategies.
// Each database driver implements this interface to provide driver-specific behavior.
type DriverStrategy interface {
	// Name returns the driver name used by golang-migrate.
	Name() string

	// BuildDatabaseURL constructs the database URL with driver-specific parameters.
	BuildDatabaseURL(dsn string, params DatabaseParams) (string, error)

	// SupportsMultiStatement indicates if the driver supports multi-statement migrations.
	SupportsMultiStatement() bool

	// RecommendedLockTimeout returns the recommended lock timeout for this driver.
	RecommendedLockTimeout() time.Duration

	// Validate performs driver-specific validation on the configuration.
	Validate(config Config) error
}

// DatabaseParams holds parameters for database URL construction.
type DatabaseParams struct {
	LockTimeout           time.Duration
	StatementTimeout      time.Duration
	MultiStatementEnabled bool
	MultiStatementMaxSize int
	PreferSimpleProtocol  bool
}

// postgresStrategy implements DriverStrategy for PostgreSQL.
type postgresStrategy struct{}

// NewPostgresStrategy creates a new PostgreSQL driver strategy.
func NewPostgresStrategy() DriverStrategy {
	return &postgresStrategy{}
}

func (p *postgresStrategy) Name() string {
	return "postgres"
}

func (p *postgresStrategy) BuildDatabaseURL(dsn string, params DatabaseParams) (string, error) {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid PostgreSQL DSN format: %w", err)
	}

	query := parsedURL.Query()

	if params.LockTimeout > 0 {
		lockTimeoutSeconds := int(params.LockTimeout.Seconds())
		query.Set("x-migrations-table-lock-timeout", fmt.Sprintf("%ds", lockTimeoutSeconds))
	}

	if params.StatementTimeout > 0 {
		stmtTimeoutMs := int(params.StatementTimeout.Milliseconds())
		query.Set("x-statement-timeout", fmt.Sprintf("%dms", stmtTimeoutMs))
	}

	if params.MultiStatementEnabled {
		query.Set("x-multi-statement", "true")
		if params.MultiStatementMaxSize > 0 {
			query.Set("x-multi-statement-max-size", fmt.Sprintf("%d", params.MultiStatementMaxSize))
		}
	}

	if params.PreferSimpleProtocol {
		query.Set("prefer_simple_protocol", "true")
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (p *postgresStrategy) SupportsMultiStatement() bool {
	return true
}

func (p *postgresStrategy) RecommendedLockTimeout() time.Duration {
	return 30 * time.Second
}

func (p *postgresStrategy) Validate(config Config) error {
	return nil
}

// cockroachStrategy implements DriverStrategy for CockroachDB.
type cockroachStrategy struct{}

// NewCockroachStrategy creates a new CockroachDB driver strategy.
func NewCockroachStrategy() DriverStrategy {
	return &cockroachStrategy{}
}

func (c *cockroachStrategy) Name() string {
	return "postgres"
}

func (c *cockroachStrategy) BuildDatabaseURL(dsn string, params DatabaseParams) (string, error) {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid CockroachDB DSN format: %w", err)
	}

	query := parsedURL.Query()

	lockTimeout := params.LockTimeout
	if lockTimeout == 0 {
		lockTimeout = c.RecommendedLockTimeout()
	}
	lockTimeoutSeconds := int(lockTimeout.Seconds())
	query.Set("x-migrations-table-lock-timeout", fmt.Sprintf("%ds", lockTimeoutSeconds))

	if params.StatementTimeout > 0 {
		stmtTimeoutMs := int(params.StatementTimeout.Milliseconds())
		query.Set("x-statement-timeout", fmt.Sprintf("%dms", stmtTimeoutMs))
	}

	if params.MultiStatementEnabled {
		query.Set("x-multi-statement", "true")
		if params.MultiStatementMaxSize > 0 {
			query.Set("x-multi-statement-max-size", fmt.Sprintf("%d", params.MultiStatementMaxSize))
		}
	}

	if params.PreferSimpleProtocol {
		query.Set("prefer_simple_protocol", "true")
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (c *cockroachStrategy) SupportsMultiStatement() bool {
	return true
}

func (c *cockroachStrategy) RecommendedLockTimeout() time.Duration {
	return 60 * time.Second
}

func (c *cockroachStrategy) Validate(config Config) error {
	if config.LockTimeout > 0 && config.LockTimeout < 30*time.Second {
		return fmt.Errorf("CockroachDB requires lock timeout >= 30s due to distributed nature (got: %v)", config.LockTimeout)
	}
	return nil
}

// GetDriverStrategy returns the appropriate driver strategy based on the driver type.
func GetDriverStrategy(driver Driver) (DriverStrategy, error) {
	switch driver {
	case DriverPostgres:
		return NewPostgresStrategy(), nil
	case DriverCockroachDB:
		return NewCockroachStrategy(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidDriver, driver)
	}
}
