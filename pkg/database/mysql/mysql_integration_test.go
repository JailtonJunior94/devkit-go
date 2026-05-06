//go:build integration

package mysql_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const mysqlImage = "mysql:8"

// setupMySQL starts a MySQL container and returns a connected Manager.
func setupMySQL(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        mysqlImage,
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "rootpass",
			"MYSQL_USER":          "test",
			"MYSQL_PASSWORD":      "test",
			"MYSQL_DATABASE":      "testdb",
		},
		WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(90 * time.Second),
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

	mapped, err := container.MappedPort(ctx, "3306")
	require.NoError(t, err)

	portNum, err := strconv.Atoi(mapped.Port())
	require.NoError(t, err)

	cfg := mysql.MySQLConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
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

	// create test table
	db := mgr.DBTX(context.Background())
	_, err = db.ExecContext(context.Background(),
		"CREATE TABLE IF NOT EXISTS items (id INT AUTO_INCREMENT PRIMARY KEY, value VARCHAR(255) NOT NULL)",
	)
	require.NoError(t, err)

	return mgr
}

func TestMySQL_Ping(t *testing.T) {
	mgr := setupMySQL(t)
	ctx := context.Background()
	require.NoError(t, mgr.Ping(ctx))
}

func TestMySQL_Driver(t *testing.T) {
	mgr := setupMySQL(t)
	require.Equal(t, database.DriverMySQL, mgr.Driver())
}

func TestMySQL_UoW_CommitPersists(t *testing.T) {
	mgr := setupMySQL(t)
	ctx := context.Background()

	u := uow.New[int64](mgr)

	rowsAffected, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (int64, error) {
		res, execErr := tx.ExecContext(ctx, "INSERT INTO items (value) VALUES (?)", "hello")
		if execErr != nil {
			return 0, execErr
		}
		return res.RowsAffected()
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE value = ?", "hello")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 1, count)
}

func TestMySQL_UoW_RollbackOnError(t *testing.T) {
	mgr := setupMySQL(t)
	ctx := context.Background()

	u := uow.New[struct{}](mgr)

	_, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO items (value) VALUES (?)", "will-rollback"); execErr != nil {
			return struct{}{}, execErr
		}
		return struct{}{}, errIntentional
	})

	require.ErrorIs(t, err, errIntentional)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE value = ?", "will-rollback")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count)
}

var errIntentional = errors.New("intentional error")
