package manager

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func resolveConfig(cfg DriverConfig) (DriverConfig, error) {
	if cfg != nil {
		return mergeConfigWithEnvDefaults(cfg)
	}

	return resolveEnvConfig()
}

func resolveEnvConfig() (DriverConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	if driver == "" {
		return nil, fmt.Errorf("%w: config is required", database.ErrInvalidConfig)
	}

	port, err := envInt("DB_PORT")
	if err != nil {
		return nil, err
	}

	switch driver {
	case string(database.DriverPostgres):
		return postgres.PostgresConfig{
			DSN:        os.Getenv("DB_DSN"),
			Host:       os.Getenv("DB_HOST"),
			Port:       port,
			User:       os.Getenv("DB_USER"),
			Password:   os.Getenv("DB_PASSWORD"),
			Database:   envFirst("DB_DATABASE", "DB_NAME"),
			SSLMode:    os.Getenv("DB_SSLMODE"),
			SearchPath: os.Getenv("DB_SEARCH_PATH"),
		}, nil
	case string(database.DriverCockroach):
		return cockroach.CockroachConfig{
			DSN:        os.Getenv("DB_DSN"),
			Host:       os.Getenv("DB_HOST"),
			Port:       port,
			User:       os.Getenv("DB_USER"),
			Password:   os.Getenv("DB_PASSWORD"),
			Database:   envFirst("DB_DATABASE", "DB_NAME"),
			SSLMode:    os.Getenv("DB_SSLMODE"),
			SearchPath: os.Getenv("DB_SEARCH_PATH"),
		}, nil
	case string(database.DriverMySQL):
		return mysql.MySQLConfig{
			DSN:      os.Getenv("DB_DSN"),
			Host:     os.Getenv("DB_HOST"),
			Port:     port,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Database: envFirst("DB_DATABASE", "DB_NAME"),
		}, nil
	case string(database.DriverMSSQL):
		return mssql.MSSQLConfig{
			DSN:           os.Getenv("DB_DSN"),
			Host:          os.Getenv("DB_HOST"),
			Port:          port,
			User:          os.Getenv("DB_USER"),
			Password:      os.Getenv("DB_PASSWORD"),
			Database:      envFirst("DB_DATABASE", "DB_NAME"),
			DefaultSchema: os.Getenv("DB_DEFAULT_SCHEMA"),
		}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported DB_DRIVER %q", database.ErrInvalidConfig, driver)
	}
}

func mergeConfigWithEnvDefaults(cfg DriverConfig) (DriverConfig, error) {
	switch c := cfg.(type) {
	case postgres.PostgresConfig:
		return mergePostgresConfigWithEnv(c)
	case cockroach.CockroachConfig:
		return mergeCockroachConfigWithEnv(c)
	case mysql.MySQLConfig:
		return mergeMySQLConfigWithEnv(c)
	case mssql.MSSQLConfig:
		return mergeMSSQLConfigWithEnv(c)
	default:
		return nil, fmt.Errorf("%w: unsupported driver config type %T", database.ErrInvalidConfig, cfg)
	}
}

func mergePostgresConfigWithEnv(cfg postgres.PostgresConfig) (postgres.PostgresConfig, error) {
	if cfg.DSN != "" {
		return cfg, nil
	}

	if !hasPostgresStructuredFields(cfg) {
		cfg.DSN = os.Getenv("DB_DSN")
	}
	cfg.Host = firstNonEmpty(cfg.Host, os.Getenv("DB_HOST"))
	cfg.User = firstNonEmpty(cfg.User, os.Getenv("DB_USER"))
	cfg.Password = firstNonEmpty(cfg.Password, os.Getenv("DB_PASSWORD"))
	cfg.Database = firstNonEmpty(cfg.Database, envFirst("DB_DATABASE", "DB_NAME"))
	cfg.SSLMode = firstNonEmpty(cfg.SSLMode, os.Getenv("DB_SSLMODE"))
	cfg.SearchPath = firstNonEmpty(cfg.SearchPath, os.Getenv("DB_SEARCH_PATH"))

	port, err := mergeEnvPort(cfg.Port)
	if err != nil {
		return postgres.PostgresConfig{}, err
	}
	cfg.Port = port

	return cfg, nil
}

func mergeCockroachConfigWithEnv(cfg cockroach.CockroachConfig) (cockroach.CockroachConfig, error) {
	if cfg.DSN != "" {
		return cfg, nil
	}

	if !hasCockroachStructuredFields(cfg) {
		cfg.DSN = os.Getenv("DB_DSN")
	}
	cfg.Host = firstNonEmpty(cfg.Host, os.Getenv("DB_HOST"))
	cfg.User = firstNonEmpty(cfg.User, os.Getenv("DB_USER"))
	cfg.Password = firstNonEmpty(cfg.Password, os.Getenv("DB_PASSWORD"))
	cfg.Database = firstNonEmpty(cfg.Database, envFirst("DB_DATABASE", "DB_NAME"))
	cfg.SSLMode = firstNonEmpty(cfg.SSLMode, os.Getenv("DB_SSLMODE"))
	cfg.SearchPath = firstNonEmpty(cfg.SearchPath, os.Getenv("DB_SEARCH_PATH"))

	port, err := mergeEnvPort(cfg.Port)
	if err != nil {
		return cockroach.CockroachConfig{}, err
	}
	cfg.Port = port

	return cfg, nil
}

func mergeMySQLConfigWithEnv(cfg mysql.MySQLConfig) (mysql.MySQLConfig, error) {
	if cfg.DSN != "" {
		return cfg, nil
	}

	if !hasMySQLStructuredFields(cfg) {
		cfg.DSN = os.Getenv("DB_DSN")
	}
	cfg.Host = firstNonEmpty(cfg.Host, os.Getenv("DB_HOST"))
	cfg.User = firstNonEmpty(cfg.User, os.Getenv("DB_USER"))
	cfg.Password = firstNonEmpty(cfg.Password, os.Getenv("DB_PASSWORD"))
	cfg.Database = firstNonEmpty(cfg.Database, envFirst("DB_DATABASE", "DB_NAME"))

	port, err := mergeEnvPort(cfg.Port)
	if err != nil {
		return mysql.MySQLConfig{}, err
	}
	cfg.Port = port

	return cfg, nil
}

func mergeMSSQLConfigWithEnv(cfg mssql.MSSQLConfig) (mssql.MSSQLConfig, error) {
	if cfg.DSN != "" {
		return cfg, nil
	}

	if !hasMSSQLStructuredFields(cfg) {
		cfg.DSN = os.Getenv("DB_DSN")
	}
	cfg.Host = firstNonEmpty(cfg.Host, os.Getenv("DB_HOST"))
	cfg.User = firstNonEmpty(cfg.User, os.Getenv("DB_USER"))
	cfg.Password = firstNonEmpty(cfg.Password, os.Getenv("DB_PASSWORD"))
	cfg.Database = firstNonEmpty(cfg.Database, envFirst("DB_DATABASE", "DB_NAME"))
	cfg.DefaultSchema = firstNonEmpty(cfg.DefaultSchema, os.Getenv("DB_DEFAULT_SCHEMA"))

	port, err := mergeEnvPort(cfg.Port)
	if err != nil {
		return mssql.MSSQLConfig{}, err
	}
	cfg.Port = port

	return cfg, nil
}

func mergeEnvPort(explicitPort int) (int, error) {
	if explicitPort != 0 {
		return explicitPort, nil
	}
	return envInt("DB_PORT")
}

func hasPostgresStructuredFields(cfg postgres.PostgresConfig) bool {
	return cfg.Host != "" ||
		cfg.Port != 0 ||
		cfg.User != "" ||
		cfg.Password != "" ||
		cfg.Database != "" ||
		cfg.SSLMode != "" ||
		cfg.SearchPath != ""
}

func hasCockroachStructuredFields(cfg cockroach.CockroachConfig) bool {
	return cfg.Host != "" ||
		cfg.Port != 0 ||
		cfg.User != "" ||
		cfg.Password != "" ||
		cfg.Database != "" ||
		cfg.SSLMode != "" ||
		cfg.SearchPath != ""
}

func hasMySQLStructuredFields(cfg mysql.MySQLConfig) bool {
	return cfg.Host != "" ||
		cfg.Port != 0 ||
		cfg.User != "" ||
		cfg.Password != "" ||
		cfg.Database != ""
}

func hasMSSQLStructuredFields(cfg mssql.MSSQLConfig) bool {
	return cfg.Host != "" ||
		cfg.Port != 0 ||
		cfg.User != "" ||
		cfg.Password != "" ||
		cfg.Database != "" ||
		cfg.DefaultSchema != ""
}

func firstNonEmpty(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func envInt(key string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid %s %q", database.ErrInvalidConfig, key, raw)
	}
	return value, nil
}
