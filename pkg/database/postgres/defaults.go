package postgres

import "github.com/jackc/pgx/v5/pgxpool"

// applyDefaults define padrões de produção seguros no cfg.
// MaxConns é sempre sobrescrito porque o pgxpool.ParseConfig já o inicializa
// como max(4, numCPU) — nunca zero — então uma verificação condicional o ignoraria silenciosamente.
// MinConns, MaxConnLifetime e MaxConnIdleTime são deixados como zero pelo ParseConfig, então
// eles são definidos apenas quando ainda não especificados. As sobreposições do usuário ocorrem no New através das
// verificações explícitas de campos do PostgresConfig que rodam após o applyDefaults.
func applyDefaults(cfg *pgxpool.Config) {
	cfg.MaxConns = DefaultMaxOpenConns // incondicional: sobrescreve o padrão interno do pgxpool
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
