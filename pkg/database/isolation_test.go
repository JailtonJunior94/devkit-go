package database_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/require"
)

func TestIsolationLevel_String(t *testing.T) {
	cases := []struct {
		level database.IsolationLevel
		want  string
	}{
		{database.LevelDefault, "LevelDefault"},
		{database.LevelReadUncommitted, "LevelReadUncommitted"},
		{database.LevelReadCommitted, "LevelReadCommitted"},
		{database.LevelWriteCommitted, "LevelWriteCommitted"},
		{database.LevelRepeatableRead, "LevelRepeatableRead"},
		{database.LevelSnapshot, "LevelSnapshot"},
		{database.LevelSerializable, "LevelSerializable"},
		{database.LevelLinearizable, "LevelLinearizable"},
		{database.IsolationLevel(99), "IsolationLevel(99)"},
		{database.IsolationLevel(-1), "IsolationLevel(-1)"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			require.Equal(t, tc.want, tc.level.String())
		})
	}
}
