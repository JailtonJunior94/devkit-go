package uow_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/require"
)

func TestWithIsolation_SetsLevel(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr, uow.WithIsolation(sql.LevelRepeatableRead))

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelRepeatableRead, mgr.lastOpts.Isolation)
}

func TestWithIsolation_DefaultIsLevelDefault(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr)

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelDefault, mgr.lastOpts.Isolation)
}

func TestWithReadOnly_SetsTrueFlag(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr, uow.WithReadOnly(true))

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.True(t, mgr.lastOpts.ReadOnly)
}

func TestWithReadOnly_DefaultIsFalse(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr)

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.False(t, mgr.lastOpts.ReadOnly)
}

func TestMultipleOptions_AllApplied(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr,
		uow.WithIsolation(sql.LevelSerializable),
		uow.WithReadOnly(true),
	)

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelSerializable, mgr.lastOpts.Isolation)
	require.True(t, mgr.lastOpts.ReadOnly)
}

func TestDo_CallOptionsOverrideConstructorDefaultsWithoutMutatingThem(t *testing.T) {
	tx := &fakeTx{}
	mgr := &fakeManager{tx: tx}
	u := uow.New[string](mgr, uow.WithIsolation(sql.LevelReadCommitted))

	_, _ = u.Do(
		context.Background(),
		func(_ context.Context, _ database.DBTX) (string, error) {
			return "", nil
		},
		uow.WithIsolation(sql.LevelSerializable),
		uow.WithReadOnly(true),
	)

	require.Equal(t, database.LevelSerializable, mgr.lastOpts.Isolation)
	require.True(t, mgr.lastOpts.ReadOnly)

	_, _ = u.Do(context.Background(), func(_ context.Context, _ database.DBTX) (string, error) {
		return "", nil
	})

	require.Equal(t, database.LevelReadCommitted, mgr.lastOpts.Isolation)
	require.False(t, mgr.lastOpts.ReadOnly)
}
