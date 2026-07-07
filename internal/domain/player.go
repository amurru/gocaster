package domain

import "context"

type PlaybackState string

const (
	PlaybackStateStopped PlaybackState = "stopped"
	PlaybackStatePlaying PlaybackState = "playing"
	PlaybackStatePaused  PlaybackState = "paused"
	PlaybackStateError   PlaybackState = "error"
)

type PlaybackStatus struct {
	State       PlaybackState
	PositionSec float64
	DurationSec float64
	ProgressPct float64
	Source      string
	CanSeek     bool
	LastError   string
}

// Player is the audio playback port. Every method takes a context.Context so
// the caller can cancel in-flight operations, set deadlines, and propagate
// request-scoped tracing (issue #11).
type Player interface {
	Play(ctx context.Context, source string) error
	Stop(ctx context.Context) error
	IsPlaying(ctx context.Context) bool

	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	TogglePause(ctx context.Context) error

	Seek(ctx context.Context, seconds float64) error

	Status(ctx context.Context) (PlaybackStatus, error)

	Close(ctx context.Context) error
}

type PlaybackMetadata struct {
	EpisodeTitle  string
	PodcastTitle  string
	Source        string
	DurationSec   float64
	PositionSec   float64
	CanSeek       bool
	CanGoNext     bool
	CanGoPrevious bool
}

// PlaybackController is the inbound control surface exposed to broadcasters
// (e.g. MPRIS media keys). Every method takes a context.Context so external
// callers are bound by the same cancellation/deadline semantics as the TUI
// (issue #11).
type PlaybackController interface {
	Play(ctx context.Context, episodeID int64) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	PlayPause(ctx context.Context) error
	Stop(ctx context.Context) error
	SeekTo(ctx context.Context, positionSec float64) error
	Status(ctx context.Context) (PlaybackStatus, error)
}

// PlaybackBroadcaster fans playback state out to external surfaces (MPRIS,
// Discord). Every method takes a context.Context so publishes can be cancelled
// on shutdown (issue #11).
type PlaybackBroadcaster interface {
	PublishState(ctx context.Context, state PlaybackState, metadata PlaybackMetadata) error
	PublishPosition(ctx context.Context, positionSec float64, durationSec float64) error
	Close(ctx context.Context) error

	SetController(ctx context.Context, controller PlaybackController)
}
