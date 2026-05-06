package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/require"
)

// stubDBTX implementa database.DBTX para testes.
type stubDBTX struct{}

func (s *stubDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *stubDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *stubDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}

func TestWithTx_FromContext_RoundTrip(t *testing.T) {
	tx := &stubDBTX{}
	ctx := database.WithTx(context.Background(), tx)

	got, ok := database.FromContext(ctx)

	require.True(t, ok)
	require.Equal(t, tx, got)
}

func TestFromContext_EmptyContext_ReturnsFalse(t *testing.T) {
	_, ok := database.FromContext(context.Background())
	require.False(t, ok)
}

func TestWithTx_OverwritesPreviousValue(t *testing.T) {
	tx1 := &stubDBTX{}
	tx2 := &stubDBTX{}

	ctx := database.WithTx(context.Background(), tx1)
	ctx = database.WithTx(ctx, tx2)

	got, ok := database.FromContext(ctx)

	require.True(t, ok)
	require.Equal(t, tx2, got)
}

func TestFromContext_NilContext_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "empty context returns false",
			ctx:  context.Background(),
			want: false,
		},
		{
			name: "context with unrelated value returns false",
			ctx:  context.WithValue(context.Background(), struct{ name string }{"unrelated"}, "value"),
			want: false,
		},
		{
			name: "context with tx returns true",
			ctx:  database.WithTx(context.Background(), &stubDBTX{}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := database.FromContext(tt.ctx)
			require.Equal(t, tt.want, ok)
		})
	}
}
