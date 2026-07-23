package domain

import "context"

// PodcastRepo is the persistence port for podcast aggregates (issue #17). Every
// method takes a context.Context as its first parameter so callers can cancel
// in-flight operations, set deadlines, and propagate request-scoped tracing
// (issue #11).
type PodcastRepo interface {
	Save(ctx context.Context, podcast *Podcast) error
	FindAll(ctx context.Context) ([]Podcast, error)
	FindByID(ctx context.Context, id int64) (*Podcast, error)
	Delete(ctx context.Context, id int64) error
	UpdateFeedHeaders(ctx context.Context, id int64, etag string, lastModified string) error
}

// EpisodeRepo is the persistence port for episodes and the cross-concern of
// marking an episode downloaded on job completion (issue #17). Every method
// takes a context.Context (issue #11).
type EpisodeRepo interface {
	SaveEpisode(ctx context.Context, episode *Episode) error
	FindEpisodesByPodcastID(ctx context.Context, id int64) ([]Episode, error)
	FindEpisodeByID(ctx context.Context, id int64) (*Episode, error)
	UpdateEpisodePlaybackState(ctx context.Context, id int64, isPlayed bool) error
	DeleteEpisode(ctx context.Context, id int64) error

	// Mark episode as downloaded (called on job completion).
	MarkEpisodeDownloaded(ctx context.Context, episodeID int64, localPath string) error
}

// DownloadJobRepo is the persistence port for download jobs (issue #17). Every
// method takes a context.Context (issue #11).
type DownloadJobRepo interface {
	SaveDownloadJob(ctx context.Context, job *DownloadJob) error
	// FindDownloadJobByID loads a single job by primary key, replacing the
	// previous O(N) FindAllDownloadJobs scan that DownloadService ran on every
	// status update.
	FindDownloadJobByID(ctx context.Context, id int64) (*DownloadJob, error)
	FindDownloadJobByEpisodeID(ctx context.Context, episodeID int64) (*DownloadJob, error)
	FindAllDownloadJobs(ctx context.Context) ([]DownloadJob, error)
	UpdateDownloadJobStatus(ctx context.Context, id int64, status DownloadStatus, bytesDownloaded int64, bytesTotal int64, errorMsg string) error
	// UpdateDownloadJobProgress persists the resume-relevant fields of a job
	// (bytes counters, temp/final paths, ETag, Last-Modified, SupportsResume)
	// without changing its status. runDownload calls this once it has learned
	// from the response headers whether the server supports range requests, so
	// a later retry can resume from the partial .part file.
	UpdateDownloadJobProgress(ctx context.Context, job *DownloadJob) error
	CountNonFailedJobs(ctx context.Context) (int, error)
	DeleteDownloadJob(ctx context.Context, id int64) error
}

// PodcastBatchRepo groups the cross-aggregate podcast+episode writes that must
// commit atomically (issue #13). PodcastService uses these in AddPodcast and
// RefreshPodcast instead of issuing Save + SaveEpisode-loop as independent
// writes, which left a partial episode set on mid-loop failure.
type PodcastBatchRepo interface {
	// SavePodcastWithEpisodes inserts the podcast and every episode in one
	// transaction: all-or-nothing. On success podcast.ID reflects the newly
	// assigned row.
	SavePodcastWithEpisodes(ctx context.Context, podcast *Podcast, episodes []Episode) error
	// AppendEpisodesAndTouchPodcast inserts the (already-filtered) new episodes
	// and updates the podcast's LastUpdated in one transaction. A failure mid-batch
	// rolls back both the episode inserts and the LastUpdated bump.
	AppendEpisodesAndTouchPodcast(ctx context.Context, podcast *Podcast, newEpisodes []Episode) error
}

// DownloadCompletionRepo atomically marks an episode downloaded and removes its
// completed job row (issue #13). Splitting the pair leaves the episode and
// downloads tables out of sync on a partial failure (downloaded episode with a
// dangling job, or a deleted job whose episode was never marked).
type DownloadCompletionRepo interface {
	CompleteDownload(ctx context.Context, episodeID int64, localPath string, jobID int64) error
}

// PodcastRepository is the union of the focused repository ports. It exists so
// the concrete SQLiteRepo and the composition root (main.go) can keep depending
// on a single type while individual services depend only on the narrow port they
// use (Interface Segregation Principle, issue #17).
type PodcastRepository interface {
	PodcastRepo
	EpisodeRepo
	DownloadJobRepo
	PodcastBatchRepo
	DownloadCompletionRepo
}
