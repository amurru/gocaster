//go:build !linux

package system

import "github.com/amurru/gocaster/internal/domain"

// NewMPRISBroadcaster returns a no-op broadcaster on non-Linux platforms.
// MPRIS/D-Bus is Linux-only; everywhere else playback state is simply not
// published. The shared noOpBroadcaster type is defined in mpris_broadcaster.go.
func NewMPRISBroadcaster() (domain.PlaybackBroadcaster, error) {
	return &noOpBroadcaster{}, nil
}
