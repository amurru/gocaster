package tui

import "strings"

// asciiLogo returns the Gocaster ASCII art logo. The width parameter controls
// how the logo is rendered: for very narrow terminals a plain text fallback
// is used.
func asciiLogo(width int) string {
	if width < 46 {
		return "Gocaster - Podcast Client"
	}

	const art = "   ______                      __\n  / ____/___  _________ ______/ /____  _____\n / / __/ __ \\/ ___/ __ `/ ___/ __/ _ \\/___/\n/ /_/ / /_/ / /__/ /_/ (__  ) /_/  __/ /\n\\____/\\____/\\___/\\__,_/____/\\__/\\___/_/"

	return art
}

// logoHeight returns the number of lines in the rendered logo.
func logoHeight(width int) int {
	return strings.Count(asciiLogo(width), "\n") + 1
}
