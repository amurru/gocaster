// internal/infrastructure/player/mpv_player_test.go
package player

import (
	"testing"
)

func TestMPVPlayer_IsPlaying(t *testing.T) {
	player := NewMPVPlayer().(*MPVPlayer)

	// Test: not playing when nothing has been played
	if player.IsPlaying() {
		t.Error("IsPlaying should return false when no media loaded")
	}

	// Note: We can't easily test actual playback without a real audio file
	// This test documents the expected behavior
}
