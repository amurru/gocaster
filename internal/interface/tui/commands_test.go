package tui

import (
	"context"
	"fmt"
	"testing"

	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
)

// TestLoadPodcastsCommandSuccess verifies loadPodcasts returns a
// podcastsLoadedMsg with no error when the repo is empty. The slice may be
// nil when no rows exist (Go zero-value behaviour).
func TestLoadPodcastsCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.loadPodcasts()()
	loaded, ok := msg.(podcastsLoadedMsg)
	if !ok {
		t.Fatalf("expected podcastsLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected no error, got %v", loaded.err)
	}
}

// TestLoadPodcastsCommandWithData verifies loadPodcasts returns podcasts
// that were seeded into the repo.
func TestLoadPodcastsCommandWithData(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	podcast := &domain.Podcast{Title: "Test", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save: %v", err)
	}

	model := newTestModel(t)
	// Replace with our seeded repo's service
	model.podcastService = newSeededPodcastService(repo)

	msg := model.loadPodcasts()()
	loaded, ok := msg.(podcastsLoadedMsg)
	if !ok {
		t.Fatalf("expected podcastsLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected no error, got %v", loaded.err)
	}
	if len(loaded.podcasts) != 1 {
		t.Fatalf("expected 1 podcast, got %d", len(loaded.podcasts))
	}
}

// TestLoadEpisodesCommandSuccess verifies loadEpisodes returns an
// episodesLoadedMsg with the correct podcastID.
func TestLoadEpisodesCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.loadEpisodes(42)()
	loaded, ok := msg.(episodesLoadedMsg)
	if !ok {
		t.Fatalf("expected episodesLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected no error, got %v", loaded.err)
	}
	if loaded.podcastID != 42 {
		t.Fatalf("expected podcastID 42, got %d", loaded.podcastID)
	}
}

func TestAddPodcastCommandSuccess(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	feedParser := tuiMockFeedParser{
		podcast: &domain.Podcast{Title: "New Show", FeedURL: "https://example.com/feed.xml"},
		episodes: []domain.Episode{
			{Title: "Ep 1", AudioURL: "https://example.com/1.mp3"},
		},
	}
	ps := application.NewPodcastService(repo, repo, repo, feedParser, nil)
	dl := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	model := NewModel(context.Background(), ps, dl, nil, nil, Settings{}, nil, "")

	msg := model.addPodcast("https://example.com/feed.xml")()
	added, ok := msg.(podcastAddedMsg)
	if !ok {
		t.Fatalf("expected podcastAddedMsg, got %T", msg)
	}
	if added.err != nil {
		t.Fatalf("expected no error, got %v", added.err)
	}
	if added.podcast == nil {
		t.Fatal("expected non-nil podcast")
	}
}

// TestRefreshPodcastCommandSuccess verifies refreshPodcast returns a
// podcastRefreshedMsg with the correct podcastID.
func TestRefreshPodcastCommandSuccess(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	podcast := &domain.Podcast{Title: "RefreshMe", FeedURL: "https://example.com/feed.xml"}
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save: %v", err)
	}

	feedParser := tuiMockFeedParser{
		episodes: []domain.Episode{
			{Title: "New Ep", AudioURL: "https://example.com/new.mp3"},
		},
	}
	ps := application.NewPodcastService(repo, repo, repo, feedParser, nil)
	dl := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	model := NewModel(context.Background(), ps, dl, nil, nil, Settings{}, nil, "")

	msg := model.refreshPodcast(podcast.ID)()
	refreshed, ok := msg.(podcastRefreshedMsg)
	if !ok {
		t.Fatalf("expected podcastRefreshedMsg, got %T", msg)
	}
	if refreshed.err != nil {
		t.Fatalf("expected no error, got %v", refreshed.err)
	}
	if refreshed.podcastID != podcast.ID {
		t.Fatalf("expected podcastID %d, got %d", podcast.ID, refreshed.podcastID)
	}
}

// TestSyncAllPodcastsCommandSuccess verifies syncAllPodcasts returns an
// allPodcastsSyncedMsg with the reason propagated.
func TestSyncAllPodcastsCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.syncAllPodcasts("periodic")()
	synced, ok := msg.(allPodcastsSyncedMsg)
	if !ok {
		t.Fatalf("expected allPodcastsSyncedMsg, got %T", msg)
	}
	if synced.err != nil {
		t.Fatalf("expected no error, got %v", synced.err)
	}
	if synced.reason != "periodic" {
		t.Fatalf("expected reason 'periodic', got %q", synced.reason)
	}
}

// TestSyncAllPodcastsCommandStartup verifies syncAllPodcasts with
// "startup" reason propagates it correctly.
func TestSyncAllPodcastsCommandStartup(t *testing.T) {
	model := newTestModel(t)

	msg := model.syncAllPodcasts("startup")()
	synced, ok := msg.(allPodcastsSyncedMsg)
	if !ok {
		t.Fatalf("expected allPodcastsSyncedMsg, got %T", msg)
	}
	if synced.reason != "startup" {
		t.Fatalf("expected reason 'startup', got %q", synced.reason)
	}
}

// TestPersistSettingsCommandWithNilSaver verifies persistSettings works
// when saveSettings is nil (no error returned).
func TestPersistSettingsCommandWithNilSaver(t *testing.T) {
	model := newTestModel(t)
	model.saveSettings = nil

	next := Settings{AutoSyncOnStartup: true, PeriodicSyncMins: 30}
	previous := Settings{AutoSyncOnStartup: false, PeriodicSyncMins: 60}

	msg := model.persistSettings(next, previous)()
	persisted, ok := msg.(settingsPersistedMsg)
	if !ok {
		t.Fatalf("expected settingsPersistedMsg, got %T", msg)
	}
	if persisted.err != nil {
		t.Fatalf("expected no error with nil saver, got %v", persisted.err)
	}
	if persisted.settings != next {
		t.Fatalf("expected settings to be next value")
	}
}

// TestPersistSettingsCommandWithSaverSuccess verifies persistSettings
// returns no error when the saver succeeds.
func TestPersistSettingsCommandWithSaverSuccess(t *testing.T) {
	model := newTestModel(t)
	model.saveSettings = func(s Settings) error { return nil }

	next := Settings{PeriodicSyncMins: 45}
	previous := Settings{PeriodicSyncMins: 60}

	msg := model.persistSettings(next, previous)()
	persisted, ok := msg.(settingsPersistedMsg)
	if !ok {
		t.Fatalf("expected settingsPersistedMsg, got %T", msg)
	}
	if persisted.err != nil {
		t.Fatalf("expected no error, got %v", persisted.err)
	}
}

// TestPersistSettingsCommandWithSaverError verifies persistSettings
// propagates errors from the saver.
func TestPersistSettingsCommandWithSaverError(t *testing.T) {
	model := newTestModel(t)
	model.saveSettings = func(s Settings) error { return fmt.Errorf("disk full") }

	next := Settings{PeriodicSyncMins: 45}
	previous := Settings{PeriodicSyncMins: 60}

	msg := model.persistSettings(next, previous)()
	persisted, ok := msg.(settingsPersistedMsg)
	if !ok {
		t.Fatalf("expected settingsPersistedMsg, got %T", msg)
	}
	if persisted.err == nil {
		t.Fatal("expected error from saver")
	}
	if persisted.err.Error() != "disk full" {
		t.Fatalf("expected 'disk full', got %q", persisted.err.Error())
	}
}

// TestLoadDownloadJobsCommandSuccess verifies loadDownloadJobs returns a
// downloadJobsLoadedMsg with no error when the repo is empty.
func TestLoadDownloadJobsCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.loadDownloadJobs()()
	loaded, ok := msg.(downloadJobsLoadedMsg)
	if !ok {
		t.Fatalf("expected downloadJobsLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected no error, got %v", loaded.err)
	}
}

func TestQueueDownloadCommandSuccess(t *testing.T) {
	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	podcast := &domain.Podcast{Title: "DLPod", FeedURL: "https://example.com/f.xml"}
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "Download Me",
		AudioURL:  "https://example.com/dl.mp3",
	}
	if err := repo.SaveEpisode(context.Background(), episode); err != nil {
		t.Fatalf("SaveEpisode: %v", err)
	}

	ps := application.NewPodcastService(repo, repo, repo, tuiMockFeedParser{}, nil)
	dl := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	model := NewModel(context.Background(), ps, dl, nil, nil, Settings{}, nil, "")

	msg := model.queueDownload(episode.ID)()
	queued, ok := msg.(downloadQueuedMsg)
	if !ok {
		t.Fatalf("expected downloadQueuedMsg, got %T", msg)
	}
	if queued.err != nil {
		t.Fatalf("expected no error, got %v", queued.err)
	}
	if queued.episodeID != episode.ID {
		t.Fatalf("expected episodeID %d, got %d", episode.ID, queued.episodeID)
	}
}

// TestStartDownloadCommandSuccess verifies startDownload returns a
// downloadStartedMsg. It may have an error if the job doesn't exist
// in the repo, but the message type is always correct.
func TestStartDownloadCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.startDownload(1)()
	started, ok := msg.(downloadStartedMsg)
	if !ok {
		t.Fatalf("expected downloadStartedMsg, got %T", msg)
	}
	if started.jobID != 1 {
		t.Fatalf("expected jobID 1, got %d", started.jobID)
	}
}

// TestRetryDownloadCommandSuccess verifies retryDownload returns a
// downloadRetriedMsg with the correct jobID.
func TestRetryDownloadCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.retryDownload(2)()
	retried, ok := msg.(downloadRetriedMsg)
	if !ok {
		t.Fatalf("expected downloadRetriedMsg, got %T", msg)
	}
	if retried.jobID != 2 {
		t.Fatalf("expected jobID 2, got %d", retried.jobID)
	}
}

// TestPlayEpisodeCommandSuccess verifies playEpisode returns an
// episodePlayedMsg with the correct episodeID.
func TestPlayEpisodeCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.playEpisode(5)()
	played, ok := msg.(episodePlayedMsg)
	if !ok {
		t.Fatalf("expected episodePlayedMsg, got %T", msg)
	}
	if played.episodeID != 5 {
		t.Fatalf("expected episodeID 5, got %d", played.episodeID)
	}
	// playEpisode calls playerService.PlayEpisode which will fail because
	// the episode doesn't exist in the repo, but the message type is correct
}

// TestPlayEpisodeCommandNilPlayer verifies playEpisode returns an error
// when playerService is nil.
func TestPlayEpisodeCommandNilPlayer(t *testing.T) {
	model := newTestModel(t)
	model.playerService = nil

	msg := model.playEpisode(1)()
	played, ok := msg.(episodePlayedMsg)
	if !ok {
		t.Fatalf("expected episodePlayedMsg, got %T", msg)
	}
	if played.err == nil {
		t.Fatal("expected error when player is nil")
	}
	if played.err.Error() != "player is unavailable" {
		t.Fatalf("expected 'player is unavailable', got %q", played.err.Error())
	}
}

// TestFetchPlaybackStatusCommandSuccess verifies fetchPlaybackStatus
// returns a playbackStatusMsg.
func TestFetchPlaybackStatusCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.fetchPlaybackStatus()()
	status, ok := msg.(playbackStatusMsg)
	if !ok {
		t.Fatalf("expected playbackStatusMsg, got %T", msg)
	}
	if status.err != nil {
		t.Fatalf("expected no error, got %v", status.err)
	}
}

// TestFetchPlaybackStatusCommandNilPlayer verifies fetchPlaybackStatus
// returns an error when playerService is nil.
func TestFetchPlaybackStatusCommandNilPlayer(t *testing.T) {
	model := newTestModel(t)
	model.playerService = nil

	msg := model.fetchPlaybackStatus()()
	status, ok := msg.(playbackStatusMsg)
	if !ok {
		t.Fatalf("expected playbackStatusMsg, got %T", msg)
	}
	if status.err == nil {
		t.Fatal("expected error when player is nil")
	}
	if status.err.Error() != "player is unavailable" {
		t.Fatalf("expected 'player is unavailable', got %q", status.err.Error())
	}
}

// TestTogglePlaybackCommandSuccess verifies togglePlayback returns a
// playbackToggledMsg.
func TestTogglePlaybackCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.togglePlayback()()
	toggled, ok := msg.(playbackToggledMsg)
	if !ok {
		t.Fatalf("expected playbackToggledMsg, got %T", msg)
	}
	if toggled.err != nil {
		t.Fatalf("expected no error, got %v", toggled.err)
	}
}

// TestTogglePlaybackCommandNilPlayer verifies togglePlayback returns an
// error when playerService is nil.
func TestTogglePlaybackCommandNilPlayer(t *testing.T) {
	model := newTestModel(t)
	model.playerService = nil

	msg := model.togglePlayback()()
	toggled, ok := msg.(playbackToggledMsg)
	if !ok {
		t.Fatalf("expected playbackToggledMsg, got %T", msg)
	}
	if toggled.err == nil {
		t.Fatal("expected error when player is nil")
	}
}

// TestSkipPlaybackCommandSuccess verifies skipPlayback returns a
// playbackSkippedMsg with the correct seconds.
func TestSkipPlaybackCommandSuccess(t *testing.T) {
	model := newTestModel(t)

	msg := model.skipPlayback(15)()
	skipped, ok := msg.(playbackSkippedMsg)
	if !ok {
		t.Fatalf("expected playbackSkippedMsg, got %T", msg)
	}
	if skipped.err != nil {
		t.Fatalf("expected no error, got %v", skipped.err)
	}
	if skipped.seconds != 15 {
		t.Fatalf("expected seconds 15, got %f", skipped.seconds)
	}
}

// TestSkipPlaybackCommandNegative verifies skipPlayback with negative
// seconds (skip backward).
func TestSkipPlaybackCommandNegative(t *testing.T) {
	model := newTestModel(t)

	msg := model.skipPlayback(-15)()
	skipped, ok := msg.(playbackSkippedMsg)
	if !ok {
		t.Fatalf("expected playbackSkippedMsg, got %T", msg)
	}
	if skipped.seconds != -15 {
		t.Fatalf("expected seconds -15, got %f", skipped.seconds)
	}
}

// TestSkipPlaybackCommandNilPlayer verifies skipPlayback returns an
// error when playerService is nil.
func TestSkipPlaybackCommandNilPlayer(t *testing.T) {
	model := newTestModel(t)
	model.playerService = nil

	msg := model.skipPlayback(15)()
	skipped, ok := msg.(playbackSkippedMsg)
	if !ok {
		t.Fatalf("expected playbackSkippedMsg, got %T", msg)
	}
	if skipped.err == nil {
		t.Fatal("expected error when player is nil")
	}
}

// TestSeekPlaybackToCommandSuccess verifies seekPlaybackTo returns a
// playbackSeekedMsg with the correct positionSec.
func TestSeekPlaybackToCommandSuccess(t *testing.T) {
	model := newTestModel(t)
	model.playbackStatus.PositionSec = 30

	msg := model.seekPlaybackTo(90)()
	seeked, ok := msg.(playbackSeekedMsg)
	if !ok {
		t.Fatalf("expected playbackSeekedMsg, got %T", msg)
	}
	if seeked.err != nil {
		t.Fatalf("expected no error, got %v", seeked.err)
	}
	if seeked.positionSec != 90 {
		t.Fatalf("expected positionSec 90, got %f", seeked.positionSec)
	}
}

// TestSeekPlaybackToCommandNilPlayer verifies seekPlaybackTo returns an
// error when playerService is nil.
func TestSeekPlaybackToCommandNilPlayer(t *testing.T) {
	model := newTestModel(t)
	model.playerService = nil

	msg := model.seekPlaybackTo(90)()
	seeked, ok := msg.(playbackSeekedMsg)
	if !ok {
		t.Fatalf("expected playbackSeekedMsg, got %T", msg)
	}
	if seeked.err == nil {
		t.Fatal("expected error when player is nil")
	}
}

// TestSeekPlaybackToCommandNoEpisode verifies seekPlaybackTo returns an
// error when no episode is selected and position is 0.
func TestSeekPlaybackToCommandNoEpisode(t *testing.T) {
	model := newTestModel(t)
	model.playbackStatus.PositionSec = 0
	model.currentEpisode = nil
	model.selectedEpisode = nil

	msg := model.seekPlaybackTo(90)()
	seeked, ok := msg.(playbackSeekedMsg)
	if !ok {
		t.Fatalf("expected playbackSeekedMsg, got %T", msg)
	}
	if seeked.err == nil {
		t.Fatal("expected error when no episode is selected")
	}
	if seeked.err.Error() != "no episode selected" {
		t.Fatalf("expected 'no episode selected', got %q", seeked.err.Error())
	}
}

// newSeededPodcastService creates a PodcastService backed by the given repo.
func newSeededPodcastService(repo *persistence.SQLiteRepo) *application.PodcastService {
	return application.NewPodcastService(repo, repo, repo, tuiMockFeedParser{}, nil)
}

// newQueueTestModel creates a Model with a real QueueService backed by an
// in-memory SQLite repo, along with a seeded episode for queue commands.
func newQueueTestModel(t *testing.T) (Model, *persistence.SQLiteRepo, domain.Episode) {
	t.Helper()

	repo, err := persistence.NewSQLiteRepo(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteRepo failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	podcast := &domain.Podcast{Title: "QueuePod", FeedURL: "https://example.com/q.xml"}
	if err := repo.Save(context.Background(), podcast); err != nil {
		t.Fatalf("Save podcast: %v", err)
	}
	episode := &domain.Episode{
		PodcastID: podcast.ID,
		Title:     "Queue Episode",
		AudioURL:  "https://example.com/q.mp3",
	}
	if err := repo.SaveEpisode(context.Background(), episode); err != nil {
		t.Fatalf("SaveEpisode: %v", err)
	}

	ps := application.NewPodcastService(repo, repo, repo, tuiMockFeedParser{}, nil)
	dl := application.NewDownloadService(repo, repo, repo, repo, "downloads", nil)
	qsvc := application.NewQueueService(repo, repo, repo, &tuiMockPlayer{}, nil, nil)
	model := NewModel(context.Background(), ps, dl, nil, qsvc, Settings{}, nil, "")

	return model, repo, *episode
}

func TestAddToQueueCmdSuccess(t *testing.T) {
	model, _, episode := newQueueTestModel(t)

	msg := model.addToQueueCmd(episode.ID)()
	added, ok := msg.(queueItemAddedMsg)
	if !ok {
		t.Fatalf("expected queueItemAddedMsg, got %T", msg)
	}
	if added.err != nil {
		t.Fatalf("expected no error, got %v", added.err)
	}
}

func TestAddToQueueCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.addToQueueCmd(1)()
	added, ok := msg.(queueItemAddedMsg)
	if !ok {
		t.Fatalf("expected queueItemAddedMsg, got %T", msg)
	}
	if added.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
	if added.err.Error() != "queue service is unavailable" {
		t.Fatalf("expected 'queue service is unavailable', got %q", added.err.Error())
	}
}

func TestAddPlayNextCmdSuccess(t *testing.T) {
	model, _, episode := newQueueTestModel(t)

	// Seed the queue so AddPlayNext has a non-empty slice to insert into.
	if err := model.queueService.AddToQueue(context.Background(), episode.ID); err != nil {
		t.Fatalf("seed queue: %v", err)
	}

	msg := model.addPlayNextCmd(episode.ID)()
	added, ok := msg.(queueItemAddedMsg)
	if !ok {
		t.Fatalf("expected queueItemAddedMsg, got %T", msg)
	}
	if added.err != nil {
		t.Fatalf("expected no error, got %v", added.err)
	}
}

func TestAddPlayNextCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.addPlayNextCmd(1)()
	added, ok := msg.(queueItemAddedMsg)
	if !ok {
		t.Fatalf("expected queueItemAddedMsg, got %T", msg)
	}
	if added.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestLoadPlaybackQueueCmdSuccess(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.loadPlaybackQueueCmd()()
	loaded, ok := msg.(playbackQueueLoadedMsg)
	if !ok {
		t.Fatalf("expected playbackQueueLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected no error, got %v", loaded.err)
	}
}

func TestLoadPlaybackQueueCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.loadPlaybackQueueCmd()()
	loaded, ok := msg.(playbackQueueLoadedMsg)
	if !ok {
		t.Fatalf("expected playbackQueueLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestNextTrackCmdEmptyQueue(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.nextTrackCmd()()
	changed, ok := msg.(queueTrackChangedMsg)
	if !ok {
		t.Fatalf("expected queueTrackChangedMsg, got %T", msg)
	}
	if changed.err == nil {
		t.Fatal("expected error on empty queue")
	}
}

func TestNextTrackCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.nextTrackCmd()()
	changed, ok := msg.(queueTrackChangedMsg)
	if !ok {
		t.Fatalf("expected queueTrackChangedMsg, got %T", msg)
	}
	if changed.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestPrevTrackCmdEmptyQueue(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.prevTrackCmd()()
	changed, ok := msg.(queueTrackChangedMsg)
	if !ok {
		t.Fatalf("expected queueTrackChangedMsg, got %T", msg)
	}
	if changed.err == nil {
		t.Fatal("expected error on empty queue")
	}
}

func TestPrevTrackCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.prevTrackCmd()()
	changed, ok := msg.(queueTrackChangedMsg)
	if !ok {
		t.Fatalf("expected queueTrackChangedMsg, got %T", msg)
	}
	if changed.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestCycleRepeatCmdSuccess(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.cycleRepeatCmd()()
	toggled, ok := msg.(repeatModeToggledMsg)
	if !ok {
		t.Fatalf("expected repeatModeToggledMsg, got %T", msg)
	}
	if toggled.err != nil {
		t.Fatalf("expected no error, got %v", toggled.err)
	}
}

func TestCycleRepeatCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.cycleRepeatCmd()()
	toggled, ok := msg.(repeatModeToggledMsg)
	if !ok {
		t.Fatalf("expected repeatModeToggledMsg, got %T", msg)
	}
	if toggled.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestToggleShuffleCmdSuccess(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.toggleShuffleCmd()()
	toggled, ok := msg.(shuffleToggledMsg)
	if !ok {
		t.Fatalf("expected shuffleToggledMsg, got %T", msg)
	}
	if toggled.err != nil {
		t.Fatalf("expected no error, got %v", toggled.err)
	}
}

func TestToggleShuffleCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.toggleShuffleCmd()()
	toggled, ok := msg.(shuffleToggledMsg)
	if !ok {
		t.Fatalf("expected shuffleToggledMsg, got %T", msg)
	}
	if toggled.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestRemoveFromQueueCmdNilService(t *testing.T) {
	model := newTestModel(t)
	model.queueService = nil

	msg := model.removeFromQueueCmd(1)()
	removed, ok := msg.(queueItemRemovedMsg)
	if !ok {
		t.Fatalf("expected queueItemRemovedMsg, got %T", msg)
	}
	if removed.err == nil {
		t.Fatal("expected error when queue service is nil")
	}
}

func TestRemoveFromQueueCmdNotFound(t *testing.T) {
	model, _, _ := newQueueTestModel(t)

	msg := model.removeFromQueueCmd(9999)()
	removed, ok := msg.(queueItemRemovedMsg)
	if !ok {
		t.Fatalf("expected queueItemRemovedMsg, got %T", msg)
	}
	if removed.err == nil {
		t.Fatal("expected error when removing non-existent item")
	}
}
