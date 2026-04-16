package domain

import "time"

type Podcast struct {
	ID          int64
	Title       string
	FeedURL     string
	Description string
	ImageURL    string
	LastUpdated time.Time
}

type Episode struct {
	ID               int64
	PodcastID        int64
	Title            string
	Description      string
	AudioURL         string
	PublishedAt      time.Time
	PlaybackDuration int // in seconds
	IsPlayed         bool
	IsDownloaded     bool
	LocalPath        string
}

type DownloadStatus string

const (
	DownloadStatusQueued      DownloadStatus = "queued"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusPaused      DownloadStatus = "paused"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusCompleted   DownloadStatus = "completed"
)

type DownloadJob struct {
	ID              int64
	EpisodeID       int64
	Status          DownloadStatus
	BytesDownloaded int64
	BytesTotal      int64
	TempPath        string
	FinalPath       string
	ETag            string
	LastModified    string
	SupportsResume  bool
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
