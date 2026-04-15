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
