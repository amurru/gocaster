package tui

import (
	"strings"
	"testing"
)

func TestAsciiLogoNarrowReturnsFallback(t *testing.T) {
	for _, width := range []int{0, 1, 20, 45} {
		got := asciiLogo(width)
		if !strings.Contains(got, "Gocaster - Podcast Client") {
			t.Fatalf("width %d: expected fallback text, got %q", width, got)
		}
	}
}

func TestAsciiLogoBoundaryReturnsArt(t *testing.T) {
	got := asciiLogo(46)
	if strings.Contains(got, "Gocaster - Podcast Client") {
		t.Fatalf("width 46: expected ASCII art, got fallback %q", got)
	}
	if !strings.Contains(got, "______") {
		t.Fatalf("width 46: expected ASCII art with distinctive fragment, got %q", got)
	}
}

func TestAsciiLogoWideReturnsArt(t *testing.T) {
	got := asciiLogo(120)
	if strings.Contains(got, "Gocaster - Podcast Client") {
		t.Fatalf("width 120: expected ASCII art, got fallback %q", got)
	}
	if !strings.Contains(got, "______") {
		t.Fatalf("width 120: expected ASCII art with distinctive fragment, got %q", got)
	}
}

func TestLogoHeightCountsLines(t *testing.T) {
	if got, want := logoHeight(120), 5; got != want {
		t.Fatalf("logoHeight(120) = %d, want %d", got, want)
	}
	if got, want := logoHeight(46), 5; got != want {
		t.Fatalf("logoHeight(46) = %d, want %d", got, want)
	}
	if got, want := logoHeight(20), 1; got != want {
		t.Fatalf("logoHeight(20) = %d, want %d (fallback is single line)", got, want)
	}
}
