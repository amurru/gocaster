package application

import (
	"fmt"
	"os"
	"sync"

	"github.com/amurru/gocaster/internal/domain"
)

type PlayerService struct {
	mu          sync.RWMutex
	repo        domain.PodcastRepository
	player      domain.Player
	broadcaster domain.PlaybackBroadcaster

	currentEpisode *domain.Episode
	currentPodcast *domain.Podcast
	lastEpisodeID  int64
}

func NewPlayerService(repo domain.PodcastRepository, player domain.Player, broadcaster domain.PlaybackBroadcaster) *PlayerService {
	svc := &PlayerService{
		repo:        repo,
		player:      player,
		broadcaster: broadcaster,
	}

	if broadcaster != nil {
		broadcaster.SetController(svc)
	}

	return svc
}

func (s *PlayerService) PlayEpisode(episodeID int64) error {
	episode, err := s.repo.FindEpisodeByID(episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	podcast, err := s.repo.FindByID(episode.PodcastID)
	if err != nil {
		return fmt.Errorf("could not find podcast: %w", err)
	}

	source := resolvePlaybackSource(*episode)

	if err := s.player.Play(source); err != nil {
		return fmt.Errorf("player failed: %w", err)
	}

	s.mu.Lock()
	s.currentEpisode = episode
	s.currentPodcast = podcast
	s.lastEpisodeID = episodeID
	s.mu.Unlock()

	s.broadcastState()

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
	if err := s.player.Stop(); err != nil {
		return err
	}

	s.mu.Lock()
	s.currentEpisode = nil
	s.currentPodcast = nil
	s.mu.Unlock()

	s.broadcastState()
	return nil
}

func (s *PlayerService) TogglePlayPause() error {
	if err := s.player.TogglePause(); err != nil {
		return err
	}

	s.broadcastState()
	return nil
}

func (s *PlayerService) Pause() error {
	if err := s.player.Pause(); err != nil {
		return err
	}

	s.broadcastState()
	return nil
}

func (s *PlayerService) Resume() error {
	if err := s.player.Resume(); err != nil {
		return err
	}

	s.broadcastState()
	return nil
}

func (s *PlayerService) Seek(seconds float64) error {
	return s.player.Seek(seconds)
}

func (s *PlayerService) SeekTo(seconds float64) error {
	return s.player.Seek(seconds)
}

func (s *PlayerService) Status() (domain.PlaybackStatus, error) {
	return s.player.Status()
}

func (s *PlayerService) PlaybackStatus() (domain.PlaybackStatus, error) {
	return s.Status()
}

func (s *PlayerService) Close() error {
	if s.broadcaster != nil {
		s.broadcaster.Close()
	}
	return s.player.Close()
}

func (s *PlayerService) broadcastState() {
	if s.broadcaster == nil {
		return
	}

	s.mu.RLock()
	episode := s.currentEpisode
	podcast := s.currentPodcast
	s.mu.RUnlock()

	status, err := s.player.Status()
	if err != nil {
		return
	}

	metadata := domain.PlaybackMetadata{
		CanSeek:       status.CanSeek,
		CanGoNext:     false,
		CanGoPrevious: false,
	}

	if episode != nil {
		metadata.EpisodeTitle = episode.Title
		metadata.Source = status.Source
		if status.DurationSec > 0 {
			metadata.DurationSec = status.DurationSec
		}
	}

	if podcast != nil {
		metadata.PodcastTitle = podcast.Title
	}

	_ = s.broadcaster.PublishState(status.State, metadata)
}

func (s *PlayerService) broadcastPosition() {
	if s.broadcaster == nil {
		return
	}

	status, err := s.player.Status()
	if err != nil {
		return
	}

	_ = s.broadcaster.PublishPosition(status.PositionSec, status.DurationSec)
}

func (s *PlayerService) Play(episodeID int64) error {
	if episodeID == 0 {
		s.mu.Lock()
		episodeID = s.lastEpisodeID
		s.mu.Unlock()

		if episodeID == 0 {
			return fmt.Errorf("no episode to play")
		}
	}

	return s.PlayEpisode(episodeID)
}

func (s *PlayerService) PlayPause() error {
	return s.TogglePlayPause()
}

func (s *PlayerService) Stop() error {
	return s.StopPlayback()
}
