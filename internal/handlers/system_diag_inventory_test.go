package handlers

import (
	"errors"
	"strings"
	"testing"
)

func TestDiagnosticCoreInventoryFinding(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		err      error
		severity string
		contains string
	}{
		{
			name:     "custom or incomplete installation with no cores",
			count:    0,
			severity: "warning",
			contains: "none installed",
		},
		{
			name:     "installed cores",
			count:    15,
			severity: "",
			contains: "",
		},
		{
			name:     "core inventory read failure",
			count:    0,
			err:      errors.New("permission denied"),
			severity: "error",
			contains: "permission denied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity, detail := diagnosticCoreInventoryFinding(tt.count, tt.err)
			if severity != tt.severity {
				t.Fatalf("severity = %q, want %q", severity, tt.severity)
			}
			if !strings.Contains(detail, tt.contains) {
				t.Fatalf("detail = %q, want to contain %q", detail, tt.contains)
			}
			if tt.severity == "" && detail != "" {
				t.Fatalf("unexpected detail for complete inventory: %q", detail)
			}
		})
	}
}
