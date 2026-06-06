package uow

import (
	"database/sql"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

type Option func(*options)

type options struct {
	isolation     database.IsolationLevel
	readOnly      bool
	observability observability.Observability
}

func defaultOptions() options {
	return options{
		isolation:     database.LevelDefault,
		observability: noop.NewProvider(),
	}
}

func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}

func WithIsolation(level sql.IsolationLevel) Option {
	return func(o *options) {
		o.isolation = toIsolationLevel(level)
	}
}

func WithReadOnly(readOnly bool) Option {
	return func(o *options) {
		o.readOnly = readOnly
	}
}

func toIsolationLevel(level sql.IsolationLevel) database.IsolationLevel {
	switch level {
	case sql.LevelReadUncommitted:
		return database.LevelReadUncommitted
	case sql.LevelReadCommitted:
		return database.LevelReadCommitted
	case sql.LevelWriteCommitted:
		return database.LevelWriteCommitted
	case sql.LevelRepeatableRead:
		return database.LevelRepeatableRead
	case sql.LevelSnapshot:
		return database.LevelSnapshot
	case sql.LevelSerializable:
		return database.LevelSerializable
	case sql.LevelLinearizable:
		return database.LevelLinearizable
	default:
		return database.LevelDefault
	}
}
