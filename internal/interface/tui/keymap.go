package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Add           key.Binding
	Submit        key.Binding
	Close         key.Binding
	SwitchPane    key.Binding
	ToggleHelp    key.Binding
	Quit          key.Binding
	PlayEpisode   key.Binding
	NextEpisode   key.Binding
	PrevEpisode   key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add feed"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open/submit"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
		SwitchPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		PlayEpisode: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "play episode"),
		),
		NextEpisode: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "next episode"),
		),
		PrevEpisode: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "prev episode"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Add, k.SwitchPane, k.PlayEpisode, k.NextEpisode, k.PrevEpisode, k.Close, k.ToggleHelp, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Add, k.SwitchPane, k.PlayEpisode, k.NextEpisode, k.PrevEpisode},
		{k.Submit, k.Close},
		{k.ToggleHelp, k.Quit},
	}
}
