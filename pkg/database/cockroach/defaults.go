package cockroach

import "github.com/jackc/pgx/v5/pgxpool"

// applyDefaults sets safe production defaults on cfg calibrated for CockroachDB.
// MaxConns is always overwritten because pgxpool.ParseConfig already initialises it
// to max(4, numCPU) — never zero — so a conditional check would silently skip it.
// MinConns, MaxConnLifetime and MaxConnIdleTime are left at zero by ParseConfig, so
// they are only set when still unspecified. User overrides happen in New via the
// explicit CockroachConfig field checks that run after applyDefaults.
func applyDefaults(cfg *pgxpool.Config) {
	cfg.MaxConns = DefaultMaxOpenConns // unconditional: override pgxpool internal default
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
