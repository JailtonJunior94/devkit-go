package postgres

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

// pingTimeout cobre o cold-start do Postgres sob contenção de testcontainers
// em CI: 5s expirava de forma intermitente quando múltiplos containers de
// driver subiam em paralelo (FATAL: starting up). 20s preserva a detecção
// rápida de falha real sem reprovar builds por flake.
const pingTimeout = 20 * time.Second

// Adapter envolve o *pgxpool.Pool e implementa o contrato driverAdapter
// consumido pelo pkg/database/manager (definido na task 3.0).
type Adapter struct {
	pool    *pgxpool.Pool
	config  PostgresConfig
	scraper *internalpool.Scraper
}

// New cria e abre um adaptador Postgres. Ele aplica os padrões do pool, valida a
// configuração e realiza um ping de 5s antes de retornar. O DSN nunca é logado.
func New(cfg PostgresConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.ResolveDSN())
	if err != nil {
		// Não wrappa o erro do pgx: a mensagem pode conter partes do DSN
		// (incluindo password). R-SEC-001: nunca expor strings de conexão.
		return nil, fmt.Errorf("%w: postgres: invalid DSN/config", database.ErrInvalidConfig)
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
		return nil, fmt.Errorf("postgres: failed to create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}

	a := &Adapter{pool: p, config: cfg}

	if obs != nil {
		info := internalpool.ConnInfo{
			Driver:   string(database.DriverPostgres),
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

func syncDSNMetadata(cfg *PostgresConfig, connConfig *pgx.ConnConfig) {
	if cfg.DSN != "" && connConfig != nil {
		cfg.Host = connConfig.Host
		cfg.Port = int(connConfig.Port)
		cfg.Database = connConfig.Database
	}
}

// Driver retorna o identificador do driver.
func (a *Adapter) Driver() database.Driver {
	return database.DriverPostgres
}

// DBTX retorna um DBTX baseado no pool para uso fora de uma transação.
func (a *Adapter) DBTX() database.DBTX {
	return &pgxPoolDBTX{pool: a.pool}
}

// BeginTx inicia uma nova transação com as opções fornecidas.
// Converte database.TxOptions para pgx.TxOptions antes de delegar para o pool.
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
		return nil, fmt.Errorf("postgres: begin tx: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// toTxIsoLevel mapeia sql.IsolationLevel para pgx.TxIsoLevel.
// Retorna database.ErrInvalidConfig para níveis não suportados (Achado 2).
func toTxIsoLevel(level database.IsolationLevel) (pgx.TxIsoLevel, error) {
	switch level {
	case database.LevelDefault:
		return "", nil
	case database.LevelReadUncommitted:
		return pgx.ReadUncommitted, nil
	case database.LevelReadCommitted:
		return pgx.ReadCommitted, nil
	case database.LevelRepeatableRead:
		return pgx.RepeatableRead, nil
	case database.LevelSerializable:
		return pgx.Serializable, nil
	default:
		return "", fmt.Errorf("%w: postgres isolation level %q not supported", database.ErrInvalidConfig, level)
	}
}

// Stats retorna um snapshot das métricas atuais do pool.
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
		Driver:   string(database.DriverPostgres),
		Host:     a.config.Host,
		Port:     a.config.Port,
		Database: a.config.Database,
	})
}

// Ping verifica se o banco de dados está acessível.
func (a *Adapter) Ping(ctx context.Context) error {
	return a.pool.Ping(ctx)
}

// Close interrompe o coletor de métricas e fecha o pool.
// É seguro chamar Close múltiplas vezes.
func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	a.pool.Close()
	return nil
}
