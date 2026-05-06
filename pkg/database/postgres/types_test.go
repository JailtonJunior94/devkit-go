package postgres_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

// Compile-time assertion: *postgres.Tx must satisfy database.DBTX.
var _ database.DBTX = (*postgres.Tx)(nil)

// TestTx_SatisfiesDBTXInterface passes as long as the file compiles.
// Behavioural tests for Tx (Exec, Query, Commit, Rollback) are in the
// integration suite (task 3.0) which requires a real Postgres instance.
func TestTx_SatisfiesDBTXInterface(_ *testing.T) {}
