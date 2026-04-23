package application

import (
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

type FeedParser interface {
	Parse(url string) (*domain.Podcast, []domain.Episode, error)
}

type PodcastService struct {
	repo    domain.PodcastRepository
	fetcher FeedParser
}

type RefreshAllResult struct {
	TotalPodcasts int
	Refreshed     int
	Failed        int
	NewEpisodes   int
}

func NewPodcastService(repo domain.PodcastRepository, fetcher FeedParser) *PodcastService {
	return &PodcastService{
		repo:    repo,
		fetcher: fetcher,
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
	if err := s.repo.Save(podcast); err != nil {
		return nil, err
	}

	for i := range episodes {
		episodes[i].PodcastID = podcast.ID
		if err := s.repo.SaveEpisode(&episodes[i]); err != nil {
			return nil, err
		}
	}

	return podcast, nil
}

func (s *PodcastService) ListPodcasts() ([]domain.Podcast, error) {
	return s.repo.FindAll()
}

func (s *PodcastService) GetPodcast(id int64) (*domain.Podcast, error) {
	return s.repo.FindByID(id)
}

func (s *PodcastService) ListEpisodes(podcastID int64) ([]domain.Episode, error) {
	return s.repo.FindEpisodesByPodcastID(podcastID)
}

func (s *PodcastService) RefreshPodcast(podcastID int64) (int, error) {
	podcast, err := s.repo.FindByID(podcastID)
	if err != nil {
		return 0, err
	}

	_, fetchedEpisodes, err := s.fetcher.Parse(podcast.FeedURL)
	if err != nil {
		return 0, err
	}

	existingEpisodes, err := s.repo.FindEpisodesByPodcastID(podcastID)
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
		if err := s.repo.SaveEpisode(&fetchedEpisodes[i]); err != nil {
			return newCount, err
		}
		newCount++
	}

	podcast.LastUpdated = time.Now()
	if err := s.repo.Save(podcast); err != nil {
		return newCount, err
	}

	return newCount, nil
}

func (s *PodcastService) RefreshAllPodcasts() (RefreshAllResult, error) {
	var result RefreshAllResult

	podcasts, err := s.repo.FindAll()
	if err != nil {
		return result, err
	}

	result.TotalPodcasts = len(podcasts)
	for _, podcast := range podcasts {
		newCount, refreshErr := s.RefreshPodcast(podcast.ID)
		if refreshErr != nil {
			result.Failed++
			continue
		}
		result.Refreshed++
		result.NewEpisodes += newCount
	}

	return result, nil
}
