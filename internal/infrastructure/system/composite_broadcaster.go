package system

import (
	"context"
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
	ctx context.Context,
	state domain.PlaybackState,
	metadata domain.PlaybackMetadata,
) error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.PublishState(ctx, state, metadata); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) PublishPosition(ctx context.Context, positionSec float64, durationSec float64) error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.PublishPosition(ctx, positionSec, durationSec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) Close(ctx context.Context) error {
	var errs []error
	for _, broadcaster := range b.broadcasters {
		if err := broadcaster.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *compositeBroadcaster) SetController(ctx context.Context, controller domain.PlaybackController) {
	for _, broadcaster := range b.broadcasters {
		broadcaster.SetController(ctx, controller)
	}
}
