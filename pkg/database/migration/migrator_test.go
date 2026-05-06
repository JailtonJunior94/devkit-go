package migration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	migratelib "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	migratedatabase "github.com/golang-migrate/migrate/v4/database"
)

// fakeManager é um stub mínimo de Manager para testes unitários.
type fakeManager struct {
	driver database.Driver
}

func (f *fakeManager) Driver() database.Driver              { return f.driver }
func (f *fakeManager) DBTX(_ context.Context) database.DBTX { return nil }
func (f *fakeManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (f *fakeManager) Ping(_ context.Context) error     { return nil }
func (f *fakeManager) Shutdown(_ context.Context) error { return nil }

var _ manager.Manager = (*fakeManager)(nil)

// TestNew_MissingDSN verifica se o New retorna ErrInvalidConfig quando WithDSN está ausente.
func TestNew_MissingDSN(t *testing.T) {
	tmpDir := t.TempDir()
	addMigrationPair(t, tmpDir, 1, "create_users")

	_, err := New(&fakeManager{driver: database.DriverPostgres}, FSPath(tmpDir))

	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
	require.Contains(t, err.Error(), "WithDSN")
}

// TestNew_EmptyDSN verifica se um DSN explicitamente vazio também é rejeitado.
func TestNew_EmptyDSN(t *testing.T) {
	tmpDir := t.TempDir()
	addMigrationPair(t, tmpDir, 1, "create_users")

	_, err := New(&fakeManager{driver: database.DriverPostgres}, FSPath(tmpDir), WithDSN(""))

	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
}

// TestNew_InvalidSource verifica se um Source não suportado retorna um erro antes
// de tentar uma conexão com o banco de dados.
func TestNew_InvalidSource(t *testing.T) {
	_, err := New(
		&fakeManager{driver: database.DriverPostgres},
		unsupportedSource{},
		WithDSN("pgx5://user:pass@localhost/db"),
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported source type")
}

// TestNormalizeDSN_Schemes verifica se as URLs postgres/postgresql são reescritas para pgx5.
func TestNormalizeDSN_Schemes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"esquema postgres", "postgres://user:pass@host/db", "pgx5://user:pass@host/db"},
		{"esquema postgresql", "postgresql://user:pass@host/db", "pgx5://user:pass@host/db"},
		{"esquema pgx5", "pgx5://user:pass@host/db", "pgx5://user:pass@host/db"},
		{"outro esquema", "mysql://user:pass@host/db", "mysql://user:pass@host/db"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.out, normalizeDSN(tc.in))
		})
	}
}

// TestWithMigrationTimeout_Option verifica se a opção é aplicada.
func TestWithMigrationTimeout_Option(t *testing.T) {
	o := defaultOptions()
	WithMigrationTimeout(5 * time.Second)(&o)
	require.Equal(t, 5*time.Second, o.timeout)
}

// TestWithMigrationTimeout_Zero_Ignored verifica se o timeout zero não é aplicado.
func TestWithMigrationTimeout_Zero_Ignored(t *testing.T) {
	o := defaultOptions()
	WithMigrationTimeout(0)(&o)
	require.Zero(t, o.timeout)
}

// TestWithObservability_Option verifica se a opção é aplicada.
func TestWithObservability_Option(t *testing.T) {
	fp := fake.NewProvider()
	o := defaultOptions()
	WithObservability(fp)(&o)
	require.Equal(t, fp, o.observability)
}

// TestWithObservability_Nil_Ignored verifica se um provedor nil não é aplicado.
func TestWithObservability_Nil_Ignored(t *testing.T) {
	o := defaultOptions()
	original := o.observability
	WithObservability(nil)(&o)
	require.Equal(t, original, o.observability)
}

// TestWithDSN_Option define o campo DSN.
func TestWithDSN_Option(t *testing.T) {
	o := defaultOptions()
	WithDSN("pgx5://user:pass@host/db")(&o)
	require.Equal(t, "pgx5://user:pass@host/db", o.dsn)
}

// TestMapError_ErrNoChange verifica o mapeamento da sentinela (perspectiva cross-package).
func TestMapError_ErrNoChange_Sentinel(t *testing.T) {
	err := mapError(migratelib.ErrNoChange)
	require.ErrorIs(t, err, ErrNoChange)
	require.ErrorIs(t, err, migratelib.ErrNoChange)
}

// TestWithTimeout_Applied verifica se o timeout por operação envolve o contexto.
func TestWithTimeout_Applied(t *testing.T) {
	m := &migrator{opts: options{timeout: 10 * time.Millisecond}}
	ctx, cancel := m.withTimeout(context.Background())
	defer cancel()

	dl, ok := ctx.Deadline()
	require.True(t, ok)
	require.False(t, dl.IsZero())
}

// TestWithTimeout_Zero_NoDeadline verifica que não há deadline quando o timeout é zero.
func TestWithTimeout_Zero_NoDeadline(t *testing.T) {
	m := &migrator{opts: options{timeout: 0}}
	ctx, cancel := m.withTimeout(context.Background())
	defer cancel()

	_, ok := ctx.Deadline()
	require.False(t, ok)
}

func TestRun_ContextCancelled_ReturnsMigrationFailed(t *testing.T) {
	m := &migrator{
		open: func() (*migratelib.Migrate, error) {
			return migratelib.NewWithInstance("stub", &stubSourceDriver{}, "stub", &stubDatabaseDriver{})
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.run(ctx, func(*migratelib.Migrate) error {
		t.Fatal("run must not execute when context is already cancelled")
		return nil
	})

	require.ErrorIs(t, err, database.ErrMigrationFailed)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRun_ContextDeadlineExceeded_ReturnsBeforeOperationFinishes(t *testing.T) {
	m := &migrator{
		open: func() (*migratelib.Migrate, error) {
			return migratelib.NewWithInstance("stub", &stubSourceDriver{}, "stub", &stubDatabaseDriver{})
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := m.run(ctx, func(*migratelib.Migrate) error {
		<-ctx.Done()
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)

	require.ErrorIs(t, err, database.ErrMigrationFailed)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 150*time.Millisecond)
}

func TestRegisteredDatabaseDrivers_IncludeSupportedDrivers(t *testing.T) {
	drivers := migratedatabase.List()
	require.Contains(t, drivers, "pgx5")
	require.Contains(t, drivers, "mysql")
	require.Contains(t, drivers, "sqlserver")
}

// helpers

func addMigrationPair(t *testing.T, dir string, seq int, name string) {
	t.Helper()
	up := filepath.Join(dir, padSeq(seq)+"_"+name+".up.sql")
	down := filepath.Join(dir, padSeq(seq)+"_"+name+".down.sql")
	require.NoError(t, os.WriteFile(up, []byte("CREATE TABLE "+name+" (id BIGINT);"), 0o600))
	require.NoError(t, os.WriteFile(down, []byte("DROP TABLE "+name+";"), 0o600))
}

func padSeq(n int) string {
	return fmt.Sprintf("%06d", n)
}

type stubSourceDriver struct{}

func (*stubSourceDriver) Open(string) (source.Driver, error) { return &stubSourceDriver{}, nil }
func (*stubSourceDriver) Close() error                       { return nil }
func (*stubSourceDriver) First() (uint, error) {
	return 0, os.ErrNotExist
}
func (*stubSourceDriver) Prev(uint) (uint, error) {
	return 0, os.ErrNotExist
}
func (*stubSourceDriver) Next(uint) (uint, error) {
	return 0, os.ErrNotExist
}
func (*stubSourceDriver) ReadUp(uint) (io.ReadCloser, string, error) {
	return nil, "", os.ErrNotExist
}
func (*stubSourceDriver) ReadDown(uint) (io.ReadCloser, string, error) {
	return nil, "", os.ErrNotExist
}

type stubDatabaseDriver struct{}

func (*stubDatabaseDriver) Open(string) (migratedatabase.Driver, error) {
	return &stubDatabaseDriver{}, nil
}
func (*stubDatabaseDriver) Close() error                { return nil }
func (*stubDatabaseDriver) Lock() error                 { return nil }
func (*stubDatabaseDriver) Unlock() error               { return nil }
func (*stubDatabaseDriver) Run(io.Reader) error         { return nil }
func (*stubDatabaseDriver) SetVersion(int, bool) error  { return nil }
func (*stubDatabaseDriver) Version() (int, bool, error) { return 0, false, nil }
func (*stubDatabaseDriver) Drop() error                 { return nil }
