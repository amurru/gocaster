package system

import "github.com/amurru/gocaster/internal/domain"

// noOpBroadcaster is a PlaybackBroadcaster that discards every call. It is the
// fallback used on non-Linux platforms (mpris_broadcaster_default.go) and when
// MPRIS setup fails on Linux (mpris_broadcaster_linux.go). The definition lives
// here, in a build-tag-free file, so both build paths share one implementation
// instead of duplicating it (issue #16).
type noOpBroadcaster struct{}

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
