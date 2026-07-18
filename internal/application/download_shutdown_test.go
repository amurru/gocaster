package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

// TestDownloadService_ShutdownWaitsForWorkers covers issue #15: Shutdown must
// not return until every runDownload goroutine has exited. Here a worker is
// parked mid-body-Read inside a server that holds the connection open until
// released, so the worker cannot leave runDownload. If Shutdown returned before
// the worker exited (the old behaviour, where Shutdown only cancelled contexts
// and returned immediately), the "Shutdown returned while worker still running"
// branch would fire.
//
// The assertion is meaningful because the cancellation path does real work
// after the read returns (failJob writes Failed status to the DB); the test
// confirms Shutdown does not overlap that work with the composition root's
// subsequent repo.Close().
func TestDownloadService_ShutdownWaitsForWorkers(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release
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

	waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusDownloading, 3*time.Second)
	time.Sleep(100 * time.Millisecond)

	baseline := goroutineCount(t)

	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	close(release)

	if got := goroutineCount(t); got > baseline {
		t.Fatalf("worker goroutine still running after Shutdown: baseline=%d, now=%d", baseline, got)
	}

	job, err := repo.FindDownloadJobByEpisodeID(context.Background(), episode.ID)
	if err != nil {
		t.Fatalf("FindDownloadJobByEpisodeID failed: %v", err)
	}
	if job.Status != domain.DownloadStatusDownloading {
		t.Fatalf("expected Downloading status after shutdown (resumable), got %s", job.Status)
	}
}

// TestDownloadService_ShutdownRejectsNewWorkers covers the second part of issue
// #15: once Shutdown has run, startWorker must refuse to spawn new workers, so a
// StartJob issued during/after teardown cannot wg.Add after Shutdown's wg.Wait
// has already returned — that would leak the worker and violate the WaitGroup
// non-concurrent-Add invariant. The queued job stays in Downloading for the next
// launch to resume.
func TestDownloadService_ShutdownRejectsNewWorkers(t *testing.T) {
	var handlerCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	repo := newDownloadTestRepo(t, srv.URL+"/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

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
	if err := svc.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}

	if handlerCalls != 0 {
		t.Fatalf("expected zero HTTP handler invocations after shutdown, got %d", handlerCalls)
	}

	done := make(chan struct{})
	go func() {
		_ = svc.Shutdown(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second Shutdown blocked; startWorker spawned a worker after Shutdown")
	}
}

// TestDownloadService_NoGoroutineLeakAfterShutdown is the goleak-style
// assertion from issue #15's acceptance criteria: after Shutdown returns, no
// runDownload goroutine remains. It snapshots the goroutine count before any
// download, runs a download to completion, shuts down, and confirms the count
// returns to baseline. Run under -race to catch any residual worker touching
// repo state after the composition root would have closed it.
func TestDownloadService_NoGoroutineLeakAfterShutdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "16")
		_, _ = w.Write(make([]byte, 16))
	}))
	defer srv.Close()

	baseline := goroutineCount(t)

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

	// Wait for the download to complete (job is deleted on success).
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
		t.Fatal("download did not complete before deadline")
	}

	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// After Shutdown, the goroutine count must be back at the baseline. A leaked
	// runDownload (e.g. a worker spawned but never waited on) keeps it elevated.
	if got := goroutineCount(t); got > baseline {
		t.Fatalf("goroutine leak: baseline=%d, after shutdown=%d", baseline, got)
	}
}

// TestDownloadService_ShutdownIdempotent mirrors the MPRIS test of the same
// name: Shutdown must be safe to call more than once. A second Shutdown is a
// no-op — there is nothing to cancel and nothing to wait on, so it returns
// immediately without re-using the WaitGroup.
func TestDownloadService_ShutdownIdempotent(t *testing.T) {
	dir := t.TempDir()
	repo := newDownloadTestRepo(t, "https://example.com/ep.mp3")
	svc := NewDownloadService(repo, repo, repo, repo, dir, nil)

	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown failed: %v", err)
	}
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown failed: %v", err)
	}
}

// goroutineCount returns the number of live goroutines, retrying briefly so the
// runtime has a chance to retire a just-exited one before we sample. Used only
// for leak-detection bounds, not exact counts.
func goroutineCount(t *testing.T) int {
	t.Helper()
	var n int
	for i := 0; i < 20; i++ {
		n = runtime.NumGoroutine()
		time.Sleep(10 * time.Millisecond)
	}
	return n
}
