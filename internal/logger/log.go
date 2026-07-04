package log

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Level defines log levels.
type Level int8

const (
	// DebugLevel defines debug log level.
	DebugLevel Level = iota
	// InfoLevel defines info log level.
	InfoLevel
	// WarnLevel defines warn log level.
	WarnLevel
	// ErrorLevel defines error log level.
	ErrorLevel
	// AuditLevel defines a security-critical log level.
	AuditLevel
)

// Logger defines the interface for the logging subsystem.
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(err error, msg string)
	Audit(msg string)
	WithComponent(component string) Logger
	WithContext(ctx context.Context) Logger
}

// zerologLogger is the zerolog implementation of the Logger interface.
type zerologLogger struct {
	logger zerolog.Logger
}

// New creates a new Logger.
func New(level Level, w io.Writer) Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	var zlevel zerolog.Level
	switch level {
	case DebugLevel:
		zlevel = zerolog.DebugLevel
	case InfoLevel:
		zlevel = zerolog.InfoLevel
	case WarnLevel:
		zlevel = zerolog.WarnLevel
	case ErrorLevel:
		zlevel = zerolog.ErrorLevel
	case AuditLevel:
		zlevel = zerolog.InfoLevel // Audit is a special case
	default:
		zlevel = zerolog.InfoLevel
	}

	output := w
	if w == nil {
		output = os.Stdout
	}

	// For human-readable output, use ConsoleWriter
	if _, ok := output.(*os.File); ok {
		output = zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}
	}

	logger := zerolog.New(output).Level(zlevel).With().Timestamp().Logger()

	return &zerologLogger{logger: logger}
}

// WithComponent adds a component to the logger's context.
func (l *zerologLogger) WithComponent(component string) Logger {
	newLogger := l.logger.With().Str("component", component).Logger()
	return &zerologLogger{logger: newLogger}
}

// WithContext adds fields from a context to the logger.
func (l *zerologLogger) WithContext(ctx context.Context) Logger {
	// Assuming correlation_id is passed in the context
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		newLogger := l.logger.With().Str("correlation_id", correlationID).Logger()
		return &zerologLogger{logger: newLogger}
	}
	return l
}

// Debug logs a message at DebugLevel.
func (l *zerologLogger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

// Info logs a message at InfoLevel.
func (l *zerologLogger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

// Warn logs a message at WarnLevel.
func (l *zerologLogger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

// Error logs a message at ErrorLevel.
func (l *zerologLogger) Error(err error, msg string) {
	l.logger.Error().Err(err).Msg(msg)
}

// Audit logs a message at a special "audit" level.
func (l *zerologLogger) Audit(msg string) {
	l.logger.Log().Str("severity", "audit").Msg(msg)
}
