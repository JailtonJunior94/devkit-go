package manager

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type driverAdapter interface {
	Driver() database.Driver
	DBTX() database.DBTX
	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
	Stats() internalpool.Stats
	Attributes() []observability.Field
	Ping(ctx context.Context) error

	Close(ctx context.Context) error
}
