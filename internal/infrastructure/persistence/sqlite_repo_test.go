package persistence

import (
	"context"
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
	err = repo.Save(context.Background(), testPodcast)
	if err != nil {
		t.Errorf("migrations not run, save failed: %v", err)
	}
}

func TestSQLiteRepo_Save(t *testing.T) {
	ctx := context.Background()
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

	err = repo.Save(ctx, podcast)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Error("Save should set ID")
	}
}

func TestSQLiteRepo_FindAll(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup: create test data
	p1 := &domain.Podcast{Title: "Podcast 1", FeedURL: "https://example.com/feed1.xml"}
	p2 := &domain.Podcast{Title: "Podcast 2", FeedURL: "https://example.com/feed2.xml"}
	repo.Save(ctx, p1)
	repo.Save(ctx, p2)

	// Test
	podcasts, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}

	if len(podcasts) != 2 {
		t.Errorf("expected 2 podcasts, got %d", len(podcasts))
	}
}

func TestSQLiteRepo_FindByID(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	p := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, p)

	// Test - found
	found, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", found.Title)
	}

	// Test - not found
	_, err = repo.FindByID(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestSQLiteRepo_Delete(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	p := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, p)

	// Test
	err := repo.Delete(ctx, p.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = repo.FindByID(ctx, p.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSQLiteRepo_SaveEpisode(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup: create podcast
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, podcast)

	// Test
	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Episode 1",
		Description: "Test episode",
		AudioURL:    "https://example.com/episode.mp3",
	}

	err := repo.SaveEpisode(ctx, episode)
	if err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	if episode.ID == 0 {
		t.Error("SaveEpisode should set ID")
	}
}

func TestSQLiteRepo_FindEpisodesByPodcastID(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, podcast)

	e1 := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	e2 := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 2", AudioURL: "https://example.com/2.mp3"}
	repo.SaveEpisode(ctx, e1)
	repo.SaveEpisode(ctx, e2)

	// Test
	episodes, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}

	if len(episodes) != 2 {
		t.Errorf("expected 2 episodes, got %d", len(episodes))
	}
}

func TestSQLiteRepo_FindEpisodeByID(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, podcast)

	episode := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	repo.SaveEpisode(ctx, episode)

	// Test - found
	found, err := repo.FindEpisodeByID(ctx, episode.ID)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if found.Title != "Ep 1" {
		t.Errorf("expected title 'Ep 1', got '%s'", found.Title)
	}

	// Test - not found
	_, err = repo.FindEpisodeByID(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestSQLiteRepo_DeleteEpisode(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewSQLiteRepo(":memory:")
	defer repo.Close()

	// Setup
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	repo.Save(ctx, podcast)

	episode := &domain.Episode{PodcastID: podcast.ID, Title: "Ep 1", AudioURL: "https://example.com/1.mp3"}
	repo.SaveEpisode(ctx, episode)

	// Test
	err := repo.DeleteEpisode(ctx, episode.ID)
	if err != nil {
		t.Fatalf("DeleteEpisode failed: %v", err)
	}

	// Verify deleted
	_, err = repo.FindEpisodeByID(ctx, episode.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// TestSQLiteRepo_ForeignKeysCascadeOnDelete covers issue #5: with
// PRAGMA foreign_keys = ON, deleting a podcast must cascade to its episodes
// and download jobs (the schema declares ON DELETE CASCADE but it was a no-op
// before the pragma was set per-connection).
func TestSQLiteRepo_ForeignKeysCascadeOnDelete(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Cascade Test", FeedURL: "https://example.com/cascade.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "Ep",
		AudioURL:  "https://example.com/cascade.mp3",
	}
	if err := repo.SaveEpisode(ctx, episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Delete the parent podcast; cascade should remove the episode and job.
	if err := repo.Delete(ctx, podcast.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if eps, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID); err != nil || len(eps) != 0 {
		t.Errorf("episodes not cascaded: err=%v eps=%d", err, len(eps))
	}
	if _, err := repo.FindDownloadJobByEpisodeID(ctx, episode.ID); err == nil {
		t.Error("download job not cascaded: expected error (no rows)")
	}
}

// TestSQLiteRepo_ForeignKeyEnforcedRejectsOrphan covers issue #5: with
// foreign_keys = ON, inserting a child row referencing a nonexistent parent
// must fail (previously it silently succeeded).
func TestSQLiteRepo_ForeignKeyEnforcedRejectsOrphan(t *testing.T) {
	ctx := context.Background()
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
	if err := repo.SaveEpisode(ctx, orphan); err == nil {
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

// TestSQLiteRepo_RunInTx_CommitsOnSuccess covers issue #13: runInTx commits
// when fn returns nil, persisting every write inside the transaction.
func TestSQLiteRepo_RunInTx_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Tx Commit", FeedURL: "https://example.com/commit.xml"}
	if err := repo.runInTx(ctx, func(tx *sql.Tx) error {
		return savePodcast(ctx, tx, podcast)
	}); err != nil {
		t.Fatalf("runInTx failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Fatal("podcast.ID not set after commit")
	}
	got, err := repo.FindByID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if got.FeedURL != podcast.FeedURL {
		t.Errorf("FeedURL = %q, want %q", got.FeedURL, podcast.FeedURL)
	}
}

// TestSQLiteRepo_RunInTx_RollsBackOnError covers issue #13: when fn returns an
// error, every write inside the transaction is rolled back. The fn inserts a
// podcast (ok) then an episode with a bogus podcast_id, which the FK pragma
// (issue #5) turns into a hard error — proving the preceding podcast insert is
// undone.
func TestSQLiteRepo_RunInTx_RollsBackOnError(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Tx Rollback", FeedURL: "https://example.com/rollback.xml"}
	err = repo.runInTx(ctx, func(tx *sql.Tx) error {
		if err := savePodcast(ctx, tx, podcast); err != nil {
			return err
		}
		// Bogus podcast_id triggers a FK violation (foreign_keys = ON, issue #5).
		orphan := &domain.Episode{PodcastID: 99999, Title: "Orphan", AudioURL: "https://example.com/o.mp3"}
		return saveEpisode(ctx, tx, orphan)
	})
	if err == nil {
		t.Fatal("expected FK violation error, got nil")
	}

	if _, err := repo.FindByID(ctx, podcast.ID); err == nil {
		t.Error("podcast row should not exist after rollback")
	}
}

// TestSQLiteRepo_RunInTx_RespectsCancelledContext covers issue #13 acceptance:
// BeginTx must bind to the caller's context. A pre-cancelled context prevents
// the transaction from starting and fn never runs.
func TestSQLiteRepo_RunInTx_RespectsCancelledContext(t *testing.T) {
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before BeginTx

	ran := false
	err = repo.runInTx(ctx, func(tx *sql.Tx) error {
		ran = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error from BeginTx with cancelled context, got nil")
	}
	if ran {
		t.Error("fn should not run when BeginTx fails")
	}
}

// TestSQLiteRepo_SavePodcastWithEpisodes covers issue #13: the podcast and all
// episodes commit together, with episode.PodcastID linked to the saved podcast.
func TestSQLiteRepo_SavePodcastWithEpisodes(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Batch", FeedURL: "https://example.com/batch.xml"}
	episodes := []domain.Episode{
		{Title: "E1", AudioURL: "https://example.com/e1.mp3"},
		{Title: "E2", AudioURL: "https://example.com/e2.mp3"},
	}
	if err := repo.SavePodcastWithEpisodes(ctx, podcast, episodes); err != nil {
		t.Fatalf("SavePodcastWithEpisodes failed: %v", err)
	}

	if podcast.ID == 0 {
		t.Fatal("podcast.ID not set")
	}
	got, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d episodes, want 2", len(got))
	}
	for _, e := range got {
		if e.PodcastID != podcast.ID {
			t.Errorf("episode %q PodcastID = %d, want %d", e.Title, e.PodcastID, podcast.ID)
		}
	}
}

// TestSQLiteRepo_SavePodcastWithEpisodes_AtomicOnDuplicateFeedURL covers issue
// #13's AddPodcast atomicity criterion: a UNIQUE violation on feed_url must
// roll back the whole batch — no podcast and no episodes committed.
func TestSQLiteRepo_SavePodcastWithEpisodes_AtomicOnDuplicateFeedURL(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	// Pre-seed the feed URL so the batch's podcast insert collides.
	if err := repo.Save(ctx, &domain.Podcast{Title: "Existing", FeedURL: "https://example.com/dup.xml"}); err != nil {
		t.Fatalf("seed Save failed: %v", err)
	}

	dup := &domain.Podcast{Title: "Dup", FeedURL: "https://example.com/dup.xml"}
	episodes := []domain.Episode{
		{Title: "Should Rollback", AudioURL: "https://example.com/rb.mp3"},
	}
	err = repo.SavePodcastWithEpisodes(ctx, dup, episodes)
	if err == nil {
		t.Fatal("expected UNIQUE violation on feed_url, got nil")
	}

	// No partial state: the dup podcast was never committed, and no orphan
	// episode with its (unassigned) feed survives.
	got, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected exactly 1 podcast after rollback, got %d", len(got))
	}
	// Confirm no episodes exist at all.
	rows, err := repo.db.QueryContext(ctx, "SELECT COUNT(*) FROM episodes")
	if err != nil {
		t.Fatalf("count episodes failed: %v", err)
	}
	defer rows.Close()
	var n int
	if !rows.Next() {
		t.Fatal("no rows from episode count")
	}
	if err := rows.Scan(&n); err != nil {
		t.Fatalf("scan episode count failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 episodes after rollback, got %d", n)
	}
}

// TestSQLiteRepo_AppendEpisodesAndTouchPodcast covers issue #13: new episodes
// and the podcast's LastUpdated bump commit together.
func TestSQLiteRepo_AppendEpisodesAndTouchPodcast(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "Refresh", FeedURL: "https://example.com/refresh.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("seed Save failed: %v", err)
	}
	before := podcast.LastUpdated

	newEps := []domain.Episode{
		{Title: "New1", AudioURL: "https://example.com/new1.mp3"},
		{Title: "New2", AudioURL: "https://example.com/new2.mp3"},
	}
	podcast.LastUpdated = before.Add(time.Hour)
	if err := repo.AppendEpisodesAndTouchPodcast(ctx, podcast, newEps); err != nil {
		t.Fatalf("AppendEpisodesAndTouchPodcast failed: %v", err)
	}

	got, err := repo.FindEpisodesByPodcastID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindEpisodesByPodcastID failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 episodes, got %d", len(got))
	}
	saved, err := repo.FindByID(ctx, podcast.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if !saved.LastUpdated.Equal(podcast.LastUpdated) {
		t.Errorf("LastUpdated = %v, want %v", saved.LastUpdated, podcast.LastUpdated)
	}
}

// TestSQLiteRepo_CompleteDownload covers issue #13: marking an episode
// downloaded and deleting its job commit together.
func TestSQLiteRepo_CompleteDownload(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	defer repo.Close()

	podcast := &domain.Podcast{Title: "DL", FeedURL: "https://example.com/dl.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	episode := &domain.Episode{PodcastID: podcast.ID, Title: "Ep", AudioURL: "https://example.com/dl.mp3"}
	if err := repo.SaveEpisode(ctx, episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}
	job := &domain.DownloadJob{EpisodeID: episode.ID, Status: domain.DownloadStatusCompleted}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	if err := repo.CompleteDownload(ctx, episode.ID, "/audio/ep.mp3", job.ID); err != nil {
		t.Fatalf("CompleteDownload failed: %v", err)
	}

	got, err := repo.FindEpisodeByID(ctx, episode.ID)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if !got.IsDownloaded {
		t.Error("episode.IsDownloaded = false, want true")
	}
	if got.LocalPath != "/audio/ep.mp3" {
		t.Errorf("LocalPath = %q, want %q", got.LocalPath, "/audio/ep.mp3")
	}
	if _, err := repo.FindDownloadJobByID(ctx, job.ID); err == nil {
		t.Error("download job should have been deleted")
	}
}
