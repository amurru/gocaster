// internal/application/podcast_service_test.go
package application

import (
	"testing"

	"github.com/amurru/gocaster/internal/infrastructure/persistence"
	"github.com/amurru/gocaster/internal/infrastructure/rss"
)

func TestPodcastService_AddAndList(t *testing.T) {
	// Use in-memory database
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	fetcher := rss.NewFeedFetcher()
	_ = NewPodcastService(repo, fetcher)

	// This requires a real RSS feed - for now we test the happy path
	// In production, use a test RSS server or mock the fetcher
	t.Skip("requires real RSS feed or mocked fetcher")
}
