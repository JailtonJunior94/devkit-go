package mssql

import (
	"fmt"

	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/sqlshared"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Adapter struct {
	*sqlshared.Adapter
}

type Tx = sqlshared.Tx

func New(cfg MSSQLConfig, obs observability.Observability) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", database.ErrInvalidConfig, err)
	}

	dsn := cfg.ResolveDSN()
	syncDSNMetadata(&cfg, dsn)

	inner, err := sqlshared.Open(sqlshared.OpenParams{
		Driver:     database.DriverMSSQL,
		DriverName: "sqlserver",
		DSN:        dsn,
		Settings: sqlshared.ConnSettings{
			MaxOpenConns: cfg.MaxOpenConns,
			MaxIdleConns: cfg.MaxIdleConns,
			ConnMaxLife:  cfg.ConnMaxLife,
			ConnMaxIdle:  cfg.ConnMaxIdle,
		},
		ApplyDefaults: applyDefaults,
		Info: internalpool.ConnInfo{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Database: cfg.Database,
		},
		PingTimeout:   cfg.PingTimeout,
		Observability: obs,
	})
	if err != nil {
		return nil, err
	}
	return &Adapter{Adapter: inner}, nil
}

func syncDSNMetadata(cfg *MSSQLConfig, dsn string) {
	if cfg.DSN != "" {
		if parsed, err := msdsn.Parse(dsn); err == nil {
			cfg.Host = parsed.Host
			cfg.Port = int(parsed.Port)
			cfg.Database = parsed.Database
		}
	}
}
