package postgres_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/stretchr/testify/require"
)

func TestPostgresConfig_Validate_DSNBypassesFieldValidation(t *testing.T) {
	cfg := postgres.PostgresConfig{DSN: "postgres://user:pass@host:5432/db"}
	require.NoError(t, cfg.Validate())
}

func TestPostgresConfig_Validate_ReturnsAggregatedErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     postgres.PostgresConfig
		wantErr []string
	}{
		{
			name:    "all required fields missing",
			cfg:     postgres.PostgresConfig{},
			wantErr: []string{"host is required", "user is required", "database is required"},
		},
		{
			name:    "only host missing",
			cfg:     postgres.PostgresConfig{User: "u", Database: "d"},
			wantErr: []string{"host is required"},
		},
		{
			name:    "only user missing",
			cfg:     postgres.PostgresConfig{Host: "h", Database: "d"},
			wantErr: []string{"user is required"},
		},
		{
			name:    "only database missing",
			cfg:     postgres.PostgresConfig{Host: "h", User: "u"},
			wantErr: []string{"database is required"},
		},
		{
			name: "all fields present — valid",
			cfg:  postgres.PostgresConfig{Host: "h", User: "u", Database: "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if len(tt.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, msg := range tt.wantErr {
				require.Contains(t, err.Error(), msg)
			}
		})
	}
}

func TestPostgresConfig_ResolveDSN_DSNTakesPrecedence(t *testing.T) {
	cfg := postgres.PostgresConfig{
		DSN:      "postgres://dsn-user:dsn-pass@dsn-host:9999/dsn-db",
		Host:     "other-host",
		User:     "other-user",
		Database: "other-db",
	}
	got := cfg.ResolveDSN()
	require.Equal(t, "postgres://dsn-user:dsn-pass@dsn-host:9999/dsn-db", got)
}

func TestPostgresConfig_ResolveDSN_BuildsFromFields(t *testing.T) {
	cfg := postgres.PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "alice",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "require",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "host=localhost")
	require.Contains(t, got, "port=5432")
	require.Contains(t, got, "user=alice")
	require.Contains(t, got, "password=secret")
	require.Contains(t, got, "dbname=mydb")
	require.Contains(t, got, "sslmode=require")
}

func TestPostgresConfig_ResolveDSN_DefaultPortAndSSLMode(t *testing.T) {
	cfg := postgres.PostgresConfig{
		Host:     "localhost",
		User:     "alice",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "port=5432")
	require.Contains(t, got, "sslmode=disable")
}

func TestPostgresConfig_ResolveDSN_SearchPathIncludedWhenSet(t *testing.T) {
	cfg := postgres.PostgresConfig{
		Host:       "localhost",
		User:       "alice",
		Database:   "mydb",
		SearchPath: "tenant_schema",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "search_path=tenant_schema")
}

func TestPostgresConfig_ResolveDSN_NoSearchPathWhenEmpty(t *testing.T) {
	cfg := postgres.PostgresConfig{Host: "h", User: "u", Database: "d"}
	got := cfg.ResolveDSN()
	require.NotContains(t, got, "search_path")
}

func TestPostgresConfig_ResolveDSN_QuotesValuesWithSpecialChars(t *testing.T) {
	// Regressão: senhas/usernames com espaços, aspas simples ou backslash
	// não podem corromper o DSN libpq. Devem ser envolvidos em aspas simples
	// com backslash escapado.
	cfg := postgres.PostgresConfig{
		Host:     "localhost",
		User:     "alice user",
		Password: `p'a\ss word`,
		Database: "mydb",
		SSLMode:  "require",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "user='alice user'")
	require.Contains(t, got, `password='p\'a\\ss word'`)
	require.Contains(t, got, "host=localhost")
	require.Contains(t, got, "dbname=mydb")
}

func TestPostgresConfig_ResolveDSN_QuotesEmptyPassword(t *testing.T) {
	cfg := postgres.PostgresConfig{
		Host:     "h",
		User:     "u",
		Password: "",
		Database: "d",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "password=''", "password vazia deve ser quotada para preservar a chave")
}

func TestPostgresConfig_Defaults(t *testing.T) {
	require.Equal(t, 25, postgres.DefaultMaxOpenConns)
	require.Equal(t, 6, postgres.DefaultMaxIdleConns)
	require.Equal(t, 30*time.Minute, postgres.DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, postgres.DefaultConnMaxIdle)
}
