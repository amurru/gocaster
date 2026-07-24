package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func (m Model) handleAddMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) && !m.submitting {
		m.state = stateBrowse
		m.input.Blur()
		m.input.Reset()
		m.setStatus("Add podcast cancelled", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) && !m.submitting {
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.setStatus("Feed URL is required", "warning")
			return m, tea.Batch(cmds...)
		}

		m.submitting = true
		m.setStatus("Fetching feed and saving episodes…", "info")
		cmds = append(cmds, m.addPodcast(value), m.spin.Tick)
		return m, tea.Batch(cmds...)
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleGoToEpisodeMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.goToInput.Blur()
		m.goToInput.Reset()
		m.setStatus("Go to episode cancelled", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) {
		value := strings.TrimSpace(m.goToInput.Value())
		if value == "" {
			m.setStatus("Episode number is required", "warning")
			return m, tea.Batch(cmds...)
		}

		var num int
		if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
			m.setStatus("Invalid episode number", "warning")
			return m, tea.Batch(cmds...)
		}

		idx := num - 1
		if idx < 0 || idx >= len(m.episodes) {
			m.setStatus(
				fmt.Sprintf("Episode %d out of range (1-%d)", num, len(m.episodes)),
				"warning",
			)
			return m, tea.Batch(cmds...)
		}

		m.state = stateBrowse
		m.goToInput.Blur()
		m.goToInput.Reset()
		m.epList.Select(idx)
		m.selectedEpisode = &m.episodes[idx]
		m.setStatus(fmt.Sprintf("Selected episode %d", num), "success")
		return m, tea.Batch(cmds...)
	}

	var inputCmd tea.Cmd
	m.goToInput, inputCmd = m.goToInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handlePlayerMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) || key.Matches(msg, m.keys.OpenPlayer) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.ToggleHelp) {
		m.state = stateHelp
		m.syncGuideViewport(true)
		m.setStatus("Help page opened.", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.SeekToTime) {
		m.openPlayerSeekModal()
		cmds = append(cmds, m.seekInput.Focus())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.TogglePlayPause) {
		if m.playbackStatus.State == domain.PlaybackStateStopped || m.playbackStatus.State == "" {
			if episode := m.displayEpisode(); episode != nil {
				cmds = append(cmds, m.playEpisode(episode.ID))
				return m, tea.Batch(cmds...)
			}
			m.setStatus("No episode available to play", "warning")
			return m, tea.Batch(cmds...)
		}
		if m.playerService == nil {
			m.setStatus("Player is unavailable", "error")
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.togglePlayback())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.SkipBackward) {
		if m.playbackStatus.State == domain.PlaybackStateStopped {
			m.setStatus("No episode is playing", "warning")
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.skipPlayback(-15))
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.SkipForward) {
		if m.playbackStatus.State == domain.PlaybackStateStopped {
			m.setStatus("No episode is playing", "warning")
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.skipPlayback(15))
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.SpeedDown) {
		if m.playbackStatus.State == domain.PlaybackStateStopped {
			m.setStatus("No episode is playing", "warning")
			return m, tea.Batch(cmds...)
		}
		newSpeed := nextSpeedDown(m.playbackStatus.Speed)
		cmds = append(cmds, m.setPlaybackSpeed(newSpeed))
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.SpeedUp) {
		if m.playbackStatus.State == domain.PlaybackStateStopped {
			m.setStatus("No episode is playing", "warning")
			return m, tea.Batch(cmds...)
		}
		newSpeed := nextSpeedUp(m.playbackStatus.Speed)
		cmds = append(cmds, m.setPlaybackSpeed(newSpeed))
		return m, tea.Batch(cmds...)
	}

	var noteCmd tea.Cmd
	m.playerNotes, noteCmd = m.playerNotes.Update(msg)
	if noteCmd != nil {
		cmds = append(cmds, noteCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handlePlayerSeekMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = statePlayer
		m.seekInput.Blur()
		m.seekInput.Reset()
		m.setStatus("Seek cancelled", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) {
		value := strings.TrimSpace(m.seekInput.Value())
		if value == "" {
			m.setStatus("Seek time is required", "warning")
			return m, tea.Batch(cmds...)
		}

		target, err := parseSeekTime(value)
		if err != nil {
			m.setStatus(err.Error(), "warning")
			return m, tea.Batch(cmds...)
		}

		m.state = statePlayer
		m.seekInput.Blur()
		m.seekInput.Reset()
		cmds = append(cmds, m.seekPlaybackTo(target))
		return m, tea.Batch(cmds...)
	}

	var inputCmd tea.Cmd
	m.seekInput, inputCmd = m.seekInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleDownloadsMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.StartDownload) {
		selected := selectedDownloadJobItem(m.queueList)
		if selected != nil && selected.Status == domain.DownloadStatusQueued {
			cmds = append(cmds, m.startDownload(selected.ID))
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.RetryDownload) {
		selected := selectedDownloadJobItem(m.queueList)
		if selected != nil && selected.Status == domain.DownloadStatusFailed {
			cmds = append(cmds, m.retryDownload(selected.ID))
		}
		return m, tea.Batch(cmds...)
	}

	var queueCmd tea.Cmd
	m.queueList, queueCmd = m.queueList.Update(msg)
	if queueCmd != nil {
		cmds = append(cmds, queueCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handlePlaybackQueueMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.NextTrack) {
		cmds = append(cmds, m.nextTrackCmd())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.PrevTrack) {
		cmds = append(cmds, m.prevTrackCmd())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.CycleRepeat) {
		cmds = append(cmds, m.cycleRepeatCmd())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.ToggleShuffle) {
		cmds = append(cmds, m.toggleShuffleCmd())
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.DeleteQueueItem) {
		if selected := selectedPlaybackQueueItem(m.playbackQueueList); selected != nil {
			cmds = append(cmds, m.removeFromQueueCmd(selected.QueueItemView.Item.ID))
		}
		return m, tea.Batch(cmds...)
	}

	var listCmd tea.Cmd
	m.playbackQueueList, listCmd = m.playbackQueueList.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleSettingsMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.editingInterval {
		if key.Matches(msg, m.keys.Close) {
			m.editingInterval = false
			m.intervalInput.Blur()
			m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
			m.setStatus("Interval edit cancelled", "info")
			return m, tea.Batch(cmds...)
		}
		if key.Matches(msg, m.keys.Submit) {
			value := strings.TrimSpace(m.intervalInput.Value())
			minutes, err := strconv.Atoi(value)
			if err != nil || minutes <= 0 {
				m.setStatus("Interval must be a positive integer", "warning")
				return m, tea.Batch(cmds...)
			}
			prev := m.settings
			next := m.settings
			next.PeriodicSyncMins = minutes
			m.editingInterval = false
			m.intervalInput.Blur()
			cmds = append(cmds, m.persistSettings(next, prev))
			return m, tea.Batch(cmds...)
		}
		var inputCmd tea.Cmd
		m.intervalInput, inputCmd = m.intervalInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.editingDiscordID {
		if key.Matches(msg, m.keys.Close) {
			m.editingDiscordID = false
			m.discordInput.Blur()
			m.discordInput.SetValue(m.settings.DiscordClientID)
			m.setStatus("Discord client ID edit cancelled", "info")
			return m, tea.Batch(cmds...)
		}
		if key.Matches(msg, m.keys.Submit) {
			prev := m.settings
			next := m.settings
			next.DiscordClientID = strings.TrimSpace(m.discordInput.Value())
			if next.DiscordPresence && next.DiscordClientID == "" {
				m.setStatus(
					"Discord client ID is required when Discord presence is enabled",
					"warning",
				)
				return m, tea.Batch(cmds...)
			}
			m.editingDiscordID = false
			m.discordInput.Blur()
			cmds = append(cmds, m.persistSettings(next, prev))
			return m, tea.Batch(cmds...)
		}
		var inputCmd tea.Cmd
		m.discordInput, inputCmd = m.discordInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.editingTheme {
		if key.Matches(msg, m.keys.Close) {
			m.editingTheme = false
			m.setStatus("Theme selection cancelled", "info")
			return m, tea.Batch(cmds...)
		}
		if key.Matches(msg, m.keys.Submit) {
			prev := m.settings
			next := m.settings
			next.ThemeName = m.themeList[m.selectedThemeIndex]
			m.editingTheme = false
			// Apply theme immediately for preview
			m.theme = styles.LoadTheme(next.ThemeName, m.customThemesDir)
			cmds = append(cmds, m.persistSettings(next, prev))
			m.setStatus(fmt.Sprintf("Theme changed to %s", next.ThemeName), "success")
			return m, tea.Batch(cmds...)
		}
		switch msg.String() {
		case "left", "h":
			if m.selectedThemeIndex > 0 {
				m.selectedThemeIndex--
				// Apply theme preview
				m.theme = styles.LoadTheme(m.themeList[m.selectedThemeIndex], m.customThemesDir)
			}
			return m, tea.Batch(cmds...)
		case "right", "l":
			if m.selectedThemeIndex < len(m.themeList)-1 {
				m.selectedThemeIndex++
				// Apply theme preview
				m.theme = styles.LoadTheme(m.themeList[m.selectedThemeIndex], m.customThemesDir)
			}
			return m, tea.Batch(cmds...)
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	switch msg.String() {
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, tea.Batch(cmds...)
	case "down", "j":
		if m.settingsCursor < 5 {
			m.settingsCursor++
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) || key.Matches(msg, m.keys.PlayEpisode) {
		prev := m.settings
		next := m.settings
		switch m.settingsCursor {
		case 0:
			next.AutoSyncOnStartup = !next.AutoSyncOnStartup
		case 1:
			next.PeriodicSync = !next.PeriodicSync
		case 2:
			m.editingInterval = true
			cmds = append(cmds, m.intervalInput.Focus())
			return m, tea.Batch(cmds...)
		case 3:
			if !next.DiscordPresence && strings.TrimSpace(next.DiscordClientID) == "" {
				m.setStatus("Set Discord client ID before enabling Discord presence", "warning")
				return m, tea.Batch(cmds...)
			}
			next.DiscordPresence = !next.DiscordPresence
		case 4:
			m.editingDiscordID = true
			cmds = append(cmds, m.discordInput.Focus())
			return m, tea.Batch(cmds...)
		case 5:
			m.editingTheme = true
			m.selectedThemeIndex = findThemeIndex(m.settings.ThemeName, m.themeList)
			m.setStatus(
				"Use left/right arrows to preview themes, Enter to confirm, Esc to cancel",
				"info",
			)
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.persistSettings(next, prev))
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}
