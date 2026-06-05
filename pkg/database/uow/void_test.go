package uow_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
)

func TestNewVoid_CommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	u := uow.NewVoid(&fakeManager{tx: tx})

	result, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (struct{}, error) {
		return struct{}{}, nil
	})

	require.NoError(t, err)
	require.Equal(t, struct{}{}, result)
	require.True(t, tx.committed)
}

func TestNewVoid_RollsBackOnError(t *testing.T) {
	tx := &fakeTx{}
	u := uow.NewVoid(&fakeManager{tx: tx})

	_, err := u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (struct{}, error) {
		return struct{}{}, errSentinel
	})

	require.ErrorIs(t, err, errSentinel)
	require.True(t, tx.rolledBack)
	require.False(t, tx.committed)
}

var errSentinel = errNew("sentinel error")

type sentinelError string

func errNew(s string) error           { return sentinelError(s) }
func (e sentinelError) Error() string { return string(e) }
