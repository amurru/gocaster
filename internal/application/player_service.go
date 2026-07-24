package application

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/amurru/gocaster/internal/domain"
)

type PlayerService struct {
	mu          sync.RWMutex
	episodes    domain.EpisodeRepo
	podcasts    domain.PodcastRepo
	player      domain.Player
	broadcaster domain.PlaybackBroadcaster
	logger      domain.Logger
	queueSvc    *QueueService

	currentEpisode *domain.Episode
	currentPodcast *domain.Podcast
	lastEpisodeID  int64
}

// NewPlayerService accepts only the focused repository ports PlayerService
// uses — episode reads/writes and podcast reads (issue #17). The concrete
// SQLiteRepo satisfies both via the union PodcastRepository, so the composition
// root passes the same repo. logger defaults to NoopLogger when nil (issue #14).
func NewPlayerService(episodes domain.EpisodeRepo, podcasts domain.PodcastRepo, player domain.Player, broadcaster domain.PlaybackBroadcaster, logger domain.Logger, queueSvc *QueueService) *PlayerService {
	if logger == nil {
		logger = domain.NoopLogger{}
	}
	svc := &PlayerService{
		episodes:    episodes,
		podcasts:    podcasts,
		player:      player,
		broadcaster: broadcaster,
		logger:      logger,
		queueSvc:    queueSvc,
	}

	if broadcaster != nil {
		broadcaster.SetController(context.Background(), svc)
	}

	return svc
}

func (s *PlayerService) PlayEpisode(ctx context.Context, episodeID int64) error {
	episode, err := s.episodes.FindEpisodeByID(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	podcast, err := s.podcasts.FindByID(ctx, episode.PodcastID)
	if err != nil {
		return fmt.Errorf("could not find podcast: %w", err)
	}

	source := resolvePlaybackSource(*episode)

	if err := s.player.Play(ctx, source); err != nil {
		return fmt.Errorf("player failed: %w", err)
	}

	s.mu.Lock()
	s.currentEpisode = episode
	s.currentPodcast = podcast
	s.lastEpisodeID = episodeID
	s.mu.Unlock()

	s.broadcastState(ctx)

	if !episode.IsPlayed {
		episode.IsPlayed = true
		if err := s.episodes.UpdateEpisodePlaybackState(ctx, episode.ID, true); err != nil {
			s.logger.Warn("failed to mark episode as played", "episode_id", episode.ID, "err", err)
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

func (s *PlayerService) StopPlayback(ctx context.Context) error {
	if err := s.player.Stop(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	s.currentEpisode = nil
	s.currentPodcast = nil
	s.mu.Unlock()

	s.broadcastState(ctx)
	return nil
}

func (s *PlayerService) TogglePlayPause(ctx context.Context) error {
	if err := s.player.TogglePause(ctx); err != nil {
		return err
	}

	s.broadcastState(ctx)
	return nil
}

func (s *PlayerService) Pause(ctx context.Context) error {
	if err := s.player.Pause(ctx); err != nil {
		return err
	}

	s.broadcastState(ctx)
	return nil
}

func (s *PlayerService) Resume(ctx context.Context) error {
	if err := s.player.Resume(ctx); err != nil {
		return err
	}

	s.broadcastState(ctx)
	return nil
}

func (s *PlayerService) Seek(ctx context.Context, seconds float64) error {
	return s.player.Seek(ctx, seconds)
}

func (s *PlayerService) SetSpeed(ctx context.Context, speed float64) error {
	return s.player.SetSpeed(ctx, speed)
}

func (s *PlayerService) GetSpeed(ctx context.Context) float64 {
	return s.player.GetSpeed(ctx)
}

func (s *PlayerService) SeekTo(ctx context.Context, seconds float64) error {
	return s.player.Seek(ctx, seconds)
}

func (s *PlayerService) Status(ctx context.Context) (domain.PlaybackStatus, error) {
	return s.player.Status(ctx)
}

func (s *PlayerService) PlaybackStatus(ctx context.Context) (domain.PlaybackStatus, error) {
	return s.Status(ctx)
}

func (s *PlayerService) Close(ctx context.Context) error {
	if s.broadcaster != nil {
		s.broadcaster.Close(ctx)
	}
	return s.player.Close(ctx)
}

func (s *PlayerService) broadcastState(ctx context.Context) {
	if s.broadcaster == nil {
		return
	}

	s.mu.RLock()
	episode := s.currentEpisode
	podcast := s.currentPodcast
	s.mu.RUnlock()

	status, err := s.player.Status(ctx)
	if err != nil {
		return
	}

	metadata := domain.PlaybackMetadata{
		CanSeek:       status.CanSeek,
		CanGoNext:     s.queueSvc != nil && s.queueSvc.HasNext(),
		CanGoPrevious: s.queueSvc != nil && s.queueSvc.HasPrevious(),
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

	_ = s.broadcaster.PublishState(ctx, status.State, metadata)
}

func (s *PlayerService) Play(ctx context.Context, episodeID int64) error {
	if episodeID == 0 {
		s.mu.Lock()
		episodeID = s.lastEpisodeID
		s.mu.Unlock()

		if episodeID == 0 {
			return fmt.Errorf("no episode to play")
		}
	}

	return s.PlayEpisode(ctx, episodeID)
}

func (s *PlayerService) PlayPause(ctx context.Context) error {
	return s.TogglePlayPause(ctx)
}

func (s *PlayerService) Stop(ctx context.Context) error {
	return s.StopPlayback(ctx)
}

func (s *PlayerService) Next(ctx context.Context) error {
	if s.queueSvc == nil {
		return fmt.Errorf("queue not available")
	}
	return s.queueSvc.Next(ctx)
}

func (s *PlayerService) Previous(ctx context.Context) error {
	if s.queueSvc == nil {
		return fmt.Errorf("queue not available")
	}
	return s.queueSvc.Previous(ctx)
}
