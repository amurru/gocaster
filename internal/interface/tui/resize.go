package tui

import (
	"sort"
	"strconv"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/domain"
)

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	contentWidth := m.contentWidth()
	m.help.SetWidth(max(contentWidth, 20))

	appFrameHeight := m.theme.App.GetVerticalFrameSize()
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	bodyHeight := m.height - appFrameHeight - headerHeight - footerHeight
	m.bodyHeight = max(bodyHeight, 1)

	// These are now set inside the render methods, which have the final say
	// on height, but we can still set the width here.
	if m.shouldStackPanes() {
		m.listWidth = max(contentWidth-4, 16)
		m.detailWidth = max(contentWidth-4, 16)
		m.list.SetSize(m.listWidth, max((m.bodyHeight-1)/2, 1))
		m.detailHeight = max(m.bodyHeight-max((m.bodyHeight-1)/2, 1)-1, 1)
	} else {
		gap := 1
		leftWidth := max(contentWidth/3, 24)
		rightWidth := max(contentWidth-leftWidth-gap, 24)
		if leftWidth+rightWidth+gap > contentWidth {
			rightWidth = max(contentWidth-leftWidth-gap, 24)
		}
		if leftWidth+rightWidth+gap > contentWidth {
			leftWidth = max(contentWidth-rightWidth-gap, 24)
		}
		m.listWidth = max(leftWidth-4, 16)
		m.list.SetSize(m.listWidth, max(m.bodyHeight, 1))
		m.detailWidth = max(rightWidth-4, 16)
		m.detailHeight = max(m.bodyHeight, 1)
	}

	m.input.SetWidth(min(max(contentWidth-12, 20), 72))
	m.goToInput.SetWidth(min(max(contentWidth-12, 20), 72))
	m.seekInput.SetWidth(min(max(contentWidth-12, 20), 72))
	m.syncDetailViewport(false)
	m.syncGuideViewport(false)
	m.syncPlayerViewport(false)
}

func (m *Model) openAddModal() {
	m.state = stateAddPodcast
	m.input.Reset()
	m.input.Placeholder = "https://example.com/feed.xml"
}

func (m *Model) openGoToEpisodeModal() {
	m.state = stateGoToEpisode
	m.goToInput.Reset()
	m.goToInput.Placeholder = "episode number"
}

func (m *Model) openPlayerPage() {
	m.state = statePlayer
	if m.currentEpisode == nil {
		if episode := m.displayEpisode(); episode != nil {
			clone := *episode
			m.currentEpisode = &clone
		}
	}
	if m.currentPodcast == nil && m.selectedPodcast != nil {
		clone := *m.selectedPodcast
		m.currentPodcast = &clone
	}
	m.syncPlayerViewport(true)
	m.setStatus("Player opened", "info")
}

func (m *Model) openPlayerSeekModal() {
	m.state = statePlayerSeek
	m.seekInput.Reset()
	m.seekInput.Placeholder = "mm:ss or hh:mm:ss"
}

func (m *Model) toggleFocus() {
	if m.focus == focusLibrary {
		m.focus = focusDetail
		if len(m.episodes) > 0 {
			m.setStatus(
				"Detail pane focused. Use j/k or arrow keys to navigate episodes, enter/space to play.",
				"info",
			)
		} else {
			m.setStatus("Detail pane focused. Use arrow keys or PgUp/PgDn to scroll.", "info")
		}
		return
	}
	m.focus = focusLibrary
	m.setStatus("Podcast list focused.", "info")
}

func (m *Model) toggleEpisodeSort() {
	if m.sortOrder == sortNewestFirst {
		m.sortOrder = sortOldestFirst
		m.setStatus("Sorting: oldest episodes first", "info")
	} else {
		m.sortOrder = sortNewestFirst
		m.setStatus("Sorting: newest episodes first", "info")
	}
	m.rebuildEpisodeList()
}

func (m *Model) rebuildEpisodeList() {
	sorted := make([]domain.Episode, len(m.episodes))
	copy(sorted, m.episodes)

	sortByPublishedAt(sorted, m.sortOrder == sortOldestFirst)

	previousID := int64(0)
	if m.selectedEpisode != nil {
		previousID = m.selectedEpisode.ID
	}

	items := make([]list.Item, len(sorted))
	for i, episode := range sorted {
		items[i] = EpisodeItem{Episode: episode}.WithTheme(m.theme)
	}
	m.epList.SetItems(items)

	m.episodes = sorted

	if previousID > 0 {
		for i, ep := range m.episodes {
			if ep.ID == previousID {
				m.epList.Select(i)
				m.selectedEpisode = &m.episodes[i]
				break
			}
		}
	}
}

func sortByPublishedAt(episodes []domain.Episode, oldestFirst bool) {
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].PublishedAt.IsZero() && episodes[j].PublishedAt.IsZero() {
			return false
		}
		if episodes[i].PublishedAt.IsZero() {
			return !oldestFirst
		}
		if episodes[j].PublishedAt.IsZero() {
			return oldestFirst
		}
		if oldestFirst {
			return episodes[i].PublishedAt.Before(episodes[j].PublishedAt)
		}
		return episodes[i].PublishedAt.After(episodes[j].PublishedAt)
	})
}

func (m *Model) setStatus(text, kind string) {
	m.status = text
	m.kind = kind
}

func (m Model) isBusy() bool {
	return m.loadingLibrary || m.loadingDetail || m.submitting
}

func (m *Model) openDownloadsQueue() {
	m.state = stateDownloads
	m.setStatus("Download queue opened", "info")
}

func (m *Model) openPlaybackQueuePage() {
	m.state = statePlaybackQueue
	m.playbackQueueLoaded = false
	m.setStatus("Playback queue opened", "info")
}

func (m *Model) openSettingsPage() {
	m.state = stateSettings
	m.settingsCursor = 0
	m.editingInterval = false
	m.editingDiscordID = false
	m.editingTheme = false
	m.intervalInput.Blur()
	m.discordInput.Blur()
	m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
	m.discordInput.SetValue(m.settings.DiscordClientID)
	m.selectedThemeIndex = findThemeIndex(m.settings.ThemeName, m.themeList)
}

func (m *Model) syncDetailViewport(reset bool) {
	width := max(m.detailPaneWidth(), 16)
	height := max(m.detailHeight-2, 5)

	m.detail.SetWidth(width)
	m.detail.SetHeight(height)
	m.detail.SetContent(m.renderDetailContent(height))
	if reset {
		m.detail.GotoTop()
	}
}

func (m *Model) syncGuideViewport(reset bool) {
	width := max(m.contentWidth()-8, 20)
	height := max(m.bodyHeight-4, 6)

	m.guide.SetWidth(width)
	m.guide.SetHeight(height)
	m.guide.SetContent(m.renderGuideContent(width))
	if reset {
		m.guide.GotoTop()
	}
}

func (m *Model) syncPlayerViewport(reset bool) {
	width := max(m.contentWidth()-8, 20)
	height := max(m.bodyHeight-14, 6)

	m.playerNotes.SetWidth(width)
	m.playerNotes.SetHeight(height)
	m.playerNotes.SetContent(m.renderPlayerNotesContent(width))
	if reset {
		m.playerNotes.GotoTop()
	}
}
