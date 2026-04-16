package player

import (
	"testing"
)

func TestMPVPlayer_IsPlaying(t *testing.T) {
	player := NewMPVPlayer()

	if player.IsPlaying() {
		t.Error("IsPlaying should return false when no media loaded")
	}

	if err := player.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
