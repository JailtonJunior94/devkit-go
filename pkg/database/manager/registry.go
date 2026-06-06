package manager

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type DriverAdapter interface {
	Driver() database.Driver
	DBTX() database.DBTX
	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
	Stats() internalpool.Stats
	Attributes() []observability.Field
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
}

type AdapterFactory func(cfg DriverConfig, obs observability.Observability) (DriverAdapter, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]AdapterFactory{}
)

func RegisterDriverFactory(cfg DriverConfig, factory AdapterFactory) {
	if cfg == nil || factory == nil {
		return
	}
	key := typeKey(cfg)
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[key] = factory
}

func lookupDriverFactory(cfg DriverConfig) (AdapterFactory, bool) {
	key := typeKey(cfg)
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[key]
	return f, ok
}

func typeKey(cfg DriverConfig) string {
	t := reflect.TypeOf(cfg)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath() + "." + t.Name()
}

func unsupportedDriverError(cfg DriverConfig) error {
	return fmt.Errorf("%w: unsupported driver config type %T", database.ErrInvalidConfig, cfg)
}
