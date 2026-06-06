package sqlshared

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const DefaultPingTimeout = 20 * time.Second

type ConnSettings struct {
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	ConnMaxIdle  time.Duration
}

type OpenParams struct {
	Driver        database.Driver
	DriverName    string
	DSN           string
	Settings      ConnSettings
	ApplyDefaults func(*sql.DB)
	Info          internalpool.ConnInfo
	PingTimeout   time.Duration
	Observability observability.Observability
}

type Adapter struct {
	db       *sql.DB
	poolDBTX *PoolDBTX
	driver   database.Driver
	info     internalpool.ConnInfo
	scraper  *internalpool.Scraper
}

func Open(p OpenParams) (*Adapter, error) {
	db, err := sql.Open(p.DriverName, p.DSN)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: invalid DSN/config", database.ErrInvalidConfig, p.Driver)
	}

	if p.ApplyDefaults != nil {
		p.ApplyDefaults(db)
	}
	applySettings(db, p.Settings)

	pingTimeout := p.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = DefaultPingTimeout
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: ping failed: %w", p.Driver, err)
	}

	info := p.Info
	info.Driver = string(p.Driver)

	a := &Adapter{db: db, driver: p.Driver, info: info}
	a.poolDBTX = &PoolDBTX{db: db}
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

func applySettings(db *sql.DB, s ConnSettings) {
	if s.MaxOpenConns > 0 {
		db.SetMaxOpenConns(s.MaxOpenConns)
	}
	if s.MaxIdleConns > 0 {
		db.SetMaxIdleConns(s.MaxIdleConns)
	}
	if s.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(s.ConnMaxLife)
	}
	if s.ConnMaxIdle > 0 {
		db.SetConnMaxIdleTime(s.ConnMaxIdle)
	}
}

func (a *Adapter) Driver() database.Driver { return a.driver }

func (a *Adapter) DBTX() database.DBTX { return a.poolDBTX }

func (a *Adapter) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	sqlOpts := &sql.TxOptions{
		Isolation: sql.IsolationLevel(opts.Isolation),
		ReadOnly:  opts.ReadOnly,
	}
	tx, err := a.db.BeginTx(ctx, sqlOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: begin tx: %w", a.driver, err)
	}
	return &Tx{tx: tx}, nil
}

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
	return internalpool.SafeAttrs(a.info)
}

func (a *Adapter) Ping(ctx context.Context) error { return a.db.PingContext(ctx) }

func (a *Adapter) Close(_ context.Context) error {
	if a.scraper != nil {
		a.scraper.Stop()
	}
	return a.db.Close()
}
