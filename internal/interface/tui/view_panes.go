package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/html"
	"github.com/amurru/gocaster/internal/interface/tui/components"
)

func (m Model) renderHelpPage() string {
	width := m.contentWidth()
	title := m.theme.SectionTitle.Render("Help & Shortcuts")
	subtitle := m.theme.MutedText.Render("How to use Gocaster and navigate the interface.")
	logo := m.theme.MutedText.Render(asciiLogo(max(width-4, 20)))
	panel := m.theme.PanelFocused.Width(max(width-4, 20))

	return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
		logo,
		"",
		title,
		subtitle,
		"",
		m.guide.View(),
	))
}

func (m Model) renderDownloadsPage() string {
	title := m.theme.SectionTitle.Render("Download Queue")
	subtitle := m.theme.MutedText.Render("Manage your downloads. Press s to start, r to retry.")
	panel := m.theme.Panel

	paneHeight := max(m.bodyHeight, 1)
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)

	m.queueList.SetSize(max(m.contentWidth()-4, 20), innerHeight)
	body := m.queueList.View()

	if len(m.queueList.Items()) == 0 {
		body = m.theme.MutedText.Render(
			"No downloads in queue.\n\nPress 'd' on an episode to download it.",
		)
	}

	return panel.Width(max(m.contentWidth()-4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderSettingsPage() string {
	title := m.theme.SectionTitle.Render("Settings")
	subtitle := m.theme.MutedText.Render(
		"Configure sync, Discord presence, and theme. Use j/k to move, Enter or Space to toggle/edit.",
	)
	panel := m.theme.PanelFocused

	rows := []string{
		fmt.Sprintf("Auto-sync on startup: %s", onOff(m.settings.AutoSyncOnStartup)),
		fmt.Sprintf("Periodic sync enabled: %s", onOff(m.settings.PeriodicSync)),
		fmt.Sprintf("Periodic sync interval (minutes): %d", m.settings.PeriodicSyncMins),
		fmt.Sprintf("Discord Rich Presence enabled: %s", onOff(m.settings.DiscordPresence)),
		fmt.Sprintf("Discord client ID: %s", valueOrPlaceholder(m.settings.DiscordClientID)),
		fmt.Sprintf("Theme: %s", m.themeList[m.selectedThemeIndex]),
	}

	for i := range rows {
		row := rows[i]
		if i == m.settingsCursor {
			if i == 2 && m.editingInterval {
				row = fmt.Sprintf("Periodic sync interval (minutes): %s", m.intervalInput.View())
			}
			if i == 4 && m.editingDiscordID {
				row = fmt.Sprintf("Discord client ID: %s", m.discordInput.View())
			}
			if i == 5 && m.editingTheme {
				row = fmt.Sprintf("Theme: %s", m.renderThemeSelector())
			}
			row = m.theme.Card.Width(max(m.contentWidth()-8, 20)).Render(row)
		} else {
			row = m.theme.Body.Render(row)
		}
		rows[i] = row
	}

	hint := m.theme.MutedText.Render("Esc to return. Press ? for help.")
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		rows[0],
		rows[1],
		rows[2],
		rows[3],
		rows[4],
		rows[5],
		"",
		hint,
	)

	return panel.Width(max(m.contentWidth()-4, 20)).
		Height(max(m.bodyHeight, 1)).
		MaxHeight(max(m.bodyHeight, 1)).
		Render(content)
}

func (m Model) renderPodcastPane() string {
	title := m.theme.SectionTitle.Render("Podcasts")
	subtitle := m.theme.MutedText.Render("Browse your subscriptions. Press / to filter.")
	panel := m.theme.Panel
	if m.focus == focusLibrary {
		panel = m.theme.PanelFocused
	}
	paneHeight := max(m.bodyHeight, 1)
	if m.shouldStackPanes() {
		paneHeight = max((m.bodyHeight-1)/2, 1)
	}
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)
	m.list.SetSize(max(m.listWidth, 1), innerHeight)

	body := m.list.View()
	if len(m.list.Items()) == 0 && !m.loadingLibrary {
		body = m.theme.MutedText.Render("No podcasts yet.\n\nPress 'a' to add an RSS feed.")
	}

	return panel.Width(max(m.listWidth+4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderDetailPane() string {
	panel := m.theme.Panel
	if m.focus == focusDetail {
		panel = m.theme.PanelFocused
	}
	paneHeight := max(m.detailHeight, 1)

	title := m.theme.SectionTitle.Render("Details")
	subtitle := m.theme.MutedText.Render("Show details. Press / to filter episodes.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)

	if m.selectedPodcast == nil {
		return panel.Width(max(m.detailWidth+4, 20)).
			Height(paneHeight).
			MaxHeight(paneHeight).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				header,
				m.theme.MutedText.Render(
					"Select a podcast to see its description and recent episodes.",
				),
			))
	}

	return panel.Width(max(m.detailWidth+4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.renderDetailContent(innerHeight),
		))
}

func (m Model) renderDetailContent(availableHeight int) string {
	if m.selectedPodcast == nil {
		return ""
	}

	wrapWidth := max(m.detailPaneWidth()-4, 16)

	detailParts := []string{
		m.theme.SectionTitle.Render(m.selectedPodcast.Title),
		lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.Label.Render("Feed "),
			m.theme.MutedText.Render(m.selectedPodcast.FeedURL),
		),
	}

	if !m.selectedPodcast.LastUpdated.IsZero() {
		detailParts = append(detailParts, lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.Label.Render("Updated "),
			m.theme.MutedText.Render(m.selectedPodcast.LastUpdated.Format(time.DateOnly)),
		))
	}

	description := strings.TrimSpace(m.selectedPodcast.Description)
	if description == "" {
		description = "No description available."
	}

	descriptionWrapped := m.theme.Body.Render(lipgloss.Wrap(description, wrapWidth, ""))
	descLines := lipgloss.Height(descriptionWrapped)

	episodesHeading := m.theme.SectionTitle.Render("Recent Episodes")
	episodesHeadingHeight := lipgloss.Height(episodesHeading)

	minEpisodesHeight := 3

	availableForDescription := availableHeight - episodesHeadingHeight - minEpisodesHeight
	if availableForDescription < 1 {
		availableForDescription = 1
	}

	if descLines > availableForDescription {
		truncated := truncateLines(descriptionWrapped, availableForDescription)
		descriptionWrapped = m.theme.Body.Render(truncated)
	}

	topCard := m.theme.Card.Width(max(m.detailPaneWidth(), 16)).
		Render(strings.Join(detailParts, "\n") + "\n" + descriptionWrapped)

	episodesHeight := max(
		availableHeight-lipgloss.Height(topCard)-episodesHeadingHeight,
		minEpisodesHeight,
	)
	episodes := m.renderEpisodes(episodesHeight)

	return lipgloss.JoinVertical(lipgloss.Left,
		topCard,
		episodesHeading,
		episodes,
	)
}

func (m Model) renderEpisodes(availableHeight int) string {
	if m.loadingDetail {
		return components.RenderLoading(m.theme, m.spin.View(), "Loading episodes…")
	}

	if len(m.episodes) == 0 {
		return m.theme.MutedText.Render("No stored episodes for this feed yet.")
	}

	m.epList.SetSize(max(m.detailPaneWidth(), 16), max(availableHeight, 3))

	return m.epList.View()
}

func (m Model) detailPaneWidth() int {
	if m.detailWidth <= 0 || m.contentWidth() <= 0 {
		return max(m.contentWidth()-4, 16)
	}
	return m.detailWidth
}

func (m Model) renderPlayerPage() string {
	title := m.theme.SectionTitle.Render("Player")
	subtitle := m.theme.MutedText.Render("Control playback, jump around, and read episode notes.")
	panel := m.theme.PanelFocused.Width(max(m.contentWidth()-4, 20))

	episode := m.displayEpisode()
	if episode == nil {
		return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
			title,
			subtitle,
			"",
			m.theme.MutedText.Render(
				"No episode is selected. Go back to the library and press p on an episode, or start playback first.",
			),
		))
	}

	podcastTitle := valueOrPlaceholder("")
	if m.currentPodcast != nil && m.currentPodcast.Title != "" {
		podcastTitle = m.currentPodcast.Title
	} else if m.selectedPodcast != nil {
		podcastTitle = m.selectedPodcast.Title
	}

	stateLabel := "stopped"
	switch m.playbackStatus.State {
	case domain.PlaybackStatePlaying:
		stateLabel = "playing"
	case domain.PlaybackStatePaused:
		stateLabel = "paused"
	case domain.PlaybackStateError:
		stateLabel = "error"
	}

	progressLine := formatPlaybackTime(m.playbackStatus.PositionSec)
	if m.playbackStatus.DurationSec > 0 {
		progressLine = fmt.Sprintf(
			"%s / %s",
			formatPlaybackTime(m.playbackStatus.PositionSec),
			formatPlaybackTime(m.playbackStatus.DurationSec),
		)
	}

	progressBar := components.RenderProgressBar(
		m.theme,
		m.playbackStatus.ProgressPct,
		max(m.contentWidth()-12, 24),
	)
	controls := m.theme.Card.Width(max(m.contentWidth()-8, 20)).Render(strings.Join([]string{
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.theme.Label.Render("Episode "),
			m.theme.Body.Render(episode.Title),
		),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.theme.Label.Render("Podcast "),
			m.theme.Body.Render(podcastTitle),
		),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.theme.Label.Render("State "),
			m.theme.Body.Render(strings.ToUpper(stateLabel)),
		),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.theme.Label.Render("Progress "),
			m.theme.Body.Render(progressLine),
		),
		progressBar,
	}, "\n"))

	notesHeading := m.theme.SectionTitle.Render("Episode Notes")
	notes := m.playerNotes.View()
	if strings.TrimSpace(episode.Description) == "" {
		notes = m.theme.MutedText.Render("No episode notes available.")
	}

	return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		controls,
		"",
		notesHeading,
		notes,
	))
}

func (m Model) renderPlayerNotesContent(width int) string {
	episode := m.displayEpisode()
	if episode == nil {
		return ""
	}
	wrapWidth := max(width-4, 16)
	description := strings.TrimSpace(episode.Description)
	if description == "" {
		return ""
	}
	return m.theme.Body.Render(lipgloss.Wrap(description, wrapWidth, ""))
}

func (m Model) renderPlaybackQueuePage() string {
	title := m.theme.SectionTitle.Render("Playback Queue")
	subtitle := m.theme.MutedText.Render("Manage the playback queue. Press d to remove, n/p for next/prev track.")
	panel := m.theme.PanelFocused

	paneHeight := max(m.bodyHeight, 1)
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)

	m.playbackQueueList.SetSize(max(m.contentWidth()-4, 20), innerHeight)
	body := m.playbackQueueList.View()

	if !m.playbackQueueLoaded {
		body = m.theme.MutedText.Render("Loading queue...")
	} else if len(m.playbackQueueList.Items()) == 0 {
		body = m.theme.MutedText.Render(
			"Queue is empty.\n\nPress 'e' on an episode to add it to the queue.",
		)
	}

	return panel.Width(max(m.contentWidth()-4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderShownotesPage() string {
	width := m.contentWidth()
	panel := m.theme.PanelFocused.Width(max(width-4, 20))

	episode := m.displayEpisode()
	if episode == nil {
		title := m.theme.SectionTitle.Render("Shownotes")
		return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			m.theme.MutedText.Render("No episode selected."),
		))
	}

	podcastTitle := valueOrPlaceholder("")
	if m.currentPodcast != nil && m.currentPodcast.Title != "" {
		podcastTitle = m.currentPodcast.Title
	} else if m.selectedPodcast != nil {
		podcastTitle = m.selectedPodcast.Title
	}

	title := m.theme.SectionTitle.Render("Shownotes")
	subtitle := m.theme.MutedText.Render(podcastTitle + " - " + episode.Title)

	content := m.shownotesViewport.View()
	if strings.TrimSpace(episode.Description) == "" {
		content = m.theme.MutedText.Render("No episode notes available.")
	}

	return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		content,
	))
}

func (m Model) renderShownotesContent(width int) string {
	episode := m.displayEpisode()
	if episode == nil {
		return ""
	}
	wrapWidth := max(width-4, 16)
	description := strings.TrimSpace(episode.Description)
	if description == "" {
		return ""
	}
	content, err := html.ConvertToText(description)
	if err != nil || strings.TrimSpace(content) == "" {
		return ""
	}
	return m.theme.Body.Render(lipgloss.Wrap(content, wrapWidth, ""))
}

func (m Model) renderGuideContent(width int) string {
	wrapWidth := max(width-4, 16)

	shortcuts := []string{
		m.theme.SectionTitle.Render("Shortcuts"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			m.theme.Label.Render("a") + "  Add a podcast feed",
			m.theme.Label.Render("r") + "  Refresh selected podcast feed",
			m.theme.Label.Render("g") + "  Go to episode by number (in detail pane)",
			m.theme.Label.Render("s") + "  Toggle episode sort order (newest/oldest first)",
			m.theme.Label.Render("S") + "  Open settings",
			m.theme.Label.Render("p") + "  Open the player screen",
			m.theme.Label.Render("tab") + "  Switch focus between the library and detail panes",
			m.theme.Label.Render("enter") + "  Confirm actions in dialogs and list filtering",
			m.theme.Label.Render("esc") + "  Close dialogs or leave this help page",
			m.theme.Label.Render("?") + "  Open or close this help page",
			m.theme.Label.Render("q / ctrl+c") + "  Quit the app",
			m.theme.Label.Render(
				"↑ ↓ / j k / pgup pgdn",
			) + "  Move through lists or scroll focused content",
			m.theme.Label.Render("/") + "  Filter the focused list (podcasts or episodes)",
			m.theme.Label.Render("enter / space") + "  Play the selected episode",
		}, "\n")),
	}

	usage := []string{
		m.theme.SectionTitle.Render("How To Use The App"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			"1. Start in the podcast library on the left. If the library is empty, press " + m.theme.Label.Render(
				"a",
			) + " and paste an RSS feed URL.",
			"2. Move through podcasts with the list keys. The selected show loads metadata and stored episodes into the detail pane.",
			"3. Press " + m.theme.Label.Render(
				"tab",
			) + " to focus the detail pane when you want to navigate episodes or scroll long descriptions.",
			"4. In the detail pane, use " + m.theme.Label.Render(
				"j/k",
			) + " or arrow keys to navigate between episodes. The selected episode is highlighted with an accent border.",
			"5. Press " + m.theme.Label.Render(
				"g",
			) + " to jump directly to an episode by number.",
			"6. Press " + m.theme.Label.Render(
				"p",
			) + " to open the player screen for the selected episode.",
			"7. In the player screen, use " + m.theme.Label.Render(
				"space",
			) + " to play or pause, " + m.theme.Label.Render("← →") + " to skip 15 seconds, and " + m.theme.Label.Render("t") + " to jump to a specific time.",
			"8. Press " + m.theme.Label.Render(
				"enter",
			) + " or " + m.theme.Label.Render(
				"space",
			) + " to play the selected episode.",
			"9. Episodes show a " + lipgloss.NewStyle().
				Foreground(m.theme.Success).
				Bold(true).
				Render("NEW") +
				" indicator for unplayed episodes and a " + m.theme.MutedText.Render(
				"PLAYED",
			) + " indicator for played ones.",
			"10. Press " + m.theme.Label.Render(
				"tab",
			) + " again to return focus to the podcast list.",
			"11. Press " + m.theme.Label.Render("?") + " any time to revisit this help page.",
		}, "\n\n")),
	}

	tips := []string{
		m.theme.SectionTitle.Render("What You're Looking At"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			"The left pane is your podcast library.",
			"The right pane shows the selected podcast description, feed info, and recent episodes.",
			"The player screen shows the current episode notes, progress, and playback controls.",
			"Episodes with a " + lipgloss.NewStyle().
				Foreground(m.theme.Success).
				Bold(true).
				Render("NEW") +
				" indicator haven't been played yet.",
			"Episodes with a " + m.theme.MutedText.Render(
				"PLAYED",
			) + " indicator have been played.",
			"The selected episode has a highlighted left border in the accent color.",
			"The status bar at the bottom shows feedback for loading, errors, and actions.",
		}, "\n")),
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		shortcuts[0],
		shortcuts[1],
		"",
		usage[0],
		usage[1],
		"",
		tips[0],
		tips[1],
	)
}
