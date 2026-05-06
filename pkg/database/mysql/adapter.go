package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/go-sql-driver/mysql"
)

// pingTimeout cobre o cold-start do MySQL sob contenção de testcontainers em
// CI: 5s expirava de forma intermitente quando múltiplos containers subiam em
// paralelo. 20s preserva detecção rápida de falha real sem reprovar builds.
const pingTimeout = 20 * time.Second

// Adapter wraps *sql.DB with the MySQL driver and implements the driverAdapter
// contract consumed by pkg/database/manager.
type Adapter struct {
	db      *sql.DB
	config  MySQLConfig
	scraper *internalpool.Scraper
}

// New cria e abre um adaptador MySQL. Ele aplica os padrões do pool, valida a
// configuração e realiza um ping de 5s antes de retornar. O DSN nunca é logado.
func New(cfg MySQLConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	dsn := cfg.ResolveDSN()
	// Sincroniza metadados do DSN de volta para a config para observabilidade (Achado 1).
	syncDSNMetadata(&cfg, dsn)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		// Não wrappa o erro do driver: a mensagem pode conter partes do DSN
		// (incluindo password). R-SEC-001: nunca expor strings de conexão.
		return nil, fmt.Errorf("%w: mysql: invalid DSN/config", database.ErrInvalidConfig)
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
		return nil, fmt.Errorf("mysql: ping failed: %w", err)
	}

	a := &Adapter{db: db, config: cfg}

	if obs != nil {
		info := internalpool.ConnInfo{
			Driver:   string(database.DriverMySQL),
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

func syncDSNMetadata(cfg *MySQLConfig, dsn string) {
	if cfg.DSN != "" {
		if parsed, err := mysql.ParseDSN(dsn); err == nil {
			cfg.Database = parsed.DBName
		}
	}
}

// Driver returns the driver identifier.
func (a *Adapter) Driver() database.Driver {
	return database.DriverMySQL
}

// DBTX retorna um DBTX baseado no pool para uso fora de uma transação.
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
		return nil, fmt.Errorf("mysql: begin tx: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Stats retorna um snapshot das métricas atuais do pool.
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
		Driver:   string(database.DriverMySQL),
		Host:     a.config.Host,
		Port:     a.config.Port,
		Database: a.config.Database,
	})
}

// Ping checks that the database is reachable.
func (a *Adapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Close interrompe o coletor de métricas e fecha o pool.
// É seguro chamar Close múltiplas vezes.
func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	return a.db.Close()
}
