package database

import "context"

type txContextKey struct{}

func WithTx(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func FromContext(ctx context.Context) (DBTX, bool) {
	tx, ok := ctx.Value(txContextKey{}).(DBTX)
	return tx, ok
}
