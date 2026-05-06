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

// WithShutdownTimeout define o timeout do encerramento gracioso (padrão 15s).
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

// WithSQLLogging habilita ou desabilita o log de consultas SQL (padrão desabilitado).
// Quando habilitado com um provedor de observabilidade noop, reverte para slog.Default().
func WithSQLLogging(enabled bool) Option {
	return func(o *options) {
		o.sqlLogging = enabled
	}
}

// WithObservability injeta um provedor de observabilidade (padrão noop).
func WithObservability(obs observability.Observability) Option {
	return func(o *options) {
		if obs != nil {
			o.observability = obs
		}
	}
}

// WithReadOnly sinaliza que o Manager é usado em modo somente leitura.
// As implementações de UoW devem forçar BEGIN READ ONLY quando isso estiver definido.
func WithReadOnly(readOnly bool) Option {
	return func(o *options) {
		o.readOnly = readOnly
	}
}

// WithPoolStatsInterval define o intervalo entre as coletas de estatísticas do pool (padrão 10s).
func WithPoolStatsInterval(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.poolStatsInterval = d
		}
	}
}

// WithStartupMigrationFS injeta um fs.FS (ex.: embed.FS) como fonte das migrações
// executadas no startup. root é o subdiretório dentro do fsys que contém os
// arquivos .sql. Quando definida, prevalece sobre o caminho de filesystem
// padrão (migrations/<driver>) e sobre WithStartupMigrationDir.
//
// Atende RF-15 (suporte a embed.FS) em conjunto com RF-02/RF-10 (startup
// automático e bloqueante).
func WithStartupMigrationFS(fsys fs.FS, root string) Option {
	return func(o *options) {
		if fsys == nil {
			return
		}
		o.startupMigrationFS = fsys
		o.startupMigrationRoot = root
	}
}

// WithStartupMigrationDir sobrescreve o diretório default de migrações de startup
// (migrations/<driver>) por um caminho absoluto ou relativo informado pelo
// consumidor. Ignorada quando WithStartupMigrationFS também é fornecida.
func WithStartupMigrationDir(dir string) Option {
	return func(o *options) {
		if dir != "" {
			o.startupMigrationDir = dir
		}
	}
}
