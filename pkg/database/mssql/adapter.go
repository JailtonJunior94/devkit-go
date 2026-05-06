package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/microsoft/go-mssqldb/msdsn"
)

// pingTimeout cobre o cold-start do MSSQL sob contenção de testcontainers em
// CI: 5s expirava de forma intermitente quando múltiplos containers subiam em
// paralelo. 20s preserva detecção rápida de falha real sem reprovar builds.
const pingTimeout = 20 * time.Second

// Adapter wraps *sql.DB with the MSSQL driver and implements the driverAdapter
// contract consumed by pkg/database/manager.
type Adapter struct {
	db      *sql.DB
	config  MSSQLConfig
	scraper *internalpool.Scraper
}

// New creates and opens an MSSQL adapter. It applies pool defaults, validates the
// config, and performs a 5 s ping before returning. DSN is never logged.
func New(cfg MSSQLConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	dsn := cfg.ResolveDSN()
	// Sincroniza metadados do DSN de volta para a config para observabilidade.
	syncDSNMetadata(&cfg, dsn)

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		// Não wrappa o erro do driver: a mensagem pode conter partes do DSN
		// (incluindo password). R-SEC-001: nunca expor strings de conexão.
		return nil, fmt.Errorf("%w: mssql: invalid DSN/config", database.ErrInvalidConfig)
	}

	applyDefaults(db)

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLife)
	}
	if cfg.ConnMaxIdle > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdle)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mssql: ping failed: %w", err)
	}

	a := &Adapter{db: db, config: cfg}

	if obs != nil {
		info := internalpool.ConnInfo{
			Driver:   string(database.DriverMSSQL),
			Host:     cfg.Host,
			Port:     cfg.Port,
			Database: cfg.Database,
		}
		attrs := internalpool.SafeAttrs(info)
		a.scraper = internalpool.NewScraper(
			a.Stats,
			obs.Metrics(),
			internalpool.DefaultScrapeInterval,
			attrs...,
		)
	}

	return a, nil
}

// syncDSNMetadata populates Host, Port, and Database from a parsed DSN so that
// observability attributes are always populated even when the caller supplies
// only a DSN string.
func syncDSNMetadata(cfg *MSSQLConfig, dsn string) {
	if cfg.DSN != "" {
		// msdsn.Parse returns (Config, error) — two values only.
		if parsed, err := msdsn.Parse(dsn); err == nil {
			cfg.Host = parsed.Host
			cfg.Port = int(parsed.Port)
			cfg.Database = parsed.Database
		}
	}
}

// Driver returns the driver identifier.
func (a *Adapter) Driver() database.Driver {
	return database.DriverMSSQL
}

// DBTX returns a pool-backed DBTX for use outside a transaction.
func (a *Adapter) DBTX() database.DBTX {
	return &sqlDBDBTX{db: a.db}
}

// BeginTx starts a new transaction with the given options.
func (a *Adapter) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	sqlOpts := &sql.TxOptions{
		Isolation: sql.IsolationLevel(opts.Isolation),
		ReadOnly:  opts.ReadOnly,
	}
	tx, err := a.db.BeginTx(ctx, sqlOpts)
	if err != nil {
		return nil, fmt.Errorf("mssql: begin tx: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Stats returns a snapshot of the current pool metrics.
func (a *Adapter) Stats() internalpool.Stats {
	s := a.db.Stats()
	return internalpool.Stats{
		OpenConnections: s.OpenConnections,
		Idle:            s.Idle,
		WaitCount:       s.WaitCount,
		WaitDuration:    s.WaitDuration,
	}
}

func (a *Adapter) Attributes() []observability.Field {
	return internalpool.SafeAttrs(internalpool.ConnInfo{
		Driver:   string(database.DriverMSSQL),
		Host:     a.config.Host,
		Port:     a.config.Port,
		Database: a.config.Database,
	})
}

// Ping checks that the database is reachable.
func (a *Adapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Close stops the metrics scraper and closes the pool.
// It is safe to call Close multiple times.
func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	return a.db.Close()
}
