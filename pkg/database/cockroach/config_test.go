package cockroach_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/cockroach"
	"github.com/stretchr/testify/require"
)

func TestCockroachConfig_Validate_DSNBypassesFieldValidation(t *testing.T) {
	cfg := cockroach.CockroachConfig{DSN: "postgresql://user:pass@host:26257/db"}
	require.NoError(t, cfg.Validate())
}

func TestCockroachConfig_Validate_ReturnsAggregatedErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     cockroach.CockroachConfig
		wantErr []string
	}{
		{
			name:    "all required fields missing",
			cfg:     cockroach.CockroachConfig{},
			wantErr: []string{"host is required", "user is required", "database is required"},
		},
		{
			name:    "only host missing",
			cfg:     cockroach.CockroachConfig{User: "u", Database: "d"},
			wantErr: []string{"host is required"},
		},
		{
			name:    "only user missing",
			cfg:     cockroach.CockroachConfig{Host: "h", Database: "d"},
			wantErr: []string{"user is required"},
		},
		{
			name:    "only database missing",
			cfg:     cockroach.CockroachConfig{Host: "h", User: "u"},
			wantErr: []string{"database is required"},
		},
		{
			name: "all fields present — valid",
			cfg:  cockroach.CockroachConfig{Host: "h", User: "u", Database: "d"},
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

func TestCockroachConfig_ResolveDSN_DSNTakesPrecedence(t *testing.T) {
	cfg := cockroach.CockroachConfig{
		DSN:      "postgresql://dsn-user:dsn-pass@dsn-host:26257/dsn-db",
		Host:     "other-host",
		User:     "other-user",
		Database: "other-db",
	}
	got := cfg.ResolveDSN()
	require.Equal(t, "postgresql://dsn-user:dsn-pass@dsn-host:26257/dsn-db", got)
}

func TestCockroachConfig_ResolveDSN_BuildsFromFields(t *testing.T) {
	cfg := cockroach.CockroachConfig{
		Host:     "localhost",
		Port:     26257,
		User:     "alice",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "require",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "host=localhost")
	require.Contains(t, got, "port=26257")
	require.Contains(t, got, "user=alice")
	require.Contains(t, got, "password=secret")
	require.Contains(t, got, "dbname=mydb")
	require.Contains(t, got, "sslmode=require")
}

func TestCockroachConfig_ResolveDSN_DefaultPortAndSSLMode(t *testing.T) {
	cfg := cockroach.CockroachConfig{
		Host:     "localhost",
		User:     "alice",
		Database: "mydb",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "port=26257")
	require.Contains(t, got, "sslmode=disable")
}

func TestCockroachConfig_ResolveDSN_SearchPathIncludedWhenSet(t *testing.T) {
	cfg := cockroach.CockroachConfig{
		Host:       "localhost",
		User:       "alice",
		Database:   "mydb",
		SearchPath: "tenant_schema",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "search_path=tenant_schema")
}

func TestCockroachConfig_ResolveDSN_NoSearchPathWhenEmpty(t *testing.T) {
	cfg := cockroach.CockroachConfig{Host: "h", User: "u", Database: "d"}
	got := cfg.ResolveDSN()
	require.NotContains(t, got, "search_path")
}

func TestCockroachConfig_ResolveDSN_QuotesValuesWithSpecialChars(t *testing.T) {
	cfg := cockroach.CockroachConfig{
		Host:     "localhost",
		User:     "alice user",
		Password: `p'a\ss word`,
		Database: "mydb",
		SSLMode:  "require",
	}
	got := cfg.ResolveDSN()
	require.Contains(t, got, "user='alice user'")
	require.Contains(t, got, `password='p\'a\\ss word'`)
}

func TestCockroachConfig_Defaults(t *testing.T) {
	require.Equal(t, 50, cockroach.DefaultMaxOpenConns)
	require.Equal(t, 10, cockroach.DefaultMaxIdleConns)
	require.Equal(t, 15*time.Minute, cockroach.DefaultConnMaxLife)
	require.Equal(t, 5*time.Minute, cockroach.DefaultConnMaxIdle)
}
