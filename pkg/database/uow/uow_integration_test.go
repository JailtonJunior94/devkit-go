//go:build integration
// +build integration

package uow

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupPostgresTestDB creates a real PostgreSQL database using testcontainers.
// This tests actual PostgreSQL behavior including MVCC, deadlocks, and SSI.
func setupPostgresTestDB(t *testing.T) *sql.DB {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Open connection
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	// Verify connectivity
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	// Create test table with PostgreSQL syntax
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_orders (
			id SERIAL PRIMARY KEY,
			status TEXT NOT NULL,
			total DECIMAL(10,2) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func TestIntegration_UnitOfWork_SuccessfulCommit(t *testing.T) {
	db := setupPostgresTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
		return err
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify data was committed
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestIntegration_UnitOfWork_RollbackOnError(t *testing.T) {
	db := setupPostgresTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	expectedErr := errors.New("business logic error")

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
		if err != nil {
			return err
		}
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got: %v", expectedErr, err)
	}

	// Verify data was rolled back
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 rows (rollback), got %d", count)
	}
}

func TestIntegration_UnitOfWork_PanicRecoveryRollback(t *testing.T) {
	db := setupPostgresTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	func() {
		defer func() {
			_ = recover() // Catch panic to continue test
		}()

		_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
			_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
			if err != nil {
				return err
			}
			panic("simulated panic")
		})
	}()

	// Verify data was rolled back despite panic
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 rows (panic rollback), got %d", count)
	}
}

func TestIntegration_UnitOfWork_ContextCancellation(t *testing.T) {
	db := setupPostgresTestDB(t)

	uow := NewUnitOfWork(db)

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		t.Error("function should not be executed with cancelled context")
		return nil
	})

	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestIntegration_UnitOfWork_ContextCancelledDuringExecution(t *testing.T) {
	db := setupPostgresTestDB(t)

	uow := NewUnitOfWork(db)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		// Insert data
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
		if err != nil {
			return err
		}

		// Simulate long-running operation
		time.Sleep(200 * time.Millisecond)

		// Try to insert more data
		_, err = db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "completed", 200.00)
		return err
	})

	// Should return context cancellation error
	if err == nil {
		t.Fatal("expected error for cancelled context during execution")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}

	// Verify transaction was rolled back
	var count int
	queryErr := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if queryErr != nil {
		t.Fatalf("failed to query: %v", queryErr)
	}

	if count != 0 {
		t.Errorf("expected 0 rows (rollback due to context cancellation), got %d", count)
	}
}

func TestIntegration_UnitOfWork_SerializableIsolation(t *testing.T) {
	db := setupPostgresTestDB(t)

	// Test with Serializable isolation level
	// PostgreSQL implements true SSI (Serializable Snapshot Isolation)
	uow := NewUnitOfWork(db, WithIsolationLevel(sql.LevelSerializable))
	ctx := context.Background()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
		return err
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify data was committed
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestIntegration_UnitOfWork_ReadOnly(t *testing.T) {
	db := setupPostgresTestDB(t)

	// Insert test data first
	_, err := db.Exec("INSERT INTO test_orders (status, total) VALUES ($1, $2)", "pending", 100.00)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Test read-only transaction
	uow := NewUnitOfWork(db, WithReadOnly(true))
	ctx := context.Background()

	err = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		var count int
		return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	})

	if err != nil {
		t.Fatalf("expected no error for read operation, got: %v", err)
	}

	// Verify read-only transaction rejects writes
	err = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES ($1, $2)", "new", 200.00)
		return err
	})

	// PostgreSQL should reject write in read-only transaction
	if err == nil {
		t.Error("expected error for write operation in read-only transaction")
	}
}
