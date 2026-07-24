package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Add               key.Binding
	Submit            key.Binding
	Close             key.Binding
	SwitchPane        key.Binding
	ToggleHelp        key.Binding
	Quit              key.Binding
	OpenPlayer        key.Binding
	TogglePlayPause   key.Binding
	SkipBackward      key.Binding
	SkipForward       key.Binding
	SeekToTime        key.Binding
	PlayEpisode       key.Binding
	NextEpisode       key.Binding
	PrevEpisode       key.Binding
	RefreshPodcast    key.Binding
	Filter            key.Binding
	GoToEpisode       key.Binding
	ToggleEpisodeSort key.Binding
	DownloadQueue     key.Binding
	DownloadEpisode   key.Binding
	StartDownload     key.Binding
	RetryDownload     key.Binding
	Settings          key.Binding
	SpeedDown         key.Binding
	SpeedUp           key.Binding
	AddToQueue        key.Binding
	AddPlayNext       key.Binding
	OpenQueue         key.Binding
	OpenShownotes     key.Binding
	NextTrack         key.Binding
	PrevTrack         key.Binding
	CycleRepeat       key.Binding
	ToggleShuffle     key.Binding
	DeleteQueueItem   key.Binding
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
		OpenPlayer: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "player"),
		),
		TogglePlayPause: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "play/pause"),
		),
		SkipBackward: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "skip back 15s"),
		),
		SkipForward: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "skip forward 15s"),
		),
		SeekToTime: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "seek to time"),
		),
		PlayEpisode: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter/space", "play"),
		),
		NextEpisode: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "next episode"),
		),
		PrevEpisode: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "prev episode"),
		),
		RefreshPodcast: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		GoToEpisode: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to"),
		),
		ToggleEpisodeSort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort episodes"),
		),
		DownloadQueue: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "downloads"),
		),
		DownloadEpisode: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "download episode"),
		),
		StartDownload: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start"),
		),
		RetryDownload: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
		Settings: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "settings"),
		),
		SpeedDown: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "slower"),
		),
		SpeedUp: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "faster"),
		),
		AddToQueue: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "add to queue"),
		),
		AddPlayNext: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "play next"),
		),
		OpenQueue: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "queue"),
		),
		OpenShownotes: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "shownotes"),
		),
		NextTrack: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next track"),
		),
		PrevTrack: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev track"),
		),
		CycleRepeat: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "repeat"),
		),
		ToggleShuffle: key.NewBinding(
			key.WithKeys("#"),
			key.WithHelp("#", "shuffle"),
		),
		DeleteQueueItem: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "remove"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Add,
		k.RefreshPodcast,
		k.GoToEpisode,
		k.ToggleEpisodeSort,
		k.DownloadQueue,
		k.Settings,
		k.SwitchPane,
		k.PlayEpisode,
		k.NextEpisode,
		k.PrevEpisode,
		k.Filter,
		k.Close,
		k.ToggleHelp,
		k.Quit,
	}
}

func (k keyMap) FooterShortcuts(state string, focus string) []key.Binding {
	switch state {
	case "downloads":
		return []key.Binding{k.Close, k.StartDownload, k.RetryDownload, k.ToggleHelp, k.Quit}
	case "help":
		return []key.Binding{k.Close, k.ToggleHelp}
	case "player":
		return []key.Binding{k.Close, k.TogglePlayPause, k.SkipBackward, k.SkipForward, k.SeekToTime, k.SpeedDown, k.SpeedUp, k.ToggleHelp, k.Quit}
	case "player_seek":
		return []key.Binding{k.Close, k.Submit, k.ToggleHelp, k.Quit}
	case "settings":
		return []key.Binding{k.Close, k.Submit, k.Settings, k.ToggleHelp, k.Quit}
	case "playback_queue":
		return []key.Binding{k.Close, k.NextTrack, k.PrevTrack, k.CycleRepeat, k.ToggleShuffle, k.DeleteQueueItem, k.ToggleHelp, k.Quit}
	case "shownotes":
		return []key.Binding{k.Close, k.ToggleHelp, k.Quit}
	default:
		if focus == "detail" {
			return []key.Binding{
				k.SwitchPane,
				k.ToggleEpisodeSort,
				k.DownloadEpisode,
				k.GoToEpisode,
				k.OpenPlayer,
				k.OpenShownotes,
				k.PlayEpisode,
				k.Filter,
				k.ToggleHelp,
				k.Quit,
			}
		}
		return []key.Binding{
			k.Add,
			k.RefreshPodcast,
			k.SwitchPane,
			k.DownloadQueue,
			k.Settings,
			k.OpenPlayer,
			k.Filter,
			k.ToggleHelp,
			k.Quit,
		}
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.Add,
			k.RefreshPodcast,
			k.GoToEpisode,
			k.ToggleEpisodeSort,
			k.DownloadQueue,
			k.Settings,
			k.SwitchPane,
			k.OpenPlayer,
			k.PlayEpisode,
			k.NextEpisode,
			k.PrevEpisode,
			k.Filter,
		},
		{k.Submit, k.Close},
		{k.ToggleHelp, k.Quit},
	}
}
