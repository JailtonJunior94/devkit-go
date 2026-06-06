package worker

import (
	"context"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

type slogLogger struct {
	l *slog.Logger
}

func (s *slogLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {
	s.l.DebugContext(ctx, msg, fieldsToArgs(fields)...)
}

func (s *slogLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {
	s.l.InfoContext(ctx, msg, fieldsToArgs(fields)...)
}

func (s *slogLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {
	s.l.WarnContext(ctx, msg, fieldsToArgs(fields)...)
}

func (s *slogLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {
	s.l.ErrorContext(ctx, msg, fieldsToArgs(fields)...)
}

func (s *slogLogger) With(fields ...observability.Field) observability.Logger {
	return &slogLogger{l: s.l.With(fieldsToArgs(fields)...)}
}

type obsFromSlog struct {
	tracer  observability.Tracer
	logger  observability.Logger
	metrics observability.Metrics
}

func newObsFromSlog(logger *slog.Logger) observability.Observability {
	n := noop.NewProvider()
	return &obsFromSlog{
		tracer:  n.Tracer(),
		logger:  &slogLogger{l: logger},
		metrics: n.Metrics(),
	}
}

func (o *obsFromSlog) Tracer() observability.Tracer     { return o.tracer }
func (o *obsFromSlog) Logger() observability.Logger     { return o.logger }
func (o *obsFromSlog) Metrics() observability.Metrics   { return o.metrics }
func (o *obsFromSlog) Shutdown(_ context.Context) error { return nil }

func fieldsToArgs(fields []observability.Field) []any {
	args := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		args = append(args, f.Key, f.AnyValue())
	}
	return args
}
