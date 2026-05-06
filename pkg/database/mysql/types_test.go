package mysql_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
)

// Compile-time assertion: *mysql.Tx must satisfy database.Tx.
var _ database.Tx = (*mysql.Tx)(nil)

// TestTx_SatisfiesDBTXInterface passes as long as the file compiles.
// Behavioural tests for Tx (Exec, Query, Commit, Rollback) are in the
// integration suite which requires a real MySQL instance.
func TestTx_SatisfiesDBTXInterface(_ *testing.T) {}
