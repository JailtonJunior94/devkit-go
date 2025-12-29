package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func main() {
	ctx := context.Background()

	// Example 1: Basic Connection
	basicExample(ctx)

	// Example 2: Using DSN String
	dsnExample(ctx)

	// Example 3: Production Configuration
	productionExample(ctx)

	// Example 4: Health Check
	healthCheckExample(ctx)
}

// basicExample demonstrates a basic database connection
func basicExample(ctx context.Context) {
	fmt.Println("=== Basic Connection Example ===")

	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("testdb"),
	)

	if err := db.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	fmt.Println("✓ Successfully connected to PostgreSQL")

	// Example query
	sqlDB := db.DB()
	var version string
	if err := sqlDB.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		log.Printf("Failed to query version: %v", err)
		return
	}

	fmt.Printf("✓ PostgreSQL version: %s\n\n", version)
}

// dsnExample demonstrates connection using DSN string
func dsnExample(ctx context.Context) {
	fmt.Println("=== DSN Connection Example ===")

	// You can use a complete DSN string
	// This is useful when getting the connection string from environment variables
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=testdb sslmode=disable"

	db := postgres.New(
		postgres.WithDSN(dsn),
	)

	if err := db.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	fmt.Println("✓ Successfully connected using DSN")

	// Verify connection
	if err := db.HealthCheck(ctx); err != nil {
		log.Printf("Health check failed: %v", err)
		return
	}

	fmt.Println("✓ Health check passed\n")
}

// productionExample demonstrates a production-ready configuration
func productionExample(ctx context.Context) {
	fmt.Println("=== Production Configuration Example ===")

	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("app_user"),
		postgres.WithPassword("secure_password"),
		postgres.WithDatabase("production_db"),
		postgres.WithSSLMode("prefer"),
		postgres.WithMaxOpenConns(100),
		postgres.WithMaxIdleConns(20),
		postgres.WithConnMaxLifetime(30*time.Minute),
		postgres.WithConnMaxIdleTime(5*time.Minute),
		postgres.WithMaxRetries(5),
		postgres.WithRetryInterval(3*time.Second),
		postgres.WithConnectTimeout(15*time.Second),
		postgres.WithPingTimeout(5*time.Second),
	)

	if err := db.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	fmt.Println("✓ Connected with production settings")

	// Check pool stats
	sqlDB := db.DB()
	stats := sqlDB.Stats()
	fmt.Printf("✓ Pool Stats - Open: %d, Idle: %d, InUse: %d\n\n",
		stats.OpenConnections, stats.Idle, stats.InUse)
}

// healthCheckExample demonstrates health check functionality
func healthCheckExample(ctx context.Context) {
	fmt.Println("=== Health Check Example ===")

	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("testdb"),
	)

	if err := db.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	// Perform health check
	if err := db.HealthCheck(ctx); err != nil {
		log.Printf("Health check failed: %v", err)
		return
	}

	fmt.Println("✓ Database health check passed")

	// Simulate periodic health checks
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Second)
	checkCount := 0

	fmt.Println("✓ Starting periodic health checks (15s)...")

	for {
		select {
		case <-ticker.C:
			checkCount++
			if err := db.HealthCheck(ctx); err != nil {
				log.Printf("Health check #%d failed: %v", checkCount, err)
			} else {
				fmt.Printf("✓ Health check #%d passed\n", checkCount)
			}
		case <-timeout:
			fmt.Printf("✓ Completed %d health checks\n\n", checkCount)
			return
		}
	}
}
