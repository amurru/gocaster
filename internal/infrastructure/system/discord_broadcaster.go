package system

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	discordclient "github.com/hugolgst/rich-go/client"
)

type discordBroadcaster struct {
	mu sync.Mutex

	clientID string
	loggedIn bool

	state       domain.PlaybackState
	metadata    domain.PlaybackMetadata
	positionSec float64
	durationSec float64
}

func NewDiscordBroadcaster(clientID string) (domain.PlaybackBroadcaster, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fmt.Errorf("discord client ID is required")
	}
	return &discordBroadcaster{clientID: clientID}, nil
}

func (b *discordBroadcaster) PublishState(
	state domain.PlaybackState,
	metadata domain.PlaybackMetadata,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = state
	b.metadata = metadata
	if metadata.DurationSec > 0 {
		b.durationSec = metadata.DurationSec
	}
	if metadata.PositionSec > 0 {
		b.positionSec = metadata.PositionSec
	}

	if state == domain.PlaybackStateStopped || state == domain.PlaybackStateError {
		return b.clearActivityLocked()
	}

	return b.publishActivityLocked()
}

func (b *discordBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.positionSec = positionSec
	if durationSec > 0 {
		b.durationSec = durationSec
	}

	if b.state != domain.PlaybackStatePlaying {
		return nil
	}
	return b.publishActivityLocked()
}

func (b *discordBroadcaster) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.loggedIn {
		discordclient.Logout()
		b.loggedIn = false
	}
	return nil
}

func (b *discordBroadcaster) SetController(controller domain.PlaybackController) {}

func (b *discordBroadcaster) ensureLoggedInLocked() error {
	if b.loggedIn {
		return nil
	}
	if err := discordclient.Login(b.clientID); err != nil {
		return err
	}
	b.loggedIn = true
	return nil
}

func (b *discordBroadcaster) clearActivityLocked() error {
	if !b.loggedIn {
		return nil
	}

	if err := discordclient.SetActivity(discordclient.Activity{}); err != nil {
		b.loggedIn = false
		return err
	}
	return nil
}

func (b *discordBroadcaster) publishActivityLocked() error {
	if err := b.ensureLoggedInLocked(); err != nil {
		return err
	}

	details := strings.TrimSpace(b.metadata.EpisodeTitle)
	if details == "" {
		details = "Listening on Gocaster"
	}
	stateText := strings.TrimSpace(b.metadata.PodcastTitle)
	if stateText == "" {
		stateText = "Podcast"
	}

	activity := discordclient.Activity{
		Details:   details,
		State:     stateText,
		LargeText: "Gocaster",
		SmallText: strings.ToUpper(string(b.state)),
	}

	if b.state == domain.PlaybackStatePlaying && b.durationSec > 0 {
		start := time.Now().Add(-time.Duration(b.positionSec * float64(time.Second)))
		end := start.Add(time.Duration(b.durationSec * float64(time.Second)))
		activity.Timestamps = &discordclient.Timestamps{
			Start: &start,
			End:   &end,
		}
	}

	if err := discordclient.SetActivity(activity); err != nil {
		b.loggedIn = false
		return err
	}
	return nil
}
