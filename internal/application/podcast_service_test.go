package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
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

func (m mockFeedParser) Parse(_ context.Context, _ string, _ domain.FeedHeaders) (*domain.Podcast, []domain.Episode, domain.FeedHeaders, error) {
	return m.podcast, m.episodes, domain.FeedHeaders{}, m.err
}

type mockFeedParserResponse struct {
	podcast  *domain.Podcast
	episodes []domain.Episode
	err      error
}

type mockFeedParserByURL struct {
	responses map[string]mockFeedParserResponse
}

func (m mockFeedParserByURL) Parse(_ context.Context, url string, _ domain.FeedHeaders) (*domain.Podcast, []domain.Episode, domain.FeedHeaders, error) {
	resp, ok := m.responses[url]
	if !ok {
		return nil, nil, domain.FeedHeaders{}, nil
	}
	return resp.podcast, resp.episodes, domain.FeedHeaders{}, resp.err
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
	service := NewPodcastService(repo, repo, repo, fetcher, nil)

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

// TestPodcastService_AddPodcastAtomicOnDuplicateFeedURL covers issue #13's
// AddPodcast atomicity criterion: when the podcast insert collides with an
// existing feed_url (UNIQUE violation), the whole batch rolls back — no podcast
// and no partial episodes are committed.
func TestPodcastService_AddPodcastAtomicOnDuplicateFeedURL(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Pre-seed the feed URL so AddPodcast's podcast insert collides.
	if err := repo.Save(ctx, &domain.Podcast{Title: "Existing", FeedURL: "https://example.com/dup.xml"}); err != nil {
		t.Fatalf("seed Save failed: %v", err)
	}

	fetcher := mockFeedParser{
		podcast: &domain.Podcast{
			Title:   "Dup",
			FeedURL: "https://example.com/dup.xml", // collides with the seed row
		},
		episodes: []domain.Episode{
			{Title: "Should Rollback", AudioURL: "https://example.com/rb.mp3"},
		},
	}
	service := NewPodcastService(repo, repo, repo, fetcher, nil)

	if _, err := service.AddPodcast(ctx, "https://example.com/dup.xml"); err == nil {
		t.Fatal("expected AddPodcast to fail on duplicate feed_url, got nil")
	}

	// Exactly one podcast (the seed) and zero episodes survive.
	got, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 podcast after rollback, got %d", len(got))
	}
	for _, p := range got {
		eps, err := repo.FindEpisodesByPodcastID(ctx, p.ID)
		if err != nil {
			t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
		}
		if len(eps) != 0 {
			t.Errorf("expected 0 episodes for podcast %q, got %d", p.Title, len(eps))
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

	service := NewPodcastService(repo, repo, repo, mockFeedParser{}, nil)

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
	service := NewPodcastService(repo, repo, repo, fetcher, nil)

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

	service := NewPodcastService(repo, repo, repo, parser, nil)
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

type slowParser struct {
	maxConcurrent atomic.Int64
	inflight      atomic.Int64
	delay         time.Duration
	byURL         map[string]mockFeedParserResponse
}

func (p *slowParser) Parse(_ context.Context, url string, _ domain.FeedHeaders) (*domain.Podcast, []domain.Episode, domain.FeedHeaders, error) {
	cur := p.inflight.Add(1)
	defer p.inflight.Add(-1)

	for {
		old := p.maxConcurrent.Load()
		if cur <= old || p.maxConcurrent.CompareAndSwap(old, cur) {
			break
		}
	}

	time.Sleep(p.delay)

	resp, ok := p.byURL[url]
	if !ok {
		return nil, nil, domain.FeedHeaders{}, errors.New("unknown feed")
	}
	return resp.podcast, resp.episodes, domain.FeedHeaders{}, resp.err
}

func TestRefreshAllPodcastsRunsConcurrently(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	feeds := make(map[string]mockFeedParserResponse)
	for i := range 10 {
		p := domain.Podcast{
			Title:   "Feed " + string(rune('A'+i)),
			FeedURL: "https://example.com/feed" + string(rune('0'+i)) + ".xml",
		}
		if err := repo.Save(ctx, &p); err != nil {
			t.Fatal(err)
		}
		feeds[p.FeedURL] = mockFeedParserResponse{podcast: &p}
	}

	parser := &slowParser{
		delay: 50 * time.Millisecond,
		byURL: feeds,
	}

	service := NewPodcastService(repo, repo, repo, parser, nil)
	result, err := service.RefreshAllPodcastsWithConcurrency(ctx, 3)
	if err != nil {
		t.Fatalf("RefreshAllPodcastsWithConcurrency failed: %v", err)
	}

	if result.TotalPodcasts != 10 {
		t.Fatalf("expected TotalPodcasts=10, got %d", result.TotalPodcasts)
	}
	if result.Refreshed != 10 {
		t.Fatalf("expected Refreshed=10, got %d", result.Refreshed)
	}

	max := parser.maxConcurrent.Load()
	if max > 3 {
		t.Fatalf("expected at most 3 concurrent parses, observed %d", max)
	}
	if max < 2 {
		t.Fatalf("expected at least 2 concurrent parses (parallelism), observed %d", max)
	}
}

func TestRefreshAllPodcastsWithConcurrencyCollectsErrors(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	okPodcast := &domain.Podcast{Title: "Good", FeedURL: "https://example.com/good.xml"}
	failPodcast := &domain.Podcast{Title: "Bad", FeedURL: "https://example.com/bad.xml"}
	if err := repo.Save(ctx, okPodcast); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, failPodcast); err != nil {
		t.Fatal(err)
	}

	parser := mockFeedParserByURL{
		responses: map[string]mockFeedParserResponse{
			okPodcast.FeedURL:   {podcast: okPodcast},
			failPodcast.FeedURL: {err: errors.New("network timeout")},
		},
	}

	service := NewPodcastService(repo, repo, repo, parser, nil)
	result, err := service.RefreshAllPodcastsWithConcurrency(ctx, 2)
	if err != nil {
		t.Fatalf("RefreshAllPodcastsWithConcurrency failed: %v", err)
	}

	if result.Refreshed != 1 {
		t.Errorf("expected Refreshed=1, got %d", result.Refreshed)
	}
	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(result.Failures))
	}
	if result.Failures[0].Title != "Bad" {
		t.Errorf("expected failure for 'Bad', got %q", result.Failures[0].Title)
	}
}

func TestRefreshAllPodcastsDefaultConcurrency(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	p := &domain.Podcast{Title: "One", FeedURL: "https://example.com/one.xml"}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	service := NewPodcastService(repo, repo, repo, mockFeedParserByURL{
		responses: map[string]mockFeedParserResponse{
			p.FeedURL: {podcast: p},
		},
	}, nil)

	result, err := service.RefreshAllPodcasts(ctx)
	if err != nil {
		t.Fatalf("RefreshAllPodcasts failed: %v", err)
	}
	if result.Refreshed != 1 {
		t.Fatalf("expected Refreshed=1, got %d", result.Refreshed)
	}
}

func TestRefreshAllPodcastsWithConcurrencyRejectsZeroLimit(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	p := &domain.Podcast{Title: "One", FeedURL: "https://example.com/one.xml"}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	parser := &slowParser{
		delay: 10 * time.Millisecond,
		byURL: map[string]mockFeedParserResponse{
			p.FeedURL: {podcast: p},
		},
	}

	service := NewPodcastService(repo, repo, repo, parser, nil)
	result, err := service.RefreshAllPodcastsWithConcurrency(ctx, 0)
	if err != nil {
		t.Fatalf("RefreshAllPodcastsWithConcurrency failed: %v", err)
	}
	if result.Refreshed != 1 {
		t.Fatalf("expected Refreshed=1 with fallback concurrency, got %d", result.Refreshed)
	}
}

func TestRefreshAllPodcastsWithConcurrencyCancelledContext(t *testing.T) {
	ctx := context.Background()
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	var mu sync.Mutex
	var saved []domain.Podcast
	for i := range 5 {
		p := domain.Podcast{
			Title:   "Feed",
			FeedURL: "https://example.com/f" + string(rune('0'+i)) + ".xml",
		}
		if err := repo.Save(ctx, &p); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		saved = append(saved, p)
		mu.Unlock()
	}

	feeds := make(map[string]mockFeedParserResponse)
	for _, p := range saved {
		feeds[p.FeedURL] = mockFeedParserResponse{podcast: &p}
	}

	parser := &slowParser{
		delay: 200 * time.Millisecond,
		byURL: feeds,
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	service := NewPodcastService(repo, repo, repo, parser, nil)
	_, err = service.RefreshAllPodcastsWithConcurrency(cancelCtx, 2)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
