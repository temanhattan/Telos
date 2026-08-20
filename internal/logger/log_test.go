package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  zerolog.Level
	}{
		{"DebugLevel", DebugLevel, zerolog.DebugLevel},
		{"InfoLevel", InfoLevel, zerolog.InfoLevel},
		{"WarnLevel", WarnLevel, zerolog.WarnLevel},
		{"ErrorLevel", ErrorLevel, zerolog.ErrorLevel},
		{"AuditLevel", AuditLevel, zerolog.InfoLevel}, // Audit is special
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(tt.level, &buf)
			zlogger := logger.(*zerologLogger)
			if zlogger.logger.GetLevel() != tt.want {
				t.Errorf("New() level = %v, want %v", zlogger.logger.GetLevel(), tt.want)
			}
		})
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, &buf)
	logger = logger.WithComponent("test-component")
	logger.Info("test message")

	if !strings.Contains(buf.String(), `"component":"test-component"`) {
		t.Errorf("log output should contain component, but it doesn't. got: %s", buf.String())
	}
}

func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, &buf)

	ctx := context.WithValue(context.Background(), correlationIDKey, "test-correlation-id")
	logger = logger.WithContext(ctx)
	logger.Info("test message")

	if !strings.Contains(buf.String(), `"correlation_id":"test-correlation-id"`) {
		t.Errorf("log output should contain correlation_id, but it doesn't. got: %s", buf.String())
	}
}

func TestLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(DebugLevel, &buf)

	logger.Debug("debug message")
	if !strings.Contains(buf.String(), `"level":"debug"`) {
		t.Errorf("log output should contain debug level, but it doesn't. got: %s", buf.String())
	}
	buf.Reset()

	logger.Info("info message")
	if !strings.Contains(buf.String(), `"level":"info"`) {
		t.Errorf("log output should contain info level, but it doesn't. got: %s", buf.String())
	}
	buf.Reset()

	logger.Warn("warn message")
	if !strings.Contains(buf.String(), `"level":"warn"`) {
		t.Errorf("log output should contain warn level, but it doesn't. got: %s", buf.String())
	}
	buf.Reset()

	err := errors.New("test error")
	logger.Error(err, "error message")
	if !strings.Contains(buf.String(), `"level":"error"`) {
		t.Errorf("log output should contain error level, but it doesn't. got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"error":"test error"`) {
		t.Errorf("log output should contain error message, but it doesn't. got: %s", buf.String())
	}
}

func TestAudit(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, &buf)
	logger.Audit("audit message")

	if !strings.Contains(buf.String(), `"severity":"audit"`) {
		t.Errorf("log output should contain audit severity, but it doesn't. got: %s", buf.String())
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, &buf) // Passing buffer directly for JSON output
	logger.Info("test message")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	if err != nil {
		t.Fatalf("failed to unmarshal log output: %v", err)
	}

	if logEntry["level"] != "info" {
		t.Errorf("expected level to be 'info', got '%v'", logEntry["level"])
	}
	if logEntry["message"] != "test message" {
		t.Errorf("expected message to be 'test message', got '%v'", logEntry["message"])
	}
}
