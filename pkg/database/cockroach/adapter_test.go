package cockroach

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Regressão (Achado #1 / R-SEC-001): em DSN inválido, o erro retornado por New
// não pode incluir o conteúdo do DSN — em particular, o password —, que chegaria
// via wrapping do erro do pgxpool.ParseConfig.
func TestNew_InvalidDSN_DoesNotLeakCredentials(t *testing.T) {
	const password = "sup3r-s3cr3t-p4ssw0rd"
	cfg := CockroachConfig{DSN: "postgres://user:" + password + "@host:not-a-port/db"}

	_, err := New(cfg, nil)

	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
	require.NotContains(t, err.Error(), password)
	require.NotContains(t, err.Error(), cfg.DSN)
}

// I5: CockroachDB only supports SERIALIZABLE and READ COMMITTED isolation.
// Any other level must be rejected with database.ErrInvalidConfig instead of
// being silently mapped (or surfaced later as a server-side runtime error).
func TestToTxIsoLevel_RejectsUnsupportedLevels(t *testing.T) {
	tests := []struct {
		name  string
		level database.IsolationLevel
	}{
		{"read uncommitted", database.LevelReadUncommitted},
		{"repeatable read", database.LevelRepeatableRead},
		{"snapshot", database.LevelSnapshot},
		{"linearizable", database.LevelLinearizable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toTxIsoLevel(tt.level)
			require.Error(t, err)
			require.ErrorIs(t, err, database.ErrInvalidConfig)
		})
	}
}

func TestToTxIsoLevel_AcceptsSupportedLevels(t *testing.T) {
	tests := []struct {
		name  string
		level database.IsolationLevel
		want  pgx.TxIsoLevel
	}{
		{"default", database.LevelDefault, ""},
		{"read committed", database.LevelReadCommitted, pgx.ReadCommitted},
		{"serializable", database.LevelSerializable, pgx.Serializable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toTxIsoLevel(tt.level)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSyncDSNMetadata(t *testing.T) {
	cfg := CockroachConfig{DSN: "postgres://user:pass@host:5432/db"}
	connConfig := &pgx.ConnConfig{}
	connConfig.Host = "host"
	connConfig.Port = 5432
	connConfig.Database = "db"

	syncDSNMetadata(&cfg, connConfig)

	require.Equal(t, "host", cfg.Host)
	require.Equal(t, 5432, cfg.Port)
	require.Equal(t, "db", cfg.Database)
}
