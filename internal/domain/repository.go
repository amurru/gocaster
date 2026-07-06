package domain

// PodcastRepo is the persistence port for podcast aggregates (issue #17).
type PodcastRepo interface {
	Save(podcast *Podcast) error
	FindAll() ([]Podcast, error)
	FindByID(id int64) (*Podcast, error)
	Delete(id int64) error
}

// EpisodeRepo is the persistence port for episodes and the cross-concern of
// marking an episode downloaded on job completion (issue #17).
type EpisodeRepo interface {
	SaveEpisode(episode *Episode) error
	FindEpisodesByPodcastID(id int64) ([]Episode, error)
	FindEpisodeByID(id int64) (*Episode, error)
	UpdateEpisodePlaybackState(id int64, isPlayed bool) error
	DeleteEpisode(id int64) error

	// Mark episode as downloaded (called on job completion).
	MarkEpisodeDownloaded(episodeID int64, localPath string) error
}

// DownloadJobRepo is the persistence port for download jobs (issue #17).
type DownloadJobRepo interface {
	SaveDownloadJob(job *DownloadJob) error
	// FindDownloadJobByID loads a single job by primary key, replacing the
	// previous O(N) FindAllDownloadJobs scan that DownloadService ran on every
	// status update.
	FindDownloadJobByID(id int64) (*DownloadJob, error)
	FindDownloadJobByEpisodeID(episodeID int64) (*DownloadJob, error)
	FindAllDownloadJobs() ([]DownloadJob, error)
	UpdateDownloadJobStatus(id int64, status DownloadStatus, bytesDownloaded int64, bytesTotal int64, errorMsg string) error
	// UpdateDownloadJobProgress persists the resume-relevant fields of a job
	// (bytes counters, temp/final paths, ETag, Last-Modified, SupportsResume)
	// without changing its status. runDownload calls this once it has learned
	// from the response headers whether the server supports range requests, so
	// a later retry can resume from the partial .part file.
	UpdateDownloadJobProgress(job *DownloadJob) error
	CountNonFailedJobs() (int, error)
	DeleteDownloadJob(id int64) error
}

// PodcastRepository is the union of the three focused repository ports. It
// exists so the concrete SQLiteRepo and the composition root (main.go) can keep
// depending on a single type while individual services depend only on the
// narrow port they use (Interface Segregation Principle, issue #17).
type PodcastRepository interface {
	PodcastRepo
	EpisodeRepo
	DownloadJobRepo
}
