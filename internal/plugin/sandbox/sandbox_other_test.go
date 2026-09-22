//go:build !linux

package sandbox_test

import (
	"context"
	"os"
	"strings"
	"telos/internal/plugin/sandbox"
	"testing"
)

func TestFallbackSandboxStderrLimitValidation(t *testing.T) {
	sb := sandbox.NewFallbackSandbox(nil)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to locate test executable: %v", err)
	}

	tests := []struct {
		limit int64
		name  string
	}{
		{limit: 0, name: "zero_limit"},
		{limit: -1, name: "negative_limit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pol := &sandbox.Policy{
				Executables: []string{executable},
				StderrLimit: tc.limit,
			}
			_, err := sb.Exec(context.Background(), executable, "/", nil, pol)
			if err == nil {
				t.Fatalf("expected error for StderrLimit %d, got nil", tc.limit)
			}
			if !strings.Contains(err.Error(), "strictly positive") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}
