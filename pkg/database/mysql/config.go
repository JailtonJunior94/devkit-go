package mysql

import (
	"errors"
	"fmt"
	"time"
)

const (
	// DefaultMaxOpenConns is the default maximum number of open connections in the pool.
	DefaultMaxOpenConns = 20
	// DefaultMaxIdleConns is the default number of idle connections kept ready.
	DefaultMaxIdleConns = 5
	// DefaultConnMaxLife is the default maximum lifetime of a single connection.
	DefaultConnMaxLife = 10 * time.Minute
	// DefaultConnMaxIdle is the default maximum idle time before a connection is closed.
	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort = 3306
)

// MySQLConfig holds connection configuration for the MySQL adapter.
// When DSN is non-empty it takes precedence over individual fields.
type MySQLConfig struct {
	// DSN is the full connection string. Takes precedence over all other fields when set.
	DSN string

	// Individual fields — used only when DSN is empty.
	Host     string
	Port     int
	User     string
	Password string
	Database string

	// Pool configuration — zero values use package defaults (applied by applyDefaults).
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	ConnMaxIdle  time.Duration
}

// driverConfig mirrors the manager.DriverConfig marker interface.
type driverConfig interface {
	driverConfigMarker()
	Validate() error
}

// Compile-time assertion that MySQLConfig satisfies driverConfig.
var _ driverConfig = MySQLConfig{}

// driverConfigMarker satisfies manager.DriverConfig.
func (MySQLConfig) driverConfigMarker() {}

// Validate returns an aggregated error listing all missing required fields.
// When DSN is set the individual connection fields are not validated.
func (c MySQLConfig) Validate() error {
	if c.DSN != "" {
		return nil
	}

	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("mysql: host is required"))
	}
	if c.User == "" {
		errs = append(errs, errors.New("mysql: user is required"))
	}
	if c.Database == "" {
		errs = append(errs, errors.New("mysql: database is required"))
	}
	return errors.Join(errs...)
}

// ResolveDSN returns the connection string to use.
// DSN field takes precedence; otherwise one is built from the individual fields.
// The format follows go-sql-driver/mysql DSN conventions.
func (c MySQLConfig) ResolveDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	// Format: user:password@tcp(host:port)/database?parseTime=true
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.User, c.Password, c.Host, port, c.Database,
	)
}

// SanitizedDSN returns a DSN safe for use in logs — password is replaced with "*".
func (c MySQLConfig) SanitizedDSN() string {
	if c.DSN != "" {
		return "<dsn-redacted>"
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	return fmt.Sprintf(
		"%s:*@tcp(%s:%d)/%s?parseTime=true",
		c.User, c.Host, port, c.Database,
	)
}
