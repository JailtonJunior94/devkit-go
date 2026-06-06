package cockroach

import "github.com/jackc/pgx/v5/pgxpool"

func applyDefaults(cfg *pgxpool.Config) {
	cfg.MaxConns = DefaultMaxOpenConns
	if cfg.MinConns == 0 {
		cfg.MinConns = DefaultMaxIdleConns
	}
	if cfg.MaxConnLifetime == 0 {
		cfg.MaxConnLifetime = DefaultConnMaxLife
	}
	if cfg.MaxConnIdleTime == 0 {
		cfg.MaxConnIdleTime = DefaultConnMaxIdle
	}
}
