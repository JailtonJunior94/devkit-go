//go:build integration

package mssql_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	mssqlImage    = "mcr.microsoft.com/mssql/server:2022-latest"
	mssqlPassword = "Test@Pass123!"
)

// setupMSSQL starts a MSSQL container and returns a connected Manager plus the config used.
func setupMSSQL(t *testing.T) (manager.Manager, mssql.MSSQLConfig) {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        mssqlImage,
		ExposedPorts: []string{"1433/tcp"},
		Env: map[string]string{
			"ACCEPT_EULA": "Y",
			"SA_PASSWORD": mssqlPassword,
			"MSSQL_PID":   "Developer",
		},
		WaitingFor: wait.ForListeningPort("1433/tcp").WithStartupTimeout(120 * time.Second),
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

	mapped, err := container.MappedPort(ctx, "1433")
	require.NoError(t, err)

	portNum, err := strconv.Atoi(mapped.Port())
	require.NoError(t, err)

	cfg := mssql.MSSQLConfig{
		Host:     host,
		Port:     portNum,
		User:     "sa",
		Password: mssqlPassword,
		Database: "master",
	}

	// MSSQL needs a brief moment after the port is open before accepting logins.
	time.Sleep(5 * time.Second)

	mgr, err := manager.New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := mgr.Shutdown(shutdownCtx); serr != nil {
			t.Logf("manager shutdown: %v", serr)
		}
	})

	// Create test table using the default schema.
	db := mgr.DBTX(context.Background())
	_, err = db.ExecContext(context.Background(),
		"CREATE TABLE dbo.items (id INT IDENTITY(1,1) PRIMARY KEY, value NVARCHAR(255) NOT NULL)",
	)
	require.NoError(t, err)

	return mgr, cfg
}

func TestMSSQL_Ping(t *testing.T) {
	mgr, _ := setupMSSQL(t)
	ctx := context.Background()
	require.NoError(t, mgr.Ping(ctx))
}

func TestMSSQL_Driver(t *testing.T) {
	mgr, _ := setupMSSQL(t)
	require.Equal(t, database.DriverMSSQL, mgr.Driver())
}

func TestMSSQL_DefaultSchema_StoredInConfig(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		Host:          "localhost",
		User:          "sa",
		Password:      mssqlPassword,
		Database:      "master",
		DefaultSchema: "dbo",
	}
	require.Equal(t, "dbo", cfg.DefaultSchema)
}

func TestMSSQL_UoW_CommitPersists(t *testing.T) {
	mgr, _ := setupMSSQL(t)
	ctx := context.Background()

	u := uow.New[int64](mgr)

	rowsAffected, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (int64, error) {
		res, execErr := tx.ExecContext(ctx,
			"INSERT INTO dbo.items (value) VALUES (@p1)",
			"hello",
		)
		if execErr != nil {
			return 0, execErr
		}
		return res.RowsAffected()
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dbo.items WHERE value = @p1",
		"hello",
	)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 1, count)
}

func TestMSSQL_UoW_RollbackOnError(t *testing.T) {
	mgr, _ := setupMSSQL(t)
	ctx := context.Background()

	u := uow.New[struct{}](mgr)

	_, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		if _, execErr := tx.ExecContext(ctx,
			"INSERT INTO dbo.items (value) VALUES (@p1)",
			"will-rollback",
		); execErr != nil {
			return struct{}{}, execErr
		}
		return struct{}{}, errIntentional
	})

	require.ErrorIs(t, err, errIntentional)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dbo.items WHERE value = @p1",
		"will-rollback",
	)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count)
}

func TestMSSQL_DefaultSchema_DoesNotMutatePrincipalAndQualifiedQueriesStillWork(t *testing.T) {
	adminMgr, cfg := setupMSSQL(t)
	ctx := context.Background()
	adminDB := adminMgr.DBTX(ctx)

	const (
		appLogin  = "schema_app_login"
		appUser   = "schema_app_user"
		appSchema = "testschema"
	)

	_, err := adminDB.ExecContext(ctx,
		"CREATE LOGIN schema_app_login WITH PASSWORD = 'Test@Pass123!', CHECK_POLICY = OFF",
	)
	require.NoError(t, err)

	_, err = adminDB.ExecContext(ctx, "CREATE USER schema_app_user FOR LOGIN schema_app_login")
	require.NoError(t, err)

	_, err = adminDB.ExecContext(ctx, "CREATE SCHEMA testschema AUTHORIZATION schema_app_user")
	require.NoError(t, err)

	_, err = adminDB.ExecContext(ctx,
		"CREATE TABLE testschema.items (id INT IDENTITY(1,1) PRIMARY KEY, label NVARCHAR(255) NOT NULL)",
	)
	require.NoError(t, err)

	_, err = adminDB.ExecContext(ctx, "GRANT INSERT, SELECT ON testschema.items TO schema_app_user")
	require.NoError(t, err)

	var beforeSchema sql.NullString
	beforeRow := adminDB.QueryRowContext(ctx,
		"SELECT default_schema_name FROM sys.database_principals WHERE name = @p1",
		appUser,
	)
	require.NoError(t, beforeRow.Scan(&beforeSchema))

	cfg.User = appLogin
	cfg.DefaultSchema = appSchema
	mgr, err := manager.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := mgr.Shutdown(shutdownCtx); serr != nil {
			t.Logf("manager shutdown: %v", serr)
		}
	})

	var afterSchema sql.NullString
	afterRow := adminDB.QueryRowContext(ctx,
		"SELECT default_schema_name FROM sys.database_principals WHERE name = @p1",
		appUser,
	)
	require.NoError(t, afterRow.Scan(&afterSchema))
	require.Equal(t, beforeSchema, afterSchema, "manager.New não pode mutar default_schema_name do principal")

	u := uow.New[int64](mgr)

	rowsAffected, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (int64, error) {
		res, execErr := tx.ExecContext(ctx, "INSERT INTO testschema.items (label) VALUES (@p1)", "schema-value")
		if execErr != nil {
			return 0, execErr
		}
		return res.RowsAffected()
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected)

	var schemaCount int
	row := adminMgr.DBTX(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM testschema.items WHERE label = @p1",
		"schema-value",
	)
	require.NoError(t, row.Scan(&schemaCount))
	require.Equal(t, 1, schemaCount)

	var dboCount int
	row = adminMgr.DBTX(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dbo.items WHERE value = @p1",
		"schema-value",
	)
	require.NoError(t, row.Scan(&dboCount))
	require.Equal(t, 0, dboCount)
}

var errIntentional = errors.New("intentional error")
