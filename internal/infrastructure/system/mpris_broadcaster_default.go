//go:build !linux

package system

import "github.com/amurru/gocaster/internal/domain"

type noOpBroadcaster struct{}

func NewMPRISBroadcaster() (domain.PlaybackBroadcaster, error) {
	return &noOpBroadcaster{}, nil
}

func (b *noOpBroadcaster) PublishState(state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	return nil
}

func (b *noOpBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	return nil
}

func (b *noOpBroadcaster) Close() error {
	return nil
}

func (b *noOpBroadcaster) SetController(controller domain.PlaybackController) {
}
