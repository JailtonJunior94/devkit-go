package migration

import (
	"strings"
	"testing"
	"time"
)

func TestPostgresStrategy_BuildDatabaseURL(t *testing.T) {
	strategy := NewPostgresStrategy()
	params := DatabaseParams{
		LockTimeout:      30 * time.Second,
		StatementTimeout: 2 * time.Minute,
	}

	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name: "converts postgresql to postgres scheme",
			dsn:  "postgresql://user:pass@localhost:5432/mydb",
			want: "postgres://",
		},
		{
			name: "keeps postgres scheme",
			dsn:  "postgres://user:pass@localhost:5432/mydb",
			want: "postgres://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strategy.BuildDatabaseURL(tt.dsn, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDatabaseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("BuildDatabaseURL() = %v, want prefix %v", got, tt.want)
			}
		})
	}
}

func TestCockroachStrategy_BuildDatabaseURL(t *testing.T) {
	strategy := NewCockroachStrategy()
	params := DatabaseParams{}

	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name: "converts postgres to cockroachdb scheme",
			dsn:  "postgres://user@localhost:26257/mydb",
			want: "cockroachdb://",
		},
		{
			name: "converts postgresql to cockroachdb scheme",
			dsn:  "postgresql://user@localhost:26257/mydb",
			want: "cockroachdb://",
		},
		{
			name: "keeps cockroachdb scheme",
			dsn:  "cockroachdb://user@localhost:26257/mydb",
			want: "cockroachdb://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strategy.BuildDatabaseURL(tt.dsn, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDatabaseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("BuildDatabaseURL() = %v, want prefix %v", got, tt.want)
			}
		})
	}
}

func TestCockroachStrategy_DefaultLockTimeout(t *testing.T) {
	strategy := NewCockroachStrategy()
	params := DatabaseParams{
		LockTimeout: 0, // Not set, should use default
	}

	got, err := strategy.BuildDatabaseURL("postgres://user@localhost:26257/mydb", params)
	if err != nil {
		t.Fatalf("BuildDatabaseURL() error = %v", err)
	}

	// Should contain the default lock timeout (60s)
	if !strings.Contains(got, "x-migrations-table-lock-timeout=60s") {
		t.Errorf("BuildDatabaseURL() should contain default lock timeout of 60s, got %v", got)
	}
}

func TestMySQLStrategy_BuildDatabaseURL(t *testing.T) {
	strategy := NewMySQLStrategy()
	params := DatabaseParams{
		MultiStatementEnabled: true,
	}

	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name: "converts to mysql scheme",
			dsn:  "mysql://user:pass@localhost:3306/mydb",
			want: "mysql://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strategy.BuildDatabaseURL(tt.dsn, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDatabaseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("BuildDatabaseURL() = %v, want prefix %v", got, tt.want)
			}
			// MySQL should have multiStatements=true
			if !strings.Contains(got, "multiStatements=true") {
				t.Errorf("BuildDatabaseURL() should contain multiStatements=true for MySQL")
			}
		})
	}
}

func TestGetDriverStrategy(t *testing.T) {
	tests := []struct {
		name       string
		driver     Driver
		wantName   string
		wantErr    bool
		wantErrMsg error
	}{
		{
			name:     "postgres driver",
			driver:   DriverPostgres,
			wantName: "postgres",
			wantErr:  false,
		},
		{
			name:     "cockroachdb driver",
			driver:   DriverCockroachDB,
			wantName: "postgres",
			wantErr:  false,
		},
		{
			name:     "mysql driver",
			driver:   DriverMySQL,
			wantName: "mysql",
			wantErr:  false,
		},
		{
			name:       "invalid driver",
			driver:     Driver("invalid"),
			wantErr:    true,
			wantErrMsg: ErrInvalidDriver,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDriverStrategy(tt.driver)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDriverStrategy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Name() != tt.wantName {
				t.Errorf("GetDriverStrategy().Name() = %v, want %v", got.Name(), tt.wantName)
			}
		})
	}
}

func TestCockroachStrategy_Validate(t *testing.T) {
	strategy := NewCockroachStrategy()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with 30s lock timeout",
			config: Config{
				LockTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid config with 60s lock timeout",
			config: Config{
				LockTimeout: 60 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid config with lock timeout < 30s",
			config: Config{
				LockTimeout: 15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "valid config with zero lock timeout (will use default)",
			config: Config{
				LockTimeout: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := strategy.Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
