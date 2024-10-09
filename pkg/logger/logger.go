package logger

type Level uint

const (
	InfoLevel Level = iota
	ErrorLevel
	WarnLevel
	DebugLevel
)

type Field struct {
	Key   string
	Value any
}

type Logger interface {
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
}
