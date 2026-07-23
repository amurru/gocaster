package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
)

func selectedPodcastItem(listModel list.Model) *domain.Podcast {
	item, ok := listModel.SelectedItem().(PodcastItem)
	if !ok {
		return nil
	}
	podcast := item.Podcast
	return &podcast
}

func selectedEpisodeItem(listModel list.Model) *EpisodeItem {
	item, ok := listModel.SelectedItem().(EpisodeItem)
	if !ok {
		return nil
	}
	episode := item
	return &episode
}

func selectedDownloadJobItem(listModel list.Model) *DownloadJobItem {
	item, ok := listModel.SelectedItem().(DownloadJobItem)
	if !ok {
		return nil
	}
	job := item
	return &job
}

func (m Model) displayEpisode() *domain.Episode {
	if m.currentEpisode != nil {
		return m.currentEpisode
	}
	if m.selectedEpisode != nil {
		episode := *m.selectedEpisode
		return &episode
	}
	return nil
}

func (m Model) episodeByID(episodeID int64) *domain.Episode {
	for i := range m.episodes {
		if m.episodes[i].ID == episodeID {
			episode := m.episodes[i]
			return &episode
		}
	}
	if m.selectedEpisode != nil && m.selectedEpisode.ID == episodeID {
		episode := *m.selectedEpisode
		return &episode
	}
	if m.currentEpisode != nil && m.currentEpisode.ID == episodeID {
		return m.currentEpisode
	}
	return nil
}

// episodeTitleForView resolves a display title for a download job. It prefers
// the episode title resolved by the download service; if that is empty (e.g.
// the episode was cascade-deleted) it falls back to a short, human-readable
// placeholder so the queue row is never blank (issue #10).
func episodeTitleForView(job application.DownloadJobView) string {
	if job.EpisodeTitle != "" {
		return job.EpisodeTitle
	}
	return fmt.Sprintf("Episode #%d", job.EpisodeID)
}

func (m Model) playingEpisodeFromStatus(status domain.PlaybackStatus) *domain.Episode {
	if status.Source == "" {
		return nil
	}
	if m.currentEpisode != nil {
		if m.currentEpisode.AudioURL == status.Source ||
			m.currentEpisode.LocalPath == status.Source {
			return m.currentEpisode
		}
	}
	for i := range m.episodes {
		if m.episodes[i].AudioURL == status.Source || m.episodes[i].LocalPath == status.Source {
			episode := m.episodes[i]
			return &episode
		}
	}
	if m.selectedEpisode != nil &&
		(m.selectedEpisode.AudioURL == status.Source || m.selectedEpisode.LocalPath == status.Source) {
		episode := *m.selectedEpisode
		return &episode
	}
	return nil
}

func formatPlaybackTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds + 0.5)
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func parseSeekTime(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("time is required")
	}

	parts := strings.Split(value, ":")
	if len(parts) == 1 {
		seconds, err := strconv.Atoi(parts[0])
		if err != nil || seconds < 0 {
			return 0, fmt.Errorf("invalid time value")
		}
		return float64(seconds), nil
	}

	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("use mm:ss or hh:mm:ss")
	}

	total := 0
	multiplier := 1
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		segment, err := strconv.Atoi(part)
		if err != nil || segment < 0 {
			return 0, fmt.Errorf("invalid time value")
		}
		total += segment * multiplier
		multiplier *= 60
	}

	return float64(total), nil
}

func suffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func valueOrPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(not set)"
	}
	return value
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return max(m.width-m.theme.App.GetHorizontalFrameSize(), 20)
}

func (m Model) shouldStackPanes() bool {
	return m.contentWidth() < 80
}

func truncateLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n")
}

func findThemeIndex(themeName string, themeList []string) int {
	for i, t := range themeList {
		if t == themeName {
			return i
		}
	}
	return 0 // default to first theme
}

// speedSteps defines the allowed playback speed values in ascending order.
var speedSteps = []float64{0.75, 1.0, 1.25, 1.5, 1.75, 2.0}

// nextSpeedUp returns the next higher speed step from the current speed.
// If current is already at the maximum, it stays at the maximum.
func nextSpeedUp(current float64) float64 {
	for _, s := range speedSteps {
		if s > current+0.001 {
			return s
		}
	}
	return speedSteps[len(speedSteps)-1]
}

// nextSpeedDown returns the next lower speed step from the current speed.
// If current is already at the minimum, it stays at the minimum.
func nextSpeedDown(current float64) float64 {
	for i := len(speedSteps) - 1; i >= 0; i-- {
		if speedSteps[i] < current-0.001 {
			return speedSteps[i]
		}
	}
	return speedSteps[0]
}
