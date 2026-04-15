package tui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
)

// Messages represent events coming back to the UI
type errMsg struct {
	err error
}
type podcastsLoadedMsg []domain.Podcast
type podcastAddedMsg struct {
	podcast *domain.Podcast
	err     error
}
type Model struct {
	// Services (DI)
	podcastService *application.PodcastService

	// UI State
	state     string // "list", "add", "detail"
	list      list.Model
	inputMode bool
	input     string
	status    string
}

func NewModel(svc *application.PodcastService) Model {
	// Initialize UI components
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Podcasts"

	return Model{
		podcastService: svc,
		list:           l,
		state:          "list",
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadPodcasts()
}

// Commands

func (m Model) loadPodcasts() tea.Cmd {
	return func() tea.Msg {
		podcasts, err := m.podcastService.ListPodcasts()
		if err != nil {
			return errMsg{err}
		}
		return podcastsLoadedMsg(podcasts)
	}
}

func (m Model) addPodcast(url string) tea.Cmd {
	return func() tea.Msg {
		podcast, err := m.podcastService.AddPodcast(url)
		return podcastAddedMsg{podcast, err}
	}
}

// Update - event loop

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.inputMode {
			if msg.String() == "enter" {
				return m, m.addPodcast(m.input)
			}
			if msg.String() == "esc" {
				m.inputMode = false
				m.input = ""
			}
			m.input += msg.String()
			return m, nil
		}
		if msg.String() == "a" {
			m.inputMode = true
			m.input = ""
		}
	case tea.MouseMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			// TODO: handle click
		}
	case podcastsLoadedMsg:
		items := make([]list.Item, len(msg))
		for i, p := range msg {
			items[i] = PodcastItem{Podcast: p}
		}
		m.list.SetItems(items)

	case podcastAddedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("Added podcast: %s", msg.podcast.Title)
			return m, m.loadPodcasts()
		}
	}

	// Default list handling
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View

func (m Model) View() tea.View {
	var content string
	if m.inputMode {
		// Build input view
		content = fmt.Sprintf(
			"Add New Podcast\n\n%s\n\n%s",
			m.input,
			"Press Enter to add, Esc to cancel",
		)
	} else {
		// Build main view
		content = m.list.View()
	}

	// Add status bar
	status := fmt.Sprintf(" [Press 'a' to add | 'q' to quit] | Status: %s ", m.status)
	styledStatus := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#3C3C3C")).
		Render(status)

	// Combine content and status
	finalLayout := lipgloss.JoinVertical(lipgloss.Left, content, styledStatus)
	v := tea.NewView(finalLayout)

	// Set view properties
	v.AltScreen = true
	v.WindowTitle = "Gocaster - TUI Podcast Manager"
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = true
	return v
}
