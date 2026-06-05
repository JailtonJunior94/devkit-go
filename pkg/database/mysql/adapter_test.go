package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncDSNMetadata(t *testing.T) {
	cfg := MySQLConfig{DSN: "user:pass@tcp(host:3306)/db"}

	syncDSNMetadata(&cfg, cfg.DSN)

	require.Equal(t, "db", cfg.Database)
}
