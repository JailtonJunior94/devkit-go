package postgres

import "time"

const (
	defaultHost            = "localhost"
	defaultPort            = 5432
	defaultUser            = "postgres"
	defaultPassword        = ""
	defaultDatabase        = "postgres"
	defaultSSLMode         = "disable"
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5 * time.Minute
	defaultConnMaxIdleTime = 10 * time.Minute
	defaultConnectTimeout  = 10 * time.Second
	defaultPingTimeout     = 5 * time.Second
	defaultMaxRetries      = 3
	defaultRetryInterval   = 2 * time.Second
)

// config holds the internal configuration for the PostgreSQL database.
type config struct {
	dsn             string
	host            string
	port            int
	user            string
	password        string
	database        string
	sslMode         string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
	connectTimeout  time.Duration
	pingTimeout     time.Duration
	maxRetries      int
	retryInterval   time.Duration
}

// defaultConfig returns a config instance with default values.
func defaultConfig() *config {
	return &config{
		host:            defaultHost,
		port:            defaultPort,
		user:            defaultUser,
		password:        defaultPassword,
		database:        defaultDatabase,
		sslMode:         defaultSSLMode,
		maxOpenConns:    defaultMaxOpenConns,
		maxIdleConns:    defaultMaxIdleConns,
		connMaxLifetime: defaultConnMaxLifetime,
		connMaxIdleTime: defaultConnMaxIdleTime,
		connectTimeout:  defaultConnectTimeout,
		pingTimeout:     defaultPingTimeout,
		maxRetries:      defaultMaxRetries,
		retryInterval:   defaultRetryInterval,
	}
}
