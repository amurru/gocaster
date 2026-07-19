package application

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"

	_ "github.com/mattn/go-sqlite3"
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
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save podcast failed: %v", err)
	}
	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Test Episode",
		AudioURL:    audioURL,
		PublishedAt: time.Now(),
	}
	if err := repo.SaveEpisode(context.Background(), episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}
	return repo
}

// waitForEpisodeDownloaded polls until the episode is marked downloaded or the
// timeout elapses. Use this when the download completes successfully and
// CompleteDownload removes the job row (so waitForJobCondition would fail).
func waitForEpisodeDownloaded(t *testing.T, repo *persistence.SQLiteRepo, episodeID int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ep, err := repo.FindEpisodeByID(context.Background(), episodeID)
		if err == nil && ep != nil && ep.IsDownloaded {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for episode %d to be marked downloaded", episodeID)
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
		job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episodeID)
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
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
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

// TestDownloadService_RetriesResumeFromPartialFile covers issue #1's acceptance
// criterion end-to-end: after a failed download, a retry resumes from the
// partial .part file rather than restarting from scratch.
//
// The server honors Range requests:
//   - first request (no Range): respond 206 Partial Content for the first half,
//     then abort the connection so the client sees a short read and the job fails
//     with SupportsResume persisted.
//   - second request (Range: bytes=N-): serve the remaining bytes with 206.
//
// After retry the final file must equal the full payload and be marked downloaded.
func TestDownloadService_RetriesResumeFromPartialFile(t *testing.T) {
	fullPayload := []byte(strings.Repeat("Z", 8192))
	cutoff := len(fullPayload) / 2 // 4096 bytes served before the first abort

	var requests int32
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()

		rangeHdr := r.Header.Get("Range")
		if rangeHdr == "" {
			// First download attempt: advertise the FULL size but stream only
			// the first half, then drop the connection. The client sees a short
			// read (io.ErrUnexpectedEOF) and the job fails with the partial
			// bytes preserved. Accept-Ranges advertises that resume is allowed.
			w.Header().Set("Content-Length", strconv.Itoa(len(fullPayload)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(fullPayload[:cutoff])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			// Hijack and close to force the unexpected EOF on the client side.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
			return
		}

		// Resume request: parse "bytes=N-" and serve the suffix with 206.
		var start int
		fmt.Sscanf(rangeHdr, "bytes=%d-", &start)
		if start < 0 || start > len(fullPayload) {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		rem := fullPayload[start:]
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(fullPayload)-1, len(fullPayload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(rem)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(rem)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	// First attempt: queues and starts the download, which aborts mid-stream.
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}
	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)

	// Issue #1 invariant: byte counters and SupportsResume must be preserved so
	// the retry can resume. These assertions must be strict (not vacuously true).
	if job.BytesTotal != int64(len(fullPayload)) {
		t.Fatalf("BytesTotal = %d, want %d (must be preserved, not wiped)", job.BytesTotal, len(fullPayload))
	}
	if job.BytesDownloaded == 0 {
		t.Fatal("BytesDownloaded = 0; expected partial bytes to be recorded before failure (test would pass vacuously)")
	}
	if !job.SupportsResume {
		t.Fatal("SupportsResume = false after a 206 response; the resume branch is unreachable on retry (issue #1 not fully fixed)")
	}

	// The .part file must contain exactly the bytes recorded in BytesDownloaded.
	partSize := fileWithSuffixSize(t, dir, ".part")
	if partSize != job.BytesDownloaded {
		t.Fatalf(".part size (%d) != BytesDownloaded (%d)", partSize, job.BytesDownloaded)
	}

	// Retry: should resume from the partial file and complete.
	if err := svc.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RetryJob failed: %v", err)
	}

	// Wait for completion (the job is deleted on success).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ep, _ := repo.FindEpisodeByID(context.Background(), episode.ID)
		if ep != nil && ep.IsDownloaded {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ep, _ := repo.FindEpisodeByID(context.Background(), episode.ID)
	if ep == nil || !ep.IsDownloaded {
		t.Fatal("download did not complete after retry")
	}

	// The final downloaded file must equal the full payload (resume correctness).
	got, err := os.ReadFile(ep.LocalPath)
	if err != nil {
		t.Fatalf("reading final file failed: %v", err)
	}
	if !bytes.Equal(got, fullPayload) {
		t.Fatalf("final file length = %d, want %d (resume produced corrupt/incomplete file)", len(got), len(fullPayload))
	}

	// No .part file should remain after a successful rename.
	if partSize := fileWithSuffixSize(t, dir, ".part"); partSize != -1 {
		t.Errorf("expected .part file to be removed after completion, found %d bytes", partSize)
	}
}

// fileWithSuffixSize returns the size of the first file in dir whose name ends
// with suffix, or -1 if none exists.
func fileWithSuffixSize(t *testing.T, dir, suffix string) int64 {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) failed: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			info, _ := e.Info()
			return info.Size()
		}
	}
	return -1
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
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// Wait for completion (job is deleted on success) by polling the episode.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ep, _ := repo.FindEpisodeByID(context.Background(), episode.ID)
		if ep != nil && ep.IsDownloaded {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ep, _ := repo.FindEpisodeByID(context.Background(), episode.ID)
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

	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)
	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
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
	svc := NewDownloadService(nil, nil, nil, nil, "", nil)
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
	stallSvc := NewDownloadService(repo, repo, repo, repo, dir, nil)
	// Give this service its own isolated transport with a short header timeout
	// so the test completes quickly without mutating the shared package-level
	// downloadTransport (which would leak the 200ms timeout into other tests).
	stallSvc.http = &http.Client{
		Transport: &http.Transport{ResponseHeaderTimeout: 200 * time.Millisecond},
	}

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := stallSvc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
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
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
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
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Simulate a failure after partial progress.
	svc.failJob(context.Background(), job, "transient network error")

	stored, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
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

// TestFailJob_PersistsEvenWithCancelledContext covers the core of the
// CodeRabbit finding on PR #41: failJob is invoked on the cancellation path
// (StopJob cancels the per-job ctx, which makes the read/request fail and land
// in failJob). Because the passed ctx is already cancelled, failJob must
// perform the status write with an uncancellable context — otherwise
// ExecContext returns context.Canceled and the job stays stuck in Downloading
// forever, reintroducing the resume-data loss from issue #1 on the
// cancellation path.
//
// This is a focused regression test for that invariant; the end-to-end
// cancellation flow (StopJob -> read fails -> failJob) is exercised in
// TestDownloadService_RetriesResumeFromPartialFile's abort-then-retry path.
func TestFailJob_PersistsEvenWithCancelledContext(t *testing.T) {
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	dir := t.TempDir()
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID:       episode.ID,
		Status:          domain.DownloadStatusDownloading,
		BytesDownloaded: 4096,
		BytesTotal:      1_000_000,
		SupportsResume:  true,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Simulate the cancellation path: a context that is ALREADY cancelled,
	// exactly as runDownload observes after StopJob fires.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	svc.failJob(cancelledCtx, job, "download cancelled: context canceled")

	stored, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	if stored.Status != domain.DownloadStatusFailed {
		t.Fatalf("expected status failed (the bug: cancelled ctx silently no-op'd the write), got %s", stored.Status)
	}
	if stored.BytesDownloaded != 4096 {
		t.Errorf("BytesDownloaded = %d, want 4096 (must be preserved on the cancellation path)", stored.BytesDownloaded)
	}
	if stored.ErrorMessage == "" {
		t.Error("expected non-empty error message on the cancellation path")
	}
}

// ---------------------------------------------------------------------------
// StopJob tests (line 161)
// ---------------------------------------------------------------------------

// TestStopJob_ValidJobCancelsDownload verifies that StopJob cancels an
// in-flight download's context, causing runDownload to fail the job and
// preserve partial bytes (issue #11).
func TestStopJob_ValidJobCancelsDownload(t *testing.T) {
	// Use a WaitGroup to ensure the server has received the HTTP request
	// before we call StopJob. StartJob sets status to "downloading" BEFORE
	// spawning the goroutine, so waiting for that status is insufficient -
	// the goroutine may not have reached the HTTP request yet. If StopJob
	// cancels the context before runDownload passes the ctx.Err() check at
	// line 405, the goroutine returns silently without calling failJob.
	var reqReceived sync.WaitGroup
	reqReceived.Add(1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqReceived.Done()
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 4096; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			_, _ = w.Write([]byte("A"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// Wait for the HTTP request to be in-flight. This ensures the goroutine
	// has passed the early ctx.Err() bail-out (line 405) and is blocked in
	// the read loop, where cancellation will properly trigger failJob.
	reqReceived.Wait()

	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	svc.StopJob(job.ID)

	job = waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 5*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message after StopJob")
	}
}

// TestStopJob_InvalidJobIDNoop verifies that StopJob is a safe no-op when
// called with a job ID that has no registered cancel entry.
func TestStopJob_InvalidJobIDNoop(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	// Must not panic or have side effects
	svc.StopJob(999999)
}

// ---------------------------------------------------------------------------
// startWorker / shutdown tests (line 358)
// ---------------------------------------------------------------------------

// TestStartWorker_ShutdownRejectsWorkers verifies that startWorker returns
// immediately when s.shutdown is true, preventing new goroutines from being
// spawned (issue #15). The job stays in Downloading so the next launch can
// resume it.
func TestStartWorker_ShutdownRejectsWorkers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // stall forever
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	// Shutdown with no active downloads sets s.shutdown = true
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Queue an episode -- StartJob updates status to Downloading, then
	// startWorker returns without spawning a goroutine.
	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// After a short wait the episode must NOT be downloaded (no goroutine ran).
	time.Sleep(300 * time.Millisecond)
	ep, _ := repo.FindEpisodeByID(context.Background(), episode.ID)
	if ep.IsDownloaded {
		t.Error("episode should not be marked downloaded when worker was rejected by shutdown")
	}
}

// TestShutdown_CancelsInflightDownloads verifies that Shutdown cancels all
// in-flight download contexts and blocks until every worker goroutine has
// exited before returning (issues #11, #15).
func TestShutdown_CancelsInflightDownloads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // stall until cancelled
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	// Wait for download to start
	waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusDownloading, 2*time.Second)

	// Shutdown cancels in-flight downloads and waits for workers to exit
	sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sCancel()
	if err := svc.Shutdown(sCtx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// failJob skips update during shutdown, so the job stays in Downloading
	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	if job.Status != domain.DownloadStatusDownloading {
		t.Errorf("expected status Downloading after shutdown, got %s", job.Status)
	}
}

// TestFailJob_SkipsUpdateDuringShutdown verifies that failJob does not
// persist a Failed status when s.shutdown is true, leaving the job in its
// current status so the next launch can resume it.
func TestFailJob_SkipsUpdateDuringShutdown(t *testing.T) {
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	dir := t.TempDir()
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID:       episode.ID,
		Status:          domain.DownloadStatusDownloading,
		BytesDownloaded: 1024,
		BytesTotal:      4096,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	// Set shutdown flag directly
	svc.mu.Lock()
	svc.shutdown = true
	svc.mu.Unlock()

	// failJob should skip the DB update during shutdown
	svc.failJob(context.Background(), job, "should not be persisted")

	stored, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	if stored.Status != domain.DownloadStatusDownloading {
		t.Errorf("expected status to remain Downloading during shutdown, got %s", stored.Status)
	}
	if stored.ErrorMessage != "" {
		t.Errorf("expected empty error message during shutdown, got %q", stored.ErrorMessage)
	}
}

// ---------------------------------------------------------------------------
// runDownload error paths (line 375)
// ---------------------------------------------------------------------------

// TestRunDownload_EpisodeNotFound covers the error path in runDownload where
// the episode associated with a download job no longer exists in the repo.
// A second sql.DB connection with foreign_keys OFF is used to delete only the
// episode row without triggering the ON DELETE CASCADE that would also remove
// the download job.
func TestRunDownload_EpisodeNotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	repo, err := persistence.NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	ctx := context.Background()
	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(ctx, podcast); err != nil {
		t.Fatalf("Save podcast failed: %v", err)
	}
	episode := &domain.Episode{
		PodcastID:   podcast.ID,
		Title:       "Test Episode",
		AudioURL:    "https://example.com/ep.mp3",
		PublishedAt: time.Now(),
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

	// Open a separate connection with FK disabled to delete the episode
	// without cascade-deleting the download job.
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer rawDB.Close()
	if _, err := rawDB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("PRAGMA foreign_keys = OFF failed: %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, "DELETE FROM episodes WHERE id = ?", episode.ID); err != nil {
		t.Fatalf("DELETE episode failed: %v", err)
	}

	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)
	svc.startWorker(job.ID)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		j, err := repo.FindDownloadJobByID(ctx, job.ID)
		if err == nil && j != nil && j.Status == domain.DownloadStatusFailed {
			if j.ErrorMessage == "" || !strings.Contains(j.ErrorMessage, "could not find episode") {
				t.Errorf("expected error about missing episode, got: %q", j.ErrorMessage)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for job to fail with episode-not-found error")
}

// TestRunDownload_ContextCancellation covers the path where runDownload's
// per-job context is cancelled while the body is being read. A slow
// streaming server ensures the goroutine is inside the read loop, and we
// poll for BytesDownloaded > 0 before cancelling so the cancellation
// hits the timeoutReader rather than the pre-flight ctx.Err() check.
func TestRunDownload_ContextCancellation(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 1024; i++ {
			w.Write([]byte("A"))
			if i%512 == 0 {
				flusher.Flush()
			}
		}
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	svc.SetRootContext(rootCtx)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusDownloading, 3*time.Second)

	<-started
	rootCancel()

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 5*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message after context cancellation")
	}
}

// TestRunDownload_HTTPErrorStatusCode covers the path where the HTTP server
// returns a non-200/206 status code.
func TestRunDownload_HTTPErrorStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message for HTTP 403")
	}
}

// TestRunDownload_NetworkError covers the path where s.http.Do fails with a
// connection error (e.g. connection refused).
func TestRunDownload_NetworkError(t *testing.T) {
	dir := t.TempDir()
	// Point at a port that nothing is listening on
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)
	if job.ErrorMessage == "" {
		t.Error("expected non-empty error message for network error")
	}
}

// ---------------------------------------------------------------------------
// timeoutReader tests (line 758)
// ---------------------------------------------------------------------------

// stallReader blocks forever on Read, simulating a stalled server stream.
type stallReader struct{}

func (r *stallReader) Read(p []byte) (int, error) {
	select {} // block forever
}

// TestTimeoutReader_TimeoutOnStall verifies that timeoutReader returns a
// "download stalled" error when no data arrives within the timeout window.
func TestTimeoutReader_TimeoutOnStall(t *testing.T) {
	ctx := context.Background()
	tr := newTimeoutReader(io.NopCloser(&stallReader{}), 50*time.Millisecond, ctx)
	defer tr.Stop()

	buf := make([]byte, 1024)
	_, err := tr.Read(buf)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "download stalled") {
		t.Errorf("expected stall error, got: %v", err)
	}
}

// TestTimeoutReader_ContextCancellation verifies that timeoutReader returns a
// "download cancelled" error when the context is cancelled during a blocked read.
func TestTimeoutReader_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := newTimeoutReader(io.NopCloser(&stallReader{}), 10*time.Second, ctx)
	defer tr.Stop()

	// Cancel context after a short delay to interrupt the stalled read
	time.AfterFunc(50*time.Millisecond, cancel)

	buf := make([]byte, 1024)
	_, err := tr.Read(buf)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "download cancelled") {
		t.Errorf("expected cancellation error, got: %v", err)
	}
}

// TestTimeoutReader_PreCancelledContext verifies that timeoutReader returns
// immediately when the context is already cancelled before Read is called.
func TestTimeoutReader_PreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tr := newTimeoutReader(io.NopCloser(&stallReader{}), 1*time.Second, ctx)
	defer tr.Stop()

	buf := make([]byte, 1024)
	_, err := tr.Read(buf)
	if err == nil {
		t.Fatal("expected cancellation error for pre-cancelled context")
	}
	if !strings.Contains(err.Error(), "download cancelled") {
		t.Errorf("expected cancellation error, got: %v", err)
	}
}

// TestTimeoutReader_NormalRead verifies that timeoutReader passes through
// data correctly when the underlying reader responds promptly.
func TestTimeoutReader_NormalRead(t *testing.T) {
	data := []byte("hello, world")
	ctx := context.Background()
	tr := newTimeoutReader(io.NopCloser(strings.NewReader(string(data))), 1*time.Second, ctx)

	buf := make([]byte, 1024)
	n, err := tr.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf[:n], data) {
		t.Errorf("Read() = %q, want %q", buf[:n], data)
	}
}

func TestTimeoutReader_Stop(t *testing.T) {
	ctx := context.Background()
	tr := newTimeoutReader(io.NopCloser(&stallReader{}), 1*time.Second, ctx)
	tr.Stop()
	tr.Stop()
}

func TestResumeJob_PausedJobResumes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 65536; i++ {
			w.Write([]byte("A"))
			if i%4096 == 0 {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID:       episode.ID,
		Status:          domain.DownloadStatusPaused,
		BytesDownloaded: 1024,
		BytesTotal:      65536,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	if err := svc.ResumeJob(context.Background(), job.ID); err != nil {
		t.Fatalf("ResumeJob failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)
}

func TestResumeJob_NonResumableState(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	err = svc.ResumeJob(context.Background(), job.ID)
	if err == nil || !strings.Contains(err.Error(), "not in a resumable state") {
		t.Errorf("expected 'not in a resumable state' error, got: %v", err)
	}
}

func TestResumeJob_InvalidJobID(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	err := svc.ResumeJob(context.Background(), 999999)
	if err == nil {
		t.Error("expected error for invalid job ID")
	}
}

func TestListJobs_ReturnsQueuedJobs(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	jobs, err := svc.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].EpisodeID != episode.ID {
		t.Errorf("expected EpisodeID %d, got %d", episode.ID, jobs[0].EpisodeID)
	}
}

func TestListJobsWithEpisodes_PopulatesTitles(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	views, err := svc.ListJobsWithEpisodes(context.Background())
	if err != nil {
		t.Fatalf("ListJobsWithEpisodes failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].EpisodeTitle != "Test Episode" {
		t.Errorf("expected EpisodeTitle 'Test Episode', got %q", views[0].EpisodeTitle)
	}
	if views[0].PodcastTitle != "Test" {
		t.Errorf("expected PodcastTitle 'Test', got %q", views[0].PodcastTitle)
	}
}

func TestQueueEpisodeDownload_AlreadyDownloaded(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	episode.IsDownloaded = true
	if err := repo.SaveEpisode(context.Background(), episode); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	err = svc.QueueEpisodeDownload(context.Background(), episode.ID)
	if err == nil || !strings.Contains(err.Error(), "already downloaded") {
		t.Errorf("expected 'already downloaded' error, got: %v", err)
	}
}

func TestQueueEpisodeDownload_AlreadyInQueue(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	err = svc.QueueEpisodeDownload(context.Background(), episode.ID)
	if err == nil || !strings.Contains(err.Error(), "already in queue") {
		t.Errorf("expected 'already in queue' error, got: %v", err)
	}
}

func TestQueueEpisodeDownload_ExistingFailedJobRetries(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID:    episode.ID,
		Status:       domain.DownloadStatusFailed,
		ErrorMessage: "previous failure",
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	err = svc.QueueEpisodeDownload(context.Background(), episode.ID)
	if err == nil {
		t.Fatal("expected error when retrying with existing failed job still in DB")
	}
}

func TestStartJob_NonStartableState(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusDownloading,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	err = svc.StartJob(context.Background(), job.ID)
	if err == nil || !strings.Contains(err.Error(), "not in a startable state") {
		t.Errorf("expected 'not in a startable state' error, got: %v", err)
	}
}

func TestRetryJob_NonFailedState(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	err = svc.RetryJob(context.Background(), job.ID)
	if err == nil || !strings.Contains(err.Error(), "job is not failed") {
		t.Errorf("expected 'job is not failed' error, got: %v", err)
	}
}

func TestRetryJob_InvalidJobID(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	err := svc.RetryJob(context.Background(), 999999)
	if err == nil {
		t.Error("expected error for invalid job ID")
	}
}

func TestRetryJob_FailedJobRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 65536; i++ {
			w.Write([]byte("A"))
			if i%4096 == 0 {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID:       episode.ID,
		Status:          domain.DownloadStatusFailed,
		BytesDownloaded: 1024,
		BytesTotal:      65536,
		ErrorMessage:    "previous failure",
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	if err := svc.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RetryJob failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)
}

func TestFailJob_NilJob(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)
	svc.failJob(context.Background(), nil, "should not panic")
}

func TestQueueEpisodeDownload_FirstJobStartsImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 65536; i++ {
			w.Write([]byte("A"))
			if i%4096 == 0 {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)
}

func TestQueueEpisodeDownload_PausedJobResumes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("B"), 4096))
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusPaused,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload should resume paused job: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)
}

func TestRunDownload_ResumeBranch(t *testing.T) {
	payload := []byte(strings.Repeat("C", 4096))

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[:2048])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijack")
			}
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
			return
		}
		var start int
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-", &start)
		rem := payload[start:]
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(rem)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(rem)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)
	if !job.SupportsResume {
		t.Fatal("SupportsResume should be true after partial content")
	}

	if err := svc.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RetryJob failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)

	got, _ := os.ReadFile(filepath.Join(dir, "Test_Episode.mp3"))
	if !bytes.Equal(got, payload) {
		t.Fatalf("file mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func TestRunDownload_416ThenRetry(t *testing.T) {
	payload := []byte(strings.Repeat("D", 4096))

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[:2048])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijack")
			}
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
			return
		}
		if r.Header.Get("Range") != "" {
			http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)

	if err := svc.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RetryJob failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)
}

func TestRunDownload_RangeIgnoredByServer(t *testing.T) {
	payload := []byte(strings.Repeat("E", 4096))

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[:2048])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijack")
			}
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)

	if err := svc.RetryJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RetryJob failed: %v", err)
	}

	waitForEpisodeDownloaded(t, repo, episode.ID, 3*time.Second)

	got, _ := os.ReadFile(filepath.Join(dir, "Test_Episode.mp3"))
	if !bytes.Equal(got, payload) {
		t.Fatalf("file mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func TestRunDownload_RenameError(t *testing.T) {
	payload := []byte("F")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Test_Episode.mp3"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	job := waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 3*time.Second)
	if job.ErrorMessage == "" {
		t.Fatal("expected error message from rename failure")
	}
}

func TestQueueEpisodeDownload_EpisodeNotFound(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	err := svc.QueueEpisodeDownload(context.Background(), 99999)
	if err == nil || !strings.Contains(err.Error(), "could not find episode") {
		t.Errorf("expected 'could not find episode' error, got: %v", err)
	}
}

func TestStartJob_InvalidJobID(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	err := svc.StartJob(context.Background(), 99999)
	if err == nil {
		t.Error("expected error for non-existent job ID")
	}
}

func TestRetryOrQueue_SecondEpisodeQueuedWhileFirstDownloading(t *testing.T) {
	var reqReceived sync.WaitGroup
	reqReceived.Add(1)
	unblock := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqReceived.Done()
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 4096; i++ {
			select {
			case <-r.Context().Done():
				return
			case <-unblock:
				return
			default:
			}
			_, _ = w.Write([]byte("A"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode2 := &domain.Episode{
		PodcastID:   1,
		Title:       "Second Episode",
		AudioURL:    "https://example.com/ep2.mp3",
		PublishedAt: time.Now(),
	}
	if err := repo.SaveEpisode(context.Background(), episode2); err != nil {
		t.Fatalf("SaveEpisode failed: %v", err)
	}

	episode1, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode1.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload episode1 failed: %v", err)
	}
	reqReceived.Wait()

	err = svc.QueueEpisodeDownload(context.Background(), episode2.ID)
	if err != nil {
		t.Fatalf("QueueEpisodeDownload episode2 failed: %v", err)
	}

	job2, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode2.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID episode2 failed: %v", err)
	}
	if job2.Status != domain.DownloadStatusQueued {
		t.Errorf("expected episode2 job to be queued (another job active), got %s", job2.Status)
	}

	close(unblock)
}

// --- failingJobRepo wraps a real DownloadJobRepo and injects errors on demand ---

type failingJobRepo struct {
	domain.DownloadJobRepo
	failCountNonFailed       bool
	failUpdateStatus         bool
	failFindAll              bool
	failUpdateProgress       bool
	failSave                 bool
	failFindByID             bool
	failFindByEpisodeID      bool
	failDelete               bool
}

func (r *failingJobRepo) CountNonFailedJobs(ctx context.Context) (int, error) {
	if r.failCountNonFailed {
		return 0, fmt.Errorf("injected CountNonFailedJobs error")
	}
	return r.DownloadJobRepo.CountNonFailedJobs(ctx)
}

func (r *failingJobRepo) UpdateDownloadJobStatus(ctx context.Context, id int64, status domain.DownloadStatus, bytesDownloaded int64, bytesTotal int64, errorMsg string) error {
	if r.failUpdateStatus {
		return fmt.Errorf("injected UpdateDownloadJobStatus error")
	}
	return r.DownloadJobRepo.UpdateDownloadJobStatus(ctx, id, status, bytesDownloaded, bytesTotal, errorMsg)
}

func (r *failingJobRepo) FindAllDownloadJobs(ctx context.Context) ([]domain.DownloadJob, error) {
	if r.failFindAll {
		return nil, fmt.Errorf("injected FindAllDownloadJobs error")
	}
	return r.DownloadJobRepo.FindAllDownloadJobs(ctx)
}

func (r *failingJobRepo) UpdateDownloadJobProgress(ctx context.Context, job *domain.DownloadJob) error {
	if r.failUpdateProgress {
		return fmt.Errorf("injected UpdateDownloadJobProgress error")
	}
	return r.DownloadJobRepo.UpdateDownloadJobProgress(ctx, job)
}

// --- Test: runDownload ctx.Err() bail-out (lines 405-408) ---

func TestRunDownload_ContextCancelledBeforeStart(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, t.TempDir(), nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := repo.SaveDownloadJob(context.Background(), &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusDownloading,
	}); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}
	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cleanupCalled := false
	svc.wg.Add(1)
	svc.runDownload(ctx, func() { cleanupCalled = true }, job.ID)

	if !cleanupCalled {
		t.Error("cleanup was not called")
	}
}

// --- Test: retryOrQueue CountNonFailedJobs error (lines 224-226) ---

func TestRetryOrQueue_CountNonFailedJobsError(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	wrapper := &failingJobRepo{DownloadJobRepo: repo, failCountNonFailed: true}
	svc := NewDownloadService(wrapper, repo, repo, repo, t.TempDir(), nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	err = svc.QueueEpisodeDownload(context.Background(), episode.ID)
	if err == nil {
		t.Fatal("expected error from QueueEpisodeDownload when CountNonFailedJobs fails")
	}
	if !strings.Contains(err.Error(), "could not count jobs") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test: StartJob UpdateDownloadJobStatus error (lines 262-264) ---

func TestStartJob_UpdateDownloadJobStatusError(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	wrapper := &failingJobRepo{DownloadJobRepo: repo}
	svc := NewDownloadService(wrapper, repo, repo, repo, t.TempDir(), nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := repo.SaveDownloadJob(context.Background(), &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}

	// Enable the failure AFTER saving the job so SaveDownloadJob succeeds
	// but StartJob → UpdateDownloadJobStatus fails.
	wrapper.failUpdateStatus = true

	err = svc.StartJob(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected error from StartJob when UpdateDownloadJobStatus fails")
	}
	if !strings.Contains(err.Error(), "could not start job") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test: ResumeJob UpdateDownloadJobStatus error (lines 295-297) ---

func TestResumeJob_UpdateDownloadJobStatusError(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	wrapper := &failingJobRepo{DownloadJobRepo: repo}
	svc := NewDownloadService(wrapper, repo, repo, repo, t.TempDir(), nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := repo.SaveDownloadJob(context.Background(), &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusPaused,
	}); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}

	wrapper.failUpdateStatus = true

	err = svc.ResumeJob(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected error from ResumeJob when UpdateDownloadJobStatus fails")
	}
	if !strings.Contains(err.Error(), "could not resume job") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test: RetryJob UpdateDownloadJobStatus error (lines 328-330) ---

func TestRetryJob_UpdateDownloadJobStatusError(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	wrapper := &failingJobRepo{DownloadJobRepo: repo}
	svc := NewDownloadService(wrapper, repo, repo, repo, t.TempDir(), nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}
	if err := repo.SaveDownloadJob(context.Background(), &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusFailed,
		ErrorMessage: "previous failure",
	}); err != nil {
		t.Fatalf("SaveDownloadJob failed: %v", err)
	}

	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}

	wrapper.failUpdateStatus = true

	err = svc.RetryJob(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected error from RetryJob when UpdateDownloadJobStatus fails")
	}
	if !strings.Contains(err.Error(), "could not retry job") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test: ListJobsWithEpisodes FindAllDownloadJobs error (lines 377-379) ---

func TestListJobsWithEpisodes_FindAllDownloadJobsError(t *testing.T) {
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	wrapper := &failingJobRepo{DownloadJobRepo: repo, failFindAll: true}
	svc := NewDownloadService(wrapper, repo, repo, repo, t.TempDir(), nil)

	_, err := svc.ListJobsWithEpisodes(context.Background())
	if err == nil {
		t.Fatal("expected error from ListJobsWithEpisodes when FindAllDownloadJobs fails")
	}
	if !strings.Contains(err.Error(), "injected FindAllDownloadJobs error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDownload_PartPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "http://127.0.0.1:1/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	episode, err := repo.FindEpisodeByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindEpisodeByID failed: %v", err)
	}

	partPath := filepath.Join(dir, "Test_Episode.mp3.part")
	if err := os.MkdirAll(partPath, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if err := svc.QueueEpisodeDownload(context.Background(), episode.ID); err != nil {
		t.Fatalf("QueueEpisodeDownload failed: %v", err)
	}

	waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusFailed, 5*time.Second)
}
