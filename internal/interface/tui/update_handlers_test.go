package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
)

// ── podcastsLoadedMsg ───────────────────────────────────────────────

func TestUpdatePodcastsLoadedSuccessSetsItems(t *testing.T) {
	model := newTestModel(t)
	model.loadingLibrary = true

	podcasts := []domain.Podcast{
		{ID: 1, Title: "Pod A", FeedURL: "https://a.com/feed.xml"},
		{ID: 2, Title: "Pod B", FeedURL: "https://b.com/feed.xml"},
	}

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: podcasts})
	m := updated.(Model)

	if m.loadingLibrary {
		t.Fatal("expected loadingLibrary to be false")
	}
	if m.selectedPodcast == nil {
		t.Fatal("expected a selected podcast")
	}
	if m.status != "Loaded 2 podcasts" {
		t.Fatalf("expected status 'Loaded 2 podcasts', got %q", m.status)
	}
}

func TestUpdatePodcastsLoadedErrorSetsErrorStatus(t *testing.T) {
	model := newTestModel(t)
	model.loadingLibrary = true

	updated, _ := model.Update(podcastsLoadedMsg{err: fmt.Errorf("network timeout")})
	m := updated.(Model)

	if m.loadingLibrary {
		t.Fatal("expected loadingLibrary to be false")
	}
	if m.status != "Failed to load podcasts" {
		t.Fatalf("expected 'Failed to load podcasts', got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

func TestUpdatePodcastsLoadedEmptyLibrary(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{}})
	m := updated.(Model)

	if m.selectedPodcast != nil {
		t.Fatal("expected nil selectedPodcast for empty library")
	}
	if !strings.Contains(m.status, "Library is empty") {
		t.Fatalf("expected 'Library is empty' status, got %q", m.status)
	}
}

// ── episodesLoadedMsg ───────────────────────────────────────────────

func TestUpdateEpisodesLoadedSuccessPopulatesEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	episodes := []domain.Episode{
		{ID: 10, PodcastID: 1, Title: "Ep 1"},
		{ID: 11, PodcastID: 1, Title: "Ep 2"},
	}
	updated, _ = m.Update(episodesLoadedMsg{podcastID: 1, episodes: episodes})
	m = updated.(Model)

	if m.loadingDetail {
		t.Fatal("expected loadingDetail to be false")
	}
	if len(m.episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(m.episodes))
	}
	if m.selectedEpisode != nil {
		t.Fatal("expected nil selectedEpisode after load")
	}
}

func TestUpdateEpisodesLoadedErrorClearsEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 1,
		episodes:  []domain.Episode{{ID: 10, PodcastID: 1, Title: "Ep 1"}},
	})
	m = updated.(Model)

	// Now send error for same podcast
	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 1,
		err:       fmt.Errorf("load failed"),
	})
	m = updated.(Model)

	if m.episodes != nil {
		t.Fatal("expected nil episodes after error")
	}
	if m.status != "Failed to load episodes" {
		t.Fatalf("expected 'Failed to load episodes', got %q", m.status)
	}
}

func TestUpdateEpisodesLoadedMismatchedPodcastIgnored(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	// Load episodes for the right podcast first
	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 1,
		episodes:  []domain.Episode{{ID: 10, PodcastID: 1, Title: "Real"}},
	})
	m = updated.(Model)

	// Send episodes for a different podcast - should be ignored
	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 999,
		episodes:  []domain.Episode{{ID: 20, PodcastID: 999, Title: "Wrong"}},
	})
	m = updated.(Model)

	if len(m.episodes) != 1 {
		t.Fatalf("expected episodes to remain 1, got %d", len(m.episodes))
	}
}

func TestUpdateEpisodesLoadedStalePodcastIgnored(t *testing.T) {
	model := newTestModel(t)
	// No selectedPodcast set - episodes should be ignored
	updated, _ := model.Update(episodesLoadedMsg{
		podcastID: 1,
		episodes:  []domain.Episode{{ID: 10, PodcastID: 1, Title: "Ep 1"}},
	})
	m := updated.(Model)

	if len(m.episodes) != 0 {
		t.Fatalf("expected 0 episodes when no podcast selected, got %d", len(m.episodes))
	}
}

// ── podcastAddedMsg ─────────────────────────────────────────────────

func TestUpdatePodcastAddedSuccessResetsToAddState(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.submitting = true

	podcast := &domain.Podcast{Title: "New Show", FeedURL: "https://new.com/f.xml"}
	updated, _ := model.Update(podcastAddedMsg{podcast: podcast})
	m := updated.(Model)

	if m.submitting {
		t.Fatal("expected submitting to be false")
	}
	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if !m.loadingLibrary {
		t.Fatal("expected loadingLibrary to be true after successful add")
	}
	if m.status != "Added New Show" {
		t.Fatalf("expected 'Added New Show', got %q", m.status)
	}
}

func TestUpdatePodcastAddedErrorSetsErrorStatus(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.submitting = true

	updated, _ := model.Update(podcastAddedMsg{err: fmt.Errorf("parse error")})
	m := updated.(Model)

	if m.submitting {
		t.Fatal("expected submitting to be false")
	}
	if !strings.Contains(m.status, "Add failed") {
		t.Fatalf("expected 'Add failed' status, got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

// ── podcastRefreshedMsg ─────────────────────────────────────────────

func TestUpdatePodcastRefreshedSuccessReloadsData(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 1,
		episodes:  []domain.Episode{{ID: 10, PodcastID: 1, Title: "Ep 1"}},
	})
	m = updated.(Model)

	updated, _ = m.Update(podcastRefreshedMsg{podcastID: 1, newCount: 3})
	m = updated.(Model)

	if !m.loadingLibrary {
		t.Fatal("expected loadingLibrary to be true")
	}
	if !m.loadingDetail {
		t.Fatal("expected loadingDetail to be true")
	}
	if m.status != "Added 3 new episodes" {
		t.Fatalf("expected 'Added 3 new episodes', got %q", m.status)
	}
}

func TestUpdatePodcastRefreshedZeroNewEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	updated, _ = m.Update(podcastRefreshedMsg{podcastID: 1, newCount: 0})
	m = updated.(Model)

	if m.status != "No new episodes" {
		t.Fatalf("expected 'No new episodes', got %q", m.status)
	}
}

func TestUpdatePodcastRefreshedSingleEpisodeNoSuffix(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	updated, _ = m.Update(podcastRefreshedMsg{podcastID: 1, newCount: 1})
	m = updated.(Model)

	if m.status != "Added 1 new episode" {
		t.Fatalf("expected 'Added 1 new episode' (no 's'), got %q", m.status)
	}
}

func TestUpdatePodcastRefreshedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(podcastRefreshedMsg{
		podcastID: 1,
		err:       fmt.Errorf("feed unavailable"),
	})
	m := updated.(Model)

	if m.loadingDetail {
		t.Fatal("expected loadingDetail to be false")
	}
	if !strings.Contains(m.status, "Refresh failed") {
		t.Fatalf("expected 'Refresh failed' status, got %q", m.status)
	}
}

// ── allPodcastsSyncedMsg ────────────────────────────────────────────

func TestUpdateAllPodcastsSyncedSuccess(t *testing.T) {
	model := newTestModel(t)

	result := application.RefreshAllResult{
		TotalPodcasts: 5,
		Refreshed:     4,
		Failed:        1,
		NewEpisodes:   12,
		Failures: []application.RefreshFailure{
			{PodcastID: 3, Title: "Bad Feed", FeedURL: "https://bad.com/f.xml", Err: fmt.Errorf("timeout")},
		},
	}

	updated, _ := model.Update(allPodcastsSyncedMsg{result: result, reason: "periodic"})
	m := updated.(Model)

	if m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to be false")
	}
	if !m.loadingLibrary {
		t.Fatal("expected loadingLibrary to be true")
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error' due to failures, got %q", m.kind)
	}
	if !strings.Contains(m.status, "12 new episodes") {
		t.Fatalf("expected '12 new episodes' in status, got %q", m.status)
	}
	if !strings.Contains(m.status, "4/5") {
		t.Fatalf("expected '4/5' in status, got %q", m.status)
	}
	if !strings.Contains(m.status, "1 failed") {
		t.Fatalf("expected '1 failed' in status, got %q", m.status)
	}
}

func TestUpdateAllPodcastsSyncedStartupReason(t *testing.T) {
	model := newTestModel(t)

	result := application.RefreshAllResult{
		TotalPodcasts: 3,
		Refreshed:     3,
		Failed:        0,
		NewEpisodes:   0,
	}

	updated, _ := model.Update(allPodcastsSyncedMsg{result: result, reason: "startup"})
	m := updated.(Model)

	if !strings.Contains(m.status, "Startup sync complete") {
		t.Fatalf("expected 'Startup sync complete' in status, got %q", m.status)
	}
}

func TestUpdateAllPodcastsSyncedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(allPodcastsSyncedMsg{err: fmt.Errorf("sync failed")})
	m := updated.(Model)

	if m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to be false")
	}
	if !strings.Contains(m.status, "Sync failed") {
		t.Fatalf("expected 'Sync failed' status, got %q", m.status)
	}
}

func TestUpdateAllPodcastsSyncedWithSelectedPodcastLoadsEpisodes(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	result := application.RefreshAllResult{
		TotalPodcasts: 1,
		Refreshed:     1,
		NewEpisodes:   0,
	}
	updated, _ = m.Update(allPodcastsSyncedMsg{result: result, reason: ""})
	m = updated.(Model)

	if !m.loadingDetail {
		t.Fatal("expected loadingDetail to be true when selectedPodcast exists")
	}
}

// ── settingsPersistedMsg ────────────────────────────────────────────

func TestUpdateSettingsPersistedSuccess(t *testing.T) {
	model := newTestModel(t)

	next := Settings{
		AutoSyncOnStartup: true,
		PeriodicSync:      true,
		PeriodicSyncMins:  30,
		DiscordClientID:   "12345",
	}
	previous := Settings{PeriodicSyncMins: 60}

	updated, _ := model.Update(settingsPersistedMsg{settings: next, previous: previous})
	m := updated.(Model)

	if m.settings != next {
		t.Fatal("expected settings to be updated")
	}
	if m.status != "Settings saved" {
		t.Fatalf("expected 'Settings saved', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
	// When periodic sync is enabled, nextPeriodicSyncAt should be set
	if m.nextPeriodicSyncAt.IsZero() {
		t.Fatal("expected nextPeriodicSyncAt to be set")
	}
}

func TestUpdateSettingsPersistedPeriodicSyncDisabled(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSync = true

	next := Settings{PeriodicSync: false, PeriodicSyncMins: 60}
	previous := Settings{PeriodicSync: true, PeriodicSyncMins: 60}

	updated, _ := model.Update(settingsPersistedMsg{settings: next, previous: previous})
	m := updated.(Model)

	if !m.nextPeriodicSyncAt.IsZero() {
		t.Fatal("expected nextPeriodicSyncAt to be zero when periodic sync disabled")
	}
}

func TestUpdateSettingsPersistedErrorReverts(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSyncMins = 60

	next := Settings{PeriodicSyncMins: 30}
	previous := Settings{PeriodicSyncMins: 60}

	updated, _ := model.Update(settingsPersistedMsg{
		settings: next,
		previous: previous,
		err:      fmt.Errorf("write failed"),
	})
	m := updated.(Model)

	if m.settings.PeriodicSyncMins != 60 {
		t.Fatalf("expected settings to revert to previous (60), got %d", m.settings.PeriodicSyncMins)
	}
	if !strings.Contains(m.status, "Settings save failed") {
		t.Fatalf("expected 'Settings save failed' in status, got %q", m.status)
	}
}

// ── errMsg ──────────────────────────────────────────────────────────

func TestUpdateErrMsgSetsErrorStatus(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(errMsg{err: fmt.Errorf("something broke")})
	m := updated.(Model)

	if m.status != "something broke" {
		t.Fatalf("expected status 'something broke', got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

// ── downloadQueuedMsg ───────────────────────────────────────────────

func TestUpdateDownloadQueuedSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadQueuedMsg{episodeID: 5})
	m := updated.(Model)

	if m.status != "Episode queued for download" {
		t.Fatalf("expected 'Episode queued for download', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdateDownloadQueuedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadQueuedMsg{
		episodeID: 5,
		err:       fmt.Errorf("queue full"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Download queue failed") {
		t.Fatalf("expected 'Download queue failed' in status, got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

// ── downloadJobsLoadedMsg ───────────────────────────────────────────

func TestUpdateDownloadJobsLoadedSuccess(t *testing.T) {
	model := newTestModel(t)

	jobs := []application.DownloadJobView{
		{
			DownloadJob: domain.DownloadJob{
				ID:          1,
				EpisodeID:   10,
				Status:      domain.DownloadStatusQueued,
				BytesTotal:  1000,
			},
			EpisodeTitle: "Episode 1",
			PodcastTitle: "Podcast 1",
		},
	}

	updated, _ := model.Update(downloadJobsLoadedMsg{jobs: jobs})
	m := updated.(Model)

	if len(m.downloadJobs) != 1 {
		t.Fatalf("expected 1 download job, got %d", len(m.downloadJobs))
	}
	items := m.queueList.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
}

func TestUpdateDownloadJobsLoadedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadJobsLoadedMsg{err: fmt.Errorf("db error")})
	m := updated.(Model)

	if m.status != "Failed to load downloads" {
		t.Fatalf("expected 'Failed to load downloads', got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

func TestUpdateDownloadJobsLoadedEmpty(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadJobsLoadedMsg{jobs: []application.DownloadJobView{}})
	m := updated.(Model)

	if len(m.downloadJobs) != 0 {
		t.Fatalf("expected 0 download jobs, got %d", len(m.downloadJobs))
	}
	items := m.queueList.Items()
	if len(items) != 0 {
		t.Fatalf("expected 0 queue items, got %d", len(items))
	}
}

// ── downloadStartedMsg ──────────────────────────────────────────────

func TestUpdateDownloadStartedSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadStartedMsg{jobID: 1})
	m := updated.(Model)

	if m.status != "Download started" {
		t.Fatalf("expected 'Download started', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdateDownloadStartedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadStartedMsg{
		jobID: 1,
		err:   fmt.Errorf("permission denied"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Start failed") {
		t.Fatalf("expected 'Start failed' in status, got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

// ── downloadRetriedMsg ──────────────────────────────────────────────

func TestUpdateDownloadRetriedSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadRetriedMsg{jobID: 1})
	m := updated.(Model)

	if m.status != "Download retry started" {
		t.Fatalf("expected 'Download retry started', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdateDownloadRetriedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(downloadRetriedMsg{
		jobID: 1,
		err:   fmt.Errorf("not found"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Retry failed") {
		t.Fatalf("expected 'Retry failed' in status, got %q", m.status)
	}
}

// ── episodePlayedMsg ────────────────────────────────────────────────

func TestUpdateEpisodePlayedSuccessSetsCurrentEpisode(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	episodes := []domain.Episode{
		{ID: 10, PodcastID: 1, Title: "Ep 1", AudioURL: "https://a.com/ep1.mp3"},
	}
	updated, _ = m.Update(episodesLoadedMsg{podcastID: 1, episodes: episodes})
	m = updated.(Model)

	// Select the episode
	m.epList.Select(0)
	m.selectedEpisode = &m.episodes[0]

	updated, _ = m.Update(episodePlayedMsg{episodeID: 10})
	m = updated.(Model)

	if m.status != "Playing episode" {
		t.Fatalf("expected 'Playing episode', got %q", m.status)
	}
	if m.currentEpisode == nil {
		t.Fatal("expected currentEpisode to be set")
	}
}

func TestUpdateEpisodePlayedSuccessFallsBackToSelectedEpisode(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	episodes := []domain.Episode{
		{ID: 10, PodcastID: 1, Title: "Ep 1", AudioURL: "https://a.com/ep1.mp3"},
	}
	updated, _ = m.Update(episodesLoadedMsg{podcastID: 1, episodes: episodes})
	m = updated.(Model)

	m.epList.Select(0)
	m.selectedEpisode = &m.episodes[0]

	// Play with a mismatched episodeID - should fall back to selectedEpisode
	updated, _ = m.Update(episodePlayedMsg{episodeID: 999})
	m = updated.(Model)

	if m.currentEpisode == nil {
		t.Fatal("expected currentEpisode to be set from selectedEpisode fallback")
	}
}

func TestUpdateEpisodePlayedSuccessSetsCurrentPodcast(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	m.selectedPodcast = &podcast
	episodes := []domain.Episode{
		{ID: 10, PodcastID: 1, Title: "Ep 1"},
	}
	updated, _ = m.Update(episodesLoadedMsg{podcastID: 1, episodes: episodes})
	m = updated.(Model)
	m.epList.Select(0)
	m.selectedEpisode = &m.episodes[0]

	updated, _ = m.Update(episodePlayedMsg{episodeID: 10})
	m = updated.(Model)

	if m.currentPodcast == nil {
		t.Fatal("expected currentPodcast to be set")
	}
}

func TestUpdateEpisodePlayedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(episodePlayedMsg{
		episodeID: 1,
		err:       fmt.Errorf("file not found"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Playback failed") {
		t.Fatalf("expected 'Playback failed' in status, got %q", m.status)
	}
	if m.kind != "error" {
		t.Fatalf("expected kind 'error', got %q", m.kind)
	}
}

// ── playbackStatusMsg ───────────────────────────────────────────────

func TestUpdatePlaybackStatusSuccess(t *testing.T) {
	model := newTestModel(t)

	status := domain.PlaybackStatus{
		State:       domain.PlaybackStatePlaying,
		PositionSec: 45.5,
		DurationSec: 300,
		Source:      "https://example.com/audio.mp3",
	}

	updated, _ := model.Update(playbackStatusMsg{status: status})
	m := updated.(Model)

	if m.playbackStatus.State != domain.PlaybackStatePlaying {
		t.Fatalf("expected state 'playing', got %q", m.playbackStatus.State)
	}
	if m.playbackStatus.PositionSec != 45.5 {
		t.Fatalf("expected position 45.5, got %f", m.playbackStatus.PositionSec)
	}
}

func TestUpdatePlaybackStatusErrorIgnored(t *testing.T) {
	model := newTestModel(t)
	model.playbackStatus.PositionSec = 10

	updated, _ := model.Update(playbackStatusMsg{err: fmt.Errorf("unavailable")})
	m := updated.(Model)

	// Status should remain unchanged on error
	if m.playbackStatus.PositionSec != 10 {
		t.Fatal("expected playbackStatus to remain unchanged on error")
	}
}

func TestUpdatePlaybackStatusMatchesEpisodeBySource(t *testing.T) {
	model := newTestModel(t)
	podcast := domain.Podcast{ID: 1, Title: "Test", FeedURL: "https://a.com/f.xml"}
	updated, _ := model.Update(podcastsLoadedMsg{podcasts: []domain.Podcast{podcast}})
	m := updated.(Model)

	episodes := []domain.Episode{
		{ID: 10, PodcastID: 1, Title: "Ep 1", AudioURL: "https://a.com/ep1.mp3"},
	}
	updated, _ = m.Update(episodesLoadedMsg{podcastID: 1, episodes: episodes})
	m = updated.(Model)

	status := domain.PlaybackStatus{
		State:  domain.PlaybackStatePlaying,
		Source: "https://a.com/ep1.mp3",
	}

	updated, _ = m.Update(playbackStatusMsg{status: status})
	m = updated.(Model)

	if m.currentEpisode == nil {
		t.Fatal("expected currentEpisode to be matched by audio source")
	}
	if m.currentEpisode.ID != 10 {
		t.Fatalf("expected currentEpisode ID 10, got %d", m.currentEpisode.ID)
	}
}

// ── playbackToggledMsg ──────────────────────────────────────────────

func TestUpdatePlaybackToggledSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackToggledMsg{})
	m := updated.(Model)

	if m.status != "Playback toggled" {
		t.Fatalf("expected 'Playback toggled', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdatePlaybackToggledError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackToggledMsg{err: fmt.Errorf("mpv crashed")})
	m := updated.(Model)

	if !strings.Contains(m.status, "Playback toggle failed") {
		t.Fatalf("expected 'Playback toggle failed' in status, got %q", m.status)
	}
}

// ── playbackSkippedMsg ──────────────────────────────────────────────

func TestUpdatePlaybackSkippedForwardSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackSkippedMsg{seconds: 15})
	m := updated.(Model)

	if m.status != "Skipped forward 15s" {
		t.Fatalf("expected 'Skipped forward 15s', got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdatePlaybackSkippedBackSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackSkippedMsg{seconds: -15})
	m := updated.(Model)

	if m.status != "Skipped back 15s" {
		t.Fatalf("expected 'Skipped back 15s', got %q", m.status)
	}
}

func TestUpdatePlaybackSkippedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackSkippedMsg{
		seconds: 15,
		err:     fmt.Errorf("cannot seek"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Seek failed") {
		t.Fatalf("expected 'Seek failed' in status, got %q", m.status)
	}
}

// ── playbackSeekedMsg ───────────────────────────────────────────────

func TestUpdatePlaybackSeekedSuccess(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackSeekedMsg{positionSec: 125})
	m := updated.(Model)

	if !strings.Contains(m.status, "Seeked to") {
		t.Fatalf("expected 'Seeked to' in status, got %q", m.status)
	}
	if m.kind != "success" {
		t.Fatalf("expected kind 'success', got %q", m.kind)
	}
}

func TestUpdatePlaybackSeekedError(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(playbackSeekedMsg{
		positionSec: 90,
		err:         fmt.Errorf("no seek support"),
	})
	m := updated.(Model)

	if !strings.Contains(m.status, "Seek failed") {
		t.Fatalf("expected 'Seek failed' in status, got %q", m.status)
	}
}

// ── tickMsg ─────────────────────────────────────────────────────────

func TestUpdateTickCmdKeepsTicking(t *testing.T) {
	model := newTestModel(t)

	_, cmd := model.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected a command to keep the tick running")
	}
}

// ── Window size ─────────────────────────────────────────────────────

func TestUpdateWindowSizeUpdatesDimensions(t *testing.T) {
	model := newTestModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(Model)

	if m.width != 120 || m.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

// ── Integration: full lifecycle ─────────────────────────────────────

func TestUpdateFullLifecyclePodcastAddThenEpisodes(t *testing.T) {
	model := newTestModel(t)

	// Load podcasts
	updated, _ := model.Update(podcastsLoadedMsg{
		podcasts: []domain.Podcast{
			{ID: 1, Title: "Show", FeedURL: "https://example.com/feed.xml"},
		},
	})
	m := updated.(Model)

	if m.selectedPodcast == nil || m.selectedPodcast.ID != 1 {
		t.Fatal("expected podcast 1 to be selected")
	}

	// Load episodes
	updated, _ = m.Update(episodesLoadedMsg{
		podcastID: 1,
		episodes: []domain.Episode{
			{ID: 10, PodcastID: 1, Title: "Ep 1"},
			{ID: 11, PodcastID: 1, Title: "Ep 2"},
		},
	})
	m = updated.(Model)

	if len(m.episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(m.episodes))
	}

	// Simulate adding a podcast - transitions state
	model.state = stateAddPodcast
	model.submitting = true
	updated, _ = model.Update(podcastAddedMsg{
		podcast: &domain.Podcast{Title: "New Show", FeedURL: "https://new.com/f.xml"},
	})
	m = updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q after add, got %q", stateBrowse, m.state)
	}
}

func TestUpdateDownloadJobLifecycle(t *testing.T) {
	model := newTestModel(t)
	model.state = stateDownloads

	// Load download jobs
	jobs := []application.DownloadJobView{
		{
			DownloadJob: domain.DownloadJob{
				ID:         1,
				EpisodeID:  10,
				Status:     domain.DownloadStatusQueued,
				BytesTotal: 1000,
			},
			EpisodeTitle: "Ep 1",
			PodcastTitle: "Pod 1",
		},
	}
	updated, _ := model.Update(downloadJobsLoadedMsg{jobs: jobs})
	m := updated.(Model)

	if len(m.downloadJobs) != 1 {
		t.Fatalf("expected 1 download job, got %d", len(m.downloadJobs))
	}

	// Start download
	updated, _ = m.Update(downloadStartedMsg{jobID: 1})
	m = updated.(Model)

	if m.status != "Download started" {
		t.Fatalf("expected 'Download started', got %q", m.status)
	}

	// Retry download
	updated, _ = m.Update(downloadRetriedMsg{jobID: 1})
	m = updated.(Model)

	if m.status != "Download retry started" {
		t.Fatalf("expected 'Download retry started', got %q", m.status)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────

func newTestModelWithRepo(t *testing.T, repo *persistence.SQLiteRepo) Model {
	t.Helper()

	podcastService := application.NewPodcastService(repo, repo, repo, tuiMockFeedParser{}, nil)
	downloadService := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	mockPlayer := &tuiMockPlayer{}
	playerService := application.NewPlayerService(repo, repo, mockPlayer, nil, nil)
	settings := Settings{PeriodicSyncMins: 60}
	save := func(Settings) error { return nil }
	return NewModel(context.Background(), podcastService, downloadService, playerService, settings, save, "")
}

// ── Integration: seeded repo download lifecycle ─────────────────────

func TestUpdateDownloadLifecycleWithSeededRepo(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	// Seed data
	podcast := &domain.Podcast{Title: "Test Pod", FeedURL: "https://example.com/p.xml"}
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save podcast: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "Test Episode",
		AudioURL:  "https://example.com/ep.mp3",
	}
	if err := repo.SaveEpisode(context.Background(), episode); err != nil {
		t.Fatalf("SaveEpisode: %v", err)
	}
	job := &domain.DownloadJob{
		EpisodeID: episode.ID,
		Status:    domain.DownloadStatusQueued,
	}
	if err := repo.SaveDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("SaveDownloadJob: %v", err)
	}

	model := newTestModelWithRepo(t, repo)
	model.state = stateDownloads

	// Load jobs
	downloadService := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	jobs, err := downloadService.ListJobsWithEpisodes(context.Background())
	if err != nil {
		t.Fatalf("ListJobsWithEpisodes: %v", err)
	}

	updated, _ := model.Update(downloadJobsLoadedMsg{jobs: jobs})
	m := updated.(Model)

	if len(m.downloadJobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(m.downloadJobs))
	}
	if m.downloadJobs[0].EpisodeTitle != "Test Episode" {
		t.Fatalf("expected 'Test Episode', got %q", m.downloadJobs[0].EpisodeTitle)
	}
}

// ── AllPodcastsSynced with failures ─────────────────────────────────

func TestUpdateAllPodcastsSyncedMultipleFailures(t *testing.T) {
	model := newTestModel(t)

	result := application.RefreshAllResult{
		TotalPodcasts: 5,
		Refreshed:     3,
		Failed:        2,
		NewEpisodes:   7,
		Failures: []application.RefreshFailure{
			{PodcastID: 2, Title: "Feed A", Err: fmt.Errorf("404 not found")},
			{PodcastID: 5, Title: "Feed B", Err: fmt.Errorf("timeout")},
		},
	}

	updated, _ := model.Update(allPodcastsSyncedMsg{result: result, reason: ""})
	m := updated.(Model)

	if !strings.Contains(m.status, "7 new episodes") {
		t.Fatalf("expected '7 new episodes' in status, got %q", m.status)
	}
	if !strings.Contains(m.status, "2 failed") {
		t.Fatalf("expected '2 failed' in status, got %q", m.status)
	}
	if !strings.Contains(m.status, "Feed A") {
		t.Fatalf("expected 'Feed A' in failure summary, got %q", m.status)
	}
}

func TestUpdateAllPodcastsSyncedZeroPodcasts(t *testing.T) {
	model := newTestModel(t)

	result := application.RefreshAllResult{
		TotalPodcasts: 0,
		Refreshed:     0,
		Failed:        0,
		NewEpisodes:   0,
	}

	updated, _ := model.Update(allPodcastsSyncedMsg{result: result, reason: ""})
	m := updated.(Model)

	if !strings.Contains(m.status, "0 new episodes") {
		t.Fatalf("expected '0 new episodes' in status, got %q", m.status)
	}
}

// ── formatPlaybackTime ──────────────────────────────────────────────

func TestFormatPlaybackTime(t *testing.T) {
	tests := map[float64]string{
		0:     "0:00",
		30:    "0:30",
		60:    "1:00",
		90:    "1:30",
		3600:  "1:00:00",
		3661:  "1:01:01",
		-10:   "0:00",
		125.5: "2:06",
	}

	for input, want := range tests {
		got := formatPlaybackTime(input)
		if got != want {
			t.Errorf("formatPlaybackTime(%v) = %q, want %q", input, got, want)
		}
	}
}

// ── suffix ──────────────────────────────────────────────────────────

func TestSuffix(t *testing.T) {
	if s := suffix(1); s != "" {
		t.Errorf("suffix(1) = %q, want empty", s)
	}
	if s := suffix(2); s != "s" {
		t.Errorf("suffix(2) = %q, want 's'", s)
	}
}

// ── onOff ───────────────────────────────────────────────────────────

func TestOnOff(t *testing.T) {
	if v := onOff(true); v != "ON" {
		t.Errorf("onOff(true) = %q, want 'ON'", v)
	}
	if v := onOff(false); v != "OFF" {
		t.Errorf("onOff(false) = %q, want 'OFF'", v)
	}
}

// ── valueOrPlaceholder ──────────────────────────────────────────────

func TestValueOrPlaceholder(t *testing.T) {
	if v := valueOrPlaceholder("hello"); v != "hello" {
		t.Errorf("valueOrPlaceholder('hello') = %q, want 'hello'", v)
	}
	if v := valueOrPlaceholder(""); v != "(not set)" {
		t.Errorf("valueOrPlaceholder('') = %q, want '(not set)'", v)
	}
	if v := valueOrPlaceholder("  "); v != "(not set)" {
		t.Errorf("valueOrPlaceholder('  ') = %q, want '(not set)'", v)
	}
}

// ── episodeTitleForView ─────────────────────────────────────────────

func TestEpisodeTitleForViewWithTitle(t *testing.T) {
	job := application.DownloadJobView{
		EpisodeTitle: "My Episode",
	}
	if got := episodeTitleForView(job); got != "My Episode" {
		t.Errorf("episodeTitleForView with title = %q, want 'My Episode'", got)
	}
}

func TestEpisodeTitleForViewWithoutTitle(t *testing.T) {
	job := application.DownloadJobView{
		DownloadJob: domain.DownloadJob{EpisodeID: 42},
	}
	if got := episodeTitleForView(job); got != "Episode #42" {
		t.Errorf("episodeTitleForView without title = %q, want 'Episode #42'", got)
	}
}

// ── findThemeIndex ──────────────────────────────────────────────────

func TestFindThemeIndex(t *testing.T) {
	themes := []string{"dark-red", "dark-blue", "light-green"}
	if idx := findThemeIndex("dark-blue", themes); idx != 1 {
		t.Errorf("findThemeIndex('dark-blue') = %d, want 1", idx)
	}
	if idx := findThemeIndex("missing", themes); idx != 0 {
		t.Errorf("findThemeIndex('missing') = %d, want 0 (default)", idx)
	}
}

// ── Time-dependent sync test ────────────────────────────────────────

func TestUpdatePeriodicSyncTriggersOnTick(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSync = true
	model.syncingAllFeeds = false
	model.nextPeriodicSyncAt = time.Now().Add(-1 * time.Minute) // already past
	model.loadingLibrary = false

	updated, cmd := model.Update(tickMsg{})
	m := updated.(Model)

	if !m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to be true after periodic sync trigger")
	}
	if cmd == nil {
		t.Fatal("expected a command for sync + tick")
	}
	if m.status != "Periodic sync started…" {
		t.Fatalf("expected 'Periodic sync started…', got %q", m.status)
	}
}

func TestUpdatePeriodicSyncDoesNotTriggerWhenNotEnabled(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSync = false
	model.syncingAllFeeds = false
	model.nextPeriodicSyncAt = time.Now().Add(-1 * time.Minute)

	updated, _ := model.Update(tickMsg{})
	m := updated.(Model)

	if m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to remain false when periodic sync disabled")
	}
}

func TestUpdatePeriodicSyncDoesNotTriggerWhenSyncing(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSync = true
	model.syncingAllFeeds = true
	model.nextPeriodicSyncAt = time.Now().Add(-1 * time.Minute)

	updated, _ := model.Update(tickMsg{})
	m := updated.(Model)

	// syncingAllFeeds should remain true (no new sync started)
	if !m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to remain true while already syncing")
	}
}

func TestUpdatePeriodicSyncDoesNotTriggerBeforeTime(t *testing.T) {
	model := newTestModel(t)
	model.settings.PeriodicSync = true
	model.syncingAllFeeds = false
	model.nextPeriodicSyncAt = time.Now().Add(10 * time.Minute) // future

	updated, _ := model.Update(tickMsg{})
	m := updated.(Model)

	if m.syncingAllFeeds {
		t.Fatal("expected syncingAllFeeds to remain false before scheduled time")
	}
}

// ── handleGoToEpisodeMode ─────────────────────────────────────────────

func TestHandleGoToEpisodeModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateGoToEpisode
	model.goToInput.SetValue("3")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if !strings.Contains(m.status, "Go to episode cancelled") {
		t.Fatalf("expected 'Go to episode cancelled', got %q", m.status)
	}
}

func TestHandleGoToEpisodeModeSubmitEmpty(t *testing.T) {
	model := newTestModel(t)
	model.state = stateGoToEpisode
	model.goToInput.SetValue("")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.state != stateGoToEpisode {
		t.Fatalf("expected state to remain %q, got %q", stateGoToEpisode, m.state)
	}
	if m.status != "Episode number is required" {
		t.Fatalf("expected 'Episode number is required', got %q", m.status)
	}
}

func TestHandleGoToEpisodeModeSubmitInvalid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateGoToEpisode
	model.goToInput.SetValue("abc")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.status != "Invalid episode number" {
		t.Fatalf("expected 'Invalid episode number', got %q", m.status)
	}
}

func TestHandleGoToEpisodeModeSubmitOutOfRange(t *testing.T) {
	model := newTestModel(t)
	model.state = stateGoToEpisode
	model.episodes = []domain.Episode{
		{ID: 1, Title: "Ep 1"},
		{ID: 2, Title: "Ep 2"},
	}
	model.goToInput.SetValue("5")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !strings.Contains(m.status, "out of range") {
		t.Fatalf("expected 'out of range' in status, got %q", m.status)
	}
}

func TestHandleGoToEpisodeModeSubmitValid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateGoToEpisode
	model.episodes = []domain.Episode{
		{ID: 1, Title: "Ep 1"},
		{ID: 2, Title: "Ep 2"},
	}
	model.goToInput.SetValue("2")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if m.selectedEpisode == nil || m.selectedEpisode.ID != 2 {
		t.Fatal("expected selectedEpisode to be set to episode 2")
	}
	if m.status != "Selected episode 2" {
		t.Fatalf("expected 'Selected episode 2', got %q", m.status)
	}
}

// ── handlePlayerSeekMode ──────────────────────────────────────────────

func TestHandlePlayerSeekModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayerSeek

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.state != statePlayer {
		t.Fatalf("expected state %q, got %q", statePlayer, m.state)
	}
	if m.status != "Seek cancelled" {
		t.Fatalf("expected 'Seek cancelled', got %q", m.status)
	}
}

func TestHandlePlayerSeekModeSubmitEmpty(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayerSeek
	model.seekInput.SetValue("")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.status != "Seek time is required" {
		t.Fatalf("expected 'Seek time is required', got %q", m.status)
	}
}

func TestHandlePlayerSeekModeSubmitInvalid(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayerSeek
	model.seekInput.SetValue("xyz")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !strings.Contains(m.status, "invalid") {
		t.Fatalf("expected 'invalid' in status, got %q", m.status)
	}
}

func TestHandlePlayerSeekModeSubmitValid(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayerSeek
	model.seekInput.SetValue("1:30")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.state != statePlayer {
		t.Fatalf("expected state %q, got %q", statePlayer, m.state)
	}
}

// ── handleDownloadsMode ───────────────────────────────────────────────

func TestHandleDownloadsModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateDownloads

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(*Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if m.status != "Returned to library" {
		t.Fatalf("expected 'Returned to library', got %q", m.status)
	}
}

func TestHandleDownloadsModeStartDownloadNoSelection(t *testing.T) {
	model := newTestModel(t)
	model.state = stateDownloads

	// 's' with no items in queue - should not panic
	updated, _ := model.Update(tea.KeyPressMsg{Code: 's'})
	_ = updated.(*Model)
	// No crash is success
}

func TestHandleDownloadsModeRetryDownloadNoSelection(t *testing.T) {
	model := newTestModel(t)
	model.state = stateDownloads

	// 'r' with no items in queue - should not panic
	updated, _ := model.Update(tea.KeyPressMsg{Code: 'r'})
	_ = updated.(*Model)
	// No crash is success
}

// ── handleSettingsMode ────────────────────────────────────────────────

func TestHandleSettingsModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if m.status != "Returned to library" {
		t.Fatalf("expected 'Returned to library', got %q", m.status)
	}
}

func TestHandleSettingsModeCursorDown(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 0

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'j'})
	m := updated.(Model)

	if m.settingsCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.settingsCursor)
	}
}

func TestHandleSettingsModeCursorUp(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 2

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'k'})
	m := updated.(Model)

	if m.settingsCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.settingsCursor)
	}
}

func TestHandleSettingsModeCursorDownAtMax(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 5

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'j'})
	m := updated.(Model)

	if m.settingsCursor != 5 {
		t.Fatalf("expected cursor to stay at 5, got %d", m.settingsCursor)
	}
}

func TestHandleSettingsModeCursorUpAtMin(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 0

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'k'})
	m := updated.(Model)

	if m.settingsCursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", m.settingsCursor)
	}
}

func TestHandleSettingsModeToggleAutoSync(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 0
	model.settings.AutoSyncOnStartup = false

	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeTogglePeriodicSync(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 1
	model.settings.PeriodicSync = false

	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeOpenIntervalEditor(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 2

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !m.editingInterval {
		t.Fatal("expected editingInterval to be true")
	}
}

func TestHandleSettingsModeToggleDiscordPresenceBlocked(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 3
	model.settings.DiscordPresence = false
	model.settings.DiscordClientID = ""

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.settings.DiscordPresence {
		t.Fatal("expected DiscordPresence to remain false when no client ID")
	}
	if !strings.Contains(m.status, "Set Discord client ID") {
		t.Fatalf("expected 'Set Discord client ID' in status, got %q", m.status)
	}
}

func TestHandleSettingsModeToggleDiscordPresence(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 3
	model.settings.DiscordPresence = false
	model.settings.DiscordClientID = "12345"

	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeOpenDiscordIDEditor(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 4

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !m.editingDiscordID {
		t.Fatal("expected editingDiscordID to be true")
	}
}

func TestHandleSettingsModeOpenThemeEditor(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.settingsCursor = 5

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !m.editingTheme {
		t.Fatal("expected editingTheme to be true")
	}
}

func TestHandleSettingsModeEditingIntervalClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingInterval = true
	model.settings.PeriodicSyncMins = 60

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.editingInterval {
		t.Fatal("expected editingInterval to be false after cancel")
	}
	if m.status != "Interval edit cancelled" {
		t.Fatalf("expected 'Interval edit cancelled', got %q", m.status)
	}
}

func TestHandleSettingsModeEditingIntervalSubmitInvalid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingInterval = true
	model.intervalInput.SetValue("abc")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.status != "Interval must be a positive integer" {
		t.Fatalf("expected 'Interval must be a positive integer', got %q", m.status)
	}
}

func TestHandleSettingsModeEditingIntervalSubmitValid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingInterval = true
	model.intervalInput.SetValue("30")

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.editingInterval {
		t.Fatal("expected editingInterval to be false after submit")
	}
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeEditingDiscordIDClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingDiscordID = true
	model.settings.DiscordClientID = "old"

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.editingDiscordID {
		t.Fatal("expected editingDiscordID to be false after cancel")
	}
	if m.status != "Discord client ID edit cancelled" {
		t.Fatalf("expected 'Discord client ID edit cancelled', got %q", m.status)
	}
}

func TestHandleSettingsModeEditingDiscordIDSubmitEmpty(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingDiscordID = true
	model.discordInput.SetValue("")
	model.settings.DiscordPresence = true

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !m.editingDiscordID {
		t.Fatal("expected editingDiscordID to remain true when empty with presence on")
	}
	if !strings.Contains(m.status, "required") {
		t.Fatalf("expected 'required' in status, got %q", m.status)
	}
}

func TestHandleSettingsModeEditingDiscordIDSubmitValid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingDiscordID = true
	model.discordInput.SetValue("99999")

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.editingDiscordID {
		t.Fatal("expected editingDiscordID to be false after submit")
	}
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeEditingThemeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.editingTheme {
		t.Fatal("expected editingTheme to be false after cancel")
	}
	if m.status != "Theme selection cancelled" {
		t.Fatalf("expected 'Theme selection cancelled', got %q", m.status)
	}
}

func TestHandleSettingsModeEditingThemeSubmit(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true
	model.selectedThemeIndex = 0
	model.themeList = []string{"dark-red", "dark-blue"}

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.editingTheme {
		t.Fatal("expected editingTheme to be false after submit")
	}
	if cmd == nil {
		t.Fatal("expected a persistSettings command to be returned")
	}
}

func TestHandleSettingsModeEditingThemeLeft(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true
	model.selectedThemeIndex = 2
	model.themeList = []string{"dark-red", "dark-blue", "dark-green"}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'h'})
	m := updated.(Model)

	if m.selectedThemeIndex != 1 {
		t.Fatalf("expected selectedThemeIndex 1, got %d", m.selectedThemeIndex)
	}
}

func TestHandleSettingsModeEditingThemeLeftAtMin(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true
	model.selectedThemeIndex = 0
	model.themeList = []string{"dark-red", "dark-blue"}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'h'})
	m := updated.(Model)

	if m.selectedThemeIndex != 0 {
		t.Fatalf("expected selectedThemeIndex to stay at 0, got %d", m.selectedThemeIndex)
	}
}

func TestHandleSettingsModeEditingThemeRight(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true
	model.selectedThemeIndex = 0
	model.themeList = []string{"dark-red", "dark-blue", "dark-green"}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'l'})
	m := updated.(Model)

	if m.selectedThemeIndex != 1 {
		t.Fatalf("expected selectedThemeIndex 1, got %d", m.selectedThemeIndex)
	}
}

func TestHandleSettingsModeEditingThemeRightAtMax(t *testing.T) {
	model := newTestModel(t)
	model.state = stateSettings
	model.editingTheme = true
	model.selectedThemeIndex = 1
	model.themeList = []string{"dark-red", "dark-blue"}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'l'})
	m := updated.(Model)

	if m.selectedThemeIndex != 1 {
		t.Fatalf("expected selectedThemeIndex to stay at 1, got %d", m.selectedThemeIndex)
	}
}

// ── handlePlayerMode ──────────────────────────────────────────────────

func TestHandlePlayerModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if m.status != "Returned to library" {
		t.Fatalf("expected 'Returned to library', got %q", m.status)
	}
}

func TestHandlePlayerModeToggleHelp(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer

	updated, _ := model.Update(tea.KeyPressMsg{Code: '?'})
	m := updated.(Model)

	if m.state != stateHelp {
		t.Fatalf("expected state %q, got %q", stateHelp, m.state)
	}
	if m.status != "Help page opened." {
		t.Fatalf("expected 'Help page opened.', got %q", m.status)
	}
}

func TestHandlePlayerModeSeekToTime(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer

	updated, _ := model.Update(tea.KeyPressMsg{Code: 't'})
	m := updated.(Model)

	if m.state != statePlayerSeek {
		t.Fatalf("expected state %q, got %q", statePlayerSeek, m.state)
	}
}

func TestHandlePlayerModeTogglePlayPauseStoppedNoEpisode(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStateStopped

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m := updated.(Model)

	if !strings.Contains(m.status, "No episode available to play") {
		t.Fatalf("expected 'No episode available to play' in status, got %q", m.status)
	}
}

func TestHandlePlayerModeTogglePlayPauseStoppedWithEpisode(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStateStopped
	model.selectedEpisode = &domain.Episode{ID: 10, AudioURL: "https://example.com/ep.mp3"}

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	_ = updated.(Model)
	// Should issue a playEpisode command - no crash is success
}

func TestHandlePlayerModeTogglePlayPausePlaying(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStatePlaying

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	_ = updated.(Model)
	// Should issue a togglePlayback command
}

func TestHandlePlayerModeSkipBackwardPlaying(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStatePlaying

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	_ = updated.(Model)
	// Should issue a skipPlayback command
}

func TestHandlePlayerModeSkipBackwardStopped(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStateStopped

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m := updated.(Model)

	if m.status != "No episode is playing" {
		t.Fatalf("expected 'No episode is playing', got %q", m.status)
	}
}

func TestHandlePlayerModeSkipForwardPlaying(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStatePlaying

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	_ = updated.(Model)
	// Should issue a skipPlayback command
}

func TestHandlePlayerModeSkipForwardStopped(t *testing.T) {
	model := newTestModel(t)
	model.state = statePlayer
	model.playbackStatus.State = domain.PlaybackStateStopped

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m := updated.(Model)

	if m.status != "No episode is playing" {
		t.Fatalf("expected 'No episode is playing', got %q", m.status)
	}
}

// ── handleAddMode ─────────────────────────────────────────────────────

func TestHandleAddModeClose(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.input.SetValue("https://example.com/feed.xml")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m := updated.(Model)

	if m.state != stateBrowse {
		t.Fatalf("expected state %q, got %q", stateBrowse, m.state)
	}
	if m.status != "Add podcast cancelled" {
		t.Fatalf("expected 'Add podcast cancelled', got %q", m.status)
	}
}

func TestHandleAddModeSubmitEmpty(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.input.SetValue("")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.state != stateAddPodcast {
		t.Fatalf("expected state to remain %q, got %q", stateAddPodcast, m.state)
	}
	if m.status != "Feed URL is required" {
		t.Fatalf("expected 'Feed URL is required', got %q", m.status)
	}
}

func TestHandleAddModeSubmitWhitespaceOnly(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.input.SetValue("   ")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if m.status != "Feed URL is required" {
		t.Fatalf("expected 'Feed URL is required', got %q", m.status)
	}
}

func TestHandleAddModeSubmitValid(t *testing.T) {
	model := newTestModel(t)
	model.state = stateAddPodcast
	model.input.SetValue("https://example.com/feed.xml")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := updated.(Model)

	if !m.submitting {
		t.Fatal("expected submitting to be true")
	}
	if m.status != "Fetching feed and saving episodes…" {
		t.Fatalf("expected 'Fetching feed and saving episodes…', got %q", m.status)
	}
}
