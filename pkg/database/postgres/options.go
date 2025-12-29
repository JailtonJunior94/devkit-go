package postgres

import "time"

// Option is a functional option for configuring the PostgreSQL database.
type Option func(*config)

// WithDSN sets the database connection string directly.
// When DSN is provided, it takes precedence over individual connection parameters.
// Example: "host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable".
func WithDSN(dsn string) Option {
	return func(c *config) {
		if dsn != "" {
			c.dsn = dsn
		}
	}
}

// WithHost sets the database host.
func WithHost(host string) Option {
	return func(c *config) {
		if host != "" {
			c.host = host
		}
	}
}

// WithPort sets the database port.
func WithPort(port int) Option {
	return func(c *config) {
		if port > 0 && port <= 65535 {
			c.port = port
		}
	}
}

// WithUser sets the database user.
func WithUser(user string) Option {
	return func(c *config) {
		if user != "" {
			c.user = user
		}
	}
}

// WithPassword sets the database password.
func WithPassword(password string) Option {
	return func(c *config) {
		c.password = password
	}
}

// WithDatabase sets the database name.
func WithDatabase(database string) Option {
	return func(c *config) {
		if database != "" {
			c.database = database
		}
	}
}

// WithSSLMode sets the SSL mode for the connection.
func WithSSLMode(sslMode string) Option {
	return func(c *config) {
		if sslMode != "" {
			c.sslMode = sslMode
		}
	}
}

// WithMaxOpenConns sets the maximum number of open connections to the database.
func WithMaxOpenConns(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxOpenConns = n
		}
	}
}

// WithMaxIdleConns sets the maximum number of idle connections in the pool.
func WithMaxIdleConns(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxIdleConns = n
		}
	}
}

// WithConnMaxLifetime sets the maximum lifetime of a connection.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.connMaxLifetime = d
		}
	}
}

// WithConnMaxIdleTime sets the maximum idle time of a connection.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.connMaxIdleTime = d
		}
	}
}

// WithConnectTimeout sets the timeout for establishing a connection.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.connectTimeout = d
		}
	}
}

// WithPingTimeout sets the timeout for ping operations.
func WithPingTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.pingTimeout = d
		}
	}
}

// WithMaxRetries sets the maximum number of connection retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *config) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithRetryInterval sets the interval between connection retry attempts.
func WithRetryInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.retryInterval = d
		}
	}
}
