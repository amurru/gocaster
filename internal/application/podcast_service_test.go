package application

import (
	"context"
	"errors"
	"strings"
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

func (m mockFeedParser) Parse(context.Context, string) (*domain.Podcast, []domain.Episode, error) {
	return m.podcast, m.episodes, m.err
}

type mockFeedParserResponse struct {
	podcast  *domain.Podcast
	episodes []domain.Episode
	err      error
}

type mockFeedParserByURL struct {
	responses map[string]mockFeedParserResponse
}

func (m mockFeedParserByURL) Parse(_ context.Context, url string) (*domain.Podcast, []domain.Episode, error) {
	resp, ok := m.responses[url]
	if !ok {
		return nil, nil, nil
	}
	return resp.podcast, resp.episodes, resp.err
}

func TestPodcastService_AddPodcastPersistsEpisodes(t *testing.T) {
	ctx := context.Background()
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
	service := NewPodcastService(repo, repo, fetcher, nil)

	podcast, err := service.AddPodcast(ctx, "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("AddPodcast failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Fatal("expected podcast ID to be assigned")
	}

	episodes, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID)
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
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Episode 1",
		AudioURL:    "https://example.com/1.mp3",
		PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.SaveEpisode(ctx, episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	service := NewPodcastService(repo, repo, mockFeedParser{}, nil)

	episodes, err := service.ListEpisodes(ctx, podcast.ID)
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
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	existing := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Existing Episode",
		AudioURL:    "https://example.com/old.mp3",
		PublishedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.SaveEpisode(ctx, existing); err != nil {
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
	service := NewPodcastService(repo, repo, fetcher, nil)

	newCount, err := service.RefreshPodcast(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("RefreshPodcast failed: %v", err)
	}

	if newCount != 2 {
		t.Fatalf("expected 2 new episodes, got %d", newCount)
	}

	stored, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}

	if len(stored) != 3 {
		t.Fatalf("expected 3 total episodes, got %d", len(stored))
	}
}

func TestPodcastService_RefreshAllPodcastsAggregatesResults(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	first := &domain.Podcast{Title: "One", FeedURL: "https://example.com/one.xml"}
	second := &domain.Podcast{Title: "Two", FeedURL: "https://example.com/two.xml"}
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("Save first podcast failed: %v", err)
	}
	if err := repo.Save(ctx, second); err != nil {
		t.Fatalf("Save second podcast failed: %v", err)
	}

	if err := repo.SaveEpisode(ctx, &domain.Episode{
		PodcastID:   first.ID,
		Title:       "Existing",
		AudioURL:    "https://example.com/existing.mp3",
		PublishedAt: time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	parser := mockFeedParserByURL{
		responses: map[string]mockFeedParserResponse{
			first.FeedURL: {
				podcast: first,
				episodes: []domain.Episode{
					{Title: "Existing", AudioURL: "https://example.com/existing.mp3"},
					{Title: "New", AudioURL: "https://example.com/new.mp3"},
				},
			},
			second.FeedURL: {
				err: errors.New("fetch failed"),
			},
		},
	}

	service := NewPodcastService(repo, repo, parser, nil)
	result, err := service.RefreshAllPodcasts(ctx)
	if err != nil {
		t.Fatalf("RefreshAllPodcasts failed: %v", err)
	}

	if result.TotalPodcasts != 2 {
		t.Fatalf("expected TotalPodcasts=2, got %d", result.TotalPodcasts)
	}
	if result.Refreshed != 1 {
		t.Fatalf("expected Refreshed=1, got %d", result.Refreshed)
	}
	if result.Failed != 1 {
		t.Fatalf("expected Failed=1, got %d", result.Failed)
	}
	if result.NewEpisodes != 1 {
		t.Fatalf("expected NewEpisodes=1, got %d", result.NewEpisodes)
	}

	// Issue #7: per-feed failures must be surfaced so callers can show which
	// feeds failed and why, not just a count.
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure recorded, got %d", len(result.Failures))
	}
	f := result.Failures[0]
	if f.PodcastID != second.ID {
		t.Errorf("failure PodcastID = %d, want %d", f.PodcastID, second.ID)
	}
	if f.Title != "Two" {
		t.Errorf("failure Title = %q, want %q", f.Title, "Two")
	}
	if f.FeedURL != second.FeedURL {
		t.Errorf("failure FeedURL = %q, want %q", f.FeedURL, second.FeedURL)
	}
	if f.Err == nil || !strings.Contains(f.Err.Error(), "fetch failed") {
		t.Errorf("failure Err = %v, want an error containing %q", f.Err, "fetch failed")
	}
}
