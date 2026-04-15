// internal/infrastructure/player/mpv_player.go
package player

import (
	"os/exec"
	"syscall"

	"github.com/amurru/gocaster/internal/domain"
)

type MPVPlayer struct {
	cmd *exec.Cmd
}

func NewMPVPlayer() domain.Player {
	return &MPVPlayer{}
}

func (p *MPVPlayer) Play(source string) error {
	// stop any existing playback
	_ = p.Stop()

	// Execute mpv in the background
	// --no-video ensures it acts as an audio player
	// --force-window=no ensures it doesn't pop up a GUI window
	p.cmd = exec.Command("mpv", "--no-video", "--force-window=no", source)

	// Start the process without waiting for it to finish
	return p.cmd.Start()
}

func (p *MPVPlayer) Stop() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *MPVPlayer) IsPlaying() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	// Check if process is still running by sending signal 0 (doesn't actually send signal, just checks if process exists)
	err := p.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}
