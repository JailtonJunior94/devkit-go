package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/migration"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Criar logger estruturado (output no console)
	logger := migration.NewSlogTextLogger(slog.LevelInfo)

	// Configurar migrator
	migrator, err := migration.New(
		migration.WithDriver(migration.DriverPostgres),
		migration.WithDSN("postgres://user:pass@localhost:5432/mydb?sslmode=disable"),
		migration.WithSource("file://migrations"),
		migration.WithLogger(logger),
		migration.WithTimeout(5*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		if err := migrator.Close(); err != nil {
			log.Printf("Warning: failed to close migrator: %v", err)
		}
	}()

	// Executar migrations UP
	log.Println("Running migrations...")
	if err := migrator.Up(ctx); err != nil {
		if migration.IsNoChangeError(err) {
			log.Println("No migrations to apply - database is up to date")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	// Verificar versão atual
	version, dirty, err := migrator.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	log.Printf("Migration completed successfully! Current version: %d (dirty: %v)", version, dirty)
	return nil
}
