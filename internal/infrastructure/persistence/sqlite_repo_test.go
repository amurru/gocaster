package persistence

import (
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

func TestNewSQLiteRepo_RunsMigrations(t *testing.T) {
	// Use temp file for test
	dbPath := ":memory:"
	repo, err := NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close() // Cleanup

	// Verify we can query the schema by checking if a save operation works
	testPodcast := &domain.Podcast{Title: "Test", FeedURL: "https://test.com/feed.xml"}
	err = repo.Save(testPodcast)
	if err != nil {
		t.Errorf("migrations not run, save failed: %v", err)
	}
}

func TestSQLiteRepo_Save(t *testing.T) {
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{
		Title:       "Test Podcast",
		FeedURL:     "https://example.com/feed.xml",
		Description: "A test podcast",
		ImageURL:    "https://example.com/image.jpg",
		LastUpdated: time.Now(),
	}

	err = repo.Save(podcast)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Error("Save should set ID")
	}
}

func TestSQLiteRepo_FindAll(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup: create test data
	p1 := &domain.Podcast{Title: "Podcast 1", FeedURL: "https://example.com/feed1.xml"}
	p2 := &domain.Podcast{Title: "Podcast 2", FeedURL: "https://example.com/feed2.xml"}
	repo.Save(p1)
	repo.Save(p2)

	// Test
	podcasts, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}

	if len(podcasts) != 2 {
		t.Errorf("expected 2 podcasts, got %d", len(podcasts))
	}
}

func TestSQLiteRepo_FindByID(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	p := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(p)

	// Test - found
	found, err := repo.FindByID(p.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", found.Title)
	}

	// Test - not found
	_, err = repo.FindByID(999)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestSQLiteRepo_Delete(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	p := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(p)

	// Test
	err := repo.Delete(p.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = repo.FindByID(p.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSQLiteRepo_SaveEpisode(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup: create podcast
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(podcast)

	// Test
	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Episode 1",
		Description: "Test episode",
		AudioURL:    "https://example.com/episode.mp3",
	}

	err := repo.SaveEpisode(episode)
	if err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	if episode.ID == 0 {
		t.Error("SaveEpisode should set ID")
	}
}

func TestSQLiteRepo_FindEpisodesByPodcastID(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(podcast)

	e1 := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	e2 := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 2", AudioURL: "https://example.com/2.mp3"}
	repo.SaveEpisode(e1)
	repo.SaveEpisode(e2)

	// Test
	episodes, err := repo.FindEpisodesByPodcastID(podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}

	if len(episodes) != 2 {
		t.Errorf("expected 2 episodes, got %d", len(episodes))
	}
}

func TestSQLiteRepo_FindEpisodeByID(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(podcast)

	episode := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	repo.SaveEpisode(episode)

	// Test - found
	found, err := repo.FindEpisodeByID(episode.ID)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if found.Title != "Ep 1" {
		t.Errorf("expected title 'Ep 1', got '%s'", found.Title)
	}

	// Test - not found
	_, err = repo.FindEpisodeByID(999)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestSQLiteRepo_DeleteEpisode(t *testing.T) {
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(podcast)

	episode := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	repo.SaveEpisode(episode)

	// Test
	err := repo.DeleteEpisode(episode.ID)
	if err != nil {
		t.Fatalf("DeleteEpisode failed: %v", err)
	}

	// Verify deleted
	_, err = repo.FindEpisodeByID(episode.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}
