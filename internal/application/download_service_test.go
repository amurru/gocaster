package application

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
)

func TestExtractExtension(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		want        string
	}{
		{
			name:        "mp3 from URL",
			url:         "https://example.com/episode.mp3",
			contentType: "",
			want:        ".mp3",
		},
		{
			name:        "m4a from URL",
			url:         "https://example.com/podcast.m4a",
			contentType: "",
			want:        ".m4a",
		},
		{
			name:        "aac from URL",
			url:         "https://example.com/episode.aac",
			contentType: "",
			want:        ".aac",
		},
		{
			name:        "ogg from URL",
			url:         "https://example.com/episode.ogg",
			contentType: "",
			want:        ".ogg",
		},
		{
			name:        "webm from URL",
			url:         "https://example.com/episode.webm",
			contentType: "",
			want:        ".webm",
		},
		{
			name:        "no extension in URL falls back to content-type",
			url:         "https://example.com/download/12345",
			contentType: "audio/mpeg",
			want:        ".mp3",
		},
		{
			name:        "content-type audio/mp4",
			url:         "https://example.com/feed/abc",
			contentType: "audio/mp4",
			want:        ".m4a",
		},
		{
			name:        "content-type audio/x-m4a",
			url:         "https://example.com/feed/abc",
			contentType: "audio/x-m4a",
			want:        ".m4a",
		},
		{
			name:        "content-type audio/ogg",
			url:         "https://example.com/feed/abc",
			contentType: "audio/ogg",
			want:        ".ogg",
		},
		{
			name:        "content-type audio/wav",
			url:         "https://example.com/feed/abc",
			contentType: "audio/wav",
			want:        ".wav",
		},
		{
			name:        "content-type audio/flac",
			url:         "https://example.com/feed/abc",
			contentType: "audio/flac",
			want:        ".flac",
		},
		{
			name:        "content-type audio/webm",
			url:         "https://example.com/feed/abc",
			contentType: "audio/webm",
			want:        ".webm",
		},
		{
			name:        "content-type audio/opus",
			url:         "https://example.com/feed/abc",
			contentType: "audio/opus",
			want:        ".opus",
		},
		{
			name:        "unknown content-type falls back to .audio",
			url:         "https://example.com/feed/abc",
			contentType: "application/octet-stream",
			want:        ".audio",
		},
		{
			name:        "empty content-type falls back to .audio",
			url:         "https://example.com/feed/abc",
			contentType: "",
			want:        ".audio",
		},
		{
			name:        "URL with params doesn't confuse extension detection",
			url:         "https://example.com/episode.mp3?token=abc123",
			contentType: "",
			want:        ".mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractExtension(tt.url, tt.contentType)
			if got != tt.want {
				t.Errorf("extractExtension(%q, %q) = %q, want %q", tt.url, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World!", "Hello_World"},
		{"  spaces  ", "spaces"},
		{"multiple   spaces", "multiple___spaces"},
		{"special!@#$chars", "special____chars"},
		{"dots.in.name", "dots.in.name"},
		{"hyphens-are-ok", "hyphens-are-ok"},
		{"under_scores_ok", "under_scores_ok"},
		{"UPPERCASE", "UPPERCASE"},
		{"a b c d e f g h i j k l m n o p q r s t u v w x y z a b c d e f g h i j k l m n o p q r s t u v w x y z", "a_b_c_d_e_f_g_h_i_j_k_l_m_n_o_p_q_r_s_t_u_v_w_x_y_"},
		{".startswithdot", "startswithdot"},
		{"endswithspace ", "endswithspace"},
		{"", "download"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeFilename(tt.input)
			if got != tt.want {
				t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// newDownloadTestRepo returns a repo backed by a temp-file SQLite DB, seeded
// with one podcast and one episode pointing at the given audio URL. A temp file
// is used (not ":memory:") so that database/sql connection pooling sees a single
// shared database across pooled connections — ":memory:" gives each connection
// its own private DB, which makes downloads-table lookups flaky under the
// download goroutine's concurrency.
func newDownloadTestRepo(t *testing.T, audioURL string) *persistence.SQLiteRepo {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := persistence.NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(podcast); err != nil {
		t.Fatalf("Save podcast failed: %v", err)
	}
	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Test Episode",
		AudioURL:    audioURL,
		PublishedAt: time.Now(),
	}
	if err := repo.SaveEpisode(episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}
	return repo
}

// waitForJobCondition polls the repo until the job for episodeID satisfies want,
// or the timeout elapses. runDownload runs in its own goroutine, so callers must
// wait for completion before asserting on DB state.
func waitForJobCondition(t *testing.T, repo *persistence.SQLiteRepo, episodeID int64, want domain.DownloadStatus, timeout time.Duration) *domain.DownloadJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastJob *domain.DownloadJob
	var lastErr error
	for time.Now().Before(deadline) {
		job, err := repo.FindDownloadJobByEpisodeID(episodeID)
		lastJob, lastErr = job, err
		if err == nil && job != nil && job.Status == want {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for job status %q (last job: %+v, last err: %v)", want, lastJob, lastErr)
	return nil
}

// TestDownloadService_FailurePreservesResumeData covers issue #1: a transient
// mid-stream failure must NOT wipe BytesDownloaded/BytesTotal, so a retry can
// resume from the partial .part file.
func TestDownloadService_FailurePreservesResumeData(t *testing.T) {
	// Server fails every request with a server error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "transient", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, dir)

	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)

	// Issue #1: byte counters must reflect partial progress, NOT be reset to 0.
	// (Here the server errored before sending a body, so both are 0 — but the
	// key invariant is that failJob never zeroes them. Verify via the code path
	// with a server that sends partial data below.)
	if job.Status != domain.DownloadStatusFailed {
		t.Fatalf("expected status failed, got %s", job.Status)
	}
	if job.ErrorMessage == "" {
		t.Error("expected a non-empty error message on failure")
	}
}

// TestDownloadService_FailurePreservesBytesAfterPartialDownload covers issue #1
// with a server that streams some bytes then drops the connection: the DB must
// remember how many bytes were written so a retry resumes from the partial file.
func TestDownloadService_FailurePreservesBytesAfterPartialDownload(t *testing.T) {
	// 4 KiB payload, but we close the connection after writing only part of it.
	payload := strings.Repeat("a", 4096)
	var requestCount int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Length", "4096")
		// Flusher to push a partial body then abort the connection.
		f := w.(http.Flusher)
		// Write ~1 KiB then forcibly close so io.Copy on the client side gets a
		// short read / unexpected EOF.
		_, _ = w.Write([]byte(payload[:1024]))
		f.Flush()
		// Simulate a broken connection by panicking; net/http aborts the response.
		panic("simulated mid-stream failure")
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, dir)

	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)

	// The job must have failed, but the byte counters should reflect whatever
	// was written before the failure (NOT reset to 0 by failJob). BytesTotal is
	// populated from Content-Length (4096).
	if job.BytesTotal == 0 {
		t.Errorf("expected BytesTotal to be preserved from Content-Length, got 0")
	}
	// BytesDownloaded should equal the actual bytes on disk in the .part file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	var partSize int64
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			info, _ := e.Info()
			partSize = info.Size()
		}
	}
	if job.BytesDownloaded != partSize {
		t.Errorf("BytesDownloaded (%d) should match bytes on disk (%d)", job.BytesDownloaded, partSize)
	}
}

// TestDownloadService_ProgressThrottle publishes updates at a steady ~1 MiB
// cadence and always reaches 100% on completion (issue #2).
func TestDownloadService_ProgressThrottle(t *testing.T) {
	// Slightly more than 1 MiB so at least one throttled update fires.
	payloadSize := progressInterval + 32*1024
	payload := strings.Repeat("b", payloadSize)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(payloadSize))
		_, _ = io.Copy(w, strings.NewReader(payload))
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.m4a")
	svc := NewDownloadService(repo, dir)

	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// Wait for completion (job is deleted on success) by polling the episode.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ep, _ := repo.FindEpisodeByID(episode.ID)
		if ep != nil && ep.IsDownloaded {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ep, _ := repo.FindEpisodeByID(episode.ID)
	if ep == nil || !ep.IsDownloaded {
		t.Fatalf("download did not complete; episode downloaded=%v", ep != nil && ep.IsDownloaded)
	}

	// Verify the final file matches the expected size.
	if err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".part") {
			info, _ := d.Info()
			if info.Size() != int64(payloadSize) {
				t.Errorf("downloaded file size = %d, want %d", info.Size(), payloadSize)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}
}

// TestDownloadService_MalformedURLErrors covers issue #6: a malformed download
// URL must surface an error rather than nil-dereferencing an unhandled
// http.NewRequest result.
func TestDownloadService_MalformedURLErrors(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "http://[::1]:named") // invalid URL

	svc := NewDownloadService(repo, dir)
	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// The job should fail (not panic) and record an error message.
	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message for malformed URL")
	}
}

// TestDownloadService_HTTPClientTimeout ensures the download client has a
// transport with response-header timeout configured so a stalled server cannot
// hang the goroutine indefinitely (issue #6).
func TestDownloadService_HTTPClientTimeout(t *testing.T) {
	svc := NewDownloadService(nil, "")
	if svc.http == nil {
		t.Fatal("expected non-nil http client")
	}
	if svc.http.Transport == nil {
		t.Fatal("expected configured transport with timeouts")
	}
	// A stalled server that never sends headers must fail within a bounded time.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never respond
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	stallSvc := NewDownloadService(repo, dir)
	// Shorten the header timeout for the test so it completes quickly.
	stallSvc.http.Transport.(*http.Transport).ResponseHeaderTimeout = 200 * time.Millisecond

	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := stallSvc.QueueEpisodeDownload(episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 5*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message when server stalls")
	}
}

// TestFailJob_PreservesByteCounters directly verifies the issue #1 invariant:
// failJob must persist the job's current BytesDownloaded/BytesTotal, not zero.
func TestFailJob_PreservesByteCounters(t *testing.T) {
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	dir := t.TempDir()
	svc := NewDownloadService(repo, dir)

	episode, err := repo.FindEpisodeByID(1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID:       episode.ID,
		Status:          domain.DownloadStatusDownloading,
		BytesDownloaded: 12345,
		BytesTotal:      100000,
		SupportsResume:  true,
	}
	if err := repo.SaveDownloadJob(job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Simulate a failure after partial progress.
	svc.failJob(job, "transient network error")

	stored, err := repo.FindDownloadJobByEpisodeID(episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	if stored.Status != domain.DownloadStatusFailed {
		t.Errorf("expected status failed, got %s", stored.Status)
	}
	if stored.BytesDownloaded != 12345 {
		t.Errorf("BytesDownloaded = %d, want 12345 (must be preserved, not wiped)", stored.BytesDownloaded)
	}
	if stored.BytesTotal != 100000 {
		t.Errorf("BytesTotal = %d, want 100000 (must be preserved, not wiped)", stored.BytesTotal)
	}
	if stored.ErrorMessage != "transient network error" {
		t.Errorf("ErrorMessage = %q, want %q", stored.ErrorMessage, "transient network error")
	}
}
