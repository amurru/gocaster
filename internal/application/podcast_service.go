package application

import (
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/rss"
)

type PodcastService struct {
	repo    domain.PodcastRepository
	fetcher *rss.FeedFetcher
}

func NewPodcastService(repo domain.PodcastRepository, fetcher *rss.FeedFetcher) *PodcastService {
	return &PodcastService{
		repo:    repo,
		fetcher: fetcher,
	}
}

// AddPodcast orchestrates fetching metadata and saving to DB
func (s *PodcastService) AddPodcast(rssUrl string) (*domain.Podcast, error) {
	// fetch metadata from rss feed
	podcast, _, err := s.fetcher.Parse(rssUrl)
	if err != nil {
		return nil, err
	}

	// save to db
	if err := s.repo.Save(podcast); err != nil {
		return nil, err
	}

	return podcast, nil
}

func (s *PodcastService) ListPodcasts() ([]domain.Podcast, error) {
	return s.repo.FindAll()
}
