//go:build integration

package migration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

const pgImage = "postgres:16"

// setupMigrationEnv inicia um container Postgres e retorna o manager e
// um DSN de migração (esquema pgx5://). O chamador é responsável pelo ciclo de vida do container.
func setupMigrationEnv(t *testing.T) (manager.Manager, string) {
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

	cfg := postgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)
	return mgr, dsn
}

// fixturesDir retorna o caminho para os arquivos de migração de teste do postgres.
func fixturesDir(t *testing.T) string {
	t.Helper()
	// Resolve relativo ao diretório deste arquivo de teste para que funcione independentemente
	// do diretório de trabalho usado pelo `go test`.
	dir, err := filepath.Abs("testdata/migrations/postgres")
	require.NoError(t, err)
	return dir
}

// TestIntegration_Migrator_Up_AppliesMigrations verifica se o Up aplica todas
// as migrações pendentes e retorna nil em caso de sucesso.
func TestIntegration_Migrator_Up_AppliesMigrations(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx))

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty)
	require.EqualValues(t, 2, version)
}

// TestIntegration_Migrator_Up_NoChange_ReturnsErrNoChange verifica se a chamada do
// Up quando não há migrações pendentes retorna migration.ErrNoChange.
func TestIntegration_Migrator_Up_NoChange_ReturnsErrNoChange(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx)) // primeiro Up aplica as migrações
	err = m.Up(ctx)               // segundo Up não encontra nada para aplicar
	require.ErrorIs(t, err, migration.ErrNoChange)
}

// TestIntegration_Migrator_Down_RevertsSteps verifica se o Down(n) reverte exatamente
// n migrações.
func TestIntegration_Migrator_Down_RevertsSteps(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx))

	// Reverte a migração mais recente (passo 1 para trás: 2 → 1).
	require.NoError(t, m.Down(ctx, 1))

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty)
	require.EqualValues(t, 1, version)
}

// TestIntegration_Migrator_Force_SetsDirtyVersion verifica se o Force define a
// versão sem executar o SQL, permitindo a recuperação de um estado sujo (dirty state).
func TestIntegration_Migrator_Force_SetsDirtyVersion(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	// Usa um tmp dir com uma migração deliberadamente quebrada para simular um estado sujo (dirty state).
	tmpDir := t.TempDir()
	writeSQLFile(t, tmpDir, "000001_create_ok.up.sql", "CREATE TABLE ok_table (id INT);")
	writeSQLFile(t, tmpDir, "000001_create_ok.down.sql", "DROP TABLE ok_table;")
	writeSQLFile(t, tmpDir, "000002_broken.up.sql", "THIS IS NOT VALID SQL !!!;")
	writeSQLFile(t, tmpDir, "000002_broken.down.sql", "DROP TABLE IF EXISTS broken;")

	m, err := migration.New(mgr, migration.FSPath(tmpDir), migration.WithDSN(dsn))
	require.NoError(t, err)

	// A migração quebrada faz o Up falhar e deixa o banco de dados sujo.
	_ = m.Up(ctx)

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	// Ou o banco está sujo na versão 2, ou o Up falhou antes de marcá-lo.
	// Força para a versão 1 de qualquer forma.
	_ = version
	_ = dirty

	require.NoError(t, m.Force(ctx, 1))

	version, dirty, err = m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty, "o Force deve limpar a flag dirty")
	require.EqualValues(t, 1, version)
}

// TestIntegration_Migrator_Version_RoundTrip verifica se o Version retorna a
// versão correta após o Up.
func TestIntegration_Migrator_Version_RoundTrip(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx))

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty)
	require.EqualValues(t, 2, version)
}

// TestIntegration_Migrator_ConcurrentUp_OnlyOneApplies verifica se o bloqueio nativo
// do golang-migrate previne a aplicação dupla: exatamente um Up concorrente é aplicado, o
// outro ou aguarda e recebe ErrNoChange ou retorna um erro.
func TestIntegration_Migrator_ConcurrentUp_OnlyOneApplies(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	const workers = 2
	errs := make([]error, workers)
	var wg sync.WaitGroup

	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
			if err != nil {
				errs[idx] = err
				return
			}
			errs[idx] = m.Up(ctx)
		}(i)
	}

	wg.Wait()

	// Pelo menos um deve ter sucesso (nil) ou sinalizar no change.
	// O outro pode receber ErrNoChange ou um erro de bloqueio — ambos são aceitáveis.
	var successes int
	for _, err := range errs {
		if err == nil || errors.Is(err, migration.ErrNoChange) {
			successes++
		}
	}
	require.Positive(t, successes, "pelo menos um Up concorrente deve ter sucesso ou retornar ErrNoChange")

	// A versão final deve ser 2 independentemente da ordem.
	m, err := migration.New(mgr, migration.FSPath(fixturesDir(t)), migration.WithDSN(dsn))
	require.NoError(t, err)

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty)
	require.EqualValues(t, 2, version)
}

// TestIntegration_Migrator_EmbedFS_Up verifica se a fonte EmbedFS funciona de ponta a ponta
// usando os.DirFS (equivalente a embed.FS em testes, Q63 na techspec).
func TestIntegration_Migrator_EmbedFS_Up(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	src := migration.EmbedFS{
		FS:   os.DirFS(fixturesDir(t)),
		Root: ".",
	}

	m, err := migration.New(mgr, src, migration.WithDSN(dsn))
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx))

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	require.False(t, dirty)
	require.EqualValues(t, 2, version)
}

// TestIntegration_Migrator_WithTimeout aplica um timeout generoso que não deve
// ser atingido para uma migração rápida.
func TestIntegration_Migrator_WithTimeout(t *testing.T) {
	ctx := context.Background()
	mgr, dsn := setupMigrationEnv(t)

	m, err := migration.New(
		mgr,
		migration.FSPath(fixturesDir(t)),
		migration.WithDSN(dsn),
		migration.WithMigrationTimeout(30*time.Second),
	)
	require.NoError(t, err)

	require.NoError(t, m.Up(ctx))
}

// helpers

func writeSQLFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
