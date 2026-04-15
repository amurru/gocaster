package components

import (
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func RenderModal(theme styles.Theme, width, height int, content string) string {
	boxWidth := min(max(width-6, 24), min(max(width/2, 48), 76))
	boxHeight := min(max(height-4, 8), min(max(height/3, 10), 16))

	modal := theme.Modal.
		Width(boxWidth).
		Height(boxHeight).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
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
