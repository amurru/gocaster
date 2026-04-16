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
