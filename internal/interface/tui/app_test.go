package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
)

type tuiMockFeedParser struct {
	podcast  *domain.Podcast
	episodes []domain.Episode
	err      error
}

func (m tuiMockFeedParser) Parse(string) (*domain.Podcast, []domain.Episode, error) {
	return m.podcast, m.episodes, m.err
}

func newTestModel(t *testing.T) Model {
	t.Helper()

	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	service := application.NewPodcastService(repo, tuiMockFeedParser{})
	return NewModel(service)
}

func keyMsg(text string, code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func TestModelWindowResizeUpdatesDimensions(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	resized := updated.(Model)

	if resized.width != 120 || resized.height != 40 {
		t.Fatalf("expected dimensions 120x40, got %dx%d", resized.width, resized.height)
	}

	if resized.list.Width() == 0 || resized.list.Height() == 0 {
		t.Fatal("expected list size to be updated on resize")
	}
}

func TestModelWindowResizeStacksOnNarrowTerminal(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	resized := updated.(Model)

	if !resized.shouldStackPanes() {
		t.Fatal("expected narrow terminal to use stacked layout")
	}

	if resized.listWidth > resized.contentWidth() || resized.detailWidth > resized.contentWidth() {
		t.Fatalf("expected pane widths within content width %d, got list=%d detail=%d", resized.contentWidth(), resized.listWidth, resized.detailWidth)
	}

	if resized.list.Width() <= 0 || resized.detail.Width() <= 0 {
		t.Fatal("expected list and detail viewports to have positive widths")
	}
}

func TestModelWindowResizeKeepsSplitPaneWidthsBounded(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 36})
	resized := updated.(Model)

	if resized.shouldStackPanes() {
		t.Fatal("expected wide terminal to keep split layout")
	}

	total := resized.listWidth + resized.detailWidth + 1
	if total > resized.contentWidth() {
		t.Fatalf("expected split panes within content width %d, got total=%d", resized.contentWidth(), total)
	}
}

func TestModelAddFlowTransitions(t *testing.T) {
	model := newTestModel(t)

	updated, cmd := model.Update(keyMsg("a", 'a'))
	current := updated.(Model)
	if current.state != stateAddPodcast {
		t.Fatalf("expected state %q, got %q", stateAddPodcast, current.state)
	}

	if cmd == nil {
		t.Fatal("expected focus command when opening add modal")
	}

	updated, _ = current.Update(keyMsg("", tea.KeyEsc))
	current = updated.(Model)
	if current.state != stateBrowse {
		t.Fatalf("expected state %q after escape, got %q", stateBrowse, current.state)
	}
}

func TestModelHelpPageTransitions(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updated.(Model)

	updated, _ = current.Update(keyMsg("?", '?'))
	current = updated.(Model)
	if current.state != stateHelp {
		t.Fatalf("expected state %q, got %q", stateHelp, current.state)
	}

	if current.guide.Width() == 0 || current.guide.Height() == 0 {
		t.Fatal("expected help viewport size to be initialized")
	}

	updated, _ = current.Update(keyMsg("", tea.KeyEsc))
	current = updated.(Model)
	if current.state != stateBrowse {
		t.Fatalf("expected state %q after escape, got %q", stateBrowse, current.state)
	}
}

func TestModelViewRendersHelpAndModalStatesAfterResize(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 78, Height: 22})
	current := updated.(Model)

	updated, _ = current.Update(keyMsg("?", '?'))
	current = updated.(Model)
	helpView := current.View().Content
	if !strings.Contains(helpView, "Help & Shortcuts") {
		t.Fatalf("expected help view to render help content, got %q", helpView)
	}

	updated, _ = current.Update(keyMsg("", tea.KeyEsc))
	current = updated.(Model)
	updated, _ = current.Update(keyMsg("a", 'a'))
	current = updated.(Model)

	modalView := current.View().Content
	if !strings.Contains(modalView, "Add Podcast") {
		t.Fatalf("expected modal view to render add dialog, got %q", modalView)
	}
}

func TestModelTabSwitchesPaneOnlyInBrowseMode(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(keyMsg("", tea.KeyTab))
	current := updated.(Model)
	if current.focus != focusDetail {
		t.Fatalf("expected focus %q, got %q", focusDetail, current.focus)
	}

	updated, _ = current.Update(keyMsg("?", '?'))
	current = updated.(Model)
	if current.state != stateHelp {
		t.Fatalf("expected state %q, got %q", stateHelp, current.state)
	}

	updated, _ = current.Update(keyMsg("", tea.KeyTab))
	current = updated.(Model)
	if current.focus != focusDetail {
		t.Fatalf("expected focus to remain %q while help is open, got %q", focusDetail, current.focus)
	}
}

func TestModelPodcastSelectionLoadsEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:          7,
		Title:       "Syntax",
		FeedURL:     "https://example.com/feed.xml",
		Description: "A long description",
		LastUpdated: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}

	updated, cmd := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	if current.selectedPodcast == nil || current.selectedPodcast.ID != podcast.ID {
		t.Fatal("expected first podcast to be selected")
	}

	if !current.loadingDetail {
		t.Fatal("expected detail loading to start after podcasts load")
	}

	if cmd == nil {
		t.Fatal("expected detail loading command")
	}

	updated, _ = current.Update(episodesLoadedMsg{
		podcastID: podcast.ID,
		episodes: []domain.Episode{
			{ID: 1, PodcastID: podcast.ID, Title: "Episode 1"},
		},
	})
	current = updated.(Model)

	if current.loadingDetail {
		t.Fatal("expected detail loading to finish")
	}

	if len(current.episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(current.episodes))
	}
}
