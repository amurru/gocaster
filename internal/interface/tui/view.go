package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/interface/tui/components"
)

func (m Model) View() tea.View {
	content := m.renderContent()
	layout := m.theme.App.Render(lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		content,
		m.renderFooter(),
	))

	if m.state == stateAddPodcast {
		layout = components.RenderModal(
			m.theme,
			max(m.width, 80),
			max(m.height, 24),
			m.renderAddModal(),
		)
	}

	if m.state == stateGoToEpisode {
		layout = components.RenderModal(
			m.theme,
			max(m.width, 80),
			max(m.height, 24),
			m.renderGoToEpisodeModal(),
		)
	}

	if m.state == statePlayerSeek {
		layout = components.RenderModal(
			m.theme,
			max(m.width, 80),
			max(m.height, 24),
			m.renderPlayerSeekModal(),
		)
	}

	view := tea.NewView(layout)
	view.AltScreen = true
	view.WindowTitle = "Gocaster"
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeCellMotion
	view.ForegroundColor = m.theme.Text
	return view
}

func (m Model) renderHeader() string {
	width := m.contentWidth()
	tagline := m.theme.MutedText.Render("Editorial podcast library")
	count := "No podcasts"
	if items := len(m.list.Items()); items > 0 {
		count = fmt.Sprintf("%d podcasts", items)
	}
	badge := m.theme.Badge.Render(count)

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.theme.Header.Width(max(width-lipgloss.Width(badge)-2, 10)).Render("Gocaster"),
		tagline,
	)

	if width < lipgloss.Width(left)+lipgloss.Width(badge)+1 {
		return lipgloss.JoinVertical(lipgloss.Left, left, badge)
	}

	spacerWidth := max(width-lipgloss.Width(left)-lipgloss.Width(badge), 1)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		lipgloss.NewStyle().Width(spacerWidth).Render(""),
		badge,
	)
}

func (m Model) renderContent() string {
	if m.state == statePlayer || m.state == statePlayerSeek {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderPlayerPage())
	}

	if m.state == stateHelp {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderHelpPage())
	}

	if m.state == stateDownloads {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderDownloadsPage())
	}

	if m.state == stateSettings {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderSettingsPage())
	}

	if m.loadingLibrary && len(m.list.Items()) == 0 {
		content := components.RenderLoading(m.theme, m.spin.View(), "Loading library…")
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
	}

	left := m.renderPodcastPane()
	right := m.renderDetailPane()

	if m.shouldStackPanes() {
		content := lipgloss.JoinVertical(lipgloss.Left, left, right)
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
	}

	gap := " "
	leftWidth := lipgloss.Width(left)
	rightWidth := max(m.contentWidth()-leftWidth-lipgloss.Width(gap), 1)
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).MaxWidth(leftWidth).Render(left),
		gap,
		lipgloss.NewStyle().Width(rightWidth).MaxWidth(rightWidth).Render(right),
	)
	return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
}
