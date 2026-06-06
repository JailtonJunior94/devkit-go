package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/worker/consumer"
	"github.com/JailtonJunior94/devkit-go/pkg/worker/consumer/database"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	startErr error
	stopErr  error
}

func (m *mockRunner) Start(_ context.Context) error { return m.startErr }
func (m *mockRunner) Stop(_ context.Context) error  { return m.stopErr }

var _ consumer.Runner = (*mockRunner)(nil)

func TestDatabaseAdapter_TechnologyIsDatabase(t *testing.T) {
	a := database.NewAdapter("test", &mockRunner{})
	require.Equal(t, "database", a.Technology())
}

func TestDatabaseAdapter_DelegatesLifecycle(t *testing.T) {
	stopErr := errors.New("db stop error")
	r := &mockRunner{stopErr: stopErr}
	a := database.NewAdapter("test", r)

	require.NoError(t, a.Start(context.Background()))
	require.ErrorIs(t, a.Stop(context.Background()), stopErr)
}

func TestDatabaseAdapter_Name(t *testing.T) {
	a := database.NewAdapter("my-db-consumer", &mockRunner{})
	require.Equal(t, "my-db-consumer", a.Name())
}
