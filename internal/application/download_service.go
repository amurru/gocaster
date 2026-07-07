package application

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

// progressInterval is the minimum delta in bytes between progress updates.
const progressInterval = 1 << 20 // 1 MiB

// stallTimeout is the maximum interval allowed between two successful body
// reads during a download. A server that accepts the connection and sends
// headers but then stops streaming data is treated as stalled and the download
// is failed, so its goroutine does not block forever (issue #6).
const stallTimeout = 60 * time.Second

// downloadTransport limits how long connecting and waiting for response headers
// may take. It deliberately does NOT cap the overall request duration, which
// would abort legitimate large downloads; an active stream is instead bounded
// by the per-read stallTimeout enforced via timeoutReader in runDownload.
var downloadTransport = &http.Transport{
	IdleConnTimeout:       60 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	ExpectContinueTimeout: 10 * time.Second,
}

type DownloadService struct {
	jobs        domain.DownloadJobRepo
	episodes    domain.EpisodeRepo
	podcasts    domain.PodcastRepo
	http        *http.Client
	downloadDir string
	logger      domain.Logger

	// rootCtx is the parent of every per-job context. It is cancelled on
	// shutdown, which cancels every in-flight runDownload goroutine (issue #11).
	// It defaults to context.Background() until SetRootContext is called by the
	// composition root.
	rootCtx context.Context

	// mu guards cancels. cancels maps a job ID to the cancel entry of its
	// per-job context, so Shutdown/StopJob can cancel a specific download and
	// runDownload can remove its own entry when it exits (issue #11).
	mu      sync.Mutex
	cancels map[int64]*cancelEntry
}

// NewDownloadService accepts only the focused repository ports a download
// needs: the download-job port (primary), the episode port (for source
// resolution and MarkEpisodeDownloaded), and the podcast port (only for
// ListJobsWithEpisodes title enrichment). The concrete SQLiteRepo satisfies all
// three via the union PodcastRepository (issue #17). logger defaults to
// NoopLogger when nil (issue #14).
func NewDownloadService(jobs domain.DownloadJobRepo, episodes domain.EpisodeRepo, podcasts domain.PodcastRepo, downloadDir string, logger domain.Logger) *DownloadService {
	if logger == nil {
		logger = domain.NoopLogger{}
	}
	return &DownloadService{
		jobs:        jobs,
		episodes:    episodes,
		podcasts:    podcasts,
		http:        &http.Client{Transport: downloadTransport},
		downloadDir: downloadDir,
		logger:      logger,
		rootCtx:     context.Background(),
		cancels:     make(map[int64]*cancelEntry),
	}
}

// SetRootContext installs the context that parents every per-job context. The
// composition root calls this with the application's root context so that
// cancelling it (on shutdown) cancels every in-flight download (issue #11).
// It must be called before any job starts.
func (s *DownloadService) SetRootContext(ctx context.Context) {
	s.mu.Lock()
	s.rootCtx = ctx
	s.mu.Unlock()
}

// registerJobCancel derives a per-job context from the root, records its cancel
// entry in s.cancels so StopJob/Shutdown can cancel it, and returns the context
// plus a cleanup function the caller defers to remove the entry once runDownload
// exits (issue #11).
//
// It is called by StartJob/ResumeJob/RetryJob BEFORE the goroutine is spawned,
// so there is no window in which StopJob(jobID) would miss a freshly-started
// job (CodeRabbit review of PR #41).
func (s *DownloadService) registerJobCancel(jobID int64) (context.Context, func()) {
	s.mu.Lock()
	parent := s.rootCtx
	// If a previous attempt's cancel is still registered (e.g. retry racing
	// with completion), cancel it before overwriting to avoid a leak.
	if prev, ok := s.cancels[jobID]; ok {
		prev.cancel()
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(parent)

	// Wrap cancel in a pointer-typed entry so it can be compared for identity
	// (Go funcs are not comparable, but pointers are). This lets cleanup tell
	// whether the map still points at THIS attempt after a concurrent retry.
	entry := &cancelEntry{cancel: cancel}

	s.mu.Lock()
	s.cancels[jobID] = entry
	s.mu.Unlock()

	cleanup := func() {
		cancel()
		s.mu.Lock()
		// Only delete if the map still points at this attempt; a retry that
		// started after we began exiting would have overwritten the entry, and
		// removing its cancel here would break its cancellation.
		if cur, ok := s.cancels[jobID]; ok && cur == entry {
			delete(s.cancels, jobID)
		}
		s.mu.Unlock()
	}
	return ctx, cleanup
}

// cancelEntry is a small wrapper that makes a cancel func uniquely
// identifiable (Go funcs are not comparable, so they cannot be tested for
// equality against a stored func; a pointer-typed wrapper can).
type cancelEntry struct {
	cancel context.CancelFunc
}

// StopJob cancels a specific in-flight download's context. The goroutine will
// observe the cancellation on its next read and fail the job with the partial
// bytes preserved (issue #11). It is a no-op if the job is not currently
// running.
func (s *DownloadService) StopJob(jobID int64) {
	s.mu.Lock()
	if entry, ok := s.cancels[jobID]; ok {
		entry.cancel()
	}
	s.mu.Unlock()
}

// Shutdown cancels every in-flight download context. It is called by the
// composition root on exit so download goroutines do not outlive the process or
// touch a closed DB (issue #11). It is safe to call concurrently with job
// start/stop.
func (s *DownloadService) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for jobID, entry := range s.cancels {
		entry.cancel()
		delete(s.cancels, jobID)
	}
	s.mu.Unlock()
	return nil
}

func (s *DownloadService) QueueEpisodeDownload(ctx context.Context, episodeID int64) error {
	episode, err := s.episodes.FindEpisodeByID(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	if episode.IsDownloaded {
		return fmt.Errorf("episode already downloaded")
	}

	existingJob, err := s.jobs.FindDownloadJobByEpisodeID(ctx, episodeID)
	if err == nil && existingJob != nil {
		if existingJob.Status == domain.DownloadStatusDownloading ||
			existingJob.Status == domain.DownloadStatusQueued {
			return fmt.Errorf("episode already in queue")
		}
		if existingJob.Status == domain.DownloadStatusFailed {
			return s.retryOrQueue(ctx, episodeID, true)
		}
		if existingJob.Status == domain.DownloadStatusPaused {
			return s.ResumeJob(ctx, existingJob.ID)
		}
	}

	return s.retryOrQueue(ctx, episodeID, false)
}

func (s *DownloadService) retryOrQueue(ctx context.Context, episodeID int64, isRetry bool) error {
	nonFailedCount, err := s.jobs.CountNonFailedJobs(ctx)
	if err != nil {
		return fmt.Errorf("could not count jobs: %w", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episodeID,
		Status:    domain.DownloadStatusQueued,
	}

	if err := s.jobs.SaveDownloadJob(ctx, job); err != nil {
		return fmt.Errorf("could not queue job: %w", err)
	}

	if nonFailedCount == 0 && !isRetry {
		return s.StartJob(ctx, job.ID)
	}

	return nil
}

func (s *DownloadService) StartJob(ctx context.Context, jobID int64) error {
	job, err := s.jobs.FindDownloadJobByID(ctx, jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusQueued && job.Status != domain.DownloadStatusFailed &&
		job.Status != domain.DownloadStatusPaused {
		return fmt.Errorf("job is not in a startable state: %s", job.Status)
	}

	if err := s.jobs.UpdateDownloadJobStatus(
		ctx,
		jobID,
		domain.DownloadStatusDownloading,
		job.BytesDownloaded,
		job.BytesTotal,
		"",
	); err != nil {
		return fmt.Errorf("could not start job: %w", err)
	}

	// Register the per-job cancel BEFORE spawning the worker so StopJob can
	// cancel this job immediately, with no window in which the entry is missing
	// (CodeRabbit review of PR #41). The worker uses this context (derived from
	// the service root, NOT the caller's ctx which dies with the tea.Cmd
	// closure) so the download outlives the request and stays cancellable via
	// StopJob/Shutdown (issue #11).
	s.startWorker(jobID)

	return nil
}

func (s *DownloadService) ResumeJob(ctx context.Context, jobID int64) error {
	job, err := s.jobs.FindDownloadJobByID(ctx, jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusPaused && job.Status != domain.DownloadStatusFailed {
		return fmt.Errorf("job is not in a resumable state: %s", job.Status)
	}

	job.Status = domain.DownloadStatusQueued
	if err := s.jobs.UpdateDownloadJobStatus(
		ctx,
		jobID,
		domain.DownloadStatusDownloading,
		job.BytesDownloaded,
		job.BytesTotal,
		"",
	); err != nil {
		return fmt.Errorf("could not resume job: %w", err)
	}

	s.startWorker(jobID)

	return nil
}

func (s *DownloadService) RetryJob(ctx context.Context, jobID int64) error {
	job, err := s.jobs.FindDownloadJobByID(ctx, jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusFailed {
		return fmt.Errorf("job is not failed: %s", job.Status)
	}

	// Preserve BytesDownloaded/BytesTotal/SupportsResume so the retry resumes
	// from the partial .part file rather than restarting from scratch (issue
	// #1). Only the status and error message are cleared. The resume branch in
	// runDownload is taken when BytesDownloaded > 0 && SupportsResume.
	job.Status = domain.DownloadStatusQueued
	job.ErrorMessage = ""

	if err := s.jobs.UpdateDownloadJobStatus(
		ctx,
		jobID,
		domain.DownloadStatusDownloading,
		job.BytesDownloaded,
		job.BytesTotal,
		"",
	); err != nil {
		return fmt.Errorf("could not retry job: %w", err)
	}

	s.startWorker(jobID)

	return nil
}

// startWorker registers the job's cancel entry and spawns runDownload under
// that context. Split out of StartJob/ResumeJob/RetryJob so all three share
// the same "register before spawn" ordering (CodeRabbit review of PR #41).
func (s *DownloadService) startWorker(jobID int64) {
	ctx, cleanup := s.registerJobCancel(jobID)
	go s.runDownload(ctx, cleanup, jobID)
}

func (s *DownloadService) ListJobs(ctx context.Context) ([]domain.DownloadJob, error) {
	return s.jobs.FindAllDownloadJobs(ctx)
}

// DownloadJobView pairs a DownloadJob with its resolved episode/podcast titles
// so the UI can render a meaningful row without a separate lookup per job
// (issue #10 — download queue rows showed blank titles because
// DownloadJobItem.EpisodeTitle was never populated).
type DownloadJobView struct {
	domain.DownloadJob
	EpisodeTitle string
	PodcastTitle string
}

// ListJobsWithEpisodes returns all download jobs enriched with the episode and
// podcast titles resolved via the repository. Jobs whose episode is missing
// (e.g. cascade-deleted) get an empty title rather than failing the whole call.
func (s *DownloadService) ListJobsWithEpisodes(ctx context.Context) ([]DownloadJobView, error) {
	jobs, err := s.jobs.FindAllDownloadJobs(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]DownloadJobView, 0, len(jobs))
	for _, job := range jobs {
		view := DownloadJobView{DownloadJob: job}
		if ep, err := s.episodes.FindEpisodeByID(ctx, job.EpisodeID); err == nil && ep != nil {
			view.EpisodeTitle = ep.Title
			if p, err := s.podcasts.FindByID(ctx, ep.PodcastID); err == nil && p != nil {
				view.PodcastTitle = p.Title
			}
		}
		views = append(views, view)
	}
	return views, nil
}

// runDownload executes a single download under the per-job context its caller
// (startWorker) already registered. The cleanup func unregisters the cancel
// entry when runDownload exits so a later retry can re-register a fresh one.
// ctx is observed at every blocking point (DB lookups, the HTTP request, each
// body Read) and the job is failed cleanly on cancellation (issue #11). Note
// failJob writes the failure with an uncancellable context, so a cancelled
// download is still persisted as Failed with its partial bytes preserved.
func (s *DownloadService) runDownload(ctx context.Context, cleanup func(), jobID int64) {
	defer cleanup()

	if err := ctx.Err(); err != nil {
		// Root already cancelled (e.g. shutdown racing with start): leave the
		// job in Downloading so the next launch can resume it, rather than
		// attempting a DB write that would itself fail under the cancelled
		// context (issue #11).
		s.logger.Debug("download skipped: context cancelled before start", "job_id", jobID, "err", err)
		return
	}

	job, err := s.jobs.FindDownloadJobByID(ctx, jobID)
	if err != nil {
		return
	}

	episode, err := s.episodes.FindEpisodeByID(ctx, job.EpisodeID)
	if err != nil || episode == nil {
		s.failJob(ctx, job, fmt.Sprintf("could not find episode: %v", err))
		return
	}

	url := episode.AudioURL
	safeName := safeFilename(episode.Title)
	contentType := ""
	ext := extractExtension(url, contentType)
	filename := safeName + ext
	extFromURL := ext != ".audio"

	partPath := filepath.Join(s.downloadDir, filename+".part")
	finalPath := filepath.Join(s.downloadDir, filename)

	if job.BytesDownloaded > 0 {
		job.TempPath = partPath
	}

	file, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(partPath)
		}
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not create file: %v", err))
			return
		}
	}
	defer file.Close()

	var req *http.Request
	requestedRange := false

	if job.BytesDownloaded > 0 && job.SupportsResume {
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not seek resume position: %v", err))
			return
		}
		info, err := file.Stat()
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not stat partial file: %v", err))
			return
		}
		resumeOffset := info.Size()

		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not build resume request: %v", err))
			return
		}
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
		requestedRange = true
	} else {
		if err := file.Truncate(0); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset partial file: %v", err))
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset write position: %v", err))
			return
		}
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not build request: %v", err))
			return
		}
	}

	resp, err := s.http.Do(req)
	if err != nil {
		s.failJob(ctx, job, fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		// Truncate resizes the file but does NOT move the write offset, which
		// a prior Seek(0, SeekEnd) in the resume branch left at resumeOffset.
		// Reset both to avoid writing a sparse/corrupt file.
		if err := file.Truncate(0); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset partial file: %v", err))
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset write position: %v", err))
			return
		}
		job.BytesDownloaded = 0
		if err := s.jobs.UpdateDownloadJobStatus(
			ctx,
			jobID,
			domain.DownloadStatusDownloading,
			0,
			job.BytesTotal,
			"",
		); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not update job status: %v", err))
			return
		}

		req2, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not rebuild request: %v", err))
			return
		}
		resp2, err := s.http.Do(req2)
		if err != nil {
			s.failJob(ctx, job, fmt.Sprintf("request failed: %v", err))
			return
		}
		defer resp2.Body.Close()
		resp = resp2
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		s.failJob(ctx, job, fmt.Sprintf("server returned: %s", resp.Status))
		return
	}

	// We sent a Range request but the server ignored it and is streaming the
	// full body (200 OK). The partial file is at resumeOffset; if we kept
	// writing there we'd produce a sparse/corrupt file. Reset both the file
	// and the counters so the body is written from offset 0.
	if requestedRange && resp.StatusCode == http.StatusOK {
		if err := file.Truncate(0); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset partial file: %v", err))
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not reset write position: %v", err))
			return
		}
		job.BytesDownloaded = 0
	}

	if job.BytesTotal == 0 {
		if ct := resp.Header.Get("Content-Length"); ct != "" {
			fmt.Sscanf(ct, "%d", &job.BytesTotal)
		}
	}

	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}
	if ext := extractExtension(url, contentType); extFromURL || ext != ".audio" {
		filename = safeName + ext
		partPath = filepath.Join(s.downloadDir, filename+".part")
		finalPath = filepath.Join(s.downloadDir, filename)
	}

	// A server signals range-request support either by returning 206 in
	// response to our Range header, or by advertising "Accept-Ranges: bytes"
	// on any response. Either is sufficient to enable resume on a retry.
	job.SupportsResume = resp.StatusCode == http.StatusPartialContent ||
		strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes")

	if etag := resp.Header.Get("ETag"); etag != "" {
		job.ETag = etag
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		job.LastModified = lm
	}

	job.TempPath = partPath
	job.FinalPath = finalPath

	// Persist the resume-relevant metadata (paths, SupportsResume, ETag,
	// Last-Modified) alongside the byte counters so that a retry after a
	// transient failure can resume from the partial .part file (issue #1).
	// The earlier status update only wrote counters; SupportsResume was never
	// persisted, which made the resume branch above unreachable on retry.
	if err := s.jobs.UpdateDownloadJobProgress(ctx, job); err != nil {
		s.failJob(ctx, job, fmt.Sprintf("could not persist job progress: %v", err))
		return
	}

	// lastReported is the byte count at which we last persisted progress; we
	// publish an update only once we cross progressInterval beyond it.
	buf := make([]byte, 32*1024)
	var bytesWrittenThisRun int64
	lastReported := job.BytesDownloaded

	// Wrap the body so a stalled stream (server stops sending mid-download)
	// fails after stallTimeout instead of blocking the goroutine forever. The
	// reader also fails when ctx is cancelled: an AfterFunc closes resp.Body on
	// cancellation, which unblocks the in-progress Read promptly and
	// deterministically (issue #11).
	body := newTimeoutReader(resp.Body, stallTimeout, ctx)

	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			wn, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				s.failJob(ctx, job, fmt.Sprintf("write failed: %v", writeErr))
				return
			}
			bytesWrittenThisRun += int64(wn)
			job.BytesDownloaded += int64(wn)

			// Throttle progress updates to a steady ~1 MiB cadence using a
			// delta against the last reported byte count, so updates fire
			// regularly regardless of chunk size or alignment (issue #2).
			if job.BytesDownloaded-lastReported >= progressInterval {
				if err := s.jobs.UpdateDownloadJobStatus(
					ctx,
					jobID,
					domain.DownloadStatusDownloading,
					job.BytesDownloaded,
					job.BytesTotal,
					"",
				); err != nil {
					s.failJob(ctx, job, fmt.Sprintf("could not update job status: %v", err))
					return
				}
				lastReported = job.BytesDownloaded
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			s.failJob(ctx, job, fmt.Sprintf("read failed: %v", readErr))
			return
		}
	}

	// Always publish a final update so the UI reaches 100% near completion,
	// even when the last chunk was smaller than the throttle interval (issue #2).
	if bytesWrittenThisRun > 0 {
		if err := s.jobs.UpdateDownloadJobStatus(
			ctx,
			jobID,
			domain.DownloadStatusDownloading,
			job.BytesDownloaded,
			job.BytesTotal,
			"",
		); err != nil {
			s.failJob(ctx, job, fmt.Sprintf("could not update job status: %v", err))
			return
		}
	}

	file.Close()

	if err := os.Rename(partPath, finalPath); err != nil {
		s.failJob(ctx, job, fmt.Sprintf("rename failed: %v", err))
		return
	}

	if err := s.episodes.MarkEpisodeDownloaded(ctx, job.EpisodeID, finalPath); err != nil {
		s.failJob(ctx, job, fmt.Sprintf("could not mark episode as downloaded: %v", err))
		return
	}

	if err := s.jobs.DeleteDownloadJob(ctx, jobID); err != nil {
		s.logger.Warn("could not delete completed job", "job_id", jobID, "err", err)
	}
}

// failJob marks job as failed while preserving the byte counters so a later
// retry can resume from the partial .part file on disk (issue #1). Only the
// status and error message change; BytesDownloaded/BytesTotal reflect what was
// actually written before the failure.
//
// The status write uses a fresh, uncancellable context on purpose: failJob is
// the cleanup path and is itself often triggered BY cancellation (StopJob /
// Shutdown cancel the per-job ctx, which makes the read fail and land here).
// Writing with the now-cancelled ctx would make ExecContext return
// context.Canceled and the job would stay stuck in Downloading — exactly the
// resume-data loss issue #1 fixed, reintroduced on the cancellation path
// (CodeRabbit review of PR #41).
func (s *DownloadService) failJob(_ context.Context, job *domain.DownloadJob, errorMsg string) {
	if job == nil {
		s.logger.Warn("could not fail job: nil job")
		return
	}
	if err := s.jobs.UpdateDownloadJobStatus(
		context.Background(),
		job.ID,
		domain.DownloadStatusFailed,
		job.BytesDownloaded,
		job.BytesTotal,
		errorMsg,
	); err != nil {
		s.logger.Warn("could not update job status on failure", "job_id", job.ID, "err", err)
	}
}

// timeoutReader wraps a response body and fails a Read if no data arrives
// within timeout since the last successful Read (or since construction), or as
// soon as ctx is cancelled. This bounds how long a stalled download body can
// block runDownload, complementing the connection/header timeouts on
// downloadTransport (issue #6), and makes an in-progress download interruptible
// by StopJob/Shutdown (issue #11). Large legitimate downloads are unaffected
// because the deadline resets on every Read that returns data.
//
// To make cancellation prompt and deterministic, newTimeoutReader registers a
// context.AfterFunc that closes the body when ctx is cancelled. Closing the
// body unblocks any in-progress Read on the underlying connection regardless of
// transport behavior — relying on ctx.Done() in the select alone is NOT enough,
// because an http.Response.Body blocked in a buffered Read does not always
// observe the context cancellation within a useful timeframe (CodeRabbit review
// of PR #41).
type timeoutReader struct {
	r       io.Reader
	closer  io.Closer
	timeout time.Duration
	ctx     context.Context
	stop    func() bool
}

func newTimeoutReader(body io.ReadCloser, timeout time.Duration, ctx context.Context) *timeoutReader {
	tr := &timeoutReader{r: body, closer: body, timeout: timeout, ctx: ctx}
	// context.AfterFunc runs body.Close() when ctx is cancelled, which unblocks
	// the goroutine in Read() below that is stuck in body.Read. AfterFunc is
	// stopped on the final Read returning an error / on Stop(), so a normal
	// EOF does not trigger a spurious Close (the deferred resp.Body.Close in
	// runDownload handles the normal path).
	tr.stop = context.AfterFunc(ctx, func() {
		_ = body.Close()
	})
	return tr
}

// Stop unregisters the AfterFunc hook. runDownload does NOT need to call this
// (the GC + the deferred resp.Body.Close handle cleanup), but it is available
// so a future caller can detach early.
func (t *timeoutReader) Stop() {
	if t.stop != nil {
		t.stop()
	}
}

func (t *timeoutReader) Read(p []byte) (int, error) {
	if err := t.ctx.Err(); err != nil {
		return 0, fmt.Errorf("download cancelled: %w", err)
	}
	type readResult struct {
		n   int
		err error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		n, err := t.r.Read(p)
		resultCh <- readResult{n, err}
	}()

	timer := time.NewTimer(t.timeout)
	defer timer.Stop()

	select {
	case res := <-resultCh:
		return res.n, res.err
	case <-timer.C:
		// Stall: best-effort close the body to nudge the inner Read toward
		// returning. The inner goroutine's send is on a buffered (size 1)
		// channel, so it never blocks even if we return here without receiving.
		_ = t.closer.Close()
		return 0, fmt.Errorf("download stalled: no data for %s", t.timeout)
	case <-t.ctx.Done():
		// Cancellation: the AfterFunc registered in newTimeoutReader closes the
		// body, which unblocks the inner Read. Do NOT drain resultCh here —
		// draining would block until the inner Read actually returns, which
		// defeats the point of cancelling. The buffered resultCh lets the inner
		// goroutine exit without a receiver (CodeRabbit review of PR #41).
		return 0, fmt.Errorf("download cancelled: %w", t.ctx.Err())
	}
}

func safeFilename(name string) string {
	safe := regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(name, "_")
	safe = strings.Trim(safe, "._")
	if len(safe) > 50 {
		safe = safe[:50]
	}
	if safe == "" {
		safe = "download"
	}
	return safe
}

func extractExtension(url string, contentType string) string {
	if ext := findExtensionFromURL(url); ext != "" {
		return ext
	}
	if ext := findExtensionFromContentType(contentType); ext != "" {
		return ext
	}
	return ".audio"
}

func findExtensionFromURL(url string) string {
	exts := []string{".mp3", ".m4a", ".aac", ".ogg", ".wav", ".flac", ".opus", ".webm"}
	lower := strings.ToLower(url)
	for _, ext := range exts {
		if strings.Contains(lower, ext) {
			return ext
		}
	}
	return ""
}

func findExtensionFromContentType(contentType string) string {
	types := map[string]string{
		"audio/mpeg":  ".mp3",
		"audio/mp4":   ".m4a",
		"audio/x-m4a": ".m4a",
		"audio/aac":   ".aac",
		"audio/x-aac": ".aac",
		"audio/ogg":   ".ogg",
		"audio/wav":   ".wav",
		"audio/x-wav": ".wav",
		"audio/flac":  ".flac",
		"audio/webm":  ".webm",
		"audio/opus":  ".opus",
	}
	ct := strings.TrimSpace(strings.Split(contentType, ";")[0])
	if ext, ok := types[ct]; ok {
		return ext
	}
	return ""
}
