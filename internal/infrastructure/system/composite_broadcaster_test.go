package system

import (
	"errors"
	"testing"

	"github.com/amurru/gocaster/internal/domain"
)

type mockBroadcaster struct {
	stateCalls      int
	positionCalls   int
	closeCalls      int
	controllerCalls int
	stateErr        error
	positionErr     error
	closeErr        error
}

func (m *mockBroadcaster) PublishState(domain.PlaybackState, domain.PlaybackMetadata) error {
	m.stateCalls++
	return m.stateErr
}

func (m *mockBroadcaster) PublishPosition(float64, float64) error {
	m.positionCalls++
	return m.positionErr
}

func (m *mockBroadcaster) Close() error {
	m.closeCalls++
	return m.closeErr
}

func (m *mockBroadcaster) SetController(domain.PlaybackController) {
	m.controllerCalls++
}

func TestCompositeBroadcasterFanOut(t *testing.T) {
	first := &mockBroadcaster{}
	second := &mockBroadcaster{}
	composite := NewCompositeBroadcaster(first, second)

	composite.SetController(nil)
	if err := composite.PublishState(domain.PlaybackStatePlaying, domain.PlaybackMetadata{}); err != nil {
		t.Fatalf("PublishState failed: %v", err)
	}
	if err := composite.PublishPosition(10, 100); err != nil {
		t.Fatalf("PublishPosition failed: %v", err)
	}
	if err := composite.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if first.stateCalls != 1 || second.stateCalls != 1 {
		t.Fatalf("expected state fanout, got first=%d second=%d", first.stateCalls, second.stateCalls)
	}
	if first.positionCalls != 1 || second.positionCalls != 1 {
		t.Fatalf("expected position fanout, got first=%d second=%d", first.positionCalls, second.positionCalls)
	}
	if first.closeCalls != 1 || second.closeCalls != 1 {
		t.Fatalf("expected close fanout, got first=%d second=%d", first.closeCalls, second.closeCalls)
	}
	if first.controllerCalls != 1 || second.controllerCalls != 1 {
		t.Fatalf(
			"expected controller fanout, got first=%d second=%d",
			first.controllerCalls,
			second.controllerCalls,
		)
	}
}

func TestCompositeBroadcasterAggregatesErrors(t *testing.T) {
	first := &mockBroadcaster{stateErr: errors.New("state one")}
	second := &mockBroadcaster{stateErr: errors.New("state two")}
	composite := NewCompositeBroadcaster(first, second)

	err := composite.PublishState(domain.PlaybackStatePlaying, domain.PlaybackMetadata{})
	if err == nil {
		t.Fatal("expected aggregated error from state publish")
	}

	if !errors.Is(err, first.stateErr) {
		t.Fatal("expected first error to be present")
	}
	if !errors.Is(err, second.stateErr) {
		t.Fatal("expected second error to be present")
	}
}
