package tui

import (
	"context"
	"strconv"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/interface/tui/components"
	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

type viewState string
type paneFocus string

const (
	stateBrowse      viewState = "browse"
	stateAddPodcast  viewState = "add_podcast"
	stateGoToEpisode viewState = "go_to_episode"
	statePlayer      viewState = "player"
	statePlayerSeek  viewState = "player_seek"
	stateHelp        viewState = "help"
	stateDownloads   viewState = "downloads"
	stateSettings         viewState = "settings"
	statePlaybackQueue    viewState = "playback_queue"

	focusLibrary paneFocus = "library"
	focusDetail  paneFocus = "detail"
	focusQueue   paneFocus = "queue"
)

type episodeSortOrder string

const (
	sortNewestFirst episodeSortOrder = "newest"
	sortOldestFirst episodeSortOrder = "oldest"
)

// Messages represent events coming back to the UI.
type errMsg struct {
	err error
}

type podcastsLoadedMsg struct {
	podcasts []domain.Podcast
	err      error
}

type podcastAddedMsg struct {
	podcast *domain.Podcast
	err     error
}

type episodesLoadedMsg struct {
	podcastID int64
	episodes  []domain.Episode
	err       error
}

type podcastRefreshedMsg struct {
	podcastID int64
	newCount  int
	err       error
}

type downloadJobsLoadedMsg struct {
	jobs []application.DownloadJobView
	err  error
}

type downloadQueuedMsg struct {
	episodeID int64
	err       error
}

type downloadStartedMsg struct {
	jobID int64
	err   error
}

type downloadRetriedMsg struct {
	jobID int64
	err   error
}

type episodePlayedMsg struct {
	episodeID int64
	err       error
}

type playbackStatusMsg struct {
	status domain.PlaybackStatus
	err    error
}

type playbackToggledMsg struct {
	err error
}

type playbackSkippedMsg struct {
	seconds float64
	err     error
}

type playbackSeekedMsg struct {
	positionSec float64
	err         error
}

type playbackSpeedChangedMsg struct {
	speed float64
	err   error
}

type allPodcastsSyncedMsg struct {
	result application.RefreshAllResult
	err    error
	reason string
}

type settingsPersistedMsg struct {
	settings Settings
	previous Settings
	err      error
}

type playbackQueueLoadedMsg struct {
	views []application.QueueItemView
	err   error
}

type queueItemAddedMsg struct {
	err error
}

type queueItemRemovedMsg struct {
	err error
}

type queueTrackChangedMsg struct {
	err error
}

type repeatModeToggledMsg struct {
	err error
}

type shuffleToggledMsg struct {
	err error
}

type Settings struct {
	AutoSyncOnStartup bool
	PeriodicSync      bool
	PeriodicSyncMins  int
	DiscordPresence   bool
	DiscordClientID   string
	ThemeName         string
}

type Model struct {
	podcastService   *application.PodcastService
	downloadService  *application.DownloadService
	playerService    *application.PlayerService
	queueService     *application.QueueService
	// ctx is the application root context. It is cancelled on shutdown so every
	// in-flight service call (and the downloads it parents) observes
	// cancellation (issue #11). tea.Cmd closures capture it via the Model value
	// they close over.
	ctx context.Context

	state         viewState
	keys          keyMap
	theme         styles.Theme
	help          help.Model
	list          list.Model
	epList        list.Model
	detail        viewport.Model
	playerNotes   viewport.Model
	guide         viewport.Model
	input         textinput.Model
	goToInput     textinput.Model
	seekInput     textinput.Model
	intervalInput textinput.Model
	discordInput  textinput.Model
	spin          spinner.Model
	status        string
	kind          string

	width  int
	height int

	bodyHeight   int
	listWidth    int
	detailWidth  int
	detailHeight int

	loadingLibrary bool
	loadingDetail  bool
	submitting     bool

	focus           paneFocus
	selectedPodcast *domain.Podcast
	episodes        []domain.Episode
	selectedEpisode *domain.Episode
	sortOrder       episodeSortOrder
	previousState   viewState

	downloadJobs    []application.DownloadJobView
	queueList       list.Model
	playbackQueueList list.Model
	queueViews        []application.QueueItemView

	playbackStatus     domain.PlaybackStatus
	currentPodcast     *domain.Podcast
	currentEpisode     *domain.Episode
	settings           Settings
	saveSettings       func(Settings) error
	settingsCursor     int
	editingInterval    bool
	editingDiscordID   bool
	editingTheme       bool
	themeList          []string
	selectedThemeIndex int
	customThemesDir    string
	syncingAllFeeds    bool
	nextPeriodicSyncAt time.Time
}

func NewModel(
	ctx context.Context,
	svc *application.PodcastService,
	dsvc *application.DownloadService,
	psvc *application.PlayerService,
	qsvc *application.QueueService,
	settings Settings,
	saveSettings func(Settings) error,
	customThemesDir string,
) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	if settings.PeriodicSyncMins <= 0 {
		settings.PeriodicSyncMins = 60
	}
	theme := styles.LoadTheme(settings.ThemeName, customThemesDir)
	delegate := components.NewPodcastDelegate(theme)
	episodeDelegate := components.NewEpisodeDelegate(theme)
	downloadJobDelegate := components.NewDownloadJobDelegate(theme)

	podcastList := list.New([]list.Item{}, delegate, 0, 0)
	podcastList = configureListStyles(podcastList, theme)
	podcastList.Title = "Podcasts"
	podcastList.SetStatusBarItemName("podcast", "podcasts")
	podcastList.Styles.Title = theme.Header

	episodeList := list.New([]list.Item{}, episodeDelegate, 0, 0)
	episodeList = configureListStyles(episodeList, theme)
	episodeList.Title = "Episodes"
	episodeList.SetStatusBarItemName("episode", "episodes")

	detailViewport := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	detailViewport.SoftWrap = true
	detailViewport.FillHeight = true
	detailViewport.MouseWheelEnabled = true
	detailViewport.MouseWheelDelta = 2

	playerNotesViewport := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	playerNotesViewport.SoftWrap = true
	playerNotesViewport.FillHeight = true
	playerNotesViewport.MouseWheelEnabled = true
	playerNotesViewport.MouseWheelDelta = 2

	guideViewport := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	guideViewport.SoftWrap = true
	guideViewport.FillHeight = true
	guideViewport.MouseWheelEnabled = true
	guideViewport.MouseWheelDelta = 2

	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "https://example.com/feed.xml"
	input.CharLimit = 512
	input.SetVirtualCursor(true)
	input.SetWidth(56)

	goToInput := textinput.New()
	goToInput.Prompt = ""
	goToInput.Placeholder = "episode number"
	goToInput.CharLimit = 6
	goToInput.SetVirtualCursor(true)
	goToInput.SetWidth(20)

	seekInput := textinput.New()
	seekInput.Prompt = ""
	seekInput.Placeholder = "mm:ss or hh:mm:ss"
	seekInput.CharLimit = 12
	seekInput.SetVirtualCursor(true)
	seekInput.SetWidth(20)

	intervalInput := textinput.New()
	intervalInput.Prompt = ""
	intervalInput.Placeholder = "60"
	intervalInput.CharLimit = 4
	intervalInput.SetVirtualCursor(true)
	intervalInput.SetWidth(8)
	intervalInput.SetValue(strconv.Itoa(settings.PeriodicSyncMins))

	discordInput := textinput.New()
	discordInput.Prompt = ""
	discordInput.Placeholder = "Discord Application Client ID"
	discordInput.CharLimit = 64
	discordInput.SetVirtualCursor(true)
	discordInput.SetWidth(36)
	discordInput.SetValue(settings.DiscordClientID)

	spin := spinner.New(spinner.WithSpinner(spinner.Line))
	spin.Style = lipgloss.NewStyle().Foreground(theme.Accent)

	helpModel := help.New()
	helpModel.ShowAll = false
	helpModel.Styles.ShortKey = theme.HelpText
	helpModel.Styles.ShortDesc = theme.HelpText
	helpModel.Styles.FullKey = theme.HelpText
	helpModel.Styles.FullDesc = theme.HelpText

	downloadQueueList := list.New([]list.Item{}, downloadJobDelegate, 0, 0)
	downloadQueueList = configureListStyles(downloadQueueList, theme)
	downloadQueueList.Title = "Downloads"
	downloadQueueList.SetStatusBarItemName("download", "downloads")

	pbqDelegate := components.NewPlaybackQueueDelegate(theme)
	playbackQueueList := list.New([]list.Item{}, pbqDelegate, 0, 0)
	playbackQueueList = configureListStyles(playbackQueueList, theme)
	playbackQueueList.Title = "Queue"
	playbackQueueList.SetStatusBarItemName("item", "items")

	return Model{
		podcastService:  svc,
		downloadService: dsvc,
		playerService:   psvc,
		queueService:    qsvc,
		ctx:             ctx,
		state:           stateBrowse,
		keys:            defaultKeyMap(),
		theme:           theme,
		help:            helpModel,
		list:            podcastList,
		epList:          episodeList,
		queueList:          downloadQueueList,
		playbackQueueList:  playbackQueueList,
		detail:             detailViewport,
		guide:           guideViewport,
		playerNotes:     playerNotesViewport,
		input:           input,
		goToInput:       goToInput,
		seekInput:       seekInput,
		intervalInput:   intervalInput,
		discordInput:    discordInput,
		spin:            spin,
		status:          "Ready",
		kind:            "info",
		loadingLibrary:  true,
		focus:           focusLibrary,
		selectedPodcast: nil,
		episodes:        nil,
		selectedEpisode: nil,
		sortOrder:       sortNewestFirst,
		settings:        settings,
		saveSettings:    saveSettings,
		customThemesDir: customThemesDir,
		themeList: func() []string {
			themes := styles.GetAllThemes(customThemesDir)
			return themes
		}(),
		selectedThemeIndex: func() int {
			themes := styles.GetAllThemes(customThemesDir)
			return findThemeIndex(settings.ThemeName, themes)
		}(),
		syncingAllFeeds: settings.AutoSyncOnStartup,
		nextPeriodicSyncAt: func() time.Time {
			if settings.PeriodicSync {
				return time.Now().Add(time.Duration(settings.PeriodicSyncMins) * time.Minute)
			}
			return time.Time{}
		}(),
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadPodcasts(), m.spin.Tick, tickCmd()}
	if m.settings.AutoSyncOnStartup {
		cmds = append(cmds, m.syncAllPodcasts("startup"))
	}
	return tea.Batch(cmds...)
}

// tickCmd returns a command that ticks every second for badge flashing.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// tickMsg is a message that fires every second for UI updates like badge flashing.
type tickMsg struct{}

// configureListStyles applies the shared list presentation that every list in
// the app uses: no title/help/status bar chrome, disabled quit keybinding, and
// a consistent filter/placeholder/no-items/status-bar/pagination palette drawn
// from theme. Extracted so NewModel does not repeat the same ~14 lines for each
// of the podcast, episode, and download-queue lists. Per-list differences
// (Title, SetStatusBarItemName, and the podcast list's highlighted header) are
// set at the call site after this returns.
func configureListStyles(l list.Model, theme styles.Theme) list.Model {
	l.DisableQuitKeybindings()
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.Styles.Filter.Focused.Prompt = theme.Label
	l.Styles.Filter.Blurred.Prompt = theme.MutedText
	l.Styles.Filter.Focused.Text = theme.Body
	l.Styles.Filter.Blurred.Text = theme.Body
	l.Styles.Filter.Focused.Placeholder = theme.MutedText
	l.Styles.Filter.Blurred.Placeholder = theme.MutedText
	l.Styles.NoItems = theme.MutedText
	l.Styles.StatusBar = theme.MutedText
	l.Styles.PaginationStyle = theme.MutedText
	return l
}
