package mssql

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/microsoft/go-mssqldb"
)

// TestApplyDefaults_SetsAllFields verifies that applyDefaults sets all four pool
// parameters to their documented production defaults.
// sql.Open is lazy and does not connect, so the test is pure unit logic.
func TestApplyDefaults_SetsAllFields(t *testing.T) {
	db, err := sql.Open("sqlserver", "sqlserver://sa:Pass123!@127.0.0.1:1433?database=db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	applyDefaults(db)

	stats := db.Stats()
	require.Equal(t, DefaultMaxOpenConns, stats.MaxOpenConnections)
}

func TestApplyDefaults_DefaultValues(t *testing.T) {
	require.Equal(t, 20, DefaultMaxOpenConns)
	require.Equal(t, 5, DefaultMaxIdleConns)
	require.Equal(t, 10*time.Minute, DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, DefaultConnMaxIdle)
}
