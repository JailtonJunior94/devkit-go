package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database for testing.
//
// IMPORTANTE: Estes testes usam SQLite por simplicidade e velocidade.
// SQLite tem diferenças significativas em relação a PostgreSQL:
//   - Concurrency: SQLite usa locks, PostgreSQL usa MVCC
//   - Isolation: Semântica de isolation levels é diferente
//   - Errors: Tipos de erro são diferentes
//
// Para testes de integração completos com PostgreSQL real, use:
//   go test -tags=integration ./pkg/database/uow
//
// Trade-off: SQLite é suficiente para testar a LÓGICA do Unit of Work
// (commit, rollback, panic recovery, context), mas não comportamento
// específico de PostgreSQL (deadlocks, serialization failures).
func setupTestDB(t *testing.T) *sql.DB {
	// Create a temporary file for the database
	tmpfile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// Clean up the temp file when test ends
	t.Cleanup(func() {
		_ = os.Remove(tmpfile.Name())
	})

	// Open database with WAL mode and busy timeout for better concurrency support
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000", tmpfile.Name()))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status TEXT NOT NULL,
			total DECIMAL(10,2) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func TestUnitOfWork_SuccessfulCommit(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
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

func TestUnitOfWork_RollbackOnError(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	expectedErr := errors.New("business logic error")

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
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

func TestUnitOfWork_PanicRecovery(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to be re-thrown")
		}
	}()

	_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
		if err != nil {
			return err
		}
		panic("simulated panic")
	})

	// This code should not be reached
	t.Error("code after panic should not execute")
}

func TestUnitOfWork_PanicRecoveryRollback(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	func() {
		defer func() {
			_ = recover() // Catch panic to continue test
		}()

		_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
			_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
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

func TestUnitOfWork_ConcurrentTransactions(t *testing.T) {
	db := setupTestDB(t)

	// Enable concurrent connections for this test
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(0)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	const numGoroutines = 20
	var wg sync.WaitGroup
	successCount := 0
	lockErrorCount := 0
	var mu sync.Mutex

	// Launch multiple concurrent transactions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
				// Simulate some work
				time.Sleep(time.Millisecond * 2)
				_, err := db.ExecContext(ctx,
					"INSERT INTO test_orders (status, total) VALUES (?, ?)",
					"pending",
					float64(id))
				return err
			})

			mu.Lock()
			if err != nil {
				// SQLite lock errors are expected under high concurrency - not a UoW bug
				if err.Error() == "database table is locked" {
					lockErrorCount++
				} else {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				successCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify successful transactions were committed
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != successCount {
		t.Errorf("expected %d rows (matching successful transactions), got %d", successCount, count)
	}

	// Verify we had some successful concurrent transactions (proves concurrency works)
	// SQLite has limitations with concurrent writes, so we're lenient here
	minExpected := numGoroutines / 3
	if successCount < minExpected {
		t.Errorf("expected at least %d successful transactions, got %d (lock errors: %d)",
			minExpected, successCount, lockErrorCount)
	}

	t.Logf("Concurrent test completed: %d successful, %d lock errors (SQLite limitation)",
		successCount, lockErrorCount)
}

func TestUnitOfWork_ConcurrentWithFailures(t *testing.T) {
	db := setupTestDB(t)

	// Enable concurrent connections for this test
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(0)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	const numGoroutines = 20
	var wg sync.WaitGroup
	successCount := 0
	intentionalFailCount := 0
	var mu sync.Mutex

	// Launch concurrent transactions, half will fail intentionally
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
				_, err := db.ExecContext(ctx,
					"INSERT INTO test_orders (status, total) VALUES (?, ?)",
					"pending",
					float64(id))
				if err != nil {
					return err
				}

				// Fail every other transaction intentionally
				if id%2 == 0 {
					return errors.New("simulated error")
				}
				return nil
			})

			mu.Lock()
			if err == nil {
				successCount++
			} else if err.Error() == "simulated error" {
				intentionalFailCount++
			}
			// Ignore SQLite lock errors for this test
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify only successful transactions were committed
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != successCount {
		t.Errorf("expected %d rows, got %d", successCount, count)
	}

	// Verify we had some intentional failures (proves rollback works)
	if intentionalFailCount == 0 {
		t.Error("expected some intentional failures to test rollback")
	}

	t.Logf("Concurrent with failures test: %d successful, %d intentional failures (rollback tested)",
		successCount, intentionalFailCount)
}

func TestUnitOfWork_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)

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

func TestUnitOfWork_ContextCancelledDuringExecution(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)

	// Create context with timeout that will expire during execution
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		// Insert data
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
		if err != nil {
			return err
		}

		// Simulate long-running operation
		time.Sleep(100 * time.Millisecond)

		// Try to insert more data (this should succeed but transaction will be rolled back)
		_, err = db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "completed", 200.00)
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

func TestUnitOfWork_WithIsolationLevel(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db, WithIsolationLevel(sql.LevelSerializable))
	ctx := context.Background()

	err := uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
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

func TestUnitOfWork_WithReadOnly(t *testing.T) {
	db := setupTestDB(t)

	// Insert test data first
	_, err := db.Exec("INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	uow := NewUnitOfWork(db, WithReadOnly(true))
	ctx := context.Background()

	err = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		var count int
		return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	})

	if err != nil {
		t.Fatalf("expected no error for read operation, got: %v", err)
	}
}

func TestUnitOfWork_NilDB(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when creating UnitOfWork with nil DB")
		}
	}()

	_ = NewUnitOfWork(nil)
}

func TestUnitOfWork_MultiplePanicsSequential(t *testing.T) {
	db := setupTestDB(t)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	// Test that multiple panics in sequence are handled correctly
	for i := 0; i < 3; i++ {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("iteration %d: expected panic to be propagated", i)
				}
			}()

			_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
				_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", float64(i))
				if err != nil {
					return err
				}
				panic("simulated panic")
			})
		}()
	}

	// Verify no data was committed due to panics
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_orders").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 rows (all panicked), got %d", count)
	}
}

// Benchmark tests.
func BenchmarkUnitOfWork_Sequential(b *testing.B) {
	// Create a mock testing.T for setup
	t := &testing.T{}
	db := setupTestDB(t)
	// setupTestDB uses t.Cleanup, so we need to defer close manually for benchmarks
	defer func() {
		if err := db.Close(); err != nil {
			b.Logf("failed to close database: %v", err)
		}
	}()

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
			_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
			return err
		})
	}
}

func BenchmarkUnitOfWork_Concurrent(b *testing.B) {
	// Create a mock testing.T for setup
	t := &testing.T{}
	db := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			b.Logf("failed to close database: %v", err)
		}
	}()

	db.SetMaxOpenConns(20)

	uow := NewUnitOfWork(db)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
				_, err := db.ExecContext(ctx, "INSERT INTO test_orders (status, total) VALUES (?, ?)", "pending", 100.00)
				return err
			})
		}
	})
}
