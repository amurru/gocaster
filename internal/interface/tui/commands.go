package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m Model) loadPodcasts() tea.Cmd {
	return func() tea.Msg {
		podcasts, err := m.podcastService.ListPodcasts()
		return podcastsLoadedMsg{podcasts: podcasts, err: err}
	}
}

func (m Model) loadEpisodes(podcastID int64) tea.Cmd {
	return func() tea.Msg {
		episodes, err := m.podcastService.ListEpisodes(podcastID)
		return episodesLoadedMsg{podcastID: podcastID, episodes: episodes, err: err}
	}
}

func (m Model) addPodcast(url string) tea.Cmd {
	return func() tea.Msg {
		podcast, err := m.podcastService.AddPodcast(url)
		return podcastAddedMsg{podcast: podcast, err: err}
	}
}

func (m Model) refreshPodcast(podcastID int64) tea.Cmd {
	return func() tea.Msg {
		newCount, err := m.podcastService.RefreshPodcast(podcastID)
		return podcastRefreshedMsg{podcastID: podcastID, newCount: newCount, err: err}
	}
}

func (m Model) syncAllPodcasts(reason string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.podcastService.RefreshAllPodcasts()
		return allPodcastsSyncedMsg{result: result, err: err, reason: reason}
	}
}

func (m Model) persistSettings(next Settings, previous Settings) tea.Cmd {
	return func() tea.Msg {
		if m.saveSettings == nil {
			return settingsPersistedMsg{settings: next, previous: previous}
		}
		err := m.saveSettings(next)
		return settingsPersistedMsg{settings: next, previous: previous, err: err}
	}
}

func (m Model) loadDownloadJobs() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.downloadService.ListJobsWithEpisodes()
		return downloadJobsLoadedMsg{jobs: jobs, err: err}
	}
}

func (m Model) queueDownload(episodeID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.QueueEpisodeDownload(episodeID)
		return downloadQueuedMsg{episodeID: episodeID, err: err}
	}
}

func (m Model) startDownload(jobID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.StartJob(jobID)
		return downloadStartedMsg{jobID: jobID, err: err}
	}
}

func (m Model) retryDownload(jobID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.RetryJob(jobID)
		return downloadRetriedMsg{jobID: jobID, err: err}
	}
}

func (m Model) playEpisode(episodeID int64) tea.Cmd {
	return func() tea.Msg {
		if m.playerService == nil {
			return episodePlayedMsg{episodeID: episodeID, err: fmt.Errorf("player is unavailable")}
		}
		err := m.playerService.PlayEpisode(episodeID)
		return episodePlayedMsg{episodeID: episodeID, err: err}
	}
}

func (m Model) fetchPlaybackStatus() tea.Cmd {
	return func() tea.Msg {
		if m.playerService == nil {
			return playbackStatusMsg{err: fmt.Errorf("player is unavailable")}
		}
		status, err := m.playerService.PlaybackStatus()
		return playbackStatusMsg{status: status, err: err}
	}
}

func (m Model) togglePlayback() tea.Cmd {
	return func() tea.Msg {
		if m.playerService == nil {
			return playbackToggledMsg{err: fmt.Errorf("player is unavailable")}
		}
		err := m.playerService.TogglePlayPause()
		return playbackToggledMsg{err: err}
	}
}

func (m Model) skipPlayback(seconds float64) tea.Cmd {
	return func() tea.Msg {
		if m.playerService == nil {
			return playbackSkippedMsg{seconds: seconds, err: fmt.Errorf("player is unavailable")}
		}
		err := m.playerService.Seek(seconds)
		return playbackSkippedMsg{seconds: seconds, err: err}
	}
}

func (m Model) seekPlaybackTo(positionSec float64) tea.Cmd {
	return func() tea.Msg {
		if m.playerService == nil {
			return playbackSeekedMsg{
				positionSec: positionSec,
				err:         fmt.Errorf("player is unavailable"),
			}
		}
		current := m.playbackStatus.PositionSec
		if current == 0 && m.currentEpisode == nil && m.selectedEpisode == nil {
			return playbackSeekedMsg{
				positionSec: positionSec,
				err:         fmt.Errorf("no episode selected"),
			}
		}
		err := m.playerService.Seek(positionSec - current)
		return playbackSeekedMsg{positionSec: positionSec, err: err}
	}
}
