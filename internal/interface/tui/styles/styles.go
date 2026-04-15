package styles

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Background   color.Color
	Surface      color.Color
	SurfaceAlt   color.Color
	Border       color.Color
	Accent       color.Color
	AccentSoft   color.Color
	Text         color.Color
	Muted        color.Color
	Success      color.Color
	Danger       color.Color
	Warning      color.Color
	App          lipgloss.Style
	Header       lipgloss.Style
	SectionTitle lipgloss.Style
	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
	Card         lipgloss.Style
	Label        lipgloss.Style
	Body         lipgloss.Style
	MutedText    lipgloss.Style
	StatusBar    lipgloss.Style
	HelpText     lipgloss.Style
	Badge        lipgloss.Style
	Input        lipgloss.Style
	InputFocused lipgloss.Style
	Modal        lipgloss.Style
	Divider      lipgloss.Style
}

func NewTheme() Theme {
	background := lipgloss.Color("#111315")
	surface := lipgloss.Color("#1A1D1F")
	surfaceAlt := lipgloss.Color("#232729")
	border := lipgloss.Color("#3A403F")
	accent := lipgloss.Color("#E07A5F")
	accentSoft := lipgloss.Color("#F2CC8F")
	text := lipgloss.Color("#F4F1EA")
	muted := lipgloss.Color("#B8B0A2")
	success := lipgloss.Color("#81B29A")
	danger := lipgloss.Color("#E76F51")
	warning := lipgloss.Color("#F2CC8F")

	return Theme{
		Background: background,
		Surface:    surface,
		SurfaceAlt: surfaceAlt,
		Border:     border,
		Accent:     accent,
		AccentSoft: accentSoft,
		Text:       text,
		Muted:      muted,
		Success:    success,
		Danger:     danger,
		Warning:    warning,
		App: lipgloss.NewStyle().
			Foreground(text).
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Foreground(text).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(border).
			Bold(true),
		SectionTitle: lipgloss.NewStyle().
			Foreground(accentSoft).
			Bold(true),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1),
		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1),
		Card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1),
		Label: lipgloss.NewStyle().
			Foreground(accentSoft).
			Bold(true),
		Body: lipgloss.NewStyle().
			Foreground(text),
		MutedText: lipgloss.NewStyle().
			Foreground(muted),
		StatusBar: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(border).
			Padding(0, 1),
		HelpText: lipgloss.NewStyle().
			Foreground(muted),
		Badge: lipgloss.NewStyle().
			Foreground(accentSoft).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentSoft).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1),
		InputFocused: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		Modal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(accent).
			Padding(1, 2),
		Divider: lipgloss.NewStyle().
			Foreground(border),
	}
}

func (t Theme) StatusStyle(kind string) lipgloss.Style {
	style := lipgloss.NewStyle().
		Foreground(t.Text).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)

	switch kind {
	case "success":
		return style.BorderForeground(t.Success).Foreground(t.Success)
	case "error":
		return style.BorderForeground(t.Danger).Foreground(t.Danger)
	case "warning":
		return style.BorderForeground(t.Warning).Foreground(t.Warning)
	default:
		return style
	}
}
