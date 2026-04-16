package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

type tuiMockPlayer struct {
	playErr error
}

func (m *tuiMockPlayer) Play(source string) error {
	return m.playErr
}

func (m *tuiMockPlayer) Stop() error {
	return nil
}

func (m *tuiMockPlayer) IsPlaying() bool {
	return false
}

func (m *tuiMockPlayer) Pause() error {
	return nil
}

func (m *tuiMockPlayer) Resume() error {
	return nil
}

func (m *tuiMockPlayer) TogglePause() error {
	return nil
}

func (m *tuiMockPlayer) Seek(seconds float64) error {
	return nil
}

func (m *tuiMockPlayer) Status() (domain.PlaybackStatus, error) {
	return domain.PlaybackStatus{}, nil
}

func (m *tuiMockPlayer) Close() error {
	return nil
}

func newTestModel(t *testing.T) Model {
	t.Helper()

	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	podcastService := application.NewPodcastService(repo, tuiMockFeedParser{})
	downloadService := application.NewDownloadService(repo, "downloads")
	mockPlayer := &tuiMockPlayer{}
	playerService := application.NewPlayerService(repo, mockPlayer, nil)
	return NewModel(podcastService, downloadService, playerService)
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

func TestModelViewHeightStaysWithinWindowAfterResize(t *testing.T) {
	model := newTestModel(t)

	podcasts := make([]domain.Podcast, 30)
	for i := range podcasts {
		podcasts[i] = domain.Podcast{
			ID:      int64(i + 1),
			Title:   fmt.Sprintf("Podcast %d", i+1),
			FeedURL: fmt.Sprintf("https://example.com/%d.xml", i+1),
		}
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: podcasts})
	current := updated.(Model)

	windowHeight := 24
	for i := 0; i < 6; i++ {
		updated, _ = current.Update(tea.WindowSizeMsg{Width: 100, Height: windowHeight})
		current = updated.(Model)

		renderedHeight := lipgloss.Height(current.View().Content)
		if renderedHeight > windowHeight {
			t.Fatalf("expected rendered height <= %d after resize %d, got %d", windowHeight, i+1, renderedHeight)
		}
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

func TestModelRefreshKeyTriggersRefreshCommand(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	updated, cmd := current.Update(keyMsg("r", 'r'))
	current = updated.(Model)

	if cmd == nil {
		t.Fatal("expected refresh command when pressing r with selected podcast")
	}

	updated, _ = current.Update(podcastRefreshedMsg{podcastID: podcast.ID, newCount: 3})
	current = updated.(Model)

	if current.status != "Added 3 new episodes" {
		t.Fatalf("expected status 'Added 3 new episodes', got %q", current.status)
	}
}

func TestModelEpisodeFilterWithSlashInDetailPane(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}
	episodes := []domain.Episode{
		{ID: 1, PodcastID: podcast.ID, Title: "Episode One - Introduction"},
		{ID: 2, PodcastID: podcast.ID, Title: "Episode Two - Advanced Topics"},
		{ID: 3, PodcastID: podcast.ID, Title: "Episode Three - Deep Dive"},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	_, _ = model.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: episodes})
	current := updated.(Model)

	current.focus = focusDetail
	current.epList.SetFilterState(list.Filtering)

	updated, cmd := current.Update(keyMsg("/", '/'))
	current = updated.(Model)

	if cmd == nil {
		t.Fatal("expected filter command when pressing / in detail focus")
	}

	if !current.epList.SettingFilter() {
		t.Fatal("expected episode list to be in filter mode after pressing /")
	}
}

func TestModelPodcastListFilterWithSlash(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	current.focus = focusLibrary

	updated, cmd := current.Update(keyMsg("/", '/'))
	current = updated.(Model)

	if cmd == nil {
		t.Fatal("expected filter command when pressing / in library focus")
	}

	if !current.list.SettingFilter() {
		t.Fatal("expected podcast list to be in filter mode after pressing /")
	}
}

func TestModelEpisodeTickDoesNotResetItemsWhenFilterApplied(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}
	episodes := []domain.Episode{
		{ID: 1, PodcastID: podcast.ID, Title: "Episode One - Introduction"},
		{ID: 2, PodcastID: podcast.ID, Title: "Episode Two - Advanced Topics"},
		{ID: 3, PodcastID: podcast.ID, Title: "Episode Three - Deep Dive"},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	_, _ = model.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: episodes})
	current := updated.(Model)

	current.focus = focusDetail
	current.epList.SetFilterState(list.FilterApplied)

	episodeCountBeforeTick := len(current.epList.Items())

	updated, _ = current.Update(tickMsg{})
	current = updated.(Model)

	episodeCountAfterTick := len(current.epList.Items())

	if episodeCountAfterTick != episodeCountBeforeTick {
		t.Fatalf("expected episode count %d after tick when filter applied, got %d",
			episodeCountBeforeTick, episodeCountAfterTick)
	}
}

func TestModelToggleEpisodeSortFlipsState(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}
	now := time.Now()
	episodes := []domain.Episode{
		{ID: 1, PodcastID: podcast.ID, Title: "Episode One", PublishedAt: now.Add(-2 * 24 * time.Hour)},
		{ID: 2, PodcastID: podcast.ID, Title: "Episode Two", PublishedAt: now},
		{ID: 3, PodcastID: podcast.ID, Title: "Episode Three", PublishedAt: now.Add(-1 * 24 * time.Hour)},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	updated, _ = current.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: episodes})
	current = updated.(Model)

	if current.sortOrder != sortNewestFirst {
		t.Fatalf("expected default sort order %q, got %q", sortNewestFirst, current.sortOrder)
	}

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	if current.sortOrder != sortOldestFirst {
		t.Fatalf("expected sort order %q after toggle, got %q", sortOldestFirst, current.sortOrder)
	}

	if current.status != "Sorting: oldest episodes first" {
		t.Fatalf("expected status 'Sorting: oldest episodes first', got %q", current.status)
	}

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	if current.sortOrder != sortNewestFirst {
		t.Fatalf("expected sort order %q after second toggle, got %q", sortNewestFirst, current.sortOrder)
	}

	if current.status != "Sorting: newest episodes first" {
		t.Fatalf("expected status 'Sorting: newest episodes first', got %q", current.status)
	}
}

func TestModelToggleEpisodeSortOrdersEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}
	now := time.Now()
	episodes := []domain.Episode{
		{ID: 1, PodcastID: podcast.ID, Title: "Episode One", PublishedAt: now.Add(-2 * 24 * time.Hour)},
		{ID: 2, PodcastID: podcast.ID, Title: "Episode Two", PublishedAt: now},
		{ID: 3, PodcastID: podcast.ID, Title: "Episode Three", PublishedAt: now.Add(-1 * 24 * time.Hour)},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	updated, _ = current.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: episodes})
	current = updated.(Model)

	firstItem := current.epList.Items()[0]
	firstEp := firstItem.(EpisodeItem)
	if firstEp.Title() != "Episode Two" {
		t.Fatalf("expected first episode 'Episode Two' (newest), got %q", firstEp.Title())
	}

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	firstItem = current.epList.Items()[0]
	firstEp = firstItem.(EpisodeItem)
	if firstEp.Title() != "Episode One" {
		t.Fatalf("expected first episode 'Episode One' (oldest), got %q", firstEp.Title())
	}
}

func TestModelToggleEpisodeSortIgnoredWhenNoEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	updated, _ = current.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: []domain.Episode{}})
	current = updated.(Model)

	originalStatus := current.status

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	if current.sortOrder != sortNewestFirst {
		t.Fatalf("expected sort order to remain %q when no episodes, got %q", sortNewestFirst, current.sortOrder)
	}

	if current.status == "Sorting: oldest episodes first" || current.status == "Sorting: newest episodes first" {
		t.Fatalf("expected no sort status change when no episodes, got %q", current.status)
	}

	_ = originalStatus
}

func TestModelToggleEpisodeSortPreservesSelection(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{
		ID:      7,
		Title:   "Syntax",
		FeedURL: "https://example.com/feed.xml",
	}
	now := time.Now()
	episodes := []domain.Episode{
		{ID: 1, PodcastID: podcast.ID, Title: "Episode One", PublishedAt: now.Add(-2 * 24 * time.Hour)},
		{ID: 2, PodcastID: podcast.ID, Title: "Episode Two", PublishedAt: now},
		{ID: 3, PodcastID: podcast.ID, Title: "Episode Three", PublishedAt: now.Add(-1 * 24 * time.Hour)},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	current := updated.(Model)

	updated, _ = current.Update(episodesLoadedMsg{podcastID: podcast.ID, episodes: episodes})
	current = updated.(Model)

	current.epList.Select(1)
	current.selectedEpisode = &current.episodes[1]

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	if current.selectedEpisode == nil || current.selectedEpisode.ID != 3 {
		t.Fatalf("expected selected episode ID 3 (same as before sort), got %d", current.selectedEpisode.ID)
	}

	updated, _ = current.Update(keyMsg("s", 's'))
	current = updated.(Model)

	if current.selectedEpisode == nil || current.selectedEpisode.ID != 3 {
		t.Fatalf("expected selected episode ID 3 after second toggle, got %d", current.selectedEpisode.ID)
	}
}
