package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestApplyDefaults_SetsAllZeroFields verifies that every zero-value field gets
// the documented production default when applyDefaults is called.
func TestApplyDefaults_SetsAllZeroFields(t *testing.T) {
	cfg := &pgxpool.Config{}
	applyDefaults(cfg)

	require.Equal(t, int32(DefaultMaxOpenConns), cfg.MaxConns)
	require.Equal(t, int32(DefaultMaxIdleConns), cfg.MinConns)
	require.Equal(t, DefaultConnMaxLife, cfg.MaxConnLifetime)
	require.Equal(t, DefaultConnMaxIdle, cfg.MaxConnIdleTime)
}

// TestApplyDefaults_MaxConnsAlwaysOverwritten verifies that MaxConns is always set to the
// project default, regardless of any pre-existing value.
//
// pgxpool.ParseConfig initialises MaxConns to max(4, numCPU) — never zero — so a
// conditional check would silently skip it and leave the pool undersized. User-provided
// values override the project default via the explicit PostgresConfig field checks in New,
// not via applyDefaults.
func TestApplyDefaults_MaxConnsAlwaysOverwritten(t *testing.T) {
	tests := []struct {
		name         string
		initialConns int32
	}{
		{"zero (fresh Config)", 0},
		{"pgxpool internal default (4)", 4},
		{"above project default (50)", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &pgxpool.Config{MaxConns: tt.initialConns}
			applyDefaults(cfg)
			require.Equal(t, int32(DefaultMaxOpenConns), cfg.MaxConns)
		})
	}
}

// TestApplyDefaults_OtherFieldsPreservedWhenNonZero verifies that MinConns,
// MaxConnLifetime and MaxConnIdleTime are not overwritten when already set.
// These fields ARE left at zero by pgxpool.ParseConfig, so conditional assignment is safe.
func TestApplyDefaults_OtherFieldsPreservedWhenNonZero(t *testing.T) {
	cfg := &pgxpool.Config{
		MinConns:        10,
		MaxConnLifetime: 15 * time.Minute,
		MaxConnIdleTime: 3 * time.Minute,
	}
	applyDefaults(cfg)

	require.Equal(t, int32(10), cfg.MinConns)
	require.Equal(t, 15*time.Minute, cfg.MaxConnLifetime)
	require.Equal(t, 3*time.Minute, cfg.MaxConnIdleTime)
}

// TestApplyDefaults_PartialOverride verifies that zero fields for MinConns/lifetimes are
// filled in while MaxConns is always reset to the project default (unconditional).
func TestApplyDefaults_PartialOverride(t *testing.T) {
	cfg := &pgxpool.Config{MaxConns: 100}
	applyDefaults(cfg)

	// MaxConns is always overwritten — 100 becomes the project default.
	require.Equal(t, int32(DefaultMaxOpenConns), cfg.MaxConns)
	require.Equal(t, int32(DefaultMaxIdleConns), cfg.MinConns)
	require.Equal(t, DefaultConnMaxLife, cfg.MaxConnLifetime)
	require.Equal(t, DefaultConnMaxIdle, cfg.MaxConnIdleTime)
}
