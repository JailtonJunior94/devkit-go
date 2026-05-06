package migration

import (
	"errors"
	"fmt"

	migratelib "github.com/golang-migrate/migrate/v4"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

// ErrNoChange é retornado pelo Up quando não existem migrações pendentes.
// Envolve o migrate.ErrNoChange para fornecer uma sentinela local do pacote para os chamadores.
var ErrNoChange = errors.New("migration: no change")

// mapError converte erros do golang-migrate para sentinelas do pacote.
// migrate.ErrNoChange → ErrNoChange; qualquer outro erro → envolto em ErrMigrationFailed.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: %w", ErrNoChange, migratelib.ErrNoChange)
	}
	return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
}
