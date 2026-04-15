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

// Description returns styled description with NEW/PLAYED indicators.
func (e EpisodeItem) Description() string {
	dateLabel := "Unknown date"
	if !e.PublishedAt.IsZero() {
		dateLabel = e.PublishedAt.Format("Jan 02, 2006")
	}

	var status string
	if e.IsPlayed {
		status = lipgloss.NewStyle().
			Foreground(e.Theme.Muted).
			Render("PLAYED")
	} else {
		// Flashing effect: alternate based on tick
		if e.FlashTick%2 == 0 {
			status = lipgloss.NewStyle().
				Foreground(e.Theme.Success).
				Bold(true).
				Render("NEW")
		} else {
			status = lipgloss.NewStyle().
				Foreground(e.Theme.SurfaceAlt).
				Render("NEW")
		}
	}

	duration := e.Duration()
	return fmt.Sprintf("%s  •  %s  •  %s", status, dateLabel, duration)
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
