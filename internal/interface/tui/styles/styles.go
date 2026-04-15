package styles

import "charm.land/lipgloss/v2"

// Base Colors
var (
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#3C3C3C")
	TextColor      = lipgloss.Color("#FAFAFA")
)

// Component Styles
var (
	Title = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(PrimaryColor).
		Padding(0, 1).
		Bold(true)

	StatusBar = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SecondaryColor).
			Padding(0, 1)
)
