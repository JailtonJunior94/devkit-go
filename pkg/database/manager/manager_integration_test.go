//go:build integration

package manager_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const pgImage = "postgres:16"

func setupPostgresContainer(t *testing.T) postgres.PostgresConfig {
	t.Helper()

	ctx := context.Background()
	req := tc.ContainerRequest{
		Image:        pgImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		).WithDeadline(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mapped, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	portNum, err := strconv.Atoi(mapped.Port())
	require.NoError(t, err)

	return postgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}
}

func writeMigrationPair(t *testing.T, dir string, seq int, name, upSQL, downSQL string) {
	t.Helper()

	base := fmt.Sprintf("%06d_%s", seq, name)
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".up.sql"), []byte(upSQL), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".down.sql"), []byte(downSQL), 0o600))
}

func TestIntegration_New_AppliesStartupMigrationsFromConventionalPath(t *testing.T) {
	cfg := setupPostgresContainer(t)

	root := t.TempDir()
	migrationDir := filepath.Join(root, "migrations", "postgres")
	require.NoError(t, os.MkdirAll(migrationDir, 0o755))
	writeMigrationPair(
		t,
		migrationDir,
		1,
		"create_startup_users",
		"CREATE TABLE startup_users (id SERIAL PRIMARY KEY, name TEXT NOT NULL);",
		"DROP TABLE startup_users;",
	)

	t.Chdir(root)

	mgr, err := manager.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	var count int
	row := mgr.DBTX(context.Background()).QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1",
		"startup_users",
	)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 1, count, "manager.New deve retornar apenas após aplicar as startup migrations disponíveis")
}

func TestIntegration_New_StartupMigrationFailureAbortsConstruction(t *testing.T) {
	cfg := setupPostgresContainer(t)

	root := t.TempDir()
	migrationDir := filepath.Join(root, "migrations", "postgres")
	require.NoError(t, os.MkdirAll(migrationDir, 0o755))
	writeMigrationPair(
		t,
		migrationDir,
		1,
		"broken_startup_migration",
		"THIS IS NOT VALID SQL;",
		"DROP TABLE IF EXISTS broken_table;",
	)

	t.Chdir(root)

	mgr, err := manager.New(cfg)
	require.Nil(t, mgr)
	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrMigrationFailed, "falha de migration no startup deve abortar a construção do manager")
}

func TestIntegration_Shutdown_WaitsForInFlightTxAndRejectsNewTransactions(t *testing.T) {
	cfg := setupPostgresContainer(t)

	mgr, err := manager.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	tx, err := mgr.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- mgr.Shutdown(context.Background())
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown retornou cedo demais: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.Eventually(t, func() bool {
		_, err := mgr.BeginTx(context.Background(), database.TxOptions{})
		return err == database.ErrManagerClosed
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, tx.Commit(context.Background()))

	select {
	case err := <-shutdownDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown não concluiu após o commit da tx em voo")
	}
}

func TestIntegration_BeginTx_RespectsMaxOpenConns(t *testing.T) {
	cfg := setupPostgresContainer(t)
	cfg.MaxOpenConns = 1

	mgr, err := manager.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	tx1, err := mgr.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)

	type beginResult struct {
		tx  database.Tx
		err error
	}
	beginDone := make(chan beginResult, 1)
	go func() {
		tx, err := mgr.BeginTx(context.Background(), database.TxOptions{})
		beginDone <- beginResult{tx: tx, err: err}
	}()

	select {
	case result := <-beginDone:
		if result.tx != nil {
			_ = result.tx.Rollback(context.Background())
		}
		t.Fatalf("segunda tx não deveria começar antes de liberar a única conexão do pool: %v", result.err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, tx1.Commit(context.Background()))

	select {
	case result := <-beginDone:
		require.NoError(t, result.err)
		require.NotNil(t, result.tx)
		require.NoError(t, result.tx.Rollback(context.Background()))
	case <-time.After(2 * time.Second):
		t.Fatal("segunda tx não iniciou após a liberação da conexão do pool")
	}
}
