package uow

import (
	"database/sql"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

// Option configura um UnitOfWork no momento da construção.
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

// WithObservability injeta um provedor de observabilidade usado para registrar
// falhas em fluxos de excepcionais (ex.: rollback após panic). Padrão: noop.
func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}

// WithIsolation define o nível de isolamento da transação para cada chamada Do.
// O padrão é sql.LevelDefault, que delega ao padrão do próprio driver (RF-11).
func WithIsolation(level sql.IsolationLevel) Option {
	return func(o *options) {
		o.isolation = toIsolationLevel(level)
	}
}

// WithReadOnly marca cada transação iniciada por esta UnitOfWork como somente leitura,
// fazendo com que o driver emita BEGIN READ ONLY (RF-36).
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
