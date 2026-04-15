package application

import (
	"fmt"

	"github.com/amurru/gocaster/internal/domain"
)

type PlayerService struct {
	repo   domain.PodcastRepository
	player domain.Player
}

func NewPlayerService(repo domain.PodcastRepository, player domain.Player) *PlayerService {
	return &PlayerService{
		repo:   repo,
		player: player,
	}
}

// PlayEpisode decides how to play an episode and updates domain state.
func (s *PlayerService) PlayEpisode(episodeID int64) error {
	episode, err := s.repo.FindEpisodeByID(episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	// Determine source
	// If episode is downloaded, play the local file for reliability/speed.
	// Otherwise, stream from the remote URL.
	source := episode.AudioURL
	if episode.IsDownloaded && episode.LocalPath != "" {
		source = episode.LocalPath
	}

	// Delegate actual playback to the Player interface
	if err := s.player.Play(source); err != nil {
		return fmt.Errorf("player failed: %w", err)
	}

	// Mark as played
	// TODO: make configurable e.g. mark as played after 80% of the length
	// now: starting playback counts as played
	if !episode.IsPlayed {
		episode.IsPlayed = true
		if err := s.repo.SaveEpisode(episode); err != nil {
			// log error, don't stop playback just because db update failed
			fmt.Printf("Warning: failed to mark episode as played: %v\n", err)
		}
	}
	return nil
}

// StopPlayback stops the current audio.
func (s *PlayerService) StopPlayback() error {
	return s.player.Stop()
}

// TogglePlayPause toggles state if the player supports it.
func (s *PlayerService) TogglePlayPause() error {
	// TODO: implement
	return nil
}
