package migration

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Driver != DriverPostgres {
		t.Errorf("expected default driver to be postgres, got %s", cfg.Driver)
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected default timeout to be 5m, got %v", cfg.Timeout)
	}

	if cfg.LockTimeout != 30*time.Second {
		t.Errorf("expected default lock timeout to be 30s, got %v", cfg.LockTimeout)
	}

	if !cfg.MultiStatementEnabled {
		t.Error("expected multi-statement to be enabled by default")
	}

	if cfg.Logger == nil {
		t.Error("expected logger to be set by default")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errType error
	}{
		{
			name: "valid config",
			config: Config{
				Driver:  DriverPostgres,
				DSN:     "postgres://user:pass@localhost:5432/mydb",
				Source:  "file://migrations",
				Logger:  NewNoopLogger(),
				Timeout: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid driver",
			config: Config{
				Driver:  Driver("invalid"),
				DSN:     "postgres://user:pass@localhost:5432/mydb",
				Source:  "file://migrations",
				Logger:  NewNoopLogger(),
				Timeout: 5 * time.Minute,
			},
			wantErr: true,
			errType: ErrInvalidDriver,
		},
		{
			name: "missing DSN",
			config: Config{
				Driver:  DriverPostgres,
				DSN:     "",
				Source:  "file://migrations",
				Logger:  NewNoopLogger(),
				Timeout: 5 * time.Minute,
			},
			wantErr: true,
			errType: ErrMissingDSN,
		},
		{
			name: "missing source",
			config: Config{
				Driver:  DriverPostgres,
				DSN:     "postgres://user:pass@localhost:5432/mydb",
				Source:  "",
				Logger:  NewNoopLogger(),
				Timeout: 5 * time.Minute,
			},
			wantErr: true,
			errType: ErrMissingSource,
		},
		{
			name: "invalid timeout",
			config: Config{
				Driver:  DriverPostgres,
				DSN:     "postgres://user:pass@localhost:5432/mydb",
				Source:  "file://migrations",
				Logger:  NewNoopLogger(),
				Timeout: 0,
			},
			wantErr: true,
			errType: ErrInvalidTimeout,
		},
		{
			name: "invalid lock timeout",
			config: Config{
				Driver:      DriverPostgres,
				DSN:         "postgres://user:pass@localhost:5432/mydb",
				Source:      "file://migrations",
				Logger:      NewNoopLogger(),
				Timeout:     5 * time.Minute,
				LockTimeout: -1 * time.Second,
			},
			wantErr: true,
			errType: ErrInvalidLockTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractDatabaseNameFromDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "simple dsn",
			dsn:  "postgres://user:pass@localhost:5432/mydb",
			want: "mydb",
		},
		{
			name: "dsn with query params",
			dsn:  "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			want: "mydb",
		},
		{
			name: "dsn with multiple query params",
			dsn:  "postgres://user:pass@localhost:5432/testdb?sslmode=disable&connect_timeout=10",
			want: "testdb",
		},
		{
			name: "empty dsn",
			dsn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDatabaseNameFromDSN(tt.dsn)
			if got != tt.want {
				t.Errorf("extractDatabaseNameFromDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigGetDatabaseName(t *testing.T) {
	t.Run("uses configured name", func(t *testing.T) {
		cfg := Config{
			DSN:          "postgres://user:pass@localhost:5432/actualdb",
			DatabaseName: "customname",
		}

		got := cfg.GetDatabaseName()
		if got != "customname" {
			t.Errorf("expected customname, got %s", got)
		}
	})

	t.Run("extracts from DSN", func(t *testing.T) {
		cfg := Config{
			DSN: "postgres://user:pass@localhost:5432/extracteddb",
		}

		got := cfg.GetDatabaseName()
		if got != "extracteddb" {
			t.Errorf("expected extracteddb, got %s", got)
		}
	})
}
