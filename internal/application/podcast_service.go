package application

import (
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

type FeedParser interface {
	Parse(url string) (*domain.Podcast, []domain.Episode, error)
}

type PodcastService struct {
	podcasts domain.PodcastRepo
	episodes domain.EpisodeRepo
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
// uses — podcast and episode persistence (issue #17). The concrete SQLiteRepo
// satisfies both via the union PodcastRepository. logger defaults to NoopLogger
// when nil (issue #14).
func NewPodcastService(podcasts domain.PodcastRepo, episodes domain.EpisodeRepo, fetcher FeedParser, logger domain.Logger) *PodcastService {
	if logger == nil {
		logger = domain.NoopLogger{}
	}
	return &PodcastService{
		podcasts: podcasts,
		episodes: episodes,
		fetcher:  fetcher,
		logger:   logger,
	}
}

// AddPodcast orchestrates fetching metadata and saving to DB
func (s *PodcastService) AddPodcast(rssUrl string) (*domain.Podcast, error) {
	// fetch metadata from rss feed
	podcast, episodes, err := s.fetcher.Parse(rssUrl)
	if err != nil {
		return nil, err
	}

	// save to db
	if err := s.podcasts.Save(podcast); err != nil {
		return nil, err
	}

	for i := range episodes {
		episodes[i].PodcastID = podcast.ID
		if err := s.episodes.SaveEpisode(&episodes[i]); err != nil {
			return nil, err
		}
	}

	return podcast, nil
}

func (s *PodcastService) ListPodcasts() ([]domain.Podcast, error) {
	return s.podcasts.FindAll()
}

func (s *PodcastService) GetPodcast(id int64) (*domain.Podcast, error) {
	return s.podcasts.FindByID(id)
}

func (s *PodcastService) ListEpisodes(podcastID int64) ([]domain.Episode, error) {
	return s.episodes.FindEpisodesByPodcastID(podcastID)
}

func (s *PodcastService) RefreshPodcast(podcastID int64) (int, error) {
	podcast, err := s.podcasts.FindByID(podcastID)
	if err != nil {
		return 0, err
	}

	_, fetchedEpisodes, err := s.fetcher.Parse(podcast.FeedURL)
	if err != nil {
		return 0, err
	}

	existingEpisodes, err := s.episodes.FindEpisodesByPodcastID(podcastID)
	if err != nil {
		return 0, err
	}
	existingUrls := make(map[string]bool)
	for _, ep := range existingEpisodes {
		existingUrls[ep.AudioURL] = true
	}

	newCount := 0
	for i := range fetchedEpisodes {
		if existingUrls[fetchedEpisodes[i].AudioURL] {
			continue
		}
		fetchedEpisodes[i].PodcastID = podcastID
		if err := s.episodes.SaveEpisode(&fetchedEpisodes[i]); err != nil {
			return newCount, err
		}
		newCount++
	}

	podcast.LastUpdated = time.Now()
	if err := s.podcasts.Save(podcast); err != nil {
		return newCount, err
	}

	return newCount, nil
}

func (s *PodcastService) RefreshAllPodcasts() (RefreshAllResult, error) {
	var result RefreshAllResult

	podcasts, err := s.podcasts.FindAll()
	if err != nil {
		return result, err
	}

	result.TotalPodcasts = len(podcasts)
	for _, podcast := range podcasts {
		newCount, refreshErr := s.RefreshPodcast(podcast.ID)
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
