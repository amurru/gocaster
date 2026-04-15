package components

import (
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func RenderModal(theme styles.Theme, width, height int, content string) string {
	boxWidth := min(max(width/2+8, 48), 85)
	boxHeight := min(max(height-16, 8), height-14)

	modal := theme.Modal.
		Width(boxWidth).
		Height(boxHeight).
		Render(content)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
