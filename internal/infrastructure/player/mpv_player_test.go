package player

import (
	"context"
	"testing"
)

func TestMPVPlayer_IsPlaying(t *testing.T) {
	ctx := context.Background()
	player := NewMPVPlayer()

	if player.IsPlaying(ctx) {
		t.Error("IsPlaying should return false when no media loaded")
	}

	if err := player.Close(ctx); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestMPVPlayer_AudioOutputOption verifies the audio-output option is stored on
// the player and defaults to "auto" (mpv autodetect) when not specified. This
// guards against regressing the hardcoded ao=pulse that broke non-PulseAudio
// systems (issue #4).
func TestMPVPlayer_AudioOutputOption(t *testing.T) {
	// Default: auto-detection, no hardcoded backend.
	p1 := NewMPVPlayer()
	mpv1 := p1.(*MPVPlayer)
	if mpv1.audioOutput != "auto" {
		t.Errorf("default audioOutput = %q, want %q", mpv1.audioOutput, "auto")
	}
	_ = p1.Close(context.Background())

	// Explicit backend is honored.
	p2 := NewMPVPlayer(WithAudioOutput("alsa"))
	mpv2 := p2.(*MPVPlayer)
	if mpv2.audioOutput != "alsa" {
		t.Errorf("audioOutput = %q, want %q", mpv2.audioOutput, "alsa")
	}
	_ = p2.Close(context.Background())

	// Whitespace is trimmed.
	p3 := NewMPVPlayer(WithAudioOutput("  pipewire  "))
	mpv3 := p3.(*MPVPlayer)
	if mpv3.audioOutput != "pipewire" {
		t.Errorf("audioOutput = %q, want %q", mpv3.audioOutput, "pipewire")
	}
	_ = p3.Close(context.Background())
}

// TestToFloat64_NoPanic covers issue #3: toFloat64 must not panic on any of the
// dynamic types libmpv may return (float64, int64, string, nil, ...), coercing
// to the value or falling back to 0.
func TestToFloat64_NoPanic(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
	}{
		{"float64", 12.5, 12.5},
		{"float32", float32(2.5), 2.5},
		{"int64", int64(100), 100},
		{"int", 7, 7},
		{"string non-numeric", "not a number", 0},
		{"nil", nil, 0},
		{"bool", true, 0},
		{"[]byte", []byte("123"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toFloat64(tt.in)
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
