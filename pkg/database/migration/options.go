package migration

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

type Option func(*options)

type options struct {
	dsn           string
	timeout       time.Duration
	observability observability.Observability
}

func defaultOptions() options {
	return options{
		observability: noop.NewProvider(),
	}
}

func WithDSN(dsn string) Option {
	return func(o *options) {
		o.dsn = dsn
	}
}

func WithMigrationTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.timeout = d
		}
	}
}

func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}
