package agent

import (
	"testing"
)

func TestThinkingOptions(t *testing.T) {
	tests := []struct {
		mode    string
		wantLen int
	}{
		{"", 0},
		{"none", 0},
		{"low", 1},
		{"medium", 1},
		{"high", 1},
		{"auto", 1},
		{"Low", 1},   // case-insensitive
		{"HIGH", 1},  // case-insensitive
		{"bogus", 0}, // unknown mode returns nil
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			opts := thinkingOptions(tt.mode)
			if len(opts) != tt.wantLen {
				t.Errorf("thinkingOptions(%q) returned %d options, want %d", tt.mode, len(opts), tt.wantLen)
			}
		})
	}
}
