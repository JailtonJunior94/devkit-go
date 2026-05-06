package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
)

// Limites de referência (estabelecidos em 2026-04-28, Apple M1 Pro, -benchtime=3s):
//
//	BenchmarkUoW_Do_Commit        medido ~48 ns/op,  1 alloc/op  → limite 1500 ns/op
//	BenchmarkUoW_Do_Rollback      medido ~67 ns/op,  2 allocs/op → limite 1500 ns/op
//	BenchmarkUoW_Do_WithIsolation medido ~48 ns/op,  1 alloc/op  → limite 1500 ns/op
//	BenchmarkUoW_Do_Concurrent    medido ~25 ns/op,  1 alloc/op  → limite 500 ns/op
//
// CI gate: make bench-check impõe estes como limites absolutos.

// benchTx é uma transação fake com overhead zero para benchmarks.
// Todos os métodos são no-ops; sem travas, sem canais, sem alocações.
type benchTx struct{}

func (t *benchTx) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}
func (t *benchTx) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}
func (t *benchTx) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row { return nil }
func (t *benchTx) Commit(_ context.Context) error                                     { return nil }
func (t *benchTx) Rollback(_ context.Context) error                                   { return nil }

// benchManager é um manager fake com overhead zero que sempre retorna a mesma tx.
type benchManager struct{ tx database.Tx }

func (m *benchManager) Driver() database.Driver { return database.DriverPostgres }
func (m *benchManager) DBTX(_ context.Context) database.DBTX {
	return m.tx
}
func (m *benchManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return m.tx, nil
}
func (m *benchManager) Ping(_ context.Context) error    { return nil }
func (m *benchManager) Shutdown(_ context.Context) error { return nil }

var benchResult string

// BenchmarkUoW_Do_Commit mede o overhead de uma chamada Do bem-sucedida
// (caminho BeginTx → fn → Commit) sem round-trips reais no banco de dados.
func BenchmarkUoW_Do_Commit(b *testing.B) {
	mgr := &benchManager{tx: &benchTx{}}
	u := uow.New[string](mgr)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
			return "ok", nil
		})
		if err != nil {
			b.Fatal(err)
		}
		benchResult = res
	}
}

// BenchmarkUoW_Do_Rollback mede o caminho de rollback (fn retorna um erro).
func BenchmarkUoW_Do_Rollback(b *testing.B) {
	mgr := &benchManager{tx: &benchTx{}}
	u := uow.New[string](mgr)
	ctx := context.Background()
	fnErr := errors.New("fn error")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
			return "", fnErr
		})
	}
}

// BenchmarkUoW_Do_WithIsolation mede o overhead introduzido ao definir um
// nível de isolamento não padrão via opção funcional.
func BenchmarkUoW_Do_WithIsolation(b *testing.B) {
	mgr := &benchManager{tx: &benchTx{}}
	u := uow.New[string](mgr, uow.WithIsolation(sql.LevelSerializable))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
			return "ok", nil
		})
	}
}

// BenchmarkUoW_Do_Concurrent mede o throughput quando múltiplas goroutines têm,
// cada uma, sua própria instância de UoW executando o Do concorrentemente (sem estado compartilhado).
func BenchmarkUoW_Do_Concurrent(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		mgr := &benchManager{tx: &benchTx{}}
		u := uow.New[string](mgr)
		for pb.Next() {
			_, _ = u.Do(ctx, func(_ context.Context, _ database.DBTX) (string, error) {
				return "ok", nil
			})
		}
	})
}
