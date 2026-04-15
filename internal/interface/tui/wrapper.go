package tui

import "github.com/amurru/gocaster/internal/domain"

// PodcastItem wraps a domain.Podcast to satisfy the list.Item interface.
// This keeps UI-specific code (FilterValue) out of your Domain layer.
type PodcastItem struct {
	domain.Podcast
}

// FilterValue is required by list.Item for filtering/searching.
func (p PodcastItem) FilterValue() string {
	return p.Podcast.Title
}

// Title and Description are optional but used by the default delegate
// to render the list rows nicely.
func (p PodcastItem) Title() string {
	return p.Podcast.Title
}

func (p PodcastItem) Description() string {
	if p.Podcast.Description == "" {
		return p.Podcast.FeedURL
	}
	return p.Podcast.Description
}

type EpisodeItem struct {
	domain.Episode
}

func (e EpisodeItem) FilterValue() string {
	return e.Episode.Title
}

func (e EpisodeItem) Title() string {
	return e.Episode.Title
}

func (e EpisodeItem) Description() string {
	if e.IsPlayed {
		return "Played"
	}
	return "New"
}
