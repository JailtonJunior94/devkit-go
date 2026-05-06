package cockroach

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pingTimeout cobre o cold-start do CRDB sob contenção de testcontainers em
// CI: 5s era suficiente em isolamento, mas expirava de forma intermitente
// quando múltiplos containers de driver subiam em paralelo. 20s preserva a
// detecção rápida de falha de conexão real sem reprovar builds por flake.
const pingTimeout = 20 * time.Second

// Adapter wraps *pgxpool.Pool and implements the driverAdapter contract
// consumed by pkg/database/manager. Internally uses pgx/v5 like the Postgres
// adapter but reports db.system=cockroach in metrics and spans.
type Adapter struct {
	pool    *pgxpool.Pool
	config  CockroachConfig
	scraper *internalpool.Scraper
}

// New creates and opens a CockroachDB adapter. It applies pool defaults, validates
// the config, and performs a 5 s ping before returning. DSN is never logged.
func New(cfg CockroachConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.ResolveDSN())
	if err != nil {
		// Não wrappa o erro do pgx: a mensagem pode conter partes do DSN
		// (incluindo password). R-SEC-001: nunca expor strings de conexão.
		return nil, fmt.Errorf("%w: cockroach: invalid DSN/config", database.ErrInvalidConfig)
	}

	// Sincroniza metadados do DSN de volta para a config para observabilidade (Achado 1).
	syncDSNMetadata(&cfg, poolCfg.ConnConfig)

	applyDefaults(poolCfg)

	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLife > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLife
	}
	if cfg.ConnMaxIdle > 0 {
		poolCfg.MaxConnIdleTime = cfg.ConnMaxIdle
	}

	p, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("cockroach: failed to create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, fmt.Errorf("cockroach: ping failed: %w", err)
	}

	a := &Adapter{pool: p, config: cfg}

	if obs != nil {
		info := internalpool.ConnInfo{
			Driver:   string(database.DriverCockroach),
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

func syncDSNMetadata(cfg *CockroachConfig, connConfig *pgx.ConnConfig) {
	if cfg.DSN != "" && connConfig != nil {
		cfg.Host = connConfig.Host
		cfg.Port = int(connConfig.Port)
		cfg.Database = connConfig.Database
	}
}

// Driver returns the driver identifier.
func (a *Adapter) Driver() database.Driver {
	return database.DriverCockroach
}

// DBTX returns a pool-backed DBTX for use outside a transaction.
func (a *Adapter) DBTX() database.DBTX {
	return &pgxPoolDBTX{pool: a.pool}
}

// BeginTx starts a new transaction with the given options.
//
// CockroachDB only supports SERIALIZABLE and READ COMMITTED isolation levels.
// Any other level (ReadUncommitted, RepeatableRead, Snapshot, etc.) is
// rejected up-front with database.ErrInvalidConfig instead of being silently
// remapped or surfaced as a runtime error from the server on the first
// statement.
func (a *Adapter) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	iso, err := toTxIsoLevel(opts.Isolation)
	if err != nil {
		return nil, err
	}
	pgxOpts := pgx.TxOptions{
		IsoLevel:   iso,
		AccessMode: pgx.ReadWrite,
	}
	if opts.ReadOnly {
		pgxOpts.AccessMode = pgx.ReadOnly
	}
	tx, err := a.pool.BeginTx(ctx, pgxOpts)
	if err != nil {
		return nil, fmt.Errorf("cockroach: begin tx: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// toTxIsoLevel maps sql.IsolationLevel to pgx.TxIsoLevel for CockroachDB.
// Returns database.ErrInvalidConfig for levels CRDB does not support.
func toTxIsoLevel(level database.IsolationLevel) (pgx.TxIsoLevel, error) {
	switch level {
	case database.LevelDefault:
		return "", nil
	case database.LevelReadCommitted:
		return pgx.ReadCommitted, nil
	case database.LevelSerializable:
		return pgx.Serializable, nil
	default:
		return "", fmt.Errorf("%w: cockroach isolation level %q not supported (use LevelDefault, LevelReadCommitted or LevelSerializable)", database.ErrInvalidConfig, level)
	}
}

// Stats returns a snapshot of the current pool metrics.
func (a *Adapter) Stats() internalpool.Stats {
	s := a.pool.Stat()
	return internalpool.Stats{
		OpenConnections: int(s.TotalConns()),
		Idle:            int(s.IdleConns()),
		WaitCount:       s.EmptyAcquireCount(),
		WaitDuration:    s.EmptyAcquireWaitTime(),
	}
}

func (a *Adapter) Attributes() []observability.Field {
	return internalpool.SafeAttrs(internalpool.ConnInfo{
		Driver:   string(database.DriverCockroach),
		Host:     a.config.Host,
		Port:     a.config.Port,
		Database: a.config.Database,
	})
}

// Ping checks that the database is reachable.
func (a *Adapter) Ping(ctx context.Context) error {
	return a.pool.Ping(ctx)
}

// Close stops the metrics scraper and closes the pool.
// It is safe to call Close multiple times.
func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	a.pool.Close()
	return nil
}
