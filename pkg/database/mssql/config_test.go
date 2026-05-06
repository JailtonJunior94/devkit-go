package mssql_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/mssql"
	"github.com/stretchr/testify/require"
)

func TestMSSQLConfig_Validate_DSNBypassesFieldValidation(t *testing.T) {
	cfg := mssql.MSSQLConfig{DSN: "sqlserver://sa:Pass@host:1433?database=testdb"}
	require.NoError(t, cfg.Validate())
}

func TestMSSQLConfig_Validate_ReturnsAggregatedErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     mssql.MSSQLConfig
		wantErr []string
	}{
		{
			name:    "all required fields missing",
			cfg:     mssql.MSSQLConfig{},
			wantErr: []string{"host is required", "user is required", "database is required"},
		},
		{
			name:    "only host missing",
			cfg:     mssql.MSSQLConfig{User: "u", Database: "d"},
			wantErr: []string{"host is required"},
		},
		{
			name:    "only user missing",
			cfg:     mssql.MSSQLConfig{Host: "h", Database: "d"},
			wantErr: []string{"user is required"},
		},
		{
			name:    "only database missing",
			cfg:     mssql.MSSQLConfig{Host: "h", User: "u"},
			wantErr: []string{"database is required"},
		},
		{
			name:    "blank default schema is invalid",
			cfg:     mssql.MSSQLConfig{Host: "h", User: "u", Database: "d", DefaultSchema: "   "},
			wantErr: []string{"default schema cannot be blank"},
		},
		{
			name: "all fields present — valid",
			cfg:  mssql.MSSQLConfig{Host: "h", User: "u", Database: "d"},
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

func TestMSSQLConfig_ResolveDSN_DSNTakesPrecedence(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		DSN:      "sqlserver://sa:Pass@dsn-host:1433?database=dsn-db",
		Host:     "other-host",
		User:     "other-user",
		Database: "other-db",
	}
	got := cfg.ResolveDSN()
	require.Equal(t, "sqlserver://sa:Pass@dsn-host:1433?database=dsn-db", got)
}

func TestMSSQLConfig_ResolveDSN_BuildsFromFields(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		Host:     "localhost",
		Port:     1433,
		User:     "alice",
		Password: "secret",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "sqlserver://")
	require.Contains(t, got, "localhost:1433")
	require.Contains(t, got, "database=mydb")
	require.Contains(t, got, "alice")
}

func TestMSSQLConfig_ResolveDSN_DefaultPort(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		Host:     "localhost",
		User:     "alice",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "localhost:1433")
}

func TestMSSQLConfig_ResolveDSN_DefaultSchema_StoredInConfig(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		Host:          "localhost",
		User:          "alice",
		Password:      "pass",
		Database:      "mydb",
		DefaultSchema: "myschema",
	}
	require.Equal(t, "myschema", cfg.DefaultSchema)
	dsn := cfg.ResolveDSN()
	require.Contains(t, dsn, "database=mydb")
}

func TestMSSQLConfig_SanitizedDSN_HidesPassword(t *testing.T) {
	cfg := mssql.MSSQLConfig{
		Host:     "localhost",
		Port:     1433,
		User:     "alice",
		Password: "supersecret",
		Database: "mydb",
	}
	got := cfg.SanitizedDSN()
	require.NotContains(t, got, "supersecret")
	require.Contains(t, got, "alice")
	require.Contains(t, got, "localhost")
	require.Contains(t, got, "*")
}

func TestMSSQLConfig_SanitizedDSN_RedactsDSN(t *testing.T) {
	cfg := mssql.MSSQLConfig{DSN: "sqlserver://alice:supersecret@host:1433?database=db"}
	got := cfg.SanitizedDSN()
	require.Equal(t, "<dsn-redacted>", got)
}

func TestMSSQLConfig_Defaults(t *testing.T) {
	require.Equal(t, 20, mssql.DefaultMaxOpenConns)
	require.Equal(t, 5, mssql.DefaultMaxIdleConns)
	require.Equal(t, 10*time.Minute, mssql.DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, mssql.DefaultConnMaxIdle)
}
