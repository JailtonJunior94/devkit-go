package migration

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

// Option configura o Migrator.
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

// WithDSN define a URL de conexão do banco de dados usada pelo golang-migrate para conectar.
// Obrigatório: New retorna database.ErrInvalidConfig quando ausente ou vazio.
// Aceita URLs padrão do postgres (postgres://, postgresql://, pgx5://).
func WithDSN(dsn string) Option {
	return func(o *options) {
		o.dsn = dsn
	}
}

// WithMigrationTimeout define um timeout por operação aplicado via context.WithTimeout.
// Quando zero (padrão), o contexto do chamador governa o prazo (deadline).
func WithMigrationTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.timeout = d
		}
	}
}

// WithObservability injeta um provedor de observabilidade para spans de migração (padrão noop).
func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}
