package manager

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

// Limites de referência (estabelecidos em 2026-04-28, Apple M1 Pro, -benchtime=3s):
//
//	BenchmarkDBTX_PoolPath        medido ~14 ns/op,  0 allocs/op → limite 500 ns/op
//	BenchmarkDBTX_TxInCtxPath     medido ~5 ns/op,   0 allocs/op → limite 200 ns/op
//	BenchmarkShutdown_Immediate   medido ~707 ns/op, 7 allocs/op → limite 15000 ns/op
//
// CI gate: make bench-check impõe estes como limites absolutos.

var benchDBTX database.DBTX

// BenchmarkDBTX_PoolPath mede DBTX(ctx) quando nenhuma transação está no contexto
// (o caminho comum de leitura/escrita que atinge o pool diretamente).
func BenchmarkDBTX_PoolPath(b *testing.B) {
	pool := &stubDBTX{}
	m := newTestManager(&mockAdapter{dbtx: pool})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchDBTX = m.DBTX(ctx)
	}
}

// BenchmarkDBTX_TxInCtxPath mede DBTX(ctx) quando uma transação ativa é
// propagada no ctx (propagação implícita ADR-004, o caminho quente transacional).
func BenchmarkDBTX_TxInCtxPath(b *testing.B) {
	tx := &stubDBTX{}
	pool := &stubDBTX{}
	m := newTestManager(&mockAdapter{dbtx: pool})
	ctx := database.WithTx(context.Background(), tx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchDBTX = m.DBTX(ctx)
	}
}

// BenchmarkShutdown_Immediate mede o overhead de uma chamada Shutdown quando o
// adaptador fecha sem atraso. Exercita o caminho sync.Once + RWMutex.
func BenchmarkShutdown_Immediate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := newTestManager(&mockAdapter{dbtx: &stubDBTX{}})
		_ = m.Shutdown(context.Background())
	}
}
