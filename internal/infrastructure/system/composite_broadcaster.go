package system

import (
	"errors"

	"github.com/amurru/gocaster/internal/domain"
)

type compositeBroadcaster struct {
	broadcasters []domain.PlaybackBroadcaster
}

func NewCompositeBroadcaster(broadcasters ...domain.PlaybackBroadcaster) domain.PlaybackBroadcaster {
	filtered := make([]domain.PlaybackBroadcaster, 0, len(broadcasters))
	for _, b := range broadcasters {
		if b != nil {
			filtered = append(filtered, b)
		}
	}
	return &compositeBroadcaster{broadcasters: filtered}
}

func (b *compositeBroadcaster) PublishState(
	state domain.PlaybackState,
	metadata domain.PlaybackMetadata,
) error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.PublishState(state, metadata); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.PublishPosition(positionSec, durationSec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) Close() error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) SetController(controller domain.PlaybackController) {
	for _, broadcaster := range b.broadcasters {
		broadcaster.SetController(controller)
	}
}
