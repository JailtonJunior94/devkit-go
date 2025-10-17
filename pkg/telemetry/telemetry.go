package o11y

type telemetry struct {
	tracer  Tracer
	metrics Metrics
	logger  Logger
}

func NewTelemetry(tracer Tracer, metrics Metrics, logger Logger) (Telemetry, error) {
	return &telemetry{
		tracer:  tracer,
		metrics: metrics,
		logger:  logger,
	}, nil
}

func (t *telemetry) Tracer() Tracer {
	return t.tracer
}

func (t *telemetry) Metrics() Metrics {
	return t.metrics
}

func (t *telemetry) Logger() Logger {
	return t.logger
}
