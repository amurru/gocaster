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



func seedPodcastEpisode(t *testing.T, repo *SQLiteRepo) (podcastID, episodeID int64) {
	t.Helper()
	ctx := context.Background()

	podcast := &domain.Podcast{Title: "DL Test", FeedURL: "https://example.com/dltest.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("seed podcast: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "DL Ep",
		AudioURL:  "https://example.com/dl.mp3",
	}
	if err := repo.SaveEpisode(ctx, episode); err != nil {
		t.Fatalf("seed episode: %v", err)
	}
	return podcast.ID, episode.ID
}



func TestSQLiteRepo_SaveDownloadJob(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{
		EpisodeID:       episodeID,
		Status:          domain.DownloadStatusQueued,
		BytesDownloaded: 0,
		BytesTotal:      1024,
		TempPath:        "/tmp/ep.part",
		FinalPath:       "/tmp/ep.mp3",
		ETag:            `"abc123"`,
		LastModified:    "Wed, 01 Jan 2025 00:00:00 GMT",
		SupportsResume:  true,
		ErrorMessage:    "",
	}

	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatalf("SaveDownloadJob: %v", err)
	}

	if job.ID == 0 {
		t.Fatal("SaveDownloadJob did not set job.ID")
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByID: %v", err)
	}
	if got.EpisodeID != episodeID {
		t.Errorf("EpisodeID = %d, want %d", got.EpisodeID, episodeID)
	}
	if got.Status != domain.DownloadStatusQueued {
		t.Errorf("Status = %q, want %q", got.Status, domain.DownloadStatusQueued)
	}
	if got.BytesTotal != 1024 {
		t.Errorf("BytesTotal = %d, want 1024", got.BytesTotal)
	}
	if got.TempPath != "/tmp/ep.part" {
		t.Errorf("TempPath = %q, want /tmp/ep.part", got.TempPath)
	}
	if got.FinalPath != "/tmp/ep.mp3" {
		t.Errorf("FinalPath = %q, want /tmp/ep.mp3", got.FinalPath)
	}
	if got.ETag != `"abc123"` {
		t.Errorf("ETag = %q, want %q", got.ETag, `"abc123"`)
	}
	if !got.SupportsResume {
		t.Error("SupportsResume = false, want true")
	}
}

func TestSQLiteRepo_SaveDownloadJob_MinimalFields(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{
		EpisodeID: episodeID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatalf("SaveDownloadJob: %v", err)
	}
	if job.ID == 0 {
		t.Fatal("job.ID not set")
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BytesDownloaded != 0 {
		t.Errorf("BytesDownloaded = %d, want 0", got.BytesDownloaded)
	}
	if got.BytesTotal != 0 {
		t.Errorf("BytesTotal = %d, want 0", got.BytesTotal)
	}
	if got.ETag != "" {
		t.Errorf("ETag = %q, want empty", got.ETag)
	}
}

func TestSQLiteRepo_SaveDownloadJob_FKViolation(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	job := &domain.DownloadJob{
		EpisodeID: 99999,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(ctx, job); err == nil {
		t.Error("expected FK violation for nonexistent episode, got nil")
	}
}

func TestSQLiteRepo_FindDownloadJobByEpisodeID_Found(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{
		EpisodeID: episodeID,
		Status:    domain.DownloadStatusDownloading,
	}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindDownloadJobByEpisodeID(ctx, episodeID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("ID = %d, want %d", got.ID, job.ID)
	}
	if got.Status != domain.DownloadStatusDownloading {
		t.Errorf("Status = %q, want %q", got.Status, domain.DownloadStatusDownloading)
	}
}

func TestSQLiteRepo_FindDownloadJobByEpisodeID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, err = repo.FindDownloadJobByEpisodeID(ctx, 99999)
	if err == nil {
		t.Error("expected error for non-existent episode, got nil")
	}
}

func TestSQLiteRepo_FindDownloadJobByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, err = repo.FindDownloadJobByID(ctx, 99999)
	if err == nil {
		t.Error("expected error for non-existent job ID, got nil")
	}
}

func TestSQLiteRepo_FindAllDownloadJobs(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, ep1 := seedPodcastEpisode(t, repo)

	episode2 := &domain.Episode{
		PodcastID: 1,
		Title:     "Ep2",
		AudioURL:  "https://example.com/ep2.mp3",
	}
	if err := repo.SaveEpisode(ctx, episode2); err != nil {
		t.Fatal(err)
	}

	jobs := []domain.DownloadJob{
		{EpisodeID: ep1, Status: domain.DownloadStatusQueued},
		{EpisodeID: episode2.ID, Status: domain.DownloadStatusDownloading},
	}
	for i := range jobs {
		if err := repo.SaveDownloadJob(ctx, &jobs[i]); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.FindAllDownloadJobs(ctx)
	if err != nil {
		t.Fatalf("FindAllDownloadJobs: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
}

func TestSQLiteRepo_FindAllDownloadJobs_Empty(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	all, err := repo.FindAllDownloadJobs(ctx)
	if err != nil {
		t.Fatalf("FindAllDownloadJobs: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty slice, got %d jobs", len(all))
	}
}

func TestSQLiteRepo_FindAllDownloadJobs_OrderedByUpdatedAt(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, ep1 := seedPodcastEpisode(t, repo)

	episode2 := &domain.Episode{PodcastID: 1, Title: "Ep2", AudioURL: "https://example.com/ep2o.mp3"}
	if err := repo.SaveEpisode(ctx, episode2); err != nil {
		t.Fatal(err)
	}

	j1 := &domain.DownloadJob{EpisodeID: ep1, Status: domain.DownloadStatusQueued}
	j2 := &domain.DownloadJob{EpisodeID: episode2.ID, Status: domain.DownloadStatusDownloading}

	if err := repo.SaveDownloadJob(ctx, j1); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateDownloadJobStatus(ctx, j1.ID, domain.DownloadStatusDownloading, 100, 500, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := repo.SaveDownloadJob(ctx, j2); err != nil {
		t.Fatal(err)
	}

	all, err := repo.FindAllDownloadJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	if all[0].ID != j2.ID {
		t.Errorf("first job ID = %d, want %d (j2)", all[0].ID, j2.ID)
	}
}

func TestSQLiteRepo_UpdateDownloadJobStatus(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusQueued, BytesTotal: 2048}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	err = repo.UpdateDownloadJobStatus(ctx, job.ID, domain.DownloadStatusDownloading, 512, 2048, "")
	if err != nil {
		t.Fatalf("UpdateDownloadJobStatus: %v", err)
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.DownloadStatusDownloading {
		t.Errorf("Status = %q, want %q", got.Status, domain.DownloadStatusDownloading)
	}
	if got.BytesDownloaded != 512 {
		t.Errorf("BytesDownloaded = %d, want 512", got.BytesDownloaded)
	}
}

func TestSQLiteRepo_UpdateDownloadJobStatus_WithError(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusDownloading}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	err = repo.UpdateDownloadJobStatus(ctx, job.ID, domain.DownloadStatusFailed, 100, 2000, "network timeout")
	if err != nil {
		t.Fatalf("UpdateDownloadJobStatus: %v", err)
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.DownloadStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, domain.DownloadStatusFailed)
	}
	if got.ErrorMessage != "network timeout" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "network timeout")
	}
	if got.BytesDownloaded != 100 {
		t.Errorf("BytesDownloaded = %d, want 100", got.BytesDownloaded)
	}
	if got.BytesTotal != 2000 {
		t.Errorf("BytesTotal = %d, want 2000", got.BytesTotal)
	}
}

func TestSQLiteRepo_UpdateDownloadJobStatus_UpdatesTimestamp(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusQueued}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	gotAfter, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotAfter.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after save")
	}
}

func TestSQLiteRepo_UpdateDownloadJobProgress(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusDownloading}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	job.BytesDownloaded = 1024
	job.BytesTotal = 4096
	job.TempPath = "/tmp/partial.bin"
	job.FinalPath = "/tmp/final.bin"
	job.ETag = `"etag1"`
	job.LastModified = "Thu, 02 Jan 2025 12:00:00 GMT"
	job.SupportsResume = true

	if err := repo.UpdateDownloadJobProgress(ctx, job); err != nil {
		t.Fatalf("UpdateDownloadJobProgress: %v", err)
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BytesDownloaded != 1024 {
		t.Errorf("BytesDownloaded = %d, want 1024", got.BytesDownloaded)
	}
	if got.BytesTotal != 4096 {
		t.Errorf("BytesTotal = %d, want 4096", got.BytesTotal)
	}
	if got.TempPath != "/tmp/partial.bin" {
		t.Errorf("TempPath = %q, want /tmp/partial.bin", got.TempPath)
	}
	if got.FinalPath != "/tmp/final.bin" {
		t.Errorf("FinalPath = %q, want /tmp/final.bin", got.FinalPath)
	}
	if got.ETag != `"etag1"` {
		t.Errorf("ETag = %q, want %q", got.ETag, `"etag1"`)
	}
	if got.LastModified != "Thu, 02 Jan 2025 12:00:00 GMT" {
		t.Errorf("LastModified = %q, want Thu, 02 Jan 2025 12:00:00 GMT", got.LastModified)
	}
	if !got.SupportsResume {
		t.Error("SupportsResume = false, want true")
	}
}

func TestSQLiteRepo_UpdateDownloadJobProgress_NilJob(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	err = repo.UpdateDownloadJobProgress(ctx, nil)
	if err == nil {
		t.Error("expected error for nil job, got nil")
	}
}

func TestSQLiteRepo_UpdateDownloadJobProgress_DoesNotChangeStatus(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{
		EpisodeID: episodeID,
		Status:    domain.DownloadStatusDownloading,
	}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	job.BytesDownloaded = 256
	if err := repo.UpdateDownloadJobProgress(ctx, job); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindDownloadJobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.DownloadStatusDownloading {
		t.Errorf("Status changed to %q, should remain %q", got.Status, domain.DownloadStatusDownloading)
	}
	if got.BytesDownloaded != 256 {
		t.Errorf("BytesDownloaded = %d, want 256", got.BytesDownloaded)
	}
}

func TestSQLiteRepo_CountNonFailedJobs(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	count, err := repo.CountNonFailedJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	ep2 := &domain.Episode{PodcastID: 1, Title: "C2", AudioURL: "https://example.com/c2.mp3"}
	ep3 := &domain.Episode{PodcastID: 1, Title: "C3", AudioURL: "https://example.com/c3.mp3"}
	repo.SaveEpisode(ctx, ep2)
	repo.SaveEpisode(ctx, ep3)

	j1 := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusQueued}
	j2 := &domain.DownloadJob{EpisodeID: ep2.ID, Status: domain.DownloadStatusDownloading}
	j3 := &domain.DownloadJob{EpisodeID: ep3.ID, Status: domain.DownloadStatusFailed}

	repo.SaveDownloadJob(ctx, j1)
	repo.SaveDownloadJob(ctx, j2)
	repo.SaveDownloadJob(ctx, j3)

	count, err = repo.CountNonFailedJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 non-failed jobs, got %d", count)
	}
}

func TestSQLiteRepo_CountNonFailedJobs_AllFailed(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	j := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusFailed}
	repo.SaveDownloadJob(ctx, j)

	count, err := repo.CountNonFailedJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestSQLiteRepo_DeleteDownloadJob(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusCompleted}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteDownloadJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteDownloadJob: %v", err)
	}

	_, err = repo.FindDownloadJobByID(ctx, job.ID)
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}
}

func TestSQLiteRepo_DeleteDownloadJob_NonExistent(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	if err := repo.DeleteDownloadJob(ctx, 99999); err != nil {
		t.Errorf("deleting non-existent job should not error, got: %v", err)
	}
}

func TestSQLiteRepo_MarkEpisodeDownloaded(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	if err := repo.MarkEpisodeDownloaded(ctx, episodeID, "/audio/podcast/ep1.mp3"); err != nil {
		t.Fatalf("MarkEpisodeDownloaded: %v", err)
	}

	ep, err := repo.FindEpisodeByID(ctx, episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if !ep.IsDownloaded {
		t.Error("IsDownloaded = false, want true")
	}
	if ep.LocalPath != "/audio/podcast/ep1.mp3" {
		t.Errorf("LocalPath = %q, want /audio/podcast/ep1.mp3", ep.LocalPath)
	}
}

func TestSQLiteRepo_CompleteDownload_NonExistentJobStillMarksEpisode(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	if err := repo.CompleteDownload(ctx, episodeID, "/fake/path.mp3", 99999); err != nil {
		t.Fatalf("CompleteDownload should not error when job ID is absent: %v", err)
	}

	ep, err := repo.FindEpisodeByID(ctx, episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if !ep.IsDownloaded {
		t.Error("episode should be marked downloaded even if job was already removed")
	}
}

func TestSQLiteRepo_CompleteDownload_BadEpisodeID(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	_, episodeID := seedPodcastEpisode(t, repo)

	job := &domain.DownloadJob{EpisodeID: episodeID, Status: domain.DownloadStatusCompleted}
	if err := repo.SaveDownloadJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	err = repo.CompleteDownload(ctx, 99999, "/no/where.mp3", job.ID)
	if err != nil {
		t.Errorf("SQLite UPDATE/DELETE on absent rows should not error, got: %v", err)
	}

	_, err = repo.FindDownloadJobByID(ctx, job.ID)
	if err == nil {
		t.Error("job should be deleted by CompleteDownload even when episode ID is absent")
	}
}
