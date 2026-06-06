package manager

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("runtime.(*cleanupBlock).clearCleanups"),
		goleak.IgnoreTopFunction("runtime.runCleanupCallbacks"),
	)
}
