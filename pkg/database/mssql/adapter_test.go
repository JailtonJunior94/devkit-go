package mssql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncDSNMetadata(t *testing.T) {
	dsn := "sqlserver://user:pass@host:1433?database=db"
	cfg := MSSQLConfig{DSN: dsn}

	syncDSNMetadata(&cfg, dsn)

	require.Equal(t, "host", cfg.Host)
	require.Equal(t, 1433, cfg.Port)
	require.Equal(t, "db", cfg.Database)
}

func TestSyncDSNMetadata_DefaultSchemaDoesNotMutateMetadata(t *testing.T) {
	dsn := "sqlserver://user:pass@host:1433?database=db"
	cfg := MSSQLConfig{
		DSN:           dsn,
		DefaultSchema: "tenant_schema",
	}

	syncDSNMetadata(&cfg, dsn)

	require.Equal(t, "host", cfg.Host)
	require.Equal(t, 1433, cfg.Port)
	require.Equal(t, "db", cfg.Database)
	require.Equal(t, "tenant_schema", cfg.DefaultSchema)
}
