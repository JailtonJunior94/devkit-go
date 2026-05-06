package mysql_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/mysql"
	"github.com/stretchr/testify/require"
)

func TestMySQLConfig_Validate_DSNBypassesFieldValidation(t *testing.T) {
	cfg := mysql.MySQLConfig{DSN: "user:pass@tcp(host:3306)/db?parseTime=true"}
	require.NoError(t, cfg.Validate())
}

func TestMySQLConfig_Validate_ReturnsAggregatedErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     mysql.MySQLConfig
		wantErr []string
	}{
		{
			name:    "all required fields missing",
			cfg:     mysql.MySQLConfig{},
			wantErr: []string{"host is required", "user is required", "database is required"},
		},
		{
			name:    "only host missing",
			cfg:     mysql.MySQLConfig{User: "u", Database: "d"},
			wantErr: []string{"host is required"},
		},
		{
			name:    "only user missing",
			cfg:     mysql.MySQLConfig{Host: "h", Database: "d"},
			wantErr: []string{"user is required"},
		},
		{
			name:    "only database missing",
			cfg:     mysql.MySQLConfig{Host: "h", User: "u"},
			wantErr: []string{"database is required"},
		},
		{
			name: "all fields present — valid",
			cfg:  mysql.MySQLConfig{Host: "h", User: "u", Database: "d"},
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

func TestMySQLConfig_ResolveDSN_DSNTakesPrecedence(t *testing.T) {
	cfg := mysql.MySQLConfig{
		DSN:      "user:pass@tcp(dsn-host:3306)/dsn-db",
		Host:     "other-host",
		User:     "other-user",
		Database: "other-db",
	}
	got := cfg.ResolveDSN()
	require.Equal(t, "user:pass@tcp(dsn-host:3306)/dsn-db", got)
}

func TestMySQLConfig_ResolveDSN_BuildsFromFields(t *testing.T) {
	cfg := mysql.MySQLConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "alice",
		Password: "secret",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "alice:secret@tcp(localhost:3306)/mydb")
	require.Contains(t, got, "parseTime=true")
}

func TestMySQLConfig_ResolveDSN_DefaultPort(t *testing.T) {
	cfg := mysql.MySQLConfig{
		Host:     "localhost",
		User:     "alice",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "tcp(localhost:3306)")
}

func TestMySQLConfig_SanitizedDSN_HidesPassword(t *testing.T) {
	cfg := mysql.MySQLConfig{
		Host:     "localhost",
		Port:     3306,
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

func TestMySQLConfig_SanitizedDSN_RedactsDSN(t *testing.T) {
	cfg := mysql.MySQLConfig{DSN: "alice:supersecret@tcp(host:3306)/db"}
	got := cfg.SanitizedDSN()
	require.Equal(t, "<dsn-redacted>", got)
}

func TestMySQLConfig_Defaults(t *testing.T) {
	require.Equal(t, 20, mysql.DefaultMaxOpenConns)
	require.Equal(t, 5, mysql.DefaultMaxIdleConns)
	require.Equal(t, 10*time.Minute, mysql.DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, mysql.DefaultConnMaxIdle)
}
