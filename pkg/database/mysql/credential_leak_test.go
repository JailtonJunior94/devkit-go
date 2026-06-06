package mysql

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/require"
)

func TestNew_InvalidDSN_DoesNotLeakCredentials(t *testing.T) {
	const password = "sup3r-s3cr3t-p4ssw0rd"
	cfg := MySQLConfig{DSN: "broken-dsn-no-slash-with-password-" + password}

	_, err := New(cfg, nil)

	require.Error(t, err)
	require.ErrorIs(t, err, database.ErrInvalidConfig)
	require.NotContains(t, err.Error(), password)
	require.NotContains(t, err.Error(), cfg.DSN)
}
