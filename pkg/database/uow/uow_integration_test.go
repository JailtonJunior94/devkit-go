//go:build integration

package uow_test

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const pgImage = "postgres:16"

// setupPostgres inicia um container Postgres e retorna um Manager conectado.
// A tabela `items (id serial primary key, value text)` é criada antes de retornar.
func setupPostgres(t *testing.T) manager.Manager {
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
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
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

	// Provisiona a tabela de teste.
	dbtx := mgr.DBTX(ctx)
	_, err = dbtx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS items (id SERIAL PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)

	return mgr
}

// TestIntegration_UoW_CommitPersistsRow verifica que um Do bem-sucedido confirma
// a linha inserida e a torna visível fora da transação.
func TestIntegration_UoW_CommitPersistsRow(t *testing.T) {
	ctx := context.Background()
	mgr := setupPostgres(t)
	u := uow.New[int64](mgr)

	rowID, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (int64, error) {
		var id int64
		row := tx.QueryRowContext(ctx, `INSERT INTO items (value) VALUES ($1) RETURNING id`, "committed")
		if scanErr := row.Scan(&id); scanErr != nil {
			return 0, scanErr
		}
		return id, nil
	})

	require.NoError(t, err)
	require.Positive(t, rowID)

	// Verifica se a linha está visível fora da transação.
	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE id = $1`, rowID)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 1, count, "linha confirmada deve estar visível após o retorno de Do")
}

// TestIntegration_UoW_RollbackDiscardsRow verifica que um erro retornado pela fn
// causa rollback e a linha não aparece no banco de dados.
func TestIntegration_UoW_RollbackDiscardsRow(t *testing.T) {
	ctx := context.Background()
	mgr := setupPostgres(t)
	u := uow.New[struct{}](mgr)

	_, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		_, execErr := tx.ExecContext(ctx, `INSERT INTO items (value) VALUES ($1)`, "to-be-rolled-back")
		if execErr != nil {
			return struct{}{}, execErr
		}
		return struct{}{}, fmt.Errorf("intentional error to trigger rollback")
	})

	require.Error(t, err)

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE value = $1`, "to-be-rolled-back")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count, "linha revertida não deve estar visível")
}

// TestIntegration_UoW_PanicRollsBack verifica que um pânico dentro da fn causa
// rollback (a linha não é persistida) e o pânico é re-propagado.
func TestIntegration_UoW_PanicRollsBack(t *testing.T) {
	ctx := context.Background()
	mgr := setupPostgres(t)
	u := uow.New[struct{}](mgr)

	recovered := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()
		_, _ = u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
			_, _ = tx.ExecContext(ctx, `INSERT INTO items (value) VALUES ($1)`, "panic-row")
			panic("simulated panic")
		})
	}()

	require.True(t, recovered, "o pânico deve ser re-propagado")

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE value = $1`, "panic-row")
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count, "a linha inserida antes do pânico não deve estar visível após o rollback")
}

// TestIntegration_UoW_SerializableIsolation verifica que o isolamento Serializable
// faz com que uma escrita concorrente conflitante falhe com um erro de serialização.
func TestIntegration_UoW_SerializableIsolation(t *testing.T) {
	ctx := context.Background()
	mgr := setupPostgres(t)

	// Insere uma linha semente que ambas as transações lerão e tentarão atualizar.
	_, err := mgr.DBTX(ctx).ExecContext(ctx, `INSERT INTO items (value) VALUES ('seed')`)
	require.NoError(t, err)

	u1 := uow.New[struct{}](mgr, uow.WithIsolation(sql.LevelSerializable))
	u2 := uow.New[struct{}](mgr, uow.WithIsolation(sql.LevelSerializable))

	// Abre a tx1 e lê a linha semente, mas ainda não faz o commit.
	tx1Ready := make(chan struct{}, 1)
	tx1Commit := make(chan struct{})
	tx1Done := make(chan error, 1)

	go func() {
		_, txErr := u1.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
			// Lê a linha semente para estabelecer um predicado de leitura para serialização.
			rows, qErr := tx.QueryContext(ctx, `SELECT value FROM items WHERE value = 'seed'`)
			if qErr != nil {
				return struct{}{}, qErr
			}
			_ = rows.Close()

			tx1Ready <- struct{}{} // sinaliza que a tx1 leu a linha
			<-tx1Commit    // aguarda o sinal para proceder com o commit

			_, wErr := tx.ExecContext(ctx, `UPDATE items SET value = 'tx1' WHERE value = 'seed'`)
			return struct{}{}, wErr
		})
		tx1Done <- txErr
	}()

	// Aguarda a tx1 abrir e ler, então a tx2 lê e escreve imediatamente.
	<-tx1Ready

	_, tx2Err := u2.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		rows, qErr := tx.QueryContext(ctx, `SELECT value FROM items WHERE value = 'seed'`)
		if qErr != nil {
			return struct{}{}, qErr
		}
		_ = rows.Close()
		_, wErr := tx.ExecContext(ctx, `UPDATE items SET value = 'tx2' WHERE value = 'seed'`)
		return struct{}{}, wErr
	})

	// Sinaliza a tx1 para proceder e fazer o commit; uma das duas deve falhar.
	close(tx1Commit)
	tx1Err := <-tx1Done

	// Pelo menos uma transação deve ter sido abortada pelo verificador de serialização.
	bothSucceeded := tx1Err == nil && tx2Err == nil
	require.False(t, bothSucceeded, "o isolamento serializable deve rejeitar pelo menos uma escrita conflitante")
}

// TestIntegration_UoW_Concurrent executa 100 goroutines que inserem, cada uma, uma linha
// dentro de sua própria UoW. Todas devem ser confirmadas com sucesso e limpas no -race.
func TestIntegration_UoW_Concurrent(t *testing.T) {
	ctx := context.Background()
	mgr := setupPostgres(t)

	const goroutines = 100
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			u := uow.New[struct{}](mgr)
			_, errs[idx] = u.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
				_, err := tx.ExecContext(ctx, `INSERT INTO items (value) VALUES ($1)`, fmt.Sprintf("goroutine-%d", idx))
				return struct{}{}, err
			})
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d failed", i)
	}

	var count int
	row := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE value LIKE 'goroutine-%'`)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, goroutines, count)
}
