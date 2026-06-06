package mssql

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxOpenConns = 20

	DefaultMaxIdleConns = 5

	DefaultConnMaxLife = 10 * time.Minute

	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort = 1433
)

type MSSQLConfig struct {
	DSN string

	Host     string
	Port     int
	User     string
	Password string
	Database string

	DefaultSchema string

	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	ConnMaxIdle  time.Duration

	PingTimeout time.Duration
}

type driverConfig interface {
	driverConfigMarker()
	Validate() error
}

var _ driverConfig = MSSQLConfig{}

func (MSSQLConfig) driverConfigMarker() {}

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
