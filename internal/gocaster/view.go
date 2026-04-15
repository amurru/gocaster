package gocaster

import tea "charm.land/bubbletea/v2"

func (m model) View() tea.View {
	// The header
	s := "Gocaster"
	return tea.NewView(s)
}
