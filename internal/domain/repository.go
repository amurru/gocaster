package domain

// Repository defines the contract for data persistence.
type PodcastRepository interface {
	Save(podcast *Podcast) error
	FindAll() ([]Podcast, error)
	FindByID(id int64) (*Podcast, error)
	Delete(id int64) error

	// Episodes
	SaveEpisode(episode *Episode) error
	FindEpisodesByPodcastID(id int64) ([]Episode, error)
	FindEpisodeByID(id int64) (*Episode, error)
	DeleteEpisode(id int64) error

	// Download Jobs
	SaveDownloadJob(job *DownloadJob) error
	FindDownloadJobByEpisodeID(episodeID int64) (*DownloadJob, error)
	FindAllDownloadJobs() ([]DownloadJob, error)
	UpdateDownloadJobStatus(id int64, status DownloadStatus, bytesDownloaded int64, bytesTotal int64, errorMsg string) error
	CountNonFailedJobs() (int, error)
	DeleteDownloadJob(id int64) error

	// Mark episode as downloaded (called on job completion)
	MarkEpisodeDownloaded(episodeID int64, localPath string) error
}
