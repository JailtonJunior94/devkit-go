package pool_test

import (
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/stretchr/testify/require"
)

func TestSafeAttrs_ContainsExpectedFields(t *testing.T) {
	info := pool.ConnInfo{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
	}

	attrs := pool.SafeAttrs(info)

	require.Len(t, attrs, 4)

	keys := make(map[string]string, len(attrs))
	for _, f := range attrs {
		keys[f.Key] = f.StringValue()
	}

	require.Equal(t, "postgres", keys["db.system"])
	require.Equal(t, "mydb", keys["db.name"])
	require.Equal(t, "localhost", keys["server.address"])
	require.Equal(t, "5432", keys["server.port"])
}

func TestSafeAttrs_NeverContainsPassword(t *testing.T) {
	info := pool.ConnInfo{
		Driver:   "postgres",
		Host:     "db.internal",
		Port:     5432,
		Database: "appdb",
	}

	attrs := pool.SafeAttrs(info)

	sensitiveTerms := []string{"password", "pass", "secret", "credential", "token"}
	for _, attr := range attrs {
		val := attr.StringValue()
		for _, term := range sensitiveTerms {
			require.False(t, strings.Contains(strings.ToLower(val), term),
				"attribute %q should not contain sensitive term %q", attr.Key, term)
			require.False(t, strings.Contains(strings.ToLower(attr.Key), term),
				"attribute key %q should not be a sensitive term", attr.Key)
		}
	}
}

func TestSafeAttrs_ZeroPortRenderedCorrectly(t *testing.T) {
	info := pool.ConnInfo{Driver: "postgres", Database: "db"}

	attrs := pool.SafeAttrs(info)

	keys := make(map[string]string, len(attrs))
	for _, f := range attrs {
		keys[f.Key] = f.StringValue()
	}
	require.Equal(t, "0", keys["server.port"])
}
