package migration

import (
	"errors"
	"fmt"

	migratelib "github.com/golang-migrate/migrate/v4"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

var ErrNoChange = errors.New("migration: no change")

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: %w", ErrNoChange, migratelib.ErrNoChange)
	}
	return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
}
