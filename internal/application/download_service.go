package application

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/amurru/gocaster/internal/domain"
)

type DownloadService struct {
	repo        domain.PodcastRepository
	http        *http.Client
	downloadDir string
}

func NewDownloadService(repo domain.PodcastRepository, downloadDir string) *DownloadService {
	return &DownloadService{
		repo:        repo,
		http:        &http.Client{},
		downloadDir: downloadDir,
	}
}

func (s *DownloadService) QueueEpisodeDownload(episodeID int64) error {
	episode, err := s.repo.FindEpisodeByID(episodeID)
	if err != nil {
		return fmt.Errorf("could not find episode: %w", err)
	}

	if episode.IsDownloaded {
		return fmt.Errorf("episode already downloaded")
	}

	existingJob, err := s.repo.FindDownloadJobByEpisodeID(episodeID)
	if err == nil && existingJob != nil {
		if existingJob.Status == domain.DownloadStatusDownloading || existingJob.Status == domain.DownloadStatusQueued {
			return fmt.Errorf("episode already in queue")
		}
		if existingJob.Status == domain.DownloadStatusFailed {
			return s.retryOrQueue(episodeID, true)
		}
		if existingJob.Status == domain.DownloadStatusPaused {
			return s.ResumeJob(existingJob.ID)
		}
	}

	return s.retryOrQueue(episodeID, false)
}

func (s *DownloadService) retryOrQueue(episodeID int64, isRetry bool) error {
	nonFailedCount, err := s.repo.CountNonFailedJobs()
	if err != nil {
		return fmt.Errorf("could not count jobs: %w", err)
	}

	job := &domain.DownloadJob{
		EpisodeID: episodeID,
		Status:    domain.DownloadStatusQueued,
	}

	if err := s.repo.SaveDownloadJob(job); err != nil {
		return fmt.Errorf("could not queue job: %w", err)
	}

	if nonFailedCount == 0 && !isRetry {
		return s.StartJob(job.ID)
	}

	return nil
}

func (s *DownloadService) StartJob(jobID int64) error {
	job, err := s.findJobByID(jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusQueued && job.Status != domain.DownloadStatusFailed && job.Status != domain.DownloadStatusPaused {
		return fmt.Errorf("job is not in a startable state: %s", job.Status)
	}

	if err := s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, job.BytesDownloaded, job.BytesTotal, ""); err != nil {
		return fmt.Errorf("could not start job: %w", err)
	}

	go s.runDownload(jobID)

	return nil
}

func (s *DownloadService) ResumeJob(jobID int64) error {
	job, err := s.findJobByID(jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusPaused && job.Status != domain.DownloadStatusFailed {
		return fmt.Errorf("job is not in a resumable state: %s", job.Status)
	}

	job.Status = domain.DownloadStatusQueued
	if err := s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, job.BytesDownloaded, job.BytesTotal, ""); err != nil {
		return fmt.Errorf("could not resume job: %w", err)
	}

	go s.runDownload(jobID)

	return nil
}

func (s *DownloadService) RetryJob(jobID int64) error {
	job, err := s.findJobByID(jobID)
	if err != nil {
		return err
	}

	if job.Status != domain.DownloadStatusFailed {
		return fmt.Errorf("job is not failed: %s", job.Status)
	}

	job.Status = domain.DownloadStatusQueued
	job.BytesDownloaded = 0
	job.BytesTotal = 0
	job.ErrorMessage = ""

	if err := s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, 0, 0, ""); err != nil {
		return fmt.Errorf("could not retry job: %w", err)
	}

	go s.runDownload(jobID)

	return nil
}

func (s *DownloadService) ListJobs() ([]domain.DownloadJob, error) {
	return s.repo.FindAllDownloadJobs()
}

func (s *DownloadService) findJobByID(jobID int64) (*domain.DownloadJob, error) {
	jobs, err := s.repo.FindAllDownloadJobs()
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if job.ID == jobID {
			return &job, nil
		}
	}

	return nil, fmt.Errorf("job not found: %d", jobID)
}

func (s *DownloadService) runDownload(jobID int64) {
	job, err := s.findJobByID(jobID)
	if err != nil {
		return
	}

	episode, err := s.repo.FindEpisodeByID(job.EpisodeID)
	if err != nil {
		s.failJob(jobID, fmt.Sprintf("could not find episode: %v", err))
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
			s.failJob(jobID, fmt.Sprintf("could not create file: %v", err))
			return
		}
	}
	defer file.Close()

	var req *http.Request
	var resumeOffset int64

	if job.BytesDownloaded > 0 && job.SupportsResume {
		file.Seek(0, io.SeekEnd)
		currentSize, _ := file.Stat()
		resumeOffset = currentSize.Size()

		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
	} else {
		file.Truncate(0)
		req, _ = http.NewRequest("GET", url, nil)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		s.failJob(jobID, fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		file.Truncate(0)
		job.BytesDownloaded = 0
		s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, 0, job.BytesTotal, "")

		req2, _ := http.NewRequest("GET", url, nil)
		resp2, err := s.http.Do(req2)
		if err != nil {
			s.failJob(jobID, fmt.Sprintf("request failed: %v", err))
			return
		}
		defer resp2.Body.Close()
		resp = resp2
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		s.failJob(jobID, fmt.Sprintf("server returned: %s", resp.Status))
		return
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

	if resp.StatusCode == http.StatusPartialContent {
		job.SupportsResume = true
	} else {
		job.SupportsResume = false
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		job.ETag = etag
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		job.LastModified = lm
	}

	job.TempPath = partPath
	job.FinalPath = finalPath

	s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, job.BytesDownloaded, job.BytesTotal, "")

	buf := make([]byte, 32*1024)
	var wrote int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			wn, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				s.failJob(jobID, fmt.Sprintf("write failed: %v", writeErr))
				return
			}
			wrote += int64(wn)
			job.BytesDownloaded += int64(wn)

			if job.BytesTotal > 0 && job.BytesDownloaded%1024000 < int64(n) {
				s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusDownloading, job.BytesDownloaded, job.BytesTotal, "")
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			s.failJob(jobID, fmt.Sprintf("read failed: %v", readErr))
			return
		}
	}

	file.Close()

	if err := os.Rename(partPath, finalPath); err != nil {
		s.failJob(jobID, fmt.Sprintf("rename failed: %v", err))
		return
	}

	if err := s.repo.MarkEpisodeDownloaded(job.EpisodeID, finalPath); err != nil {
		s.failJob(jobID, fmt.Sprintf("could not mark episode as downloaded: %v", err))
		return
	}

	if err := s.repo.DeleteDownloadJob(jobID); err != nil {
		fmt.Printf("Warning: could not delete job: %v\n", err)
	}
}

func (s *DownloadService) failJob(jobID int64, errorMsg string) {
	if err := s.repo.UpdateDownloadJobStatus(jobID, domain.DownloadStatusFailed, 0, 0, errorMsg); err != nil {
		fmt.Printf("Warning: could not update job status: %v\n", err)
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
