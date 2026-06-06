package pgxshared

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const DefaultPingTimeout = 20 * time.Second

type IsoMapper func(database.IsolationLevel) (pgx.TxIsoLevel, error)

type ConnSettings struct {
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	ConnMaxIdle  time.Duration
}

type OpenParams struct {
	Driver        database.Driver
	DSN           string
	Settings      ConnSettings
	ApplyDefaults func(*pgxpool.Config)
	IsoMapper     IsoMapper
	InfoFn        func(*pgx.ConnConfig) internalpool.ConnInfo
	PingTimeout   time.Duration
	Observability observability.Observability
}

type Adapter struct {
	pool      *pgxpool.Pool
	poolDBTX  *PoolDBTX
	driver    database.Driver
	info      internalpool.ConnInfo
	isoMapper IsoMapper
	scraper   *internalpool.Scraper
}

func Open(p OpenParams) (*Adapter, error) {
	poolCfg, err := pgxpool.ParseConfig(p.DSN)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: invalid DSN/config", database.ErrInvalidConfig, p.Driver)
	}

	if p.ApplyDefaults != nil {
		p.ApplyDefaults(poolCfg)
	}
	applySettings(poolCfg, p.Settings)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create pool: %w", p.Driver, err)
	}

	pingTimeout := p.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = DefaultPingTimeout
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%s: ping failed: %w", p.Driver, err)
	}

	info := internalpool.ConnInfo{Driver: string(p.Driver)}
	if p.InfoFn != nil {
		info = p.InfoFn(poolCfg.ConnConfig)
		info.Driver = string(p.Driver)
	}

	a := &Adapter{
		pool:      pool,
		driver:    p.Driver,
		info:      info,
		isoMapper: p.IsoMapper,
	}
	a.poolDBTX = &PoolDBTX{pool: pool}
	if p.Observability != nil {
		a.scraper = internalpool.NewScraper(
			a.Stats,
			p.Observability.Metrics(),
			internalpool.DefaultScrapeInterval,
			internalpool.SafeAttrs(info)...,
		)
	}
	return a, nil
}

func applySettings(cfg *pgxpool.Config, s ConnSettings) {
	if s.MaxOpenConns > 0 {
		cfg.MaxConns = int32(s.MaxOpenConns)
	}
	if s.MaxIdleConns > 0 {
		cfg.MinConns = int32(s.MaxIdleConns)
	}
	if s.ConnMaxLife > 0 {
		cfg.MaxConnLifetime = s.ConnMaxLife
	}
	if s.ConnMaxIdle > 0 {
		cfg.MaxConnIdleTime = s.ConnMaxIdle
	}
}

func (a *Adapter) Driver() database.Driver { return a.driver }

func (a *Adapter) DBTX() database.DBTX { return a.poolDBTX }

func (a *Adapter) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	iso, err := a.isoMapper(opts.Isolation)
	if err != nil {
		return nil, err
	}
	pgxOpts := pgx.TxOptions{IsoLevel: iso, AccessMode: pgx.ReadWrite}
	if opts.ReadOnly {
		pgxOpts.AccessMode = pgx.ReadOnly
	}
	tx, err := a.pool.BeginTx(ctx, pgxOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: begin tx: %w", a.driver, err)
	}
	return &Tx{tx: tx}, nil
}

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
	return internalpool.SafeAttrs(a.info)
}

func (a *Adapter) Ping(ctx context.Context) error { return a.pool.Ping(ctx) }

func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	a.pool.Close()
	return nil
}
