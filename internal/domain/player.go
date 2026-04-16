package domain

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

type Player interface {
	Play(source string) error
	Stop() error
	IsPlaying() bool

	Pause() error
	Resume() error
	TogglePause() error

	Seek(seconds float64) error

	Status() (PlaybackStatus, error)

	Close() error
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

type PlaybackController interface {
	Play(episodeID int64) error
	Pause() error
	Resume() error
	PlayPause() error
	Stop() error
	SeekTo(positionSec float64) error
	Status() (PlaybackStatus, error)
}

type PlaybackBroadcaster interface {
	PublishState(state PlaybackState, metadata PlaybackMetadata) error
	PublishPosition(positionSec float64, durationSec float64) error
	Close() error

	SetController(controller PlaybackController)
}
