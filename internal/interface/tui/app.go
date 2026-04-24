package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
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
	stateHelp        viewState = "help"
	stateDownloads   viewState = "downloads"
	stateSettings    viewState = "settings"

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
	jobs []domain.DownloadJob
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

type Settings struct {
	AutoSyncOnStartup bool
	PeriodicSync      bool
	PeriodicSyncMins  int
	DiscordPresence   bool
	DiscordClientID   string
}

type Model struct {
	podcastService  *application.PodcastService
	downloadService *application.DownloadService
	playerService   *application.PlayerService

	state         viewState
	keys          keyMap
	theme         styles.Theme
	help          help.Model
	list          list.Model
	epList        list.Model
	detail        viewport.Model
	guide         viewport.Model
	input         textinput.Model
	goToInput     textinput.Model
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

	downloadJobs []domain.DownloadJob
	queueList    list.Model

	playbackStatus     domain.PlaybackStatus
	settings           Settings
	saveSettings       func(Settings) error
	settingsCursor     int
	editingInterval    bool
	editingDiscordID   bool
	syncingAllFeeds    bool
	nextPeriodicSyncAt time.Time
}

func NewModel(
	svc *application.PodcastService,
	dsvc *application.DownloadService,
	psvc *application.PlayerService,
	settings Settings,
	saveSettings func(Settings) error,
) Model {
	if settings.PeriodicSyncMins <= 0 {
		settings.PeriodicSyncMins = 60
	}
	theme := styles.NewTheme()
	delegate := components.NewPodcastDelegate(theme)
	episodeDelegate := components.NewEpisodeDelegate(theme)
	downloadJobDelegate := components.NewDownloadJobDelegate(theme)

	podcastList := list.New([]list.Item{}, delegate, 0, 0)
	podcastList.Title = "Podcasts"
	podcastList.DisableQuitKeybindings()
	podcastList.SetShowTitle(false)
	podcastList.SetShowHelp(false)
	podcastList.SetShowStatusBar(false)
	podcastList.SetStatusBarItemName("podcast", "podcasts")
	podcastList.Styles.Title = theme.Header
	podcastList.Styles.Filter.Focused.Prompt = theme.Label
	podcastList.Styles.Filter.Blurred.Prompt = theme.MutedText
	podcastList.Styles.Filter.Focused.Text = theme.Body
	podcastList.Styles.Filter.Blurred.Text = theme.Body
	podcastList.Styles.Filter.Focused.Placeholder = theme.MutedText
	podcastList.Styles.Filter.Blurred.Placeholder = theme.MutedText
	podcastList.Styles.NoItems = theme.MutedText
	podcastList.Styles.StatusBar = theme.MutedText
	podcastList.Styles.PaginationStyle = theme.MutedText

	episodeList := list.New([]list.Item{}, episodeDelegate, 0, 0)
	episodeList.Title = "Episodes"
	episodeList.DisableQuitKeybindings()
	episodeList.SetShowTitle(false)
	episodeList.SetShowHelp(false)
	episodeList.SetShowStatusBar(false)
	episodeList.SetStatusBarItemName("episode", "episodes")
	episodeList.Styles.Filter.Focused.Prompt = theme.Label
	episodeList.Styles.Filter.Blurred.Prompt = theme.MutedText
	episodeList.Styles.Filter.Focused.Text = theme.Body
	episodeList.Styles.Filter.Blurred.Text = theme.Body
	episodeList.Styles.Filter.Focused.Placeholder = theme.MutedText
	episodeList.Styles.Filter.Blurred.Placeholder = theme.MutedText
	episodeList.Styles.NoItems = theme.MutedText
	episodeList.Styles.StatusBar = theme.MutedText
	episodeList.Styles.PaginationStyle = theme.MutedText

	detailViewport := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	detailViewport.SoftWrap = true
	detailViewport.FillHeight = true
	detailViewport.MouseWheelEnabled = true
	detailViewport.MouseWheelDelta = 2

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
	downloadQueueList.Title = "Downloads"
	downloadQueueList.DisableQuitKeybindings()
	downloadQueueList.SetShowTitle(false)
	downloadQueueList.SetShowHelp(false)
	downloadQueueList.SetShowStatusBar(false)
	downloadQueueList.SetStatusBarItemName("download", "downloads")
	downloadQueueList.Styles.Filter.Focused.Prompt = theme.Label
	downloadQueueList.Styles.Filter.Blurred.Prompt = theme.MutedText
	downloadQueueList.Styles.Filter.Focused.Text = theme.Body
	downloadQueueList.Styles.Filter.Blurred.Text = theme.Body
	downloadQueueList.Styles.Filter.Focused.Placeholder = theme.MutedText
	downloadQueueList.Styles.Filter.Blurred.Placeholder = theme.MutedText
	downloadQueueList.Styles.NoItems = theme.MutedText
	downloadQueueList.Styles.StatusBar = theme.MutedText
	downloadQueueList.Styles.PaginationStyle = theme.MutedText

	return Model{
		podcastService:  svc,
		downloadService: dsvc,
		playerService:   psvc,
		state:           stateBrowse,
		keys:            defaultKeyMap(),
		theme:           theme,
		help:            helpModel,
		list:            podcastList,
		epList:          episodeList,
		queueList:       downloadQueueList,
		detail:          detailViewport,
		guide:           guideViewport,
		input:           input,
		goToInput:       goToInput,
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

func (m Model) loadPodcasts() tea.Cmd {
	return func() tea.Msg {
		podcasts, err := m.podcastService.ListPodcasts()
		return podcastsLoadedMsg{podcasts: podcasts, err: err}
	}
}

func (m Model) loadEpisodes(podcastID int64) tea.Cmd {
	return func() tea.Msg {
		episodes, err := m.podcastService.ListEpisodes(podcastID)
		return episodesLoadedMsg{podcastID: podcastID, episodes: episodes, err: err}
	}
}

func (m Model) addPodcast(url string) tea.Cmd {
	return func() tea.Msg {
		podcast, err := m.podcastService.AddPodcast(url)
		return podcastAddedMsg{podcast: podcast, err: err}
	}
}

func (m Model) refreshPodcast(podcastID int64) tea.Cmd {
	return func() tea.Msg {
		newCount, err := m.podcastService.RefreshPodcast(podcastID)
		return podcastRefreshedMsg{podcastID: podcastID, newCount: newCount, err: err}
	}
}

func (m Model) syncAllPodcasts(reason string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.podcastService.RefreshAllPodcasts()
		return allPodcastsSyncedMsg{result: result, err: err, reason: reason}
	}
}

func (m Model) persistSettings(next Settings, previous Settings) tea.Cmd {
	return func() tea.Msg {
		if m.saveSettings == nil {
			return settingsPersistedMsg{settings: next, previous: previous}
		}
		err := m.saveSettings(next)
		return settingsPersistedMsg{settings: next, previous: previous, err: err}
	}
}

func (m Model) loadDownloadJobs() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.downloadService.ListJobs()
		return downloadJobsLoadedMsg{jobs: jobs, err: err}
	}
}

func (m Model) queueDownload(episodeID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.QueueEpisodeDownload(episodeID)
		return downloadQueuedMsg{episodeID: episodeID, err: err}
	}
}

func (m Model) startDownload(jobID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.StartJob(jobID)
		return downloadStartedMsg{jobID: jobID, err: err}
	}
}

func (m Model) retryDownload(jobID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.downloadService.RetryJob(jobID)
		return downloadRetriedMsg{jobID: jobID, err: err}
	}
}

func (m Model) playEpisode(episodeID int64) tea.Cmd {
	return func() tea.Msg {
		err := m.playerService.PlayEpisode(episodeID)
		return episodePlayedMsg{episodeID: episodeID, err: err}
	}
}

func (m Model) fetchPlaybackStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := m.playerService.PlaybackStatus()
		return playbackStatusMsg{status: status, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isBusy() {
		var spinCmd tea.Cmd
		m.spin, spinCmd = m.spin.Update(msg)
		if spinCmd != nil {
			cmds = append(cmds, spinCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, tea.Batch(cmds...)

	case tickMsg:
		// Keep the ticker running for badge flashing
		cmds = append(cmds, tickCmd())

		// Update playback status periodically
		if m.playerService != nil {
			cmds = append(cmds, m.fetchPlaybackStatus())
		}

		// Update episode list items with flash tick (skip if filtering is active)
		if len(m.episodes) > 0 && m.epList.FilterState() == list.Unfiltered {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(m.episodes))
			for i, episode := range m.episodes {
				items[i] = EpisodeItem{Episode: episode}.WithTheme(m.theme).WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.epList.SetItems(items))
		}

		// Update download queue progress (skip if filtering is active)
		if m.state == stateDownloads && len(m.downloadJobs) > 0 &&
			m.queueList.FilterState() == list.Unfiltered {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(m.downloadJobs))
			for i, job := range m.downloadJobs {
				items[i] = DownloadJobItem{
					DownloadJob: job,
				}.WithTheme(m.theme).
					WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.queueList.SetItems(items))
		}

		// Reload download jobs when in downloads view to get fresh progress
		if m.state == stateDownloads {
			cmds = append(cmds, m.loadDownloadJobs())
		}

		if m.settings.PeriodicSync &&
			!m.syncingAllFeeds &&
			!m.nextPeriodicSyncAt.IsZero() &&
			!time.Now().Before(m.nextPeriodicSyncAt) {
			m.syncingAllFeeds = true
			m.setStatus("Periodic sync started…", "info")
			cmds = append(cmds, m.syncAllPodcasts("periodic"), m.spin.Tick)
		}

		return m, tea.Batch(cmds...)
	case tea.PasteMsg:
		// Pass the paste directly to the input component
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			if m.state == stateHelp {
				m.state = stateBrowse
				m.setStatus("Returned to library.", "info")
			} else {
				m.state = stateHelp
				m.syncGuideViewport(true)
				m.setStatus("Help page opened.", "info")
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Settings):
			if m.state == stateSettings {
				m.state = stateBrowse
				m.editingInterval = false
				m.editingDiscordID = false
				m.intervalInput.Blur()
				m.discordInput.Blur()
				m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
				m.discordInput.SetValue(m.settings.DiscordClientID)
				m.setStatus("Returned to library.", "info")
			} else if m.state == stateBrowse {
				m.openSettingsPage()
				m.setStatus("Settings page opened.", "info")
			}
			return m, tea.Batch(cmds...)
		}

		if m.state == stateHelp {
			if key.Matches(msg, m.keys.Close) {
				m.state = stateBrowse
				m.setStatus("Returned to library.", "info")
				return m, tea.Batch(cmds...)
			}

			var guideCmd tea.Cmd
			m.guide, guideCmd = m.guide.Update(msg)
			if guideCmd != nil {
				cmds = append(cmds, guideCmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.state == stateAddPodcast {
			return m.handleAddMode(msg, cmds)
		}

		if m.state == stateGoToEpisode {
			return m.handleGoToEpisodeMode(msg, cmds)
		}

		if m.state == stateDownloads {
			return m.handleDownloadsMode(msg, cmds)
		}

		if m.state == stateSettings {
			return m.handleSettingsMode(msg, cmds)
		}

		isFiltering := m.list.FilterState() == list.Filtering ||
			m.epList.FilterState() == list.Filtering

		if key.Matches(msg, m.keys.Add) && !isFiltering {
			m.openAddModal()
			cmds = append(cmds, m.input.Focus())
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.SwitchPane) && !isFiltering {
			m.toggleFocus()
			m.syncDetailViewport(false)
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.RefreshPodcast) && !isFiltering {
			if m.selectedPodcast == nil {
				m.setStatus("No podcast selected to refresh", "warning")
				return m, tea.Batch(cmds...)
			}
			m.loadingDetail = true
			m.setStatus("Refreshing feed…", "info")
			cmds = append(cmds, m.refreshPodcast(m.selectedPodcast.ID), m.spin.Tick)
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.DownloadQueue) && !isFiltering {
			m.openDownloadsQueue()
			cmds = append(cmds, m.loadDownloadJobs(), m.spin.Tick)
			return m, tea.Batch(cmds...)
		}

		if m.focus == focusDetail && key.Matches(msg, m.keys.DownloadEpisode) && !isFiltering {
			if m.selectedEpisode != nil {
				cmds = append(cmds, m.queueDownload(m.selectedEpisode.ID))
			}
			return m, tea.Batch(cmds...)
		}

		if key.Matches(msg, m.keys.ToggleEpisodeSort) && !isFiltering && len(m.episodes) > 0 {
			m.toggleEpisodeSort()
			m.syncDetailViewport(false)
			return m, tea.Batch(cmds...)
		}

	case podcastsLoadedMsg:
		m.loadingLibrary = false
		if msg.err != nil {
			m.setStatus("Failed to load podcasts", "error")
			return m, tea.Batch(cmds...)
		}

		items := make([]list.Item, len(msg.podcasts))
		for i, podcast := range msg.podcasts {
			items[i] = PodcastItem{Podcast: podcast}
		}
		cmds = append(cmds, m.list.SetItems(items))

		if len(msg.podcasts) == 0 {
			m.selectedPodcast = nil
			m.episodes = nil
			m.syncDetailViewport(true)
			m.setStatus("Library is empty. Press 'a' to add your first feed.", "info")
			return m, tea.Batch(cmds...)
		}

		if m.selectedPodcast != nil {
			for i, podcast := range msg.podcasts {
				if podcast.ID == m.selectedPodcast.ID {
					m.list.Select(i)
					break
				}
			}
		}

		selected := selectedPodcastItem(m.list)
		if selected != nil {
			m.selectedPodcast = selected
			m.loadingDetail = true
			m.syncDetailViewport(true)
			m.setStatus(fmt.Sprintf("Loaded %d podcasts", len(msg.podcasts)), "success")
			cmds = append(cmds, m.loadEpisodes(selected.ID), m.spin.Tick)
		}
		return m, tea.Batch(cmds...)

	case episodesLoadedMsg:
		if m.selectedPodcast == nil || msg.podcastID != m.selectedPodcast.ID {
			return m, tea.Batch(cmds...)
		}

		m.loadingDetail = false
		if msg.err != nil {
			m.episodes = nil
			m.selectedEpisode = nil
			m.setStatus("Failed to load episodes", "error")
			return m, tea.Batch(cmds...)
		}

		m.episodes = msg.episodes
		m.selectedEpisode = nil

		m.rebuildEpisodeList()

		m.syncDetailViewport(false)
		return m, tea.Batch(cmds...)

	case podcastAddedMsg:
		m.submitting = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Add failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.state = stateBrowse
		m.input.Blur()
		m.input.Reset()
		m.setStatus(fmt.Sprintf("Added %s", msg.podcast.Title), "success")
		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.spin.Tick)
		return m, tea.Batch(cmds...)

	case podcastRefreshedMsg:
		m.loadingDetail = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Refresh failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.loadingDetail = true
		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.loadEpisodes(msg.podcastID), m.spin.Tick)

		if msg.newCount > 0 {
			m.setStatus(
				fmt.Sprintf("Added %d new episode%s", msg.newCount, suffix(msg.newCount)),
				"success",
			)
		} else {
			m.setStatus("No new episodes", "info")
		}
		return m, tea.Batch(cmds...)

	case allPodcastsSyncedMsg:
		m.syncingAllFeeds = false
		if m.settings.PeriodicSync {
			m.nextPeriodicSyncAt = time.Now().
				Add(time.Duration(m.settings.PeriodicSyncMins) * time.Minute)
		}
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Sync failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}

		m.loadingLibrary = true
		cmds = append(cmds, m.loadPodcasts(), m.spin.Tick)
		if m.selectedPodcast != nil {
			m.loadingDetail = true
			cmds = append(cmds, m.loadEpisodes(m.selectedPodcast.ID))
		}
		prefix := "Sync complete"
		if msg.reason == "startup" {
			prefix = "Startup sync complete"
		}
		m.setStatus(
			fmt.Sprintf(
				"%s: %d new episodes across %d/%d podcasts (%d failed)",
				prefix,
				msg.result.NewEpisodes,
				msg.result.Refreshed,
				msg.result.TotalPodcasts,
				msg.result.Failed,
			),
			"success",
		)
		return m, tea.Batch(cmds...)

	case settingsPersistedMsg:
		if msg.err != nil {
			m.settings = msg.previous
			m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
			m.discordInput.SetValue(m.settings.DiscordClientID)
			m.setStatus(fmt.Sprintf("Settings save failed: %v", msg.err), "error")
			return m, tea.Batch(cmds...)
		}
		m.settings = msg.settings
		if m.settings.PeriodicSync {
			m.nextPeriodicSyncAt = time.Now().
				Add(time.Duration(m.settings.PeriodicSyncMins) * time.Minute)
		} else {
			m.nextPeriodicSyncAt = time.Time{}
		}
		m.setStatus("Settings saved", "success")
		return m, tea.Batch(cmds...)

	case errMsg:
		m.setStatus(msg.err.Error(), "error")
		return m, tea.Batch(cmds...)

	case downloadQueuedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Download queue failed: %v", msg.err), "error")
		} else {
			m.setStatus("Episode queued for download", "success")
			cmds = append(cmds, m.loadDownloadJobs())
		}
		return m, tea.Batch(cmds...)

	case downloadJobsLoadedMsg:
		m.downloadJobs = msg.jobs
		if msg.err != nil {
			m.setStatus("Failed to load downloads", "error")
		} else {
			flashTick := time.Now().Unix()
			items := make([]list.Item, len(msg.jobs))
			for i, job := range msg.jobs {
				items[i] = DownloadJobItem{
					DownloadJob: job,
				}.WithTheme(m.theme).
					WithFlashTick(flashTick)
			}
			cmds = append(cmds, m.queueList.SetItems(items))
		}
		return m, tea.Batch(cmds...)

	case downloadStartedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Start failed: %v", msg.err), "error")
		} else {
			m.setStatus("Download started", "success")
		}
		cmds = append(cmds, m.loadDownloadJobs())
		return m, tea.Batch(cmds...)

	case downloadRetriedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Retry failed: %v", msg.err), "error")
		} else {
			m.setStatus("Download retry started", "success")
		}
		cmds = append(cmds, m.loadDownloadJobs())
		return m, tea.Batch(cmds...)

	case episodePlayedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Playback failed: %v", msg.err), "error")
		} else {
			m.setStatus("Playing episode", "success")
			cmds = append(cmds, m.fetchPlaybackStatus())
		}
		return m, tea.Batch(cmds...)

	case playbackStatusMsg:
		if msg.err == nil {
			m.playbackStatus = msg.status
		}
		return m, tea.Batch(cmds...)
	}

	if m.focus == focusDetail {
		// Handle episode navigation when detail pane is focused
		previousEpisodeID := int64(0)
		if selected := selectedEpisodeItem(m.epList); selected != nil {
			previousEpisodeID = selected.ID
		}

		// Handle play episode action before updating the list (skip if filtering)
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(msg, m.keys.PlayEpisode) && m.epList.FilterState() != list.Filtering {
				if selected := selectedEpisodeItem(m.epList); selected != nil {
					cmds = append(cmds, m.playEpisode(selected.ID))
				}
			}

			// Open "go to episode" modal when 'g' is pressed (skip if filtering)
			if key.Matches(msg, m.keys.GoToEpisode) && m.epList.FilterState() != list.Filtering &&
				m.state == stateBrowse {
				m.openGoToEpisodeModal()
				cmds = append(cmds, m.goToInput.Focus())
				return m, tea.Batch(cmds...)
			}
		}

		var epListCmd tea.Cmd
		m.epList, epListCmd = m.epList.Update(msg)
		if epListCmd != nil {
			cmds = append(cmds, epListCmd)
		}

		selected := selectedEpisodeItem(m.epList)
		if selected != nil && selected.ID != previousEpisodeID {
			m.selectedEpisode = &selected.Episode
		}

		var detailCmd tea.Cmd
		m.detail, detailCmd = m.detail.Update(msg)
		if detailCmd != nil {
			cmds = append(cmds, detailCmd)
		}
		return m, tea.Batch(cmds...)
	}

	previousID := int64(0)
	if selected := selectedPodcastItem(m.list); selected != nil {
		previousID = selected.ID
	}

	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	selected := selectedPodcastItem(m.list)
	if selected != nil && selected.ID != previousID {
		m.selectedPodcast = selected
		m.loadingDetail = true
		m.syncDetailViewport(true)
		cmds = append(cmds, m.loadEpisodes(selected.ID), m.spin.Tick)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleAddMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) && !m.submitting {
		m.state = stateBrowse
		m.input.Blur()
		m.input.Reset()
		m.setStatus("Add podcast cancelled", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) && !m.submitting {
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.setStatus("Feed URL is required", "warning")
			return m, tea.Batch(cmds...)
		}

		m.submitting = true
		m.setStatus("Fetching feed and saving episodes…", "info")
		cmds = append(cmds, m.addPodcast(value), m.spin.Tick)
		return m, tea.Batch(cmds...)
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleGoToEpisodeMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.goToInput.Blur()
		m.goToInput.Reset()
		m.setStatus("Go to episode cancelled", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) {
		value := strings.TrimSpace(m.goToInput.Value())
		if value == "" {
			m.setStatus("Episode number is required", "warning")
			return m, tea.Batch(cmds...)
		}

		var num int
		if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
			m.setStatus("Invalid episode number", "warning")
			return m, tea.Batch(cmds...)
		}

		idx := num - 1
		if idx < 0 || idx >= len(m.episodes) {
			m.setStatus(
				fmt.Sprintf("Episode %d out of range (1-%d)", num, len(m.episodes)),
				"warning",
			)
			return m, tea.Batch(cmds...)
		}

		m.state = stateBrowse
		m.goToInput.Blur()
		m.goToInput.Reset()
		m.epList.Select(idx)
		m.selectedEpisode = &m.episodes[idx]
		m.setStatus(fmt.Sprintf("Selected episode %d", num), "success")
		return m, tea.Batch(cmds...)
	}

	var inputCmd tea.Cmd
	m.goToInput, inputCmd = m.goToInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	contentWidth := m.contentWidth()
	m.help.SetWidth(max(contentWidth, 20))

	appFrameHeight := m.theme.App.GetVerticalFrameSize()
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	bodyHeight := m.height - appFrameHeight - headerHeight - footerHeight
	m.bodyHeight = max(bodyHeight, 1)

	// These are now set inside the render methods, which have the final say
	// on height, but we can still set the width here.
	if m.shouldStackPanes() {
		m.listWidth = max(contentWidth-4, 16)
		m.detailWidth = max(contentWidth-4, 16)
		m.list.SetSize(m.listWidth, max((m.bodyHeight-1)/2, 1))
		m.detailHeight = max(m.bodyHeight-max((m.bodyHeight-1)/2, 1)-1, 1)
	} else {
		gap := 1
		leftWidth := max(contentWidth/3, 24)
		rightWidth := max(contentWidth-leftWidth-gap, 24)
		if leftWidth+rightWidth+gap > contentWidth {
			rightWidth = max(contentWidth-leftWidth-gap, 24)
		}
		if leftWidth+rightWidth+gap > contentWidth {
			leftWidth = max(contentWidth-rightWidth-gap, 24)
		}
		m.listWidth = max(leftWidth-4, 16)
		m.list.SetSize(m.listWidth, max(m.bodyHeight, 1))
		m.detailWidth = max(rightWidth-4, 16)
		m.detailHeight = max(m.bodyHeight, 1)
	}

	m.input.SetWidth(min(max(contentWidth-12, 20), 72))
	m.goToInput.SetWidth(min(max(contentWidth-12, 20), 72))
	m.syncDetailViewport(false)
	m.syncGuideViewport(false)
}

func (m *Model) openAddModal() {
	m.state = stateAddPodcast
	m.input.Reset()
	m.input.Placeholder = "https://example.com/feed.xml"
}

func (m *Model) openGoToEpisodeModal() {
	m.state = stateGoToEpisode
	m.goToInput.Reset()
	m.goToInput.Placeholder = "episode number"
}

func (m *Model) toggleFocus() {
	if m.focus == focusLibrary {
		m.focus = focusDetail
		if len(m.episodes) > 0 {
			m.setStatus(
				"Detail pane focused. Use j/k or arrow keys to navigate episodes, enter/space to play.",
				"info",
			)
		} else {
			m.setStatus("Detail pane focused. Use arrow keys or PgUp/PgDn to scroll.", "info")
		}
		return
	}
	m.focus = focusLibrary
	m.setStatus("Podcast list focused.", "info")
}

func (m *Model) toggleEpisodeSort() {
	if m.sortOrder == sortNewestFirst {
		m.sortOrder = sortOldestFirst
		m.setStatus("Sorting: oldest episodes first", "info")
	} else {
		m.sortOrder = sortNewestFirst
		m.setStatus("Sorting: newest episodes first", "info")
	}
	m.rebuildEpisodeList()
}

func (m *Model) rebuildEpisodeList() {
	sorted := make([]domain.Episode, len(m.episodes))
	copy(sorted, m.episodes)

	sortByPublishedAt(sorted, m.sortOrder == sortOldestFirst)

	previousID := int64(0)
	if m.selectedEpisode != nil {
		previousID = m.selectedEpisode.ID
	}

	items := make([]list.Item, len(sorted))
	for i, episode := range sorted {
		items[i] = EpisodeItem{Episode: episode}.WithTheme(m.theme)
	}
	m.epList.SetItems(items)

	m.episodes = sorted

	if previousID > 0 {
		for i, ep := range m.episodes {
			if ep.ID == previousID {
				m.epList.Select(i)
				m.selectedEpisode = &m.episodes[i]
				break
			}
		}
	}
}

func sortByPublishedAt(episodes []domain.Episode, oldestFirst bool) {
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].PublishedAt.IsZero() && episodes[j].PublishedAt.IsZero() {
			return false
		}
		if episodes[i].PublishedAt.IsZero() {
			return !oldestFirst
		}
		if episodes[j].PublishedAt.IsZero() {
			return oldestFirst
		}
		if oldestFirst {
			return episodes[i].PublishedAt.Before(episodes[j].PublishedAt)
		}
		return episodes[i].PublishedAt.After(episodes[j].PublishedAt)
	})
}

func (m *Model) setStatus(text, kind string) {
	m.status = text
	m.kind = kind
}

func (m Model) isBusy() bool {
	return m.loadingLibrary || m.loadingDetail || m.submitting
}

func (m *Model) openDownloadsQueue() {
	m.state = stateDownloads
	m.setStatus("Download queue opened", "info")
}

func (m *Model) openSettingsPage() {
	m.state = stateSettings
	m.settingsCursor = 0
	m.editingInterval = false
	m.editingDiscordID = false
	m.intervalInput.Blur()
	m.discordInput.Blur()
	m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
	m.discordInput.SetValue(m.settings.DiscordClientID)
}

func (m *Model) handleDownloadsMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.StartDownload) {
		selected := selectedDownloadJobItem(m.queueList)
		if selected != nil && selected.Status == domain.DownloadStatusQueued {
			cmds = append(cmds, m.startDownload(selected.ID))
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.RetryDownload) {
		selected := selectedDownloadJobItem(m.queueList)
		if selected != nil && selected.Status == domain.DownloadStatusFailed {
			cmds = append(cmds, m.retryDownload(selected.ID))
		}
		return m, tea.Batch(cmds...)
	}

	var queueCmd tea.Cmd
	m.queueList, queueCmd = m.queueList.Update(msg)
	if queueCmd != nil {
		cmds = append(cmds, queueCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleSettingsMode(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.editingInterval {
		if key.Matches(msg, m.keys.Close) {
			m.editingInterval = false
			m.intervalInput.Blur()
			m.intervalInput.SetValue(strconv.Itoa(m.settings.PeriodicSyncMins))
			m.setStatus("Interval edit cancelled", "info")
			return m, tea.Batch(cmds...)
		}
		if key.Matches(msg, m.keys.Submit) {
			value := strings.TrimSpace(m.intervalInput.Value())
			minutes, err := strconv.Atoi(value)
			if err != nil || minutes <= 0 {
				m.setStatus("Interval must be a positive integer", "warning")
				return m, tea.Batch(cmds...)
			}
			prev := m.settings
			next := m.settings
			next.PeriodicSyncMins = minutes
			m.editingInterval = false
			m.intervalInput.Blur()
			cmds = append(cmds, m.persistSettings(next, prev))
			return m, tea.Batch(cmds...)
		}
		var inputCmd tea.Cmd
		m.intervalInput, inputCmd = m.intervalInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.editingDiscordID {
		if key.Matches(msg, m.keys.Close) {
			m.editingDiscordID = false
			m.discordInput.Blur()
			m.discordInput.SetValue(m.settings.DiscordClientID)
			m.setStatus("Discord client ID edit cancelled", "info")
			return m, tea.Batch(cmds...)
		}
		if key.Matches(msg, m.keys.Submit) {
			prev := m.settings
			next := m.settings
			next.DiscordClientID = strings.TrimSpace(m.discordInput.Value())
			if next.DiscordPresence && next.DiscordClientID == "" {
				m.setStatus(
					"Discord client ID is required when Discord presence is enabled",
					"warning",
				)
				return m, tea.Batch(cmds...)
			}
			m.editingDiscordID = false
			m.discordInput.Blur()
			cmds = append(cmds, m.persistSettings(next, prev))
			return m, tea.Batch(cmds...)
		}
		var inputCmd tea.Cmd
		m.discordInput, inputCmd = m.discordInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Close) {
		m.state = stateBrowse
		m.setStatus("Returned to library", "info")
		return m, tea.Batch(cmds...)
	}

	switch msg.String() {
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, tea.Batch(cmds...)
	case "down", "j":
		if m.settingsCursor < 4 {
			m.settingsCursor++
		}
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, m.keys.Submit) || key.Matches(msg, m.keys.PlayEpisode) {
		prev := m.settings
		next := m.settings
		switch m.settingsCursor {
		case 0:
			next.AutoSyncOnStartup = !next.AutoSyncOnStartup
		case 1:
			next.PeriodicSync = !next.PeriodicSync
		case 2:
			m.editingInterval = true
			cmds = append(cmds, m.intervalInput.Focus())
			return m, tea.Batch(cmds...)
		case 3:
			if !next.DiscordPresence && strings.TrimSpace(next.DiscordClientID) == "" {
				m.setStatus("Set Discord client ID before enabling Discord presence", "warning")
				return m, tea.Batch(cmds...)
			}
			next.DiscordPresence = !next.DiscordPresence
		case 4:
			m.editingDiscordID = true
			cmds = append(cmds, m.discordInput.Focus())
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.persistSettings(next, prev))
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	content := m.renderContent()
	layout := m.theme.App.Render(lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		content,
		m.renderFooter(),
	))

	if m.state == stateAddPodcast {
		layout = components.RenderModal(
			m.theme,
			max(m.width, 80),
			max(m.height, 24),
			m.renderAddModal(),
		)
	}

	if m.state == stateGoToEpisode {
		layout = components.RenderModal(
			m.theme,
			max(m.width, 80),
			max(m.height, 24),
			m.renderGoToEpisodeModal(),
		)
	}

	view := tea.NewView(layout)
	view.AltScreen = true
	view.WindowTitle = "Gocaster"
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeCellMotion
	view.ForegroundColor = m.theme.Text
	return view
}

func (m Model) renderHeader() string {
	width := m.contentWidth()
	tagline := m.theme.MutedText.Render("Editorial podcast library")
	count := "No podcasts"
	if items := len(m.list.Items()); items > 0 {
		count = fmt.Sprintf("%d podcasts", items)
	}
	badge := m.theme.Badge.Render(count)

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.theme.Header.Width(max(width-lipgloss.Width(badge)-2, 10)).Render("Gocaster"),
		tagline,
	)

	if width < lipgloss.Width(left)+lipgloss.Width(badge)+1 {
		return lipgloss.JoinVertical(lipgloss.Left, left, badge)
	}

	spacerWidth := max(width-lipgloss.Width(left)-lipgloss.Width(badge), 1)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		lipgloss.NewStyle().Width(spacerWidth).Render(""),
		badge,
	)
}

func (m Model) renderContent() string {
	if m.state == stateHelp {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderHelpPage())
	}

	if m.state == stateDownloads {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderDownloadsPage())
	}

	if m.state == stateSettings {
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(m.renderSettingsPage())
	}

	if m.loadingLibrary && len(m.list.Items()) == 0 {
		content := components.RenderLoading(m.theme, m.spin.View(), "Loading library…")
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
	}

	left := m.renderPodcastPane()
	right := m.renderDetailPane()

	if m.shouldStackPanes() {
		content := lipgloss.JoinVertical(lipgloss.Left, left, right)
		return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
	}

	gap := " "
	leftWidth := lipgloss.Width(left)
	rightWidth := max(m.contentWidth()-leftWidth-lipgloss.Width(gap), 1)
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).MaxWidth(leftWidth).Render(left),
		gap,
		lipgloss.NewStyle().Width(rightWidth).MaxWidth(rightWidth).Render(right),
	)
	return lipgloss.NewStyle().MaxHeight(max(m.bodyHeight, 1)).Render(content)
}

func (m Model) renderHelpPage() string {
	title := m.theme.SectionTitle.Render("Help & Shortcuts")
	subtitle := m.theme.MutedText.Render("How to use Gocaster and navigate the interface.")
	panel := m.theme.PanelFocused.Width(max(m.contentWidth()-4, 20))

	// TODO: add app logo
	return panel.Render(lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		m.guide.View(),
	))
}

func (m Model) renderDownloadsPage() string {
	title := m.theme.SectionTitle.Render("Download Queue")
	subtitle := m.theme.MutedText.Render("Manage your downloads. Press s to start, r to retry.")
	panel := m.theme.Panel

	paneHeight := max(m.bodyHeight, 1)
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)

	m.queueList.SetSize(max(m.contentWidth()-4, 20), innerHeight)
	body := m.queueList.View()

	if len(m.queueList.Items()) == 0 {
		body = m.theme.MutedText.Render(
			"No downloads in queue.\n\nPress 'd' on an episode to download it.",
		)
	}

	return panel.Width(max(m.contentWidth()-4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderSettingsPage() string {
	title := m.theme.SectionTitle.Render("Settings")
	subtitle := m.theme.MutedText.Render(
		"Configure sync and Discord presence. Use j/k to move, Enter or Space to toggle/edit.",
	)
	panel := m.theme.PanelFocused

	rows := []string{
		fmt.Sprintf("Auto-sync on startup: %s", onOff(m.settings.AutoSyncOnStartup)),
		fmt.Sprintf("Periodic sync enabled: %s", onOff(m.settings.PeriodicSync)),
		fmt.Sprintf("Periodic sync interval (minutes): %d", m.settings.PeriodicSyncMins),
		fmt.Sprintf("Discord Rich Presence enabled: %s", onOff(m.settings.DiscordPresence)),
		fmt.Sprintf("Discord client ID: %s", valueOrPlaceholder(m.settings.DiscordClientID)),
	}

	for i := range rows {
		row := rows[i]
		if i == m.settingsCursor {
			if i == 2 && m.editingInterval {
				row = fmt.Sprintf("Periodic sync interval (minutes): %s", m.intervalInput.View())
			}
			if i == 4 && m.editingDiscordID {
				row = fmt.Sprintf("Discord client ID: %s", m.discordInput.View())
			}
			row = m.theme.Card.Width(max(m.contentWidth()-8, 20)).Render(row)
		} else {
			row = m.theme.Body.Render(row)
		}
		rows[i] = row
	}

	hint := m.theme.MutedText.Render("Esc to return. Press ? for help.")
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		rows[0],
		rows[1],
		rows[2],
		rows[3],
		rows[4],
		"",
		hint,
	)

	return panel.Width(max(m.contentWidth()-4, 20)).
		Height(max(m.bodyHeight, 1)).
		MaxHeight(max(m.bodyHeight, 1)).
		Render(content)
}

func (m Model) renderPodcastPane() string {
	title := m.theme.SectionTitle.Render("Podcasts")
	subtitle := m.theme.MutedText.Render("Browse your subscriptions. Press / to filter.")
	panel := m.theme.Panel
	if m.focus == focusLibrary {
		panel = m.theme.PanelFocused
	}
	paneHeight := max(m.bodyHeight, 1)
	if m.shouldStackPanes() {
		paneHeight = max((m.bodyHeight-1)/2, 1)
	}
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)
	m.list.SetSize(max(m.listWidth, 1), innerHeight)

	body := m.list.View()
	if len(m.list.Items()) == 0 && !m.loadingLibrary {
		body = m.theme.MutedText.Render("No podcasts yet.\n\nPress 'a' to add an RSS feed.")
	}

	return panel.Width(max(m.listWidth+4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderDetailPane() string {
	panel := m.theme.Panel
	if m.focus == focusDetail {
		panel = m.theme.PanelFocused
	}
	paneHeight := max(m.detailHeight, 1)

	title := m.theme.SectionTitle.Render("Details")
	subtitle := m.theme.MutedText.Render("Show details. Press / to filter episodes.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	innerHeight := max(paneHeight-panel.GetVerticalFrameSize()-lipgloss.Height(header), 1)

	if m.selectedPodcast == nil {
		return panel.Width(max(m.detailWidth+4, 20)).
			Height(paneHeight).
			MaxHeight(paneHeight).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				header,
				m.theme.MutedText.Render(
					"Select a podcast to see its description and recent episodes.",
				),
			))
	}

	return panel.Width(max(m.detailWidth+4, 20)).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.renderDetailContent(innerHeight),
		))
}

func (m Model) renderDetailContent(availableHeight int) string {
	if m.selectedPodcast == nil {
		return ""
	}

	wrapWidth := max(m.detailPaneWidth()-4, 16)

	detailParts := []string{
		m.theme.SectionTitle.Render(m.selectedPodcast.Title),
		lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.Label.Render("Feed "),
			m.theme.MutedText.Render(m.selectedPodcast.FeedURL),
		),
	}

	if !m.selectedPodcast.LastUpdated.IsZero() {
		detailParts = append(detailParts, lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.Label.Render("Updated "),
			m.theme.MutedText.Render(m.selectedPodcast.LastUpdated.Format(time.DateOnly)),
		))
	}

	description := strings.TrimSpace(m.selectedPodcast.Description)
	if description == "" {
		description = "No description available."
	}

	descriptionWrapped := m.theme.Body.Render(lipgloss.Wrap(description, wrapWidth, ""))
	descLines := lipgloss.Height(descriptionWrapped)

	episodesHeading := m.theme.SectionTitle.Render("Recent Episodes")
	episodesHeadingHeight := lipgloss.Height(episodesHeading)

	minEpisodesHeight := 3

	availableForDescription := availableHeight - episodesHeadingHeight - minEpisodesHeight
	if availableForDescription < 1 {
		availableForDescription = 1
	}

	if descLines > availableForDescription {
		truncated := truncateLines(descriptionWrapped, availableForDescription)
		descriptionWrapped = m.theme.Body.Render(truncated)
	}

	topCard := m.theme.Card.Width(max(m.detailPaneWidth(), 16)).
		Render(strings.Join(detailParts, "\n") + "\n" + descriptionWrapped)

	episodesHeight := max(
		availableHeight-lipgloss.Height(topCard)-episodesHeadingHeight,
		minEpisodesHeight,
	)
	episodes := m.renderEpisodes(episodesHeight)

	return lipgloss.JoinVertical(lipgloss.Left,
		topCard,
		episodesHeading,
		episodes,
	)
}

func (m Model) renderEpisodes(availableHeight int) string {
	if m.loadingDetail {
		return components.RenderLoading(m.theme, m.spin.View(), "Loading episodes…")
	}

	if len(m.episodes) == 0 {
		return m.theme.MutedText.Render("No stored episodes for this feed yet.")
	}

	m.epList.SetSize(max(m.detailPaneWidth(), 16), max(availableHeight, 3))

	return m.epList.View()
}

func (m Model) renderAddModal() string {
	inputStyle := m.theme.Input
	if m.input.Focused() {
		inputStyle = m.theme.InputFocused
	}

	body := []string{
		m.theme.SectionTitle.Render("Add Podcast"),
		m.theme.MutedText.Render(
			"Paste an RSS feed URL. Episodes will be fetched and stored immediately.",
		),
		m.theme.Label.Render("Feed URL"),
		inputStyle.Render(m.input.View()),
	}

	if m.submitting {
		body = append(body, components.RenderLoading(m.theme, m.spin.View(), "Importing feed…"))
	} else {
		body = append(body, m.theme.MutedText.Render("Enter to submit, Esc to cancel"))
	}

	return strings.Join(body, "\n\n")
}

func (m Model) renderGoToEpisodeModal() string {
	inputStyle := m.theme.Input
	if m.goToInput.Focused() {
		inputStyle = m.theme.InputFocused
	}

	body := []string{
		m.theme.SectionTitle.Render("Go to episode"),
		m.theme.MutedText.Render(
			fmt.Sprintf("Enter episode number (1-%d) to jump directly to it.", len(m.episodes)),
		),
		m.theme.Label.Render("Episode #"),
		inputStyle.Render(m.goToInput.View()),
		m.theme.MutedText.Render("Enter to go, Esc to cancel"),
	}

	return strings.Join(body, "\n\n")
}

func (m Model) renderFooter() string {
	if m.state == stateHelp {
		status := lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.StatusStyle(m.kind).Render(m.status),
		)
		hint := m.theme.HelpText.Render(
			"Press ? or Esc to return. Use arrow keys, j/k, PgUp/PgDn, or the mouse wheel to scroll.",
		)
		return lipgloss.JoinVertical(lipgloss.Left, status, hint)
	}

	status := lipgloss.JoinHorizontal(lipgloss.Left,
		m.theme.StatusStyle(m.kind).Render(m.status),
	)

	shortcuts := m.keys.FooterShortcuts(string(m.state), string(m.focus))
	helpView := m.theme.HelpText.Render(m.help.ShortHelpView(shortcuts))
	overflowHint := m.theme.HelpText.Render(" · ? for all")
	return lipgloss.JoinVertical(lipgloss.Left, status, helpView+overflowHint)
}

func (m Model) detailPaneWidth() int {
	if m.detailWidth <= 0 || m.contentWidth() <= 0 {
		return max(m.contentWidth()-4, 16)
	}
	return m.detailWidth
}

func (m *Model) syncDetailViewport(reset bool) {
	width := max(m.detailPaneWidth(), 16)
	height := max(m.detailHeight-2, 5)

	m.detail.SetWidth(width)
	m.detail.SetHeight(height)
	m.detail.SetContent(m.renderDetailContent(height))
	if reset {
		m.detail.GotoTop()
	}
}

func (m *Model) syncGuideViewport(reset bool) {
	width := max(m.contentWidth()-8, 20)
	height := max(m.bodyHeight-4, 6)

	m.guide.SetWidth(width)
	m.guide.SetHeight(height)
	m.guide.SetContent(m.renderGuideContent(width))
	if reset {
		m.guide.GotoTop()
	}
}

func (m Model) renderGuideContent(width int) string {
	wrapWidth := max(width-4, 16)

	shortcuts := []string{
		m.theme.SectionTitle.Render("Shortcuts"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			m.theme.Label.Render("a") + "  Add a podcast feed",
			m.theme.Label.Render("r") + "  Refresh selected podcast feed",
			m.theme.Label.Render("g") + "  Go to episode by number (in detail pane)",
			m.theme.Label.Render("s") + "  Toggle episode sort order (newest/oldest first)",
			m.theme.Label.Render("S") + "  Open settings",
			m.theme.Label.Render("tab") + "  Switch focus between the library and detail panes",
			m.theme.Label.Render("enter") + "  Confirm actions in dialogs and list filtering",
			m.theme.Label.Render("esc") + "  Close dialogs or leave this help page",
			m.theme.Label.Render("?") + "  Open or close this help page",
			m.theme.Label.Render("q / ctrl+c") + "  Quit the app",
			m.theme.Label.Render(
				"↑ ↓ / j k / pgup pgdn",
			) + "  Move through lists or scroll focused content",
			m.theme.Label.Render("/") + "  Filter the focused list (podcasts or episodes)",
			m.theme.Label.Render("enter / space") + "  Play the selected episode",
		}, "\n")),
	}

	usage := []string{
		m.theme.SectionTitle.Render("How To Use The App"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			"1. Start in the podcast library on the left. If the library is empty, press " + m.theme.Label.Render(
				"a",
			) + " and paste an RSS feed URL.",
			"2. Move through podcasts with the list keys. The selected show loads metadata and stored episodes into the detail pane.",
			"3. Press " + m.theme.Label.Render(
				"tab",
			) + " to focus the detail pane when you want to navigate episodes or scroll long descriptions.",
			"4. In the detail pane, use " + m.theme.Label.Render(
				"j/k",
			) + " or arrow keys to navigate between episodes. The selected episode is highlighted with an accent border.",
			"5. Press " + m.theme.Label.Render(
				"g",
			) + " to jump directly to an episode by number.",
			"6. Press " + m.theme.Label.Render(
				"enter",
			) + " or " + m.theme.Label.Render(
				"space",
			) + " to play the selected episode.",
			"7. Episodes show a " + lipgloss.NewStyle().
				Foreground(m.theme.Success).
				Bold(true).
				Render("NEW") +
				" indicator for unplayed episodes and a " + m.theme.MutedText.Render(
				"PLAYED",
			) + " indicator for played ones.",
			"8. Press " + m.theme.Label.Render(
				"tab",
			) + " again to return focus to the podcast list.",
			"9. Press " + m.theme.Label.Render("?") + " any time to revisit this help page.",
		}, "\n\n")),
	}

	tips := []string{
		m.theme.SectionTitle.Render("What You're Looking At"),
		m.theme.Card.Width(wrapWidth).Render(strings.Join([]string{
			"The left pane is your podcast library.",
			"The right pane shows the selected podcast description, feed info, and recent episodes.",
			"Episodes with a " + lipgloss.NewStyle().
				Foreground(m.theme.Success).
				Bold(true).
				Render("NEW") +
				" indicator haven't been played yet.",
			"Episodes with a " + m.theme.MutedText.Render(
				"PLAYED",
			) + " indicator have been played.",
			"The selected episode has a highlighted left border in the accent color.",
			"The status bar at the bottom shows feedback for loading, errors, and actions.",
		}, "\n")),
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		shortcuts[0],
		shortcuts[1],
		"",
		usage[0],
		usage[1],
		"",
		tips[0],
		tips[1],
	)
}

func selectedPodcastItem(listModel list.Model) *domain.Podcast {
	item, ok := listModel.SelectedItem().(PodcastItem)
	if !ok {
		return nil
	}
	podcast := item.Podcast
	return &podcast
}

func selectedEpisodeItem(listModel list.Model) *EpisodeItem {
	item, ok := listModel.SelectedItem().(EpisodeItem)
	if !ok {
		return nil
	}
	episode := item
	return &episode
}

func selectedDownloadJobItem(listModel list.Model) *DownloadJobItem {
	item, ok := listModel.SelectedItem().(DownloadJobItem)
	if !ok {
		return nil
	}
	job := item
	return &job
}

func suffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func valueOrPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(not set)"
	}
	return value
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

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return max(m.width-m.theme.App.GetHorizontalFrameSize(), 20)
}

func (m Model) shouldStackPanes() bool {
	return m.contentWidth() < 80
}

func truncateLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n")
}
