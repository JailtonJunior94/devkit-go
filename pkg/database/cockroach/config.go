package cockroach

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultMaxOpenConns is the default maximum number of connections in the pool.
	DefaultMaxOpenConns = 50
	// DefaultMaxIdleConns is the default minimum connections kept ready (idle).
	DefaultMaxIdleConns = 10
	// DefaultConnMaxLife is the default maximum lifetime of a single connection.
	DefaultConnMaxLife = 15 * time.Minute
	// DefaultConnMaxIdle is the default maximum idle time before a connection is closed.
	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort    = 26257
	defaultSSLMode = "disable"

	// DefaultPort is the exported default port used by helper integrations.
	DefaultPort = defaultPort
	// DefaultSSLMode is the exported default sslmode used by helper integrations.
	DefaultSSLMode = defaultSSLMode
)

// CockroachConfig holds connection configuration for the CockroachDB adapter.
// When DSN is non-empty it takes precedence over individual fields.
type CockroachConfig struct {
	// DSN is the full connection string. Takes precedence over all other fields when set.
	DSN string

	// Individual fields — used only when DSN is empty.
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	SSLMode    string
	SearchPath string

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

// Compile-time assertion that CockroachConfig satisfies driverConfig.
var _ driverConfig = CockroachConfig{}

// driverConfigMarker satisfies manager.DriverConfig.
func (CockroachConfig) driverConfigMarker() {}

// Validate returns an aggregated error listing all missing required fields.
// When DSN is set the individual connection fields are not validated.
func (c CockroachConfig) Validate() error {
	if c.DSN != "" {
		return nil
	}

	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("cockroach: host is required"))
	}
	if c.User == "" {
		errs = append(errs, errors.New("cockroach: user is required"))
	}
	if c.Database == "" {
		errs = append(errs, errors.New("cockroach: database is required"))
	}
	return errors.Join(errs...)
}

// ResolveDSN returns the connection string to use.
// DSN field takes precedence; otherwise one is built from the individual fields.
func (c CockroachConfig) ResolveDSN() string {
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

// quoteLibpqValue escapa um valor para o formato libpq key=value.
// Valores vazios ou contendo whitespace, aspas simples ou backslash são
// envolvidos em aspas simples; aspas e backslashes internos são escapados com
// barra invertida, conforme a especificação do libpq.
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
