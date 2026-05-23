package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func RenderLoading(theme styles.Theme, spinnerView, label string) string {
	return theme.Card.Render(fmt.Sprintf("%s %s", spinnerView, label))
}

func RenderProgressBar(theme styles.Theme, progress float64, width int) string {
	if width < 10 {
		width = 10
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	barWidth := width - 7
	if barWidth < 4 {
		barWidth = 4
	}

	filled := int((progress / 100) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled

	filledPart := lipgloss.NewStyle().Foreground(theme.Accent).Render(strings.Repeat("█", filled))
	emptyPart := lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", empty))

	return fmt.Sprintf("[%s%s] %3.0f%%", filledPart, emptyPart, progress)
}
