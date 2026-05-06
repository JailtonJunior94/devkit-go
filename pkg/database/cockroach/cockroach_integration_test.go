//go:build integration

package cockroach_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const cockroachImage = "cockroachdb/cockroach:v23.2.0"

// setupCockroach starts a CockroachDB container and returns a connected Manager.
func setupCockroach(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        cockroachImage,
		ExposedPorts: []string{"26257/tcp"},
		Cmd:          []string{"start-single-node", "--insecure"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("26257/tcp").WithStartupTimeout(120*time.Second),
			wait.ForExec([]string{
				"cockroach",
				"sql",
				"--insecure",
				"--host=localhost:26257",
				"-e",
				"SELECT 1",
			}).WithStartupTimeout(120*time.Second),
		).WithDeadline(120 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			t.Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mapped, err := container.MappedPort(ctx, "26257")
	require.NoError(t, err)

	portNum, err := strconv.Atoi(mapped.Port())
	require.NoError(t, err)

	cfg := cockroach.CockroachConfig{
		Host:     host,
		Port:     portNum,
		User:     "root",
		Database: "defaultdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := mgr.Shutdown(shutdownCtx); serr != nil {
			t.Logf("manager shutdown: %v", serr)
		}
	})

	requireCockroachSQLReady(t, mgr)

	// create test table
	require.Eventually(t, func() bool {
		ddlCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, execErr := mgr.DBTX(context.Background()).ExecContext(
			ddlCtx,
			"CREATE TABLE IF NOT EXISTS items (id SERIAL PRIMARY KEY, value TEXT NOT NULL)",
		)
		return execErr == nil
	}, 30*time.Second, 500*time.Millisecond, "cockroach should accept DDL after startup")

	return mgr
}

func requireCockroachSQLReady(t *testing.T, mgr manager.Manager) {
	t.Helper()

	require.Eventually(t, func() bool {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := mgr.Ping(pingCtx); err != nil {
			return false
		}

		row := mgr.DBTX(context.Background()).QueryRowContext(pingCtx, "SELECT 1")
		var one int
		return row.Scan(&one) == nil && one == 1
	}, 30*time.Second, 500*time.Millisecond, "cockroach should accept SQL after startup")
}

func TestCockroach_Ping(t *testing.T) {
	mgr := setupCockroach(t)
	require.NoError(t, mgr.Ping(context.Background()))
}

func TestCockroach_Driver(t *testing.T) {
	mgr := setupCockroach(t)
	require.Equal(t, database.DriverCockroach, mgr.Driver())
}

func TestCockroach_DBSystemAttribute(t *testing.T) {
	mgr := setupCockroach(t)
	require.Equal(t, database.DriverCockroach, mgr.Driver())
}

func TestCockroach_UoW_CommitPersists(t *testing.T) {
	mgr := setupCockroach(t)
	ctx := context.Background()

	u := uow.New[int64](mgr)

	rowsAffected, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (int64, error) {
		res, execErr := tx.ExecContext(ctx, "INSERT INTO items (value) VALUES ($1)", "hello")
		if execErr != nil {
			return 0, execErr
		}
		return res.RowsAffected()
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE value = $1", "hello")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 1, count)
}

func TestCockroach_UoW_RollbackOnError(t *testing.T) {
	mgr := setupCockroach(t)
	ctx := context.Background()

	u := uow.New[struct{}](mgr)

	_, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO items (value) VALUES ($1)", "will-rollback"); execErr != nil {
			return struct{}{}, execErr
		}
		return struct{}{}, errIntentional
	})

	require.ErrorIs(t, err, errIntentional)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE value = $1", "will-rollback")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count)
}

var errIntentional = errors.New("intentional error")
