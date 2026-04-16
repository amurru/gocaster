package application

import (
	"fmt"
	"os"

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

func (s *PlayerService) PlayEpisode(episodeID int64) error {
	episode, err := s.repo.FindEpisodeByID(episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	source := resolvePlaybackSource(*episode)

	if err := s.player.Play(source); err != nil {
		return fmt.Errorf("player failed: %w", err)
	}

	if !episode.IsPlayed {
		episode.IsPlayed = true
		if err := s.repo.UpdateEpisodePlaybackState(episode.ID, true); err != nil {
			fmt.Printf("Warning: failed to mark episode as played: %v\n", err)
		}
	}
	return nil
}

func resolvePlaybackSource(episode domain.Episode) string {
	if episode.IsDownloaded && episode.LocalPath != "" {
		if _, err := os.Stat(episode.LocalPath); err == nil {
			return episode.LocalPath
		}
	}

	return episode.AudioURL
}

func (s *PlayerService) StopPlayback() error {
	return s.player.Stop()
}

func (s *PlayerService) TogglePlayPause() error {
	return s.player.TogglePause()
}

func (s *PlayerService) Pause() error {
	return s.player.Pause()
}

func (s *PlayerService) Resume() error {
	return s.player.Resume()
}

func (s *PlayerService) Seek(seconds float64) error {
	return s.player.Seek(seconds)
}

func (s *PlayerService) PlaybackStatus() (domain.PlaybackStatus, error) {
	return s.player.Status()
}

func (s *PlayerService) Close() error {
	return s.player.Close()
}
