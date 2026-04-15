package application

import (
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
)

type mockFeedParser struct {
	podcast  *domain.Podcast
	episodes []domain.Episode
	err      error
}

func (m mockFeedParser) Parse(string) (*domain.Podcast, []domain.Episode, error) {
	return m.podcast, m.episodes, m.err
}

func TestPodcastService_AddPodcastPersistsEpisodes(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	fetcher := mockFeedParser{
		podcast: &domain.Podcast{
			Title:       "Syntax FM",
			FeedURL:     "https://example.com/feed.xml",
			Description: "A dev podcast",
		},
		episodes: []domain.Episode{
			{
				Title:       "Episode 1",
				Description: "First",
				AudioURL:    "https://example.com/1.mp3",
				PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			},
			{
				Title:       "Episode 2",
				Description: "Second",
				AudioURL:    "https://example.com/2.mp3",
				PublishedAt: time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewPodcastService(repo, fetcher)

	podcast, err := service.AddPodcast("https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("AddPodcast failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Fatal("expected podcast ID to be assigned")
	}

	episodes, err := repo.FindEpisodesByPodcastID(podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}

	for _, episode := range episodes {
		if episode.PodcastID != podcast.ID {
			t.Fatalf("expected podcast ID %d, got %d", podcast.ID, episode.PodcastID)
		}
	}
}

func TestPodcastService_ListEpisodes(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Episode 1",
		AudioURL:    "https://example.com/1.mp3",
		PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.SaveEpisode(episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	service := NewPodcastService(repo, mockFeedParser{})

	episodes, err := service.ListEpisodes(podcast.ID)
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}

	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}

	if episodes[0].Title != episode.Title {
		t.Fatalf("expected %q, got %q", episode.Title, episodes[0].Title)
	}
}

func TestPodcastService_RefreshPodcastAddsOnlyNewEpisodes(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	existing := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Existing Episode",
		AudioURL:    "https://example.com/old.mp3",
		PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.SaveEpisode(existing); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	fetcher := mockFeedParser{
		podcast: &domain.Podcast{
			Title:   "Test",
			FeedURL: "https://example.com/feed.xml",
		},
		episodes: []domain.Episode{
			{
				Title:       "Existing Episode",
				AudioURL:    "https://example.com/old.mp3",
				PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			},
			{
				Title:       "New Episode 1",
				AudioURL:    "https://example.com/new1.mp3",
				PublishedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			{
				Title:       "New Episode 2",
				AudioURL:    "https://example.com/new2.mp3",
				PublishedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewPodcastService(repo, fetcher)

	newCount, err := service.RefreshPodcast(podcast.ID)
	if err != nil {
		t.Fatalf("RefreshPodcast failed: %v", err)
	}

	if newCount != 2 {
		t.Fatalf("expected 2 new episodes, got %d", newCount)
	}

	stored, err := repo.FindEpisodesByPodcastID(podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}

	if len(stored) != 3 {
		t.Fatalf("expected 3 total episodes, got %d", len(stored))
	}
}
