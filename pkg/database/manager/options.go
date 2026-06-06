package manager

import (
	"io/fs"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

const (
	defaultShutdownTimeout = 15 * time.Second
	pingInitTimeout        = 5 * time.Second
)

type options struct {
	shutdownTimeout      time.Duration
	sqlLogging           bool
	observability        observability.Observability
	readOnly             bool
	poolStatsInterval    time.Duration
	startupMigrationFS   fs.FS
	startupMigrationRoot string
	startupMigrationDir  string
}

func defaultOptions() options {
	return options{
		shutdownTimeout: defaultShutdownTimeout,
		observability:   noop.NewProvider(),
	}
}

func WithShutdownTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

func WithSQLLogging(enabled bool) Option {
	return func(o *options) {
		o.sqlLogging = enabled
	}
}

func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}

func WithReadOnly(readOnly bool) Option {
	return func(o *options) {
		o.readOnly = readOnly
	}
}

func WithPoolStatsInterval(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.poolStatsInterval = d
		}
	}
}

func WithStartupMigrationFS(fsys fs.FS, root string) Option {
	return func(o *options) {
		if fsys == nil {
			return
		}
		o.startupMigrationFS = fsys
		o.startupMigrationRoot = root
	}
}

func WithStartupMigrationDir(dir string) Option {
	return func(o *options) {
		if dir != "" {
			o.startupMigrationDir = dir
		}
	}
}
