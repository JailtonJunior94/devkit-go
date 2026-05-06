package database_test

import (
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrors_Identity(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrManagerClosed",
			err:  database.ErrManagerClosed,
			msg:  "database: manager closed",
		},
		{
			name: "ErrShutdownTimeout",
			err:  database.ErrShutdownTimeout,
			msg:  "database: shutdown timeout exceeded",
		},
		{
			name: "ErrNestedTransaction",
			err:  database.ErrNestedTransaction,
			msg:  "database: nested transaction not supported",
		},
		{
			name: "ErrInvalidConfig",
			err:  database.ErrInvalidConfig,
			msg:  "database: invalid configuration",
		},
		{
			name: "ErrMigrationFailed",
			err:  database.ErrMigrationFailed,
			msg:  "database: migration failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualError(t, tt.err, tt.msg)
		})
	}
}

func TestSentinelErrors_ErrorsIs(t *testing.T) {
	sentinels := []error{
		database.ErrManagerClosed,
		database.ErrShutdownTimeout,
		database.ErrNestedTransaction,
		database.ErrInvalidConfig,
		database.ErrMigrationFailed,
	}

	for _, sentinel := range sentinels {
		t.Run(sentinel.Error(), func(t *testing.T) {
			// erro encapsulado ainda deve coincidir via errors.Is
			wrapped := errors.Join(errors.New("context"), sentinel)
			require.ErrorIs(t, wrapped, sentinel)

			// sentinelas distintos não devem coincidir entre si
			for _, other := range sentinels {
				if other == sentinel {
					continue
				}
				require.False(t, errors.Is(sentinel, other),
					"%v should not match %v", sentinel, other)
			}
		})
	}
}
