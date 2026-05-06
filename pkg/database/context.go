package database

import "context"

type txContextKey struct{}

// WithTx injeta a tx no ctx para propagação implícita de transação.
func WithTx(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// FromContext retorna a transação ativa propagada via ctx, se houver.
func FromContext(ctx context.Context) (DBTX, bool) {
	tx, ok := ctx.Value(txContextKey{}).(DBTX)
	return tx, ok
}
