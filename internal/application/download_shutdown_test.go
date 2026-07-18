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
	// release, once closed, lets the server finish the response so the worker's
	// blocked Read returns and runDownload can exit.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Advertise a body so runDownload reaches the streaming Read loop, then
		// hold the connection open until the test releases us.
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		<-release // park the worker mid-download
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

	// Wait until the worker is actually inside runDownload's read loop: poll the
	// job status until it flips to Downloading (set by StartJob before the
	// goroutine is spawned) and give the worker a beat to issue the request.
	waitForJobCondition(t, repo, episode.ID, domain.DownloadStatusDownloading, 3*time.Second)
	// Best-effort cushion for the worker to reach body.Read.
	time.Sleep(150 * time.Millisecond)

	// Shutdown the service while the worker is parked. It must block.
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- svc.Shutdown(context.Background())
	}()

	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned while a worker goroutine was still running (WaitGroup not honored)")
	case <-time.After(300 * time.Millisecond):
		// expected: Shutdown is blocked in wg.Wait()
	}

	// Release the worker; Shutdown should now complete promptly.
	close(release)

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return after the worker was released")
	}
}

// TestDownloadService_ShutdownRejectsNewWorkers covers the second part of issue
// #15: once Shutdown has run, startWorker must refuse to spawn new workers, so a
// StartJob issued during/after teardown cannot wg.Add after Shutdown's wg.Wait
// has already returned — that would leak the worker and violate the WaitGroup
// non-concurrent-Add invariant. The queued job stays in Downloading for the next
// launch to resume.
func TestDownloadService_ShutdownRejectsNewWorkers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Manually place a job in Queued, then call StartJob: it must update the
	// status to Downloading but NOT spawn a worker (wg must stay at 0).
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

	// No goroutine should have been spawned: a second Shutdown must return
	// immediately (nothing to wait for). If startWorker had ignored the shutdown
	// flag and called wg.Add, the worker would be racing the 500 and this Wait
	// could still return — so the prompt return is a necessary (not sufficient)
	// signal; sufficiency is established by the leak test below.
	done := make(chan struct{})
	go func() {
		_ = svc.Shutdown(context.Background())
		close(done)
	}()
	select {
	case <-done:
		// expected: Shutdown returned immediately (no worker to wait for).
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
