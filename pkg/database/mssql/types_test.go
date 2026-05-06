package mssql_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
)

// Compile-time assertion: *mssql.Tx must satisfy database.Tx.
var _ database.Tx = (*mssql.Tx)(nil)

// TestTx_SatisfiesDBTXInterface passes as long as the file compiles.
// Behavioural tests for Tx (Exec, Query, Commit, Rollback) are in the
// integration suite which requires a real MSSQL instance.
func TestTx_SatisfiesDBTXInterface(_ *testing.T) {}
