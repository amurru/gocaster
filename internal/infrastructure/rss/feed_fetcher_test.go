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
		// Existing formats (preserved).
		{"HH:MM:SS format", "2:09:56", 7796},
		{"MM:SS format", "45:30", 2730},
		{"Empty string", "", 0},
		{"Single value", "30", 30},
		{"Zero duration", "0:00:00", 0},

		// Seconds as integer (issue #8 acceptance criteria).
		{"seconds 3600", "3600", 3600},

		// ISO 8601 durations (issue #8) — common in iTunes-extension tags.
		{"ISO PT1H30M", "PT1H30M", 5400},
		{"ISO PT45M", "PT45M", 2700},
		{"ISO PT45M30S", "PT45M30S", 2730},
		{"ISO PT30S", "PT30S", 30},
		{"ISO PT1H30M45S", "PT1H30M45S", 5445},
		{"ISO with days P1DT2H", "P1DT2H", 86400 + 7200},

		// Malformed input must fall through to 0, not panic (issue #8).
		{"garbage letters", "abc", 0},
		{"HH:MM:SS garbage", "xx:yy:zz", 0},
		{"ISO malformed trailing digits", "PT1H30", 0},
		{"ISO malformed unknown unit", "PT1X", 0},
		{"single colon", ":", 0},
		{"four colon parts", "1:2:3:4", 0},

		// Trailing garbage must be rejected (strconv.Atoi, not permissive
		// fmt.Sscanf). "30abc" should be 0, not 30.
		{"trailing garbage seconds", "30abc", 0},
		{"trailing garbage MM:SS", "45:30xyz", 0},
		{"trailing garbage HH:MM:SS", "1:2:3junk", 0},
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
