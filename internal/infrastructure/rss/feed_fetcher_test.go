package rss

import (
	"testing"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"HH:MM:SS format", "2:09:56", 7796},
		{"MM:SS format", "45:30", 2730},
		{"Empty string", "", 0},
		{"Single value", "30", 30},
		{"Zero duration", "0:00:00", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDuration(tt.input)
			if result != tt.expected {
				t.Errorf("parseDuration(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
