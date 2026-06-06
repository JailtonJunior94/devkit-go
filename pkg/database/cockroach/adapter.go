package cockroach

import (
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/pgxshared"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Adapter struct {
	*pgxshared.Adapter
}

type Tx = pgxshared.Tx

func New(cfg CockroachConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	inner, err := pgxshared.Open(pgxshared.OpenParams{
		Driver: database.DriverCockroach,
		DSN:    cfg.ResolveDSN(),
		Settings: pgxshared.ConnSettings{
			MaxOpenConns: cfg.MaxOpenConns,
			MaxIdleConns: cfg.MaxIdleConns,
			ConnMaxLife:  cfg.ConnMaxLife,
			ConnMaxIdle:  cfg.ConnMaxIdle,
		},
		ApplyDefaults: applyDefaults,
		IsoMapper:     toTxIsoLevel,
		InfoFn: func(c *pgx.ConnConfig) internalpool.ConnInfo {
			info := internalpool.ConnInfo{
				Host:     cfg.Host,
				Port:     cfg.Port,
				Database: cfg.Database,
			}
			if cfg.DSN != "" && c != nil {
				info.Host = c.Host
				info.Port = int(c.Port)
				info.Database = c.Database
			}
			return info
		},
		PingTimeout:   cfg.PingTimeout,
		Observability: obs,
	})
	if err != nil {
		return nil, err
	}
	return &Adapter{Adapter: inner}, nil
}

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

func syncDSNMetadata(cfg *CockroachConfig, connConfig *pgx.ConnConfig) {
	if cfg.DSN != "" && connConfig != nil {
		cfg.Host = connConfig.Host
		cfg.Port = int(connConfig.Port)
		cfg.Database = connConfig.Database
	}
}
