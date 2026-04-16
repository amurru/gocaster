package components

import (
	"strings"

	"charm.land/bubbles/v2/list"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func NewPodcastDelegate(theme styles.Theme) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)
	delegate.SetSpacing(1)

	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(theme.Text)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(theme.Muted)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.AccentSoft).
		BorderForeground(theme.Accent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.Muted).
		BorderForeground(theme.Accent)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.
		Foreground(theme.Muted)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.
		Foreground(theme.Border)
	delegate.Styles.FilterMatch = delegate.Styles.FilterMatch.
		Underline(true).
		Foreground(theme.Accent)

	return delegate
}

func NewEpisodeDelegate(theme styles.Theme) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(3)
	delegate.SetSpacing(1)

	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(theme.Text)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(theme.Muted)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.AccentSoft).
		BorderLeft(true).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.AccentSoft).
		BorderLeft(true).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.
		Foreground(theme.Muted)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.
		Foreground(theme.Border)
	delegate.Styles.FilterMatch = delegate.Styles.FilterMatch.
		Underline(true).
		Foreground(theme.Accent)

	return delegate
}

func NewDownloadJobDelegate(theme styles.Theme) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)
	delegate.SetSpacing(1)

	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(theme.Text)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(theme.Muted)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.AccentSoft).
		BorderLeft(true).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.AccentSoft).
		BorderLeft(true).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.
		Foreground(theme.Muted)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.
		Foreground(theme.Border)
	delegate.Styles.FilterMatch = delegate.Styles.FilterMatch.
		Underline(true).
		Foreground(theme.Accent)

	return delegate
}

func TruncateDescription(value string, maxLen int) string {
	text := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 1 {
		return text[:maxLen]
	}
	return text[:maxLen-1] + "…"
}
