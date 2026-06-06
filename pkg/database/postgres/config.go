package postgres

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultMaxOpenConns = 25

	DefaultMaxIdleConns = 6

	DefaultConnMaxLife = 30 * time.Minute

	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort    = 5432
	defaultSSLMode = "disable"

	DefaultPort = defaultPort

	DefaultSSLMode = defaultSSLMode
)

type PostgresConfig struct {
	DSN string

	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	SSLMode    string
	SearchPath string

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

var _ driverConfig = PostgresConfig{}

func (PostgresConfig) driverConfigMarker() {}

func (c PostgresConfig) Validate() error {
	if c.DSN != "" {
		return nil
	}

	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("postgres: host is required"))
	}
	if c.User == "" {
		errs = append(errs, errors.New("postgres: user is required"))
	}
	if c.Database == "" {
		errs = append(errs, errors.New("postgres: database is required"))
	}
	return errors.Join(errs...)
}

func (c PostgresConfig) ResolveDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = defaultSSLMode
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		quoteLibpqValue(c.Host), port,
		quoteLibpqValue(c.User), quoteLibpqValue(c.Password),
		quoteLibpqValue(c.Database), quoteLibpqValue(sslMode),
	)

	if c.SearchPath != "" {
		dsn += " search_path=" + quoteLibpqValue(c.SearchPath)
	}

	return dsn
}

func quoteLibpqValue(v string) string {
	if v != "" && !strings.ContainsAny(v, " \t\n\r\v\f'\\") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 2)
	b.WriteByte('\'')
	for _, r := range v {
		if r == '\\' || r == '\'' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}
