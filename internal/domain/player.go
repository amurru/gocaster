package domain

// Player is the "Port" for audio playback capabilities.
// The Application layer uses this to control media without knowing if it's MPV, VLC, or internal.
type Player interface {
	// Play starts playback of the given media (file path or URL)
	Play(mediaPath string) error

	// Stop halts playback
	Stop() error

	// IsPlaying returns the current state.
	IsPlaying() bool
}
