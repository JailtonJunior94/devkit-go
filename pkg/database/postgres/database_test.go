package postgres

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want *config
	}{
		{
			name: "default configuration",
			opts: nil,
			want: defaultConfig(),
		},
		{
			name: "custom host",
			opts: []Option{WithHost("custom-host")},
			want: func() *config {
				c := defaultConfig()
				c.host = "custom-host"
				return c
			}(),
		},
		{
			name: "custom port",
			opts: []Option{WithPort(5433)},
			want: func() *config {
				c := defaultConfig()
				c.port = 5433
				return c
			}(),
		},
		{
			name: "full custom configuration",
			opts: []Option{
				WithHost("custom-host"),
				WithPort(5433),
				WithUser("custom-user"),
				WithPassword("custom-password"),
				WithDatabase("custom-db"),
				WithSSLMode("require"),
				WithMaxOpenConns(50),
				WithMaxIdleConns(10),
			},
			want: func() *config {
				c := defaultConfig()
				c.host = "custom-host"
				c.port = 5433
				c.user = "custom-user"
				c.password = "custom-password"
				c.database = "custom-db"
				c.sslMode = "require"
				c.maxOpenConns = 50
				c.maxIdleConns = 10
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := New(tt.opts...).(*database)

			if db.config.host != tt.want.host {
				t.Errorf("host = %v, want %v", db.config.host, tt.want.host)
			}
			if db.config.port != tt.want.port {
				t.Errorf("port = %v, want %v", db.config.port, tt.want.port)
			}
			if db.config.user != tt.want.user {
				t.Errorf("user = %v, want %v", db.config.user, tt.want.user)
			}
			if db.config.database != tt.want.database {
				t.Errorf("database = %v, want %v", db.config.database, tt.want.database)
			}
			if db.config.sslMode != tt.want.sslMode {
				t.Errorf("sslMode = %v, want %v", db.config.sslMode, tt.want.sslMode)
			}
			if db.config.maxOpenConns != tt.want.maxOpenConns {
				t.Errorf("maxOpenConns = %v, want %v", db.config.maxOpenConns, tt.want.maxOpenConns)
			}
			if db.config.maxIdleConns != tt.want.maxIdleConns {
				t.Errorf("maxIdleConns = %v, want %v", db.config.maxIdleConns, tt.want.maxIdleConns)
			}
		})
	}
}

func TestDatabase_BuildDSN(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{
			name: "default DSN",
			opts: nil,
			want: "host=localhost port=5432 user=postgres password= dbname=postgres sslmode=disable connect_timeout=10",
		},
		{
			name: "custom DSN",
			opts: []Option{
				WithHost("db.example.com"),
				WithPort(5433),
				WithUser("admin"),
				WithPassword("secret"),
				WithDatabase("myapp"),
				WithSSLMode("require"),
			},
			want: "host=db.example.com port=5433 user=admin password=secret dbname=myapp sslmode=require connect_timeout=10",
		},
		{
			name: "with DSN string",
			opts: []Option{
				WithDSN("postgresql://user:pass@localhost:5432/mydb?sslmode=require"),
			},
			want: "postgresql://user:pass@localhost:5432/mydb?sslmode=require",
		},
		{
			name: "DSN takes precedence over individual params",
			opts: []Option{
				WithHost("ignored-host"),
				WithPort(9999),
				WithDSN("postgresql://user:pass@localhost:5432/mydb"),
				WithDatabase("ignored-db"),
			},
			want: "postgresql://user:pass@localhost:5432/mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := New(tt.opts...).(*database)
			got := db.buildDSN()

			if got != tt.want {
				t.Errorf("buildDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatabase_Close_NotConnected(t *testing.T) {
	db := New()

	err := db.Close()
	if err != ErrNotConnected {
		t.Errorf("Close() error = %v, want %v", err, ErrNotConnected)
	}
}

func TestDatabase_HealthCheck_NotConnected(t *testing.T) {
	db := New()
	ctx := context.Background()

	err := db.HealthCheck(ctx)
	if err != ErrNotConnected {
		t.Errorf("HealthCheck() error = %v, want %v", err, ErrNotConnected)
	}
}

func TestDatabase_DB(t *testing.T) {
	db := New()

	sqlDB := db.DB()
	if sqlDB != nil {
		t.Error("DB() should return nil when not connected")
	}
}

func TestWithOptions(t *testing.T) {
	t.Run("WithHost empty string", func(t *testing.T) {
		db := New(WithHost("")).(*database)
		if db.config.host != defaultHost {
			t.Errorf("host should be default when empty string provided")
		}
	})

	t.Run("WithPort invalid", func(t *testing.T) {
		db := New(WithPort(0)).(*database)
		if db.config.port != defaultPort {
			t.Errorf("port should be default when invalid value provided")
		}

		db2 := New(WithPort(70000)).(*database)
		if db2.config.port != defaultPort {
			t.Errorf("port should be default when value exceeds 65535")
		}
	})

	t.Run("WithMaxOpenConns invalid", func(t *testing.T) {
		db := New(WithMaxOpenConns(0)).(*database)
		if db.config.maxOpenConns != defaultMaxOpenConns {
			t.Errorf("maxOpenConns should be default when invalid value provided")
		}
	})

	t.Run("WithConnMaxLifetime valid", func(t *testing.T) {
		duration := 10 * time.Minute
		db := New(WithConnMaxLifetime(duration)).(*database)
		if db.config.connMaxLifetime != duration {
			t.Errorf("connMaxLifetime = %v, want %v", db.config.connMaxLifetime, duration)
		}
	})

	t.Run("WithDSN valid", func(t *testing.T) {
		dsn := "postgresql://user:pass@localhost:5432/mydb"
		db := New(WithDSN(dsn)).(*database)
		if db.config.dsn != dsn {
			t.Errorf("dsn = %v, want %v", db.config.dsn, dsn)
		}
	})

	t.Run("WithDSN empty string", func(t *testing.T) {
		db := New(WithDSN("")).(*database)
		if db.config.dsn != "" {
			t.Errorf("dsn should be empty when empty string provided")
		}
	})
}
