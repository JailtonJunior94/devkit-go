package cockroach

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults_SetsAllZeroFields(t *testing.T) {
	cfg := &pgxpool.Config{}
	applyDefaults(cfg)

	require.Equal(t, int32(DefaultMaxOpenConns), cfg.MaxConns)
	require.Equal(t, int32(DefaultMaxIdleConns), cfg.MinConns)
	require.Equal(t, DefaultConnMaxLife, cfg.MaxConnLifetime)
	require.Equal(t, DefaultConnMaxIdle, cfg.MaxConnIdleTime)
}

func TestApplyDefaults_MaxConnsAlwaysOverwritten(t *testing.T) {
	tests := []struct {
		name         string
		initialConns int32
	}{
		{"zero (fresh Config)", 0},
		{"pgxpool internal default (4)", 4},
		{"below project default (10)", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &pgxpool.Config{MaxConns: tt.initialConns}
			applyDefaults(cfg)
			require.Equal(t, int32(DefaultMaxOpenConns), cfg.MaxConns)
		})
	}
}

func TestApplyDefaults_OtherFieldsPreservedWhenNonZero(t *testing.T) {
	cfg := &pgxpool.Config{
		MinConns:        20,
		MaxConnLifetime: 20 * time.Minute,
		MaxConnIdleTime: 3 * time.Minute,
	}
	applyDefaults(cfg)

	require.Equal(t, int32(20), cfg.MinConns)
	require.Equal(t, 20*time.Minute, cfg.MaxConnLifetime)
	require.Equal(t, 3*time.Minute, cfg.MaxConnIdleTime)
}

func TestApplyDefaults_DefaultValuesMatchSpec(t *testing.T) {
	require.Equal(t, int32(50), int32(DefaultMaxOpenConns))
	require.Equal(t, int32(10), int32(DefaultMaxIdleConns))
	require.Equal(t, 15*time.Minute, DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, DefaultConnMaxIdle)
}
