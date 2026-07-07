package application

import (
	"context"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

// FeedParser is the application port for fetching an RSS feed. Parse takes a
// context.Context so a cancelled caller cancels the underlying HTTP request
// (issue #11).
type FeedParser interface {
	Parse(ctx context.Context, url string) (*domain.Podcast, []domain.Episode, error)
}

type PodcastService struct {
	podcasts domain.PodcastRepo
	episodes domain.EpisodeRepo
	batch    domain.PodcastBatchRepo
	fetcher  FeedParser
	logger   domain.Logger
}

type RefreshAllResult struct {
	TotalPodcasts int
	Refreshed     int
	Failed        int
	NewEpisodes   int
	// Failures records each podcast that could not be refreshed and why, so
	// callers (the TUI) can show which feeds failed instead of only a count.
	Failures []RefreshFailure
}

// RefreshFailure describes a single per-feed failure during RefreshAllPodcasts.
type RefreshFailure struct {
	PodcastID int64
	Title     string
	FeedURL   string
	Err       error
}

// NewPodcastService accepts only the focused repository ports PodcastService
// uses — podcast/episode persistence and the atomic podcast-batch writer
// (issues #17, #13). The concrete SQLiteRepo satisfies all via the union
// PodcastRepository. logger defaults to NoopLogger when nil (issue #14).
func NewPodcastService(
	podcasts domain.PodcastRepo,
	episodes domain.EpisodeRepo,
	batch domain.PodcastBatchRepo,
	fetcher FeedParser,
	logger domain.Logger,
) *PodcastService {
	if logger == nil {
		logger = domain.NoopLogger{}
	}
	return &PodcastService{
		podcasts: podcasts,
		episodes: episodes,
		batch:    batch,
		fetcher:  fetcher,
		logger:   logger,
	}
}

// AddPodcast orchestrates fetching metadata and saving the podcast together
// with all of its episodes in a single transaction (issue #13): all-or-nothing,
// so a mid-loop failure leaves no partial episode set behind.
func (s *PodcastService) AddPodcast(ctx context.Context, rssUrl string) (*domain.Podcast, error) {
	// fetch metadata from rss feed
	podcast, episodes, err := s.fetcher.Parse(ctx, rssUrl)
	if err != nil {
		return nil, err
	}

	if err := s.batch.SavePodcastWithEpisodes(ctx, podcast, episodes); err != nil {
		return nil, err
	}

	return podcast, nil
}

func (s *PodcastService) ListPodcasts(ctx context.Context) ([]domain.Podcast, error) {
	return s.podcasts.FindAll(ctx)
}

func (s *PodcastService) GetPodcast(ctx context.Context, id int64) (*domain.Podcast, error) {
	return s.podcasts.FindByID(ctx, id)
}

func (s *PodcastService) ListEpisodes(ctx context.Context, podcastID int64) ([]domain.Episode, error) {
	return s.episodes.FindEpisodesByPodcastID(ctx, podcastID)
}

func (s *PodcastService) RefreshPodcast(ctx context.Context, podcastID int64) (int, error) {
	podcast, err := s.podcasts.FindByID(ctx, podcastID)
	if err != nil {
		return 0, err
	}

	_, fetchedEpisodes, err := s.fetcher.Parse(ctx, podcast.FeedURL)
	if err != nil {
		return 0, err
	}

	existingEpisodes, err := s.episodes.FindEpisodesByPodcastID(ctx, podcastID)
	if err != nil {
		return 0, err
	}
	existingUrls := make(map[string]bool)
	for _, ep := range existingEpisodes {
		existingUrls[ep.AudioURL] = true
	}

	var newEpisodes []domain.Episode
	for i := range fetchedEpisodes {
		if existingUrls[fetchedEpisodes[i].AudioURL] {
			continue
		}
		newEpisodes = append(newEpisodes, fetchedEpisodes[i])
	}

	// Persist the new episodes and the LastUpdated bump in a single transaction
	// (issue #13): a mid-batch failure rolls back both, so the feed is retried in
	// full next time rather than leaving a partial set with a stale LastUpdated.
	podcast.LastUpdated = time.Now()
	if err := s.batch.AppendEpisodesAndTouchPodcast(ctx, podcast, newEpisodes); err != nil {
		return len(newEpisodes), err
	}

	return len(newEpisodes), nil
}

func (s *PodcastService) RefreshAllPodcasts(ctx context.Context) (RefreshAllResult, error) {
	var result RefreshAllResult

	podcasts, err := s.podcasts.FindAll(ctx)
	if err != nil {
		return result, err
	}

	result.TotalPodcasts = len(podcasts)
	for _, podcast := range podcasts {
		newCount, refreshErr := s.RefreshPodcast(ctx, podcast.ID)
		if refreshErr != nil {
			result.Failed++
			result.Failures = append(result.Failures, RefreshFailure{
				PodcastID: podcast.ID,
				Title:     podcast.Title,
				FeedURL:   podcast.FeedURL,
				Err:       refreshErr,
			})
			continue
		}
		result.Refreshed++
		result.NewEpisodes += newCount
	}

	return result, nil
}
