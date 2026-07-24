package application

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

type QueueService struct {
	mu          sync.RWMutex
	queueRepo   domain.QueueRepo
	episodes    domain.EpisodeRepo
	podcasts    domain.PodcastRepo
	player      domain.Player
	broadcaster domain.PlaybackBroadcaster
	logger      domain.Logger

	items []domain.QueueItem
	state domain.QueueState
}

func NewQueueService(
	queueRepo domain.QueueRepo,
	episodes domain.EpisodeRepo,
	podcasts domain.PodcastRepo,
	player domain.Player,
	broadcaster domain.PlaybackBroadcaster,
	logger domain.Logger,
) *QueueService {
	if logger == nil {
		logger = domain.NoopLogger{}
	}

	svc := &QueueService{
		queueRepo: queueRepo,
		episodes:  episodes,
		podcasts:  podcasts,
		player:    player,
		broadcaster: broadcaster,
		logger:    logger,
	}

	svc.loadFromDB(context.Background())

	return svc
}

func (s *QueueService) loadFromDB(ctx context.Context) {
	items, err := s.queueRepo.FindAllQueueItems(ctx)
	if err != nil {
		s.logger.Warn("failed to load playback queue", "err", err)
		return
	}

	state, err := s.queueRepo.GetQueueState(ctx)
	if err != nil {
		s.logger.Warn("failed to load queue state", "err", err)
		return
	}

	s.items = items
	s.state = *state
}

func (s *QueueService) AddToQueue(ctx context.Context, episodeID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	position := len(s.items)
	item := &domain.QueueItem{
		EpisodeID: episodeID,
		Position:  position,
		AddedAt:   time.Now(),
	}

	if err := s.queueRepo.SaveQueueItem(ctx, item); err != nil {
		return fmt.Errorf("save queue item: %w", err)
	}

	s.items = append(s.items, *item)
	return nil
}

func (s *QueueService) AddPlayNext(ctx context.Context, episodeID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	insertPos := s.state.CurrentIndex + 1

	for i := insertPos; i < len(s.items); i++ {
		s.items[i].Position = i + 1
		if err := s.queueRepo.SaveQueueItem(ctx, &s.items[i]); err != nil {
			return fmt.Errorf("update position: %w", err)
		}
	}

	item := &domain.QueueItem{
		EpisodeID: episodeID,
		Position:  insertPos,
		AddedAt:   time.Now(),
	}

	if err := s.queueRepo.SaveQueueItem(ctx, item); err != nil {
		return fmt.Errorf("save queue item: %w", err)
	}

	newItems := make([]domain.QueueItem, 0, len(s.items)+1)
	newItems = append(newItems, s.items[:insertPos]...)
	newItems = append(newItems, *item)
	newItems = append(newItems, s.items[insertPos:]...)
	s.items = newItems

	return nil
}

func (s *QueueService) RemoveFromQueue(ctx context.Context, itemID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, item := range s.items {
		if item.ID == itemID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("item not found in queue")
	}

	if err := s.queueRepo.DeleteQueueItem(ctx, itemID); err != nil {
		return fmt.Errorf("delete queue item: %w", err)
	}

	s.items = append(s.items[:idx], s.items[idx+1:]...)

	for i := idx; i < len(s.items); i++ {
		s.items[i].Position = i
		if err := s.queueRepo.SaveQueueItem(ctx, &s.items[i]); err != nil {
			return fmt.Errorf("update position: %w", err)
		}
	}

	if s.state.CurrentIndex >= idx && s.state.CurrentIndex > 0 {
		s.state.CurrentIndex--
		if err := s.queueRepo.SaveQueueState(ctx, &s.state); err != nil {
			return fmt.Errorf("save queue state: %w", err)
		}
	}

	return nil
}

func (s *QueueService) ClearQueue(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.queueRepo.ClearQueue(ctx); err != nil {
		return fmt.Errorf("clear queue: %w", err)
	}

	s.items = nil
	s.state.CurrentIndex = 0
	if err := s.queueRepo.SaveQueueState(ctx, &s.state); err != nil {
		return fmt.Errorf("save queue state: %w", err)
	}

	return nil
}

func (s *QueueService) GetQueue(ctx context.Context) ([]domain.QueueItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.QueueItem, len(s.items))
	copy(result, s.items)
	return result, nil
}

func (s *QueueService) Next(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		return fmt.Errorf("queue is empty")
	}

	nextIdx := s.state.CurrentIndex + 1

	if s.state.Shuffle && s.state.RepeatMode != domain.RepeatOne {
		nextIdx = s.randomNextIndex()
	}

	if nextIdx >= len(s.items) {
		switch s.state.RepeatMode {
		case domain.RepeatAll:
			nextIdx = 0
		case domain.RepeatOne:
			nextIdx = s.state.CurrentIndex
		default:
			return fmt.Errorf("end of queue")
		}
	}

	return s.playAtIndex(ctx, nextIdx)
}

func (s *QueueService) Previous(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		return fmt.Errorf("queue is empty")
	}

	prevIdx := s.state.CurrentIndex - 1
	if prevIdx < 0 {
		if s.state.RepeatMode == domain.RepeatAll {
			prevIdx = len(s.items) - 1
		} else {
			prevIdx = 0
		}
	}

	return s.playAtIndex(ctx, prevIdx)
}

func (s *QueueService) playAtIndex(ctx context.Context, idx int) error {
	if idx < 0 || idx >= len(s.items) {
		return fmt.Errorf("invalid queue index: %d", idx)
	}

	item := s.items[idx]
	episode, err := s.episodes.FindEpisodeByID(ctx, item.EpisodeID)
	if err != nil {
		return fmt.Errorf("find episode: %w", err)
	}

	source := resolvePlaybackSource(*episode)
	if err := s.player.Play(ctx, source); err != nil {
		return fmt.Errorf("play: %w", err)
	}

	s.state.CurrentIndex = idx
	if err := s.queueRepo.SaveQueueState(ctx, &s.state); err != nil {
		s.logger.Warn("failed to save queue state", "err", err)
	}

	return nil
}

func (s *QueueService) OnTrackEnded(ctx context.Context) error {
	// Quick check under read lock to avoid unnecessary Next() call.
	s.mu.RLock()
	empty := len(s.items) == 0
	s.mu.RUnlock()

	if empty {
		return nil
	}

	// Next() acquires its own write lock -- must not hold s.mu here.
	return s.Next(ctx)
}

func (s *QueueService) ToggleRepeat(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state.RepeatMode {
	case domain.RepeatNone:
		s.state.RepeatMode = domain.RepeatOne
	case domain.RepeatOne:
		s.state.RepeatMode = domain.RepeatAll
	case domain.RepeatAll:
		s.state.RepeatMode = domain.RepeatNone
	}

	// Shuffle and RepeatOne are contradictory: RepeatOne means "stay here"
	// while shuffle means "go somewhere random". Turn off shuffle when
	// RepeatOne is activated so behaviour is never ambiguous.
	if s.state.RepeatMode == domain.RepeatOne {
		s.state.Shuffle = false
	}

	return s.queueRepo.SaveQueueState(ctx, &s.state)
}

func (s *QueueService) ToggleShuffle(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Shuffle = !s.state.Shuffle
	return s.queueRepo.SaveQueueState(ctx, &s.state)
}

func (s *QueueService) GetState(ctx context.Context) (*domain.QueueState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stateCopy := s.state
	return &stateCopy, nil
}

func (s *QueueService) HasNext() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 {
		return false
	}

	if s.state.RepeatMode == domain.RepeatAll || s.state.RepeatMode == domain.RepeatOne {
		return true
	}

	return s.state.CurrentIndex < len(s.items)-1
}

func (s *QueueService) HasPrevious() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 {
		return false
	}

	if s.state.RepeatMode == domain.RepeatAll {
		return true
	}

	return s.state.CurrentIndex > 0
}

func (s *QueueService) randomNextIndex() int {
	if len(s.items) <= 1 {
		return 0
	}

	idx := rand.Intn(len(s.items))
	for idx == s.state.CurrentIndex && len(s.items) > 1 {
		idx = rand.Intn(len(s.items))
	}
	return idx
}

func (s *QueueService) GetQueueWithEpisodes(ctx context.Context) ([]QueueItemView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var views []QueueItemView
	for i, item := range s.items {
		episode, err := s.episodes.FindEpisodeByID(ctx, item.EpisodeID)
		if err != nil {
			continue
		}

		podcast, err := s.podcasts.FindByID(ctx, episode.PodcastID)
		if err != nil {
			continue
		}

		views = append(views, QueueItemView{
			Item:         item,
			EpisodeTitle: episode.Title,
			PodcastTitle: podcast.Title,
			IsCurrent:    i == s.state.CurrentIndex,
		})
	}

	return views, nil
}

func (s *QueueService) CurrentEpisodeID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 || s.state.CurrentIndex >= len(s.items) {
		return 0
	}
	return s.items[s.state.CurrentIndex].EpisodeID
}

type QueueItemView struct {
	Item         domain.QueueItem
	EpisodeTitle string
	PodcastTitle string
	IsCurrent    bool
}
