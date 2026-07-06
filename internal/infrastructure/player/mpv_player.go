package player

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/gen2brain/go-mpv"
)

var playerDebugEnabled = os.Getenv("DEBUG") != ""

func playerDebugf(format string, args ...any) {
	if playerDebugEnabled {
		fmt.Printf("[player] "+format+"\n", args...)
	}
}

type MPVPlayer struct {
	mu          sync.Mutex
	mpv         *mpv.Mpv
	source      string
	audioOutput string
}

// MPVOption configures an MPVPlayer at construction.
type MPVOption func(*MPVPlayer)

// WithAudioOutput sets the mpv audio output backend ("ao" option). "auto" (the
// default) leaves ao unset so mpv autodetects the best backend for the system;
// any other value (e.g. "pulse", "pipewire", "alsa", "jack", "coreaudio",
// "null") is passed through verbatim. This replaces the previous hardcoded
// ao=pulse that broke playback on non-PulseAudio systems (issue #4).
func WithAudioOutput(ao string) MPVOption {
	return func(p *MPVPlayer) {
		p.audioOutput = strings.TrimSpace(ao)
	}
}

func NewMPVPlayer(opts ...MPVOption) domain.Player {
	p := &MPVPlayer{audioOutput: "auto"}
	for _, opt := range opts {
		opt(p)
	}
	p.initMPV()
	return p
}

func (p *MPVPlayer) initMPV() {
	p.mpv = mpv.New()
	if p.mpv == nil {
		playerDebugf("mpv.New() returned nil")
		return
	}

	playerDebugf("mpv client created, API version: %d", p.mpv.APIVersion())

	// Audio-only config. Surface set-option errors instead of discarding them
	// so a misconfigured backend is diagnosable rather than silently broken.
	if err := p.mpv.SetOptionString("vo", "null"); err != nil {
		playerDebugf("vo=null failed: %v", err)
	}
	// Only pin ao when the user explicitly configures a backend; "auto" lets
	// mpv autodetect (the safe default across PulseAudio/PipeWire/ALSA/JACK/
	// macOS/Windows). Hardcoding ao=pulse broke playback on non-PulseAudio
	// systems (issue #4).
	if p.audioOutput != "" && p.audioOutput != "auto" {
		if err := p.mpv.SetOptionString("ao", p.audioOutput); err != nil {
			playerDebugf("ao=%s failed: %v", p.audioOutput, err)
		}
	}
	if err := p.mpv.SetOptionString("idle", "yes"); err != nil {
		playerDebugf("idle=yes failed: %v", err)
	}
	if err := p.mpv.SetOptionString("keep-open", "yes"); err != nil {
		playerDebugf("keep-open=yes failed: %v", err)
	}

	if err := p.mpv.Initialize(); err != nil {
		playerDebugf("Initialize failed: %v", err)
		p.mpv = nil
		return
	}

	playerDebugf("mpv initialized successfully")
}

func (p *MPVPlayer) Play(source string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	playerDebugf("Loading: %s", source)

	// Use Command with array - more reliable than CommandString
	err := p.mpv.Command([]string{"loadfile", source, "replace"})
	if err != nil {
		playerDebugf("loadfile error: %v (source=%q)", err, source)
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Unpause if needed
	if err := p.mpv.SetPropertyString("pause", "no"); err != nil {
		playerDebugf("SetPropertyString pause error: %v", err)
	}

	p.source = source
	playerDebugf("Playback started")
	return nil
}

func (p *MPVPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return nil
	}

	_ = p.mpv.Command([]string{"stop"})
	p.source = ""
	return nil
}

func (p *MPVPlayer) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil || p.source == "" {
		return false
	}

	pause := p.mpv.GetPropertyString("pause")
	return pause != "yes"
}

func (p *MPVPlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.SetPropertyString("pause", "yes"); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}
	return nil
}

func (p *MPVPlayer) Resume() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.SetPropertyString("pause", "no"); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}
	return nil
}

func (p *MPVPlayer) TogglePause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	pause := p.mpv.GetPropertyString("pause")

	newPause := "yes"
	if pause == "yes" {
		newPause = "no"
	}

	if err := p.mpv.SetPropertyString("pause", newPause); err != nil {
		return fmt.Errorf("failed to toggle pause: %w", err)
	}
	return nil
}

func (p *MPVPlayer) Seek(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.Command([]string{"seek", fmt.Sprintf("%f", seconds), "relative"}); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	return nil
}

func (p *MPVPlayer) Status() (domain.PlaybackStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv == nil {
		return domain.PlaybackStatus{
			State:     domain.PlaybackStateError,
			LastError: "libmpv not available",
		}, nil
	}

	status := domain.PlaybackStatus{
		Source:  p.source,
		CanSeek: true,
	}

	pause := p.mpv.GetPropertyString("pause")
	if pause == "yes" {
		status.State = domain.PlaybackStatePaused
	} else if p.source != "" {
		status.State = domain.PlaybackStatePlaying
	} else {
		status.State = domain.PlaybackStateStopped
	}

	// Use the two-value type assertion form: libmpv IPC can return int64,
	// string, or nil for a property depending on mpv version and playback
	// state, and the single-value assertion panics on any non-float64 dynamic
	// type (issue #3). toFloat64 coerces safely, falling back to 0.
	if pos, err := p.mpv.GetProperty("time-pos", mpv.FormatDouble); err == nil {
		status.PositionSec = toFloat64(pos)
	}

	if dur, err := p.mpv.GetProperty("duration", mpv.FormatDouble); err == nil {
		status.DurationSec = toFloat64(dur)
	}

	if status.DurationSec > 0 {
		status.ProgressPct = (status.PositionSec / status.DurationSec) * 100
	}

	return status, nil
}

// toFloat64 coerces a libmpv property value to float64 without panicking.
// mpv most commonly returns float64 for numeric properties, but some
// versions/states return int64 (or, rarely, a numeric string). Any value that
// is not coercible yields 0. This replaces unchecked single-value assertions
// that panicked the whole app on a non-float64 dynamic type (issue #3).
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

func (p *MPVPlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil {
		p.mpv.TerminateDestroy()
		p.mpv = nil
	}
	return nil
}
