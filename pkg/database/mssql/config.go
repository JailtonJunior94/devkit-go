package mssql

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
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

	defaultPort = 1433
)

// MSSQLConfig holds connection configuration for the MSSQL adapter.
// When DSN is non-empty it takes precedence over individual fields.
type MSSQLConfig struct {
	// DSN is the full connection string. Takes precedence over all other fields when set.
	DSN string

	// Individual fields — used only when DSN is empty.
	Host     string
	Port     int
	User     string
	Password string
	Database string
	// DefaultSchema is retained for configuration parity with other adapters.
	// The MSSQL adapter never mutates database principals to enforce it; use a
	// login already configured out-of-band or issue schema-qualified SQL.
	DefaultSchema string

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

// Compile-time assertion that MSSQLConfig satisfies driverConfig.
var _ driverConfig = MSSQLConfig{}

// driverConfigMarker satisfies manager.DriverConfig.
func (MSSQLConfig) driverConfigMarker() {}

// Validate returns an aggregated error listing all missing required fields.
// When DSN is set the individual connection fields are not validated.
func (c MSSQLConfig) Validate() error {
	if c.DSN != "" {
		if c.DefaultSchema != "" && strings.TrimSpace(c.DefaultSchema) == "" {
			return errors.New("mssql: default schema cannot be blank")
		}
		return nil
	}

	var errs []error
	if c.DefaultSchema != "" && strings.TrimSpace(c.DefaultSchema) == "" {
		errs = append(errs, errors.New("mssql: default schema cannot be blank"))
	}
	if c.Host == "" {
		errs = append(errs, errors.New("mssql: host is required"))
	}
	if c.User == "" {
		errs = append(errs, errors.New("mssql: user is required"))
	}
	if c.Database == "" {
		errs = append(errs, errors.New("mssql: database is required"))
	}
	return errors.Join(errs...)
}

// ResolveDSN returns the connection string to use.
// DSN field takes precedence; otherwise one is built from the individual fields.
// The format follows go-mssqldb sqlserver:// URL conventions.
func (c MSSQLConfig) ResolveDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	u := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, port),
	}

	q := u.Query()
	q.Set("database", c.Database)
	u.RawQuery = q.Encode()

	return u.String()
}

// SanitizedDSN returns a DSN safe for use in logs — password is replaced with "*".
func (c MSSQLConfig) SanitizedDSN() string {
	if c.DSN != "" {
		return "<dsn-redacted>"
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	return fmt.Sprintf("sqlserver://%s:*@%s:%d?database=%s",
		c.User, c.Host, port, c.Database,
	)
}
