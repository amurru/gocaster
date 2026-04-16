package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

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
	Theme     styles.Theme
	FlashTick int64
}

func (e EpisodeItem) FilterValue() string {
	return e.Episode.Title
}

func (e EpisodeItem) Title() string {
	return e.Episode.Title
}

// Description returns styled description with NEW/PLAYED/DOWNLOADED indicators.
func (e EpisodeItem) Description() string {
	dateLabel := "Unknown date"
	if !e.PublishedAt.IsZero() {
		dateLabel = e.PublishedAt.Format("Jan 02, 2006")
	}

	var played string
	if e.IsPlayed {
		played = lipgloss.NewStyle().
			Foreground(e.Theme.Muted).
			Render("PLAYED")
	} else {
		if e.FlashTick%2 == 0 {
			played = lipgloss.NewStyle().
				Foreground(e.Theme.Success).
				Bold(true).
				Render("NEW")
		} else {
			played = lipgloss.NewStyle().
				Foreground(e.Theme.SurfaceAlt).
				Render("NEW")
		}
	}

	var download string
	if e.IsDownloaded {
		download = lipgloss.NewStyle().
			Foreground(e.Theme.Success).
			Bold(true).
			Render("DOWNLOADED")
	}

	result := played + "  \u2022  " + dateLabel

	duration := e.Duration()
	if duration != "" {
		result += "  \u2022  " + duration
	}

	if download != "" {
		result += "  \u2022  " + download
	}

	return result
}

// StatusBadge returns a short status indicator for styling.
func (e EpisodeItem) StatusBadge() string {
	if e.IsPlayed {
		return "PLAYED"
	}
	return "NEW"
}

// DateLabel returns the formatted publication date.
func (e EpisodeItem) DateLabel() string {
	if e.PublishedAt.IsZero() {
		return "Unknown date"
	}
	return e.PublishedAt.Format("Jan 02, 2006")
}

// Duration returns the formatted duration string.
func (e EpisodeItem) Duration() string {
	if e.PlaybackDuration == 0 {
		return ""
	}
	hours := e.PlaybackDuration / 3600
	minutes := (e.PlaybackDuration % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// IsNew returns true if the episode has not been played.
func (e EpisodeItem) IsNew() bool {
	return !e.IsPlayed
}

// WithTheme returns a copy of the EpisodeItem with the given theme.
func (e EpisodeItem) WithTheme(theme styles.Theme) EpisodeItem {
	e.Theme = theme
	return e
}

// WithFlashTick returns a copy of the EpisodeItem with the given flash tick.
func (e EpisodeItem) WithFlashTick(tick int64) EpisodeItem {
	e.FlashTick = tick
	return e
}

type DownloadJobItem struct {
	domain.DownloadJob
	EpisodeTitle string
	Theme        styles.Theme
	FlashTick    int64
}

func (d DownloadJobItem) FilterValue() string {
	return d.EpisodeTitle
}

func (d DownloadJobItem) Title() string {
	return d.EpisodeTitle
}

func (d DownloadJobItem) Description() string {
	progress := d.Progress()
	errorMsg := ""
	if d.ErrorMessage != "" && len(d.ErrorMessage) > 40 {
		errorMsg = d.ErrorMessage[:40] + "…"
	} else {
		errorMsg = d.ErrorMessage
	}
	return fmt.Sprintf("%s  •  %s  •  %s", progress, d.StatusBadge(), errorMsg)
}

func (d DownloadJobItem) Progress() string {
	if d.BytesTotal == 0 {
		return "0 B"
	}
	percent := int(float64(d.BytesDownloaded) / float64(d.BytesTotal) * 100)
	return fmt.Sprintf("%d%%", percent)
}

func (d DownloadJobItem) StatusBadge() string {
	switch d.Status {
	case domain.DownloadStatusQueued:
		return "QUEUED"
	case domain.DownloadStatusDownloading:
		return "DOWNLOADING"
	case domain.DownloadStatusPaused:
		return "PAUSED"
	case domain.DownloadStatusFailed:
		return "FAILED"
	case domain.DownloadStatusCompleted:
		return "COMPLETED"
	default:
		return "UNKNOWN"
	}
}

func (d DownloadJobItem) WithTheme(theme styles.Theme) DownloadJobItem {
	d.Theme = theme
	return d
}

func (d DownloadJobItem) WithFlashTick(tick int64) DownloadJobItem {
	d.FlashTick = tick
	return d
}
