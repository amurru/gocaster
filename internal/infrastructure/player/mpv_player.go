package player

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/gen2brain/go-mpv"
)

type MPVPlayer struct {
	mu          sync.Mutex
	mpv         *mpv.Mpv
	source      string
	audioOutput string
	logger      domain.Logger
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

// WithLogger injects a structured logger used for debug diagnostics, replacing
// the previous magic DEBUG-env-var-gated playerDebugf that wrote to stdout and
// corrupted the TUI (issue #14). When unset, diagnostics are discarded.
func WithLogger(logger domain.Logger) MPVOption {
	return func(p *MPVPlayer) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// debug emits a player diagnostic via the injected logger (issue #14). It is a
// no-op when no logger is wired in, preserving the previous opt-in behavior
// without the magic DEBUG env var or stdout writes.
func (p *MPVPlayer) debug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
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
		p.debug("mpv.New() returned nil")
		return
	}

	p.debug("mpv client created", "api_version", p.mpv.APIVersion())

	// Audio-only config. Surface set-option errors instead of discarding them
	// so a misconfigured backend is diagnosable rather than silently broken.
	if err := p.mpv.SetOptionString("vo", "null"); err != nil {
		p.debug("vo=null failed", "err", err)
	}
	// Only pin ao when the user explicitly configures a backend; "auto" lets
	// mpv autodetect (the safe default across PulseAudio/PipeWire/ALSA/JACK/
	// macOS/Windows). Hardcoding ao=pulse broke playback on non-PulseAudio
	// systems (issue #4).
	if p.audioOutput != "" && p.audioOutput != "auto" {
		if err := p.mpv.SetOptionString("ao", p.audioOutput); err != nil {
			p.debug("ao option failed", "ao", p.audioOutput, "err", err)
		}
	}
	if err := p.mpv.SetOptionString("idle", "yes"); err != nil {
		p.debug("idle=yes failed", "err", err)
	}
	if err := p.mpv.SetOptionString("keep-open", "yes"); err != nil {
		p.debug("keep-open=yes failed", "err", err)
	}

	if err := p.mpv.Initialize(); err != nil {
		p.debug("Initialize failed", "err", err)
		p.mpv = nil
		return
	}

	p.debug("mpv initialized successfully")
}

func (p *MPVPlayer) Play(ctx context.Context, source string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Re-check after acquiring the lock: cancellation may have arrived while
	// we were waiting for it (CodeRabbit review of PR #41).
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	p.debug("loading source", "source", source)

	// Use Command with array - more reliable than CommandString
	err := p.mpv.Command([]string{"loadfile", source, "replace"})
	if err != nil {
		p.debug("loadfile error", "source", source, "err", err)
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Unpause if needed
	if err := p.mpv.SetPropertyString("pause", "no"); err != nil {
		p.debug("SetPropertyString pause error", "err", err)
	}

	p.source = source
	p.debug("playback started")
	return nil
}

func (p *MPVPlayer) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.mpv == nil {
		return nil
	}

	_ = p.mpv.Command([]string{"stop"})
	p.source = ""
	return nil
}

func (p *MPVPlayer) IsPlaying(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Re-check after acquiring the lock (CodeRabbit review of PR #41).
	if ctx.Err() != nil {
		return false
	}

	if p.mpv == nil || p.source == "" {
		return false
	}

	pause := p.mpv.GetPropertyString("pause")
	return pause != "yes"
}

func (p *MPVPlayer) Pause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.SetPropertyString("pause", "yes"); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}
	return nil
}

func (p *MPVPlayer) Resume(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.SetPropertyString("pause", "no"); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}
	return nil
}

func (p *MPVPlayer) TogglePause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

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

func (p *MPVPlayer) Seek(ctx context.Context, seconds float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if p.mpv == nil {
		return errors.New("libmpv not available")
	}

	if err := p.mpv.Command([]string{"seek", fmt.Sprintf("%f", seconds), "relative"}); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	return nil
}

func (p *MPVPlayer) Status(ctx context.Context) (domain.PlaybackStatus, error) {
	if err := ctx.Err(); err != nil {
		return domain.PlaybackStatus{State: domain.PlaybackStateError, LastError: err.Error()}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Re-check after acquiring the lock (CodeRabbit review of PR #41).
	if err := ctx.Err(); err != nil {
		return domain.PlaybackStatus{State: domain.PlaybackStateError, LastError: err.Error()}, err
	}

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

func (p *MPVPlayer) Close(ctx context.Context) error {
	// Shutdown is unconditional: a cancelled context must not prevent
	// TerminateDestroy from running (otherwise the libmpv handle leaks), so we
	// intentionally do not check ctx.Err() here. The ctx parameter is kept for
	// interface conformance and future tracing (issue #11).
	_ = ctx

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil {
		p.mpv.TerminateDestroy()
		p.mpv = nil
	}
	return nil
}
