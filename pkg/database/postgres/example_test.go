package postgres_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func ExampleNew_basic() {
	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("myuser"),
		postgres.WithPassword("mypassword"),
		postgres.WithDatabase("mydb"),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	fmt.Println("Connected to PostgreSQL")
}

func ExampleNew_withDSN() {
	db := postgres.New(
		postgres.WithDSN("postgresql://myuser:mypassword@localhost:5432/mydb?sslmode=disable"),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	fmt.Println("Connected using DSN")
}

func ExampleNew_withPool() {
	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("myuser"),
		postgres.WithPassword("mypassword"),
		postgres.WithDatabase("mydb"),
		postgres.WithMaxOpenConns(50),
		postgres.WithMaxIdleConns(10),
		postgres.WithConnMaxLifetime(5*time.Minute),
		postgres.WithConnMaxIdleTime(10*time.Minute),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	fmt.Println("Connected with custom pool settings")
}

func ExampleNew_withSSL() {
	db := postgres.New(
		postgres.WithHost("production-db.example.com"),
		postgres.WithPort(5432),
		postgres.WithUser("app_user"),
		postgres.WithPassword("secure_password"),
		postgres.WithDatabase("production_db"),
		postgres.WithSSLMode("require"),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	fmt.Println("Connected with SSL enabled")
}

func ExampleNew_withRetry() {
	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("myuser"),
		postgres.WithPassword("mypassword"),
		postgres.WithDatabase("mydb"),
		postgres.WithMaxRetries(5),
		postgres.WithRetryInterval(3*time.Second),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	fmt.Println("Connected with custom retry settings")
}

func ExampleDatabase_HealthCheck() {
	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("myuser"),
		postgres.WithPassword("mypassword"),
		postgres.WithDatabase("mydb"),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	if err := db.HealthCheck(ctx); err != nil {
		log.Printf("Health check failed: %v", err)
		return
	}

	fmt.Println("Database is healthy")
}

func ExampleDatabase_DB() {
	db := postgres.New(
		postgres.WithHost("localhost"),
		postgres.WithPort(5432),
		postgres.WithUser("myuser"),
		postgres.WithPassword("mypassword"),
		postgres.WithDatabase("mydb"),
	)

	ctx := context.Background()

	if err := db.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	sqlDB := db.DB()

	var count int
	err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	fmt.Printf("Total users: %d\n", count)
}
