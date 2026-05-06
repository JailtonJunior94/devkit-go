package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	migratelib "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/sqlserver"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

var startupMigrationDirFunc = defaultStartupMigrationDir

func runStartupMigrations(cfg DriverConfig, driver database.Driver, o options) error {
	dsn, err := resolveMigrationDSN(cfg)
	if err != nil {
		return err
	}
	dbURL := normalizeMigrationDSN(dsn)

	if o.startupMigrationFS != nil {
		return runStartupMigrationsFromFS(o.startupMigrationFS, o.startupMigrationRoot, dbURL)
	}

	dir := o.startupMigrationDir
	if dir == "" {
		dir = startupMigrationDirFunc(driver)
	}
	return runStartupMigrationsFromDir(dir, dbURL)
}

func runStartupMigrationsFromDir(dir, dbURL string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%w: stat startup migration dir %q: %w", database.ErrMigrationFailed, dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: startup migration path %q is not a directory", database.ErrMigrationFailed, dir)
	}

	m, err := migratelib.New("file://"+filepath.ToSlash(dir), dbURL)
	if err != nil {
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
	}
	return nil
}

func runStartupMigrationsFromFS(fsys fs.FS, root, dbURL string) error {
	if root == "" {
		root = "."
	}
	src, err := iofs.New(fsys, root)
	if err != nil {
		return fmt.Errorf("%w: iofs source: %w", database.ErrMigrationFailed, err)
	}
	m, err := migratelib.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
	}
	defer func() {
		_, _ = m.Close()
	}()
	if err := m.Up(); err != nil && !errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
	}
	return nil
}

func defaultStartupMigrationDir(driver database.Driver) string {
	return filepath.Join("migrations", string(driver))
}

func resolveMigrationDSN(cfg DriverConfig) (string, error) {
	switch c := cfg.(type) {
	case postgres.PostgresConfig:
		return postgresMigrationDSN(c), nil
	case cockroach.CockroachConfig:
		return cockroachMigrationDSN(c), nil
	case mysql.MySQLConfig:
		return c.ResolveDSN(), nil
	case mssql.MSSQLConfig:
		return c.ResolveDSN(), nil
	default:
		return "", fmt.Errorf("%w: unsupported driver config type %T", database.ErrInvalidConfig, cfg)
	}
}

type pgFlavorParams struct {
	DSN        string
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	SSLMode    string
	SearchPath string

	DefaultPort    int
	DefaultSSLMode string
}

func buildPgFlavorMigrationDSN(p pgFlavorParams) string {
	if p.DSN != "" {
		return p.DSN
	}

	port := p.Port
	if port == 0 {
		port = p.DefaultPort
	}

	sslMode := p.SSLMode
	if sslMode == "" {
		sslMode = p.DefaultSSLMode
	}

	u := &url.URL{
		Scheme: "pgx5",
		User:   url.UserPassword(p.User, p.Password),
		Host:   fmt.Sprintf("%s:%d", p.Host, port),
		Path:   p.Database,
	}

	query := u.Query()
	query.Set("sslmode", sslMode)
	if p.SearchPath != "" {
		query.Set("search_path", p.SearchPath)
	}
	u.RawQuery = query.Encode()

	return u.String()
}

func postgresMigrationDSN(cfg postgres.PostgresConfig) string {
	return buildPgFlavorMigrationDSN(pgFlavorParams{
		DSN:            cfg.DSN,
		Host:           cfg.Host,
		Port:           cfg.Port,
		User:           cfg.User,
		Password:       cfg.Password,
		Database:       cfg.Database,
		SSLMode:        cfg.SSLMode,
		SearchPath:     cfg.SearchPath,
		DefaultPort:    postgres.DefaultPort,
		DefaultSSLMode: postgres.DefaultSSLMode,
	})
}

func cockroachMigrationDSN(cfg cockroach.CockroachConfig) string {
	return buildPgFlavorMigrationDSN(pgFlavorParams{
		DSN:            cfg.DSN,
		Host:           cfg.Host,
		Port:           cfg.Port,
		User:           cfg.User,
		Password:       cfg.Password,
		Database:       cfg.Database,
		SSLMode:        cfg.SSLMode,
		SearchPath:     cfg.SearchPath,
		DefaultPort:    cockroach.DefaultPort,
		DefaultSSLMode: cockroach.DefaultSSLMode,
	})
}

func normalizeMigrationDSN(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "postgres://"):
		return "pgx5" + dsn[len("postgres"):]
	case strings.HasPrefix(dsn, "postgresql://"):
		return "pgx5" + dsn[len("postgresql"):]
	default:
		return dsn
	}
}
