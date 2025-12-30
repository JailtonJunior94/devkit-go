package cron_worker

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/robfig/cron/v3"
)

// cronLogger adapta o observability logger para o cron logger.
type cronLogger struct {
	serviceName string
	o11y        observability.Observability
}

// newCronLogger cria um novo cron logger.
func newCronLogger(serviceName string, o11y observability.Observability) cron.Logger {
	return &cronLogger{
		serviceName: serviceName,
		o11y:        o11y,
	}
}

// Info implementa cron.Logger.
func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	ctx := context.Background()
	fields := l.convertKeysAndValues(keysAndValues...)
	l.o11y.Logger().Info(ctx, msg, fields...)
}

// Error implementa cron.Logger.
func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	ctx := context.Background()
	fields := l.convertKeysAndValues(keysAndValues...)
	fields = append(fields, observability.Error(err))
	l.o11y.Logger().Error(ctx, msg, fields...)
}

// convertKeysAndValues converte pares chave-valor para observability.Field.
func (l *cronLogger) convertKeysAndValues(keysAndValues ...interface{}) []observability.Field {
	fields := []observability.Field{
		observability.String("worker", l.serviceName),
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			break
		}

		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}

		value := keysAndValues[i+1]
		fields = append(fields, l.convertValue(key, value))
	}

	return fields
}

// convertValue converte um valor para observability.Field.
func (l *cronLogger) convertValue(key string, value interface{}) observability.Field {
	switch v := value.(type) {
	case string:
		return observability.String(key, v)
	case int:
		return observability.Int(key, v)
	case int32:
		return observability.Int(key, int(v))
	case int64:
		return observability.Int(key, int(v))
	case bool:
		return observability.Bool(key, v)
	case error:
		return observability.String(key, v.Error())
	default:
		return observability.String(key, fmt.Sprintf("%v", v))
	}
}
