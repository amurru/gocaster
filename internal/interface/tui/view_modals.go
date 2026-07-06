package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/interface/tui/components"
)

func (m Model) renderAddModal() string {
	inputStyle := m.theme.Input
	if m.input.Focused() {
		inputStyle = m.theme.InputFocused
	}

	body := []string{
		m.theme.SectionTitle.Render("Add Podcast"),
		m.theme.MutedText.Render(
			"Paste an RSS feed URL. Episodes will be fetched and stored immediately.",
		),
		m.theme.Label.Render("Feed URL"),
		inputStyle.Render(m.input.View()),
	}

	if m.submitting {
		body = append(body, components.RenderLoading(m.theme, m.spin.View(), "Importing feed…"))
	} else {
		body = append(body, m.theme.MutedText.Render("Enter to submit, Esc to cancel"))
	}

	return strings.Join(body, "\n\n")
}

func (m Model) renderGoToEpisodeModal() string {
	inputStyle := m.theme.Input
	if m.goToInput.Focused() {
		inputStyle = m.theme.InputFocused
	}

	body := []string{
		m.theme.SectionTitle.Render("Go to episode"),
		m.theme.MutedText.Render(
			fmt.Sprintf("Enter episode number (1-%d) to jump directly to it.", len(m.episodes)),
		),
		m.theme.Label.Render("Episode #"),
		inputStyle.Render(m.goToInput.View()),
		m.theme.MutedText.Render("Enter to go, Esc to cancel"),
	}

	return strings.Join(body, "\n\n")
}

func (m Model) renderPlayerSeekModal() string {
	inputStyle := m.theme.Input
	if m.seekInput.Focused() {
		inputStyle = m.theme.InputFocused
	}

	body := []string{
		m.theme.SectionTitle.Render("Seek to time"),
		m.theme.MutedText.Render("Enter a target time like 0:30, 12:45, or 1:02:03."),
		m.theme.Label.Render("Time"),
		inputStyle.Render(m.seekInput.View()),
		m.theme.MutedText.Render("Enter to seek, Esc to cancel"),
	}

	return strings.Join(body, "\n\n")
}

func (m Model) renderFooter() string {
	if m.state == stateHelp {
		status := lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.StatusStyle(m.kind).Render(m.status),
		)
		hint := m.theme.HelpText.Render(
			"Press ? or Esc to return. Use arrow keys, j/k, PgUp/PgDn, or the mouse wheel to scroll.",
		)
		return lipgloss.JoinVertical(lipgloss.Left, status, hint)
	}

	status := lipgloss.JoinHorizontal(lipgloss.Left,
		m.theme.StatusStyle(m.kind).Render(m.status),
	)

	shortcuts := m.keys.FooterShortcuts(string(m.state), string(m.focus))
	helpView := m.theme.HelpText.Render(m.help.ShortHelpView(shortcuts))
	overflowHint := m.theme.HelpText.Render(" · ? for all")
	return lipgloss.JoinVertical(lipgloss.Left, status, helpView+overflowHint)
}

func (m Model) renderThemeSelector() string {
	current := m.themeList[m.selectedThemeIndex]
	return current
}
