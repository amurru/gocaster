package player

import (
	"errors"
	"fmt"
	"os"
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
	mu     sync.Mutex
	mpv    *mpv.Mpv
	source string
}

func NewMPVPlayer() domain.Player {
	p := &MPVPlayer{}
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

	// Audio-only config
	_ = p.mpv.SetOptionString("vo", "null")
	_ = p.mpv.SetOptionString("ao", "pulse")
	_ = p.mpv.SetOptionString("idle", "yes")
	_ = p.mpv.SetOptionString("keep-open", "yes")

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

	if pos, err := p.mpv.GetProperty("time-pos", mpv.FormatDouble); err == nil {
		if pos != nil {
			status.PositionSec = pos.(float64)
		}
	}

	if dur, err := p.mpv.GetProperty("duration", mpv.FormatDouble); err == nil {
		if dur != nil {
			status.DurationSec = dur.(float64)
		}
	}

	if status.DurationSec > 0 {
		status.ProgressPct = (status.PositionSec / status.DurationSec) * 100
	}

	return status, nil
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
