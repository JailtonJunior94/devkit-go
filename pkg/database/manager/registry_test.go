package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/require"
)

type customConfig struct {
	failValidate bool
}

func (c customConfig) Validate() error {
	if c.failValidate {
		return errors.New("custom: invalid")
	}
	return nil
}

func TestRegisterDriverFactory_RouteToCustomFactory(t *testing.T) {
	called := false
	RegisterDriverFactory(customConfig{}, func(_ DriverConfig, _ observability.Observability) (DriverAdapter, error) {
		called = true
		return &mockAdapter{driver: database.Driver("custom"), dbtx: &stubDBTX{}}, nil
	})
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, typeKey(customConfig{}))
		registryMu.Unlock()
	})

	originalRunStartupMigrationsFunc := runStartupMigrationsFunc
	runStartupMigrationsFunc = func(_ DriverConfig, _ database.Driver, _ options) error { return nil }
	t.Cleanup(func() { runStartupMigrationsFunc = originalRunStartupMigrationsFunc })

	mgr, err := New(customConfig{})
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.True(t, called, "factory registrada deve ser invocada por buildAdapter")
	require.Equal(t, database.Driver("custom"), mgr.Driver())
	require.NoError(t, mgr.Shutdown(context.Background()))
}

func TestRegisterDriverFactory_NilArgsAreIgnored(t *testing.T) {
	before := len(registry)
	RegisterDriverFactory(nil, nil)
	require.Equal(t, before, len(registry), "RegisterDriverFactory com args nil deve ser no-op")
}

func TestNewFromAdapter_BuildsManagerSkippingRegistry(t *testing.T) {
	adapter := &mockAdapter{driver: database.DriverPostgres, dbtx: &stubDBTX{}}
	mgr, err := NewFromAdapter(adapter)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.Equal(t, database.DriverPostgres, mgr.Driver())
	require.NoError(t, mgr.Shutdown(context.Background()))
	require.Equal(t, 1, adapter.closeCalls)
}

func TestNewFromAdapter_NilAdapterReturnsError(t *testing.T) {
	mgr, err := NewFromAdapter(nil)
	require.Error(t, err)
	require.Nil(t, mgr)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
}
