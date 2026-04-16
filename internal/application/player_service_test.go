package application

import (
	"testing"

	"github.com/amurru/gocaster/internal/domain"
)

type mockBroadcaster struct {
	publishedState   domain.PlaybackState
	publishedMeta    domain.PlaybackMetadata
	publishCallCount int
	controller       domain.PlaybackController
}

func (m *mockBroadcaster) PublishState(state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	m.publishedState = state
	m.publishedMeta = metadata
	m.publishCallCount++
	return nil
}

func (m *mockBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	return nil
}

func (m *mockBroadcaster) Close() error {
	return nil
}

func (m *mockBroadcaster) SetController(controller domain.PlaybackController) {
	m.controller = controller
}

type mockPlayer struct {
	playCalled   bool
	stopCalled   bool
	pauseCalled  bool
	resumeCalled bool
	toggleCalled bool
	seekCalled   bool
	source       string
	isPlaying    bool
	paused       bool
}

func (m *mockPlayer) Play(source string) error {
	m.playCalled = true
	m.source = source
	m.isPlaying = true
	m.paused = false
	return nil
}

func (m *mockPlayer) Stop() error {
	m.stopCalled = true
	m.isPlaying = false
	m.paused = false
	m.source = ""
	return nil
}

func (m *mockPlayer) IsPlaying() bool {
	return m.isPlaying && !m.paused
}

func (m *mockPlayer) Pause() error {
	m.pauseCalled = true
	m.paused = true
	return nil
}

func (m *mockPlayer) Resume() error {
	m.resumeCalled = true
	m.paused = false
	return nil
}

func (m *mockPlayer) TogglePause() error {
	m.toggleCalled = true
	m.paused = !m.paused
	return nil
}

func (m *mockPlayer) Seek(seconds float64) error {
	m.seekCalled = true
	return nil
}

func (m *mockPlayer) Status() (domain.PlaybackStatus, error) {
	state := domain.PlaybackStateStopped
	if m.isPlaying {
		if m.paused {
			state = domain.PlaybackStatePaused
		} else {
			state = domain.PlaybackStatePlaying
		}
	}
	return domain.PlaybackStatus{
		State:       state,
		Source:      m.source,
		CanSeek:     true,
		PositionSec: 100,
		DurationSec: 3600,
	}, nil
}

func (m *mockPlayer) Close() error {
	return nil
}

type mockRepo struct {
	episodes map[int64]*domain.Episode
	podcasts map[int64]*domain.Podcast
}

func (m *mockRepo) FindEpisodeByID(id int64) (*domain.Episode, error) {
	if ep, ok := m.episodes[id]; ok {
		return ep, nil
	}
	return nil, nil
}

func (m *mockRepo) FindByID(id int64) (*domain.Podcast, error) {
	if p, ok := m.podcasts[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockRepo) UpdateEpisodePlaybackState(id int64, isPlayed bool) error {
	if ep, ok := m.episodes[id]; ok {
		ep.IsPlayed = isPlayed
	}
	return nil
}

func (m *mockRepo) Save(podcast *domain.Podcast) error                         { return nil }
func (m *mockRepo) FindAll() ([]domain.Podcast, error)                         { return nil, nil }
func (m *mockRepo) Delete(id int64) error                                      { return nil }
func (m *mockRepo) SaveEpisode(episode *domain.Episode) error                  { return nil }
func (m *mockRepo) FindEpisodesByPodcastID(id int64) ([]domain.Episode, error) { return nil, nil }
func (m *mockRepo) DeleteEpisode(id int64) error                               { return nil }
func (m *mockRepo) SaveDownloadJob(job *domain.DownloadJob) error              { return nil }
func (m *mockRepo) FindDownloadJobByEpisodeID(episodeID int64) (*domain.DownloadJob, error) {
	return nil, nil
}
func (m *mockRepo) FindAllDownloadJobs() ([]domain.DownloadJob, error) { return nil, nil }
func (m *mockRepo) UpdateDownloadJobStatus(id int64, status domain.DownloadStatus, bytesDownloaded int64, bytesTotal int64, errorMsg string) error {
	return nil
}
func (m *mockRepo) CountNonFailedJobs() (int, error)                              { return 0, nil }
func (m *mockRepo) DeleteDownloadJob(id int64) error                              { return nil }
func (m *mockRepo) MarkEpisodeDownloaded(episodeID int64, localPath string) error { return nil }

func TestPlayerServiceBroadcastsOnPlay(t *testing.T) {
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{
		episodes: map[int64]*domain.Episode{
			1: {ID: 1, PodcastID: 1, Title: "Test Episode", AudioURL: "https://example.com/ep1.mp3"},
		},
		podcasts: map[int64]*domain.Podcast{
			1: {ID: 1, Title: "Test Podcast"},
		},
	}

	svc := NewPlayerService(mr, mp, mb)

	err := svc.PlayEpisode(1)
	if err != nil {
		t.Fatalf("PlayEpisode failed: %v", err)
	}

	if mb.publishedState != domain.PlaybackStatePlaying {
		t.Errorf("expected state Playing, got %v", mb.publishedState)
	}

	if mb.publishedMeta.EpisodeTitle != "Test Episode" {
		t.Errorf("expected episode title 'Test Episode', got %q", mb.publishedMeta.EpisodeTitle)
	}

	if mb.publishedMeta.PodcastTitle != "Test Podcast" {
		t.Errorf("expected podcast title 'Test Podcast', got %q", mb.publishedMeta.PodcastTitle)
	}
}

func TestPlayerServiceBroadcastsOnPause(t *testing.T) {
	mp := &mockPlayer{isPlaying: true}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mp, mb)

	_ = svc.Pause()

	if mb.publishedState != domain.PlaybackStatePaused {
		t.Errorf("expected state Paused, got %v", mb.publishedState)
	}
}

func TestPlayerServiceBroadcastsOnStop(t *testing.T) {
	mp := &mockPlayer{isPlaying: true}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mp, mb)

	_ = svc.StopPlayback()

	if mb.publishedState != domain.PlaybackStateStopped {
		t.Errorf("expected state Stopped, got %v", mb.publishedState)
	}
}

func TestPlayerServiceReplaysLastEpisode(t *testing.T) {
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{
		episodes: map[int64]*domain.Episode{
			1: {ID: 1, PodcastID: 1, Title: "First Episode", AudioURL: "https://example.com/ep1.mp3"},
		},
		podcasts: map[int64]*domain.Podcast{
			1: {ID: 1, Title: "Test Podcast"},
		},
	}

	svc := NewPlayerService(mr, mp, mb)

	_ = svc.PlayEpisode(1)

	mp.playCalled = false

	err := svc.Play(0)
	if err != nil {
		t.Fatalf("Play(0) failed: %v", err)
	}

	if !mp.playCalled {
		t.Error("expected Play to be called to replay last episode")
	}

	if mp.source != "https://example.com/ep1.mp3" {
		t.Errorf("expected source 'https://example.com/ep1.mp3', got %q", mp.source)
	}
}

func TestPlayerServiceControllerPlaysLastWhenNonePlayed(t *testing.T) {
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mp, mb)

	err := svc.Play(0)
	if err == nil {
		t.Error("expected error when Play(0) with no last episode")
	}
}

func TestPlayerServiceInboundControl(t *testing.T) {
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{
		episodes: map[int64]*domain.Episode{
			1: {ID: 1, PodcastID: 1, Title: "Test Episode", AudioURL: "https://example.com/ep1.mp3"},
		},
		podcasts: map[int64]*domain.Podcast{
			1: {ID: 1, Title: "Test Podcast"},
		},
	}

	svc := NewPlayerService(mr, mp, mb)
	_ = svc

	mb.controller.Play(1)
	if !mp.playCalled {
		t.Error("expected player.Play to be called via inbound control")
	}

	mb.controller.Pause()
	if !mp.pauseCalled {
		t.Error("expected player.Pause to be called via inbound control")
	}

	mb.controller.PlayPause()
	if !mp.toggleCalled {
		t.Error("expected player.TogglePause to be called via inbound control")
	}

	mb.controller.Stop()
	if !mp.stopCalled {
		t.Error("expected player.Stop to be called via inbound control")
	}

	mb.controller.SeekTo(60)
	if !mp.seekCalled {
		t.Error("expected player.Seek to be called via inbound control")
	}
}
