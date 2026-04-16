//go:build !linux

package system

import "github.com/amurru/gocaster/internal/domain"

type stubBroadcaster struct{}

func NewStubBroadcaster() domain.PlaybackBroadcaster {
	return &stubBroadcaster{}
}

func (b *stubBroadcaster) PublishState(state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	return nil
}

func (b *stubBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	return nil
}

func (b *stubBroadcaster) Close() error {
	return nil
}

func (b *stubBroadcaster) SetController(controller domain.PlaybackController) {
}
