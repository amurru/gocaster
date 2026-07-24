package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isBusy() {
		var spinCmd tea.Cmd
		m.spin, spinCmd = m.spin.Update(msg)
		if spinCmd != nil {
			cmds = append(cmds, spinCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, tea.Batch(cmds...)

	case tickMsg:
		// Keep the ticker running for badge flashing
		cmds = append(cmds, tickCmd())

		// Update playback status periodically
		if m.playerService != nil {
			cmds = append(cmds, m.fetchPlaybackStatus())
		}

		// Update episode list items with flash tick (skip if filtering is active)
		if len(m.episodes) > 0 && m.epList.FilterState() == list.Unfiltered {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(m.episodes))
			for i, episode := range m.episodes {
				items[i] = EpisodeItem{Episode: episode}.WithTheme(m.theme).WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.epList.SetItems(items))
		}

		// Update download queue progress (skip if filtering is active)
		if m.state == stateDownloads && len(m.downloadJobs) > 0 &&
			m.queueList.FilterState() == list.Unfiltered {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(m.downloadJobs))
			for i, job := range m.downloadJobs {
				items[i] = DownloadJobItem{
					DownloadJob:  job.DownloadJob,
					EpisodeTitle: episodeTitleForView(job),
					PodcastTitle: job.PodcastTitle,
				}.WithTheme(m.theme).
					WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.queueList.SetItems(items))
		}

		// Reload download jobs when in downloads view to get fresh progress
		if m.state == stateDownloads {
			cmds = append(cmds, m.loadDownloadJobs())
		}

		if m.settings.PeriodicSync &&
			!m.syncingAllFeeds &&
			!m.nextPeriodicSyncAt.IsZero() &&
			!time.Now().Before(m.nextPeriodicSyncAt) {
			m.syncingAllFeeds = true
			m.setStatus("Periodic sync started…", "info")
			cmds = append(cmds, m.syncAllPodcasts("periodic"), m.spin.Tick)
		}

		return m, tea.Batch(cmds...)
	case tea.PasteMsg:
		// Route the paste to whichever input is actually focused. Previously
		// this hardcoded m.input (the add-podcast field), so pasting into the
		// go-to-episode, seek, or settings inputs silently dropped the paste.
		var cmd tea.Cmd
		switch {
		case m.state == stateAddPodcast:
			m.input, cmd = m.input.Update(msg)
		case m.state == stateGoToEpisode:
			m.goToInput, cmd = m.goToInput.Update(msg)
		case m.state == statePlayerSeek:
			m.seekInput, cmd = m.seekInput.Update(msg)
		case m.state == stateSettings && m.editingInterval:
			m.intervalInput, cmd = m.intervalInput.Update(msg)
		case m.state == stateSettings && m.editingDiscordID:
			m.discordInput, cmd = m.discordInput.Update(msg)
		}
		return m, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			if m.state == stateHelp {
				if m.previousState == "" {
					m.previousState = stateBrowse
				}
				m.state = m.previousState
				m.setStatus("Returned to previous screen.", "info")
			} else {
				m.previousState = m.state
				m.state = stateHelp
				m.syncGuideViewport(true)
				m.setStatus("Help page opened.", "info")
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Settings):
			if m.state == stateSettings {
				m.state = stateBrowse
				m.editingInterval = false
				m.editingDiscordID = false
				m.intervalInput.Blur()
				m.discordInput.Blur()
				m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
				m.discordInput.SetValue(m.settings.DiscordClientID)
				m.setStatus("Returned to library.", "info")
			} else if m.state == stateBrowse {
				m.openSettingsPage()
				m.setStatus("Settings page opened.", "info")
			}
			return m, tea.Batch(cmds...)
		}

		if m.state == stateHelp {
			if key.Matches(msg, m.keys.Close) {
				m.state = stateBrowse
				m.setStatus("Returned to library.", "info")
				return m, tea.Batch(cmds...)
			}

			var guideCmd tea.Cmd
			m.guide, guideCmd = m.guide.Update(msg)
			if guideCmd != nil {
				cmds = append(cmds, guideCmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.state == stateAddPodcast {
			return m.handleAddMode(msg, cmds)
		}

		if m.state == stateGoToEpisode {
			return m.handleGoToEpisodeMode(msg, cmds)
		}

		if m.state == stateDownloads {
			return m.handleDownloadsMode(msg, cmds)
		}

		if m.state == stateSettings {
			return m.handleSettingsMode(msg, cmds)
		}

		if m.state == statePlayerSeek {
			return m.handlePlayerSeekMode(msg, cmds)
		}

		if m.state == statePlayer {
			return m.handlePlayerMode(msg, cmds)
		}

		if m.state == statePlaybackQueue {
			return m.handlePlaybackQueueMode(msg, cmds)
		}

		isFiltering := m.list.FilterState() == list.Filtering ||
			m.epList.FilterState() == list.Filtering

		if key.Matches(msg, m.keys.Add) && !isFiltering {
			m.openAddModal()
			cmds = append(cmds, m.input.Focus())
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.SwitchPane) && !isFiltering {
			m.toggleFocus()
			m.syncDetailViewport(false)
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.RefreshPodcast) && !isFiltering {
			if m.selectedPodcast == nil {
				m.setStatus("No podcast selected to refresh", "warning")
				return m, tea.Batch(cmds...)
			}
			m.loadingDetail = true
			m.setStatus("Refreshing feed…", "info")
			cmds = append(cmds, m.refreshPodcast(m.selectedPodcast.ID), m.spin.Tick)
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.DownloadQueue) && !isFiltering {
			m.openDownloadsQueue()
			cmds = append(cmds, m.loadDownloadJobs(), m.spin.Tick)
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.OpenPlayer) && !isFiltering {
			m.openPlayerPage()
			return m, tea.Batch(cmds...)
		}

		if m.focus == focusDetail && key.Matches(msg, m.keys.DownloadEpisode) && !isFiltering {
			if m.selectedEpisode != nil {
				cmds = append(cmds, m.queueDownload(m.selectedEpisode.ID))
			}
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.ToggleEpisodeSort) && !isFiltering && len(m.episodes) > 0 {
			m.toggleEpisodeSort()
			m.syncDetailViewport(false)
			return m, tea.Batch(cmds...)
		}

	case podcastsLoadedMsg:
		m.loadingLibrary = false
		if msg.err != nil {
			m.setStatus("Failed to load podcasts", "error")
			return m, tea.Batch(cmds...)
		}

		items := make([]list.Item, len(msg.podcasts))
		for i, podcast := range msg.podcasts {
			items[i] = PodcastItem{Podcast: podcast}
		}
		cmds = append(cmds, m.list.SetItems(items))

		if len(msg.podcasts) == 0 {
			m.selectedPodcast = nil
			m.episodes = nil
			m.syncDetailViewport(true)
			m.setStatus("Library is empty. Press 'a' to add your first feed.", "info")
			return m, tea.Batch(cmds...)
		}

		if m.selectedPodcast != nil {
			for i, podcast := range msg.podcasts {
				if podcast.ID == m.selectedPodcast.ID {
					m.list.Select(i)
					break
				}
			}
		}

		selected := selectedPodcastItem(m.list)
		if selected != nil {
			m.selectedPodcast = selected
			m.loadingDetail = true
			m.syncDetailViewport(true)
			m.setStatus(fmt.Sprintf("Loaded %d podcasts", len(msg.podcasts)), "success")
			cmds = append(cmds, m.loadEpisodes(selected.ID), m.spin.Tick)
		}
		return m, tea.Batch(cmds...)

	case episodesLoadedMsg:
		if m.selectedPodcast == nil || msg.podcastID != m.selectedPodcast.ID {
			return m, tea.Batch(cmds...)
		}

		m.loadingDetail = false
		if msg.err != nil {
			m.episodes = nil
			m.selectedEpisode = nil
			m.setStatus("Failed to load episodes", "error")
			return m, tea.Batch(cmds...)
		}

		m.episodes = msg.episodes
		m.selectedEpisode = nil

		m.rebuildEpisodeList()

		m.syncDetailViewport(false)
		return m, tea.Batch(cmds...)

	case podcastAddedMsg:
		m.submitting = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Add failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.state = stateBrowse
		m.input.Blur()
		m.input.Reset()
		m.setStatus(fmt.Sprintf("Added %s", msg.podcast.Title), "success")
		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.spin.Tick)
		return m, tea.Batch(cmds...)

	case podcastRefreshedMsg:
		m.loadingDetail = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Refresh failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.loadingDetail = true
		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.loadEpisodes(msg.podcastID), m.spin.Tick)

		if msg.newCount > 0 {
			m.setStatus(
				fmt.Sprintf("Added %d new episode%s", msg.newCount, suffix(msg.newCount)),
				"success",
			)
		} else {
			m.setStatus("No new episodes", "info")
		}
		return m, tea.Batch(cmds...)

	case allPodcastsSyncedMsg:
		m.syncingAllFeeds = false
		if m.settings.PeriodicSync {
			m.nextPeriodicSyncAt = time.Now().
				Add(time.Duration(m.settings.PeriodicSyncMins) * time.Minute)
		}
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Sync failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.spin.Tick)
		if m.selectedPodcast != nil {
			m.loadingDetail = true
			cmds = append(cmds, m.loadEpisodes(m.selectedPodcast.ID))
		}
		prefix := "Sync complete"
		if msg.reason == "startup" {
			prefix = "Startup sync complete"
		}
		// Summarize per-feed failures so the user can see which feeds failed
		// and why (issue #7), not just a count. Keep it short for the status bar.
		failureSummary := ""
		if len(msg.result.Failures) > 0 {
			parts := make([]string, 0, len(msg.result.Failures))
			for _, f := range msg.result.Failures {
				title := f.Title
				if title == "" {
					title = f.FeedURL
				}
				// Keep the full error; the overall 160-char cap below shortens
				// the summary. Truncating at the first colon would mangle common
				// network errors (e.g. `Get "https://…": dial tcp: …` -> `Get "https`).
				reason := f.Err.Error()
				parts = append(parts, fmt.Sprintf("%s: %s", title, reason))
			}
			failureSummary = " — " + strings.Join(parts, "; ")
			// Cap the status bar length so many failures don't overflow. Truncate
			// on a rune boundary so a multi-byte UTF-8 character isn't split into
			// invalid UTF-8 (titles/errors may contain non-ASCII).
			failureSummary = truncateRunes(failureSummary, 160)
		}
		severity := "success"
		if msg.result.Failed > 0 {
			severity = "error"
		}
		m.setStatus(
			fmt.Sprintf(
				"%s: %d new episodes across %d/%d podcasts (%d failed)%s",
				prefix,
				msg.result.NewEpisodes,
				msg.result.Refreshed,
				msg.result.TotalPodcasts,
				msg.result.Failed,
				failureSummary,
			),
			severity,
		)
		return m, tea.Batch(cmds...)

	case settingsPersistedMsg:
		if msg.err != nil {
			m.settings = msg.previous
			m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
			m.discordInput.SetValue(m.settings.DiscordClientID)
			m.setStatus(fmt.Sprintf("Settings save failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}
		m.settings = msg.settings
		if m.settings.PeriodicSync {
			m.nextPeriodicSyncAt = time.Now().
				Add(time.Duration(m.settings.PeriodicSyncMins) * time.Minute)
		} else {
			m.nextPeriodicSyncAt = time.Time{}
		}
		m.setStatus("Settings saved", "success")
		return m, tea.Batch(cmds...)

	case errMsg:
		m.setStatus(msg.err.Error(), "error")
		return m, tea.Batch(cmds...)

	case downloadQueuedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Download queue failed: %v", msg.err), "error")
		} else {
			m.setStatus("Episode queued for download", "success")
			cmds = append(cmds, m.loadDownloadJobs())
		}
		return m, tea.Batch(cmds...)

	case downloadJobsLoadedMsg:
		m.downloadJobs = msg.jobs
		if msg.err != nil {
			m.setStatus("Failed to load downloads", "error")
		} else {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(msg.jobs))
			for i, job := range msg.jobs {
				items[i] = DownloadJobItem{
					DownloadJob:  job.DownloadJob,
					EpisodeTitle: episodeTitleForView(job),
					PodcastTitle: job.PodcastTitle,
				}.WithTheme(m.theme).
					WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.queueList.SetItems(items))
		}
		return m, tea.Batch(cmds...)

	case downloadStartedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Start failed: %v", msg.err), "error")
		} else {
			m.setStatus("Download started", "success")
		}
		cmds = append(cmds, m.loadDownloadJobs())
		return m, tea.Batch(cmds...)

	case downloadRetriedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Retry failed: %v", msg.err), "error")
		} else {
			m.setStatus("Download retry started", "success")
		}
		cmds = append(cmds, m.loadDownloadJobs())
		return m, tea.Batch(cmds...)

	case episodePlayedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Playback failed: %v", msg.err), "error")
		} else {
			if episode := m.episodeByID(msg.episodeID); episode != nil {
				m.currentEpisode = episode
			} else if m.selectedEpisode != nil {
				episode := *m.selectedEpisode
				m.currentEpisode = &episode
			}
			if m.selectedPodcast != nil {
				podcast := *m.selectedPodcast
				m.currentPodcast = &podcast
			}
			if m.state == statePlayer {
				m.syncPlayerViewport(true)
			}
			m.setStatus("Playing episode", "success")
			cmds = append(cmds, m.fetchPlaybackStatus())
		}
		return m, tea.Batch(cmds...)

	case playbackStatusMsg:
		if msg.err == nil {
			m.playbackStatus = msg.status
			if episode := m.playingEpisodeFromStatus(msg.status); episode != nil {
				m.currentEpisode = episode
			}
			if m.state == statePlayer {
				m.syncPlayerViewport(false)
			}
		}
		return m, tea.Batch(cmds...)

	case playbackToggledMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Playback toggle failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}
		m.setStatus("Playback toggled", "success")
		cmds = append(cmds, m.fetchPlaybackStatus())
		return m, tea.Batch(cmds...)

	case playbackSkippedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Seek failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}
		direction := "forward"
		if msg.seconds < 0 {
			direction = "back"
		}
		m.setStatus(fmt.Sprintf("Skipped %s 15s", direction), "success")
		cmds = append(cmds, m.fetchPlaybackStatus())
		return m, tea.Batch(cmds...)

	case playbackSeekedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Seek failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}
		m.setStatus(
			fmt.Sprintf("Seeked to %s", formatPlaybackTime(msg.positionSec)),
			"success",
		)
		cmds = append(cmds, m.fetchPlaybackStatus())
		return m, tea.Batch(cmds...)

	case playbackQueueLoadedMsg:
		if msg.err != nil {
			m.setStatus("Failed to load playback queue", "error")
			return m, tea.Batch(cmds...)
		}
		m.queueViews = msg.views
		items := make([]list.Item, len(msg.views))
		for i, v := range msg.views {
			items[i] = PlaybackQueueItem{
				QueueItemView: v,
				Theme:         m.theme,
			}
		}
		cmds = append(cmds, m.playbackQueueList.SetItems(items))
		m.setStatus(fmt.Sprintf("Queue loaded (%d items)", len(msg.views)), "success")
		return m, tea.Batch(cmds...)

	case queueItemAddedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Add to queue failed: %v", msg.err), "error")
		} else {
			m.setStatus("Added to queue", "success")
			cmds = append(cmds, m.loadPlaybackQueueCmd())
		}
		return m, tea.Batch(cmds...)

	case queueItemRemovedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Remove from queue failed: %v", msg.err), "error")
		} else {
			m.setStatus("Removed from queue", "success")
			cmds = append(cmds, m.loadPlaybackQueueCmd())
		}
		return m, tea.Batch(cmds...)

	case queueTrackChangedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Track change failed: %v", msg.err), "error")
		} else {
			m.setStatus("Track changed", "success")
			cmds = append(cmds, m.loadPlaybackQueueCmd(), m.fetchPlaybackStatus())
		}
		return m, tea.Batch(cmds...)

	case repeatModeToggledMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Repeat toggle failed: %v", msg.err), "error")
		} else {
			m.setStatus("Repeat mode toggled", "success")
			cmds = append(cmds, m.loadPlaybackQueueCmd())
		}
		return m, tea.Batch(cmds...)

	case shuffleToggledMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Shuffle toggle failed: %v", msg.err), "error")
		} else {
			m.setStatus("Shuffle toggled", "success")
			cmds = append(cmds, m.loadPlaybackQueueCmd())
		}
		return m, tea.Batch(cmds...)
	}

	if m.focus == focusDetail {
		// Handle episode navigation when detail pane is focused
		previousEpisodeID := int64(0)
		if selected := selectedEpisodeItem(m.epList); selected != nil {
			previousEpisodeID = selected.ID
		}

		// Handle play episode action before updating the list (skip if filtering)
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(msg, m.keys.PlayEpisode) && m.epList.FilterState() != list.Filtering {
				if selected := selectedEpisodeItem(m.epList); selected != nil {
					cmds = append(cmds, m.playEpisode(selected.ID))
				}
			}

			// Open "go to episode" modal when 'g' is pressed (skip if filtering)
			if key.Matches(msg, m.keys.GoToEpisode) && m.epList.FilterState() != list.Filtering &&
				m.state == stateBrowse {
				m.openGoToEpisodeModal()
				cmds = append(cmds, m.goToInput.Focus())
				return m, tea.Batch(cmds...)
			}

			// Playback queue: add to queue, play next, open queue view
			if m.state == stateBrowse && m.epList.FilterState() != list.Filtering {
				if selected := selectedEpisodeItem(m.epList); selected != nil {
					if key.Matches(msg, m.keys.AddToQueue) {
						cmds = append(cmds, m.addToQueueCmd(selected.ID))
						return m, tea.Batch(cmds...)
					}
					if key.Matches(msg, m.keys.AddPlayNext) {
						cmds = append(cmds, m.addPlayNextCmd(selected.ID))
						return m, tea.Batch(cmds...)
					}
				}
				if key.Matches(msg, m.keys.OpenQueue) {
					m.openPlaybackQueuePage()
					cmds = append(cmds, m.loadPlaybackQueueCmd(), m.spin.Tick)
					return m, tea.Batch(cmds...)
				}
			}
		}

		var epListCmd tea.Cmd
		m.epList, epListCmd = m.epList.Update(msg)
		if epListCmd != nil {
			cmds = append(cmds, epListCmd)
		}

		selected := selectedEpisodeItem(m.epList)
		if selected != nil && selected.ID != previousEpisodeID {
			m.selectedEpisode = &selected.Episode
		}

		var detailCmd tea.Cmd
		m.detail, detailCmd = m.detail.Update(msg)
		if detailCmd != nil {
			cmds = append(cmds, detailCmd)
		}
		return m, tea.Batch(cmds...)
	}

	previousID := int64(0)
	if selected := selectedPodcastItem(m.list); selected != nil {
		previousID = selected.ID
	}

	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	selected := selectedPodcastItem(m.list)
	if selected != nil && selected.ID != previousID {
		m.selectedPodcast = selected
		m.loadingDetail = true
		m.syncDetailViewport(true)
		cmds = append(cmds, m.loadEpisodes(selected.ID), m.spin.Tick)
	}

	return m, tea.Batch(cmds...)
}
