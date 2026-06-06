package mysql

import (
	"errors"
	"fmt"
	"time"
)

const (
	DefaultMaxOpenConns = 20

	DefaultMaxIdleConns = 5

	DefaultConnMaxLife = 10 * time.Minute

	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort = 3306
)

type MySQLConfig struct {
	DSN string

	Host     string
	Port     int
	User     string
	Password string
	Database string

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

var _ driverConfig = MySQLConfig{}

func (MySQLConfig) driverConfigMarker() {}

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

func (c MySQLConfig) ResolveDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.User, c.Password, c.Host, port, c.Database,
	)
}

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
