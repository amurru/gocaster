package system

import (
	"testing"

	"github.com/amurru/gocaster/internal/domain"
)

func TestNewDiscordBroadcasterRequiresClientID(t *testing.T) {
	b, err := NewDiscordBroadcaster("")
	if err == nil {
		t.Fatal("expected error for empty discord client ID")
	}
	if b != nil {
		t.Fatal("expected nil broadcaster for invalid client ID")
	}
}

func TestDiscordBroadcasterStoppedStateWithoutLogin(t *testing.T) {
	b, err := NewDiscordBroadcaster("123456789012345678")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	if err := b.PublishState(domain.PlaybackStateStopped, domain.PlaybackMetadata{}); err != nil {
		t.Fatalf("stopped state should not require active discord session: %v", err)
	}
}
