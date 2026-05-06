package manager

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// driverAdapter é o contrato interno satisfeito por cada adaptador de driver.
// Não faz parte da API pública; apenas o pkg/database/manager o consome.
type driverAdapter interface {
	Driver() database.Driver
	DBTX() database.DBTX
	BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error)
	Stats() internalpool.Stats
	Attributes() []observability.Field
	Ping(ctx context.Context) error
	// Close interrompe goroutines de fundo (ex: coletor de estatísticas) e libera o pool.
	// As implementações devem interromper todas as goroutines antes de retornar.
	Close(ctx context.Context) error
}
