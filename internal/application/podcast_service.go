package application

import (
	"github.com/amurru/gocaster/internal/domain"
)

type FeedParser interface {
	Parse(url string) (*domain.Podcast, []domain.Episode, error)
}

type PodcastService struct {
	repo    domain.PodcastRepository
	fetcher FeedParser
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
