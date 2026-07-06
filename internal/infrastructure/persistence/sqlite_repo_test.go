package persistence

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	_ "github.com/mattn/go-sqlite3"
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

// TestSQLiteRepo_ForeignKeysCascadeOnDelete covers issue #5: with
// PRAGMA foreign_keys = ON, deleting a podcast must cascade to its episodes
// and download jobs (the schema declares ON DELETE CASCADE but it was a no-op
// before the pragma was set per-connection).
func TestSQLiteRepo_ForeignKeysCascadeOnDelete(t *testing.T) {
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Cascade Test", FeedURL: "https://example.com/cascade.xml"}
	if err := repo.Save(podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "Ep",
		AudioURL:  "https://example.com/cascade.mp3",
	}
	if err := repo.SaveEpisode(episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Delete the parent podcast; cascade should remove the episode and job.
	if err := repo.Delete(podcast.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if eps, err := repo.FindEpisodesByPodcastID(podcast.ID); err != nil || len(eps) != 0 {
		t.Errorf("episodes not cascaded: err=%v eps=%d", err, len(eps))
	}
	if _, err := repo.FindDownloadJobByEpisodeID(episode.ID); err == nil {
		t.Error("download job not cascaded: expected error (no rows)")
	}
}

// TestSQLiteRepo_ForeignKeyEnforcedRejectsOrphan covers issue #5: with
// foreign_keys = ON, inserting a child row referencing a nonexistent parent
// must fail (previously it silently succeeded).
func TestSQLiteRepo_ForeignKeyEnforcedRejectsOrphan(t *testing.T) {
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	orphan := &domain.Episode{
		PodcastID: 99999, // no such podcast
		Title:     "Orphan",
		AudioURL:  "https://example.com/orphan.mp3",
	}
	if err := repo.SaveEpisode(orphan); err == nil {
		t.Error("expected foreign-key violation when inserting an orphan episode, got nil")
	}
}

// TestSQLiteRepo_WALJournalMode covers issue #5: a file-backed DB must report
// WAL journal mode. (In-memory DBs keep "memory" mode, so this uses a temp file.)
func TestSQLiteRepo_WALJournalMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "wal_test.db")
	repo, err := NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	var mode string
	// Use the repo's underlying connection by querying through the public API
	// is not possible; reach into the *sql.DB via a small helper query instead.
	// journal_mode is set on the file, so a fresh raw connection sees it too.
	raw, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("reopen db failed: %v", err)
	}
	defer raw.Close()
	if err := raw.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode failed: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

// TestSQLiteRepo_ForeignKeyPragmaOn verifies the repo's own pooled connection
// has foreign_keys = ON (set via the ConnectHook). Querying the repo's *sql.DB
// (same package, so the unexported field is reachable) exercises the real
// connection the application uses.
func TestSQLiteRepo_ForeignKeyPragmaOn(t *testing.T) {
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	var fk int
	if err := repo.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys failed: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ConnectHook pragma not applied)", fk)
	}

	var bt int
	if err := repo.db.QueryRow("PRAGMA busy_timeout").Scan(&bt); err != nil {
		t.Fatalf("query busy_timeout failed: %v", err)
	}
	if bt != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", bt)
	}
}
