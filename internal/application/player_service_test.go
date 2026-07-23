package application

import (
	"context"
	"testing"

	"github.com/amurru/gocaster/internal/domain"
)

type mockBroadcaster struct {
	publishedState   domain.PlaybackState
	publishedMeta    domain.PlaybackMetadata
	publishCallCount int
	controller       domain.PlaybackController
}

func (m *mockBroadcaster) PublishState(ctx context.Context, state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	m.publishedState = state
	m.publishedMeta = metadata
	m.publishCallCount++
	return nil
}

func (m *mockBroadcaster) PublishPosition(ctx context.Context, positionSec float64, durationSec float64) error {
	return nil
}

func (m *mockBroadcaster) Close(ctx context.Context) error {
	return nil
}

func (m *mockBroadcaster) SetController(ctx context.Context, controller domain.PlaybackController) {
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

func (m *mockPlayer) Play(ctx context.Context, source string) error {
	m.playCalled = true
	m.source = source
	m.isPlaying = true
	m.paused = false
	return nil
}

func (m *mockPlayer) Stop(ctx context.Context) error {
	m.stopCalled = true
	m.isPlaying = false
	m.paused = false
	m.source = ""
	return nil
}

func (m *mockPlayer) IsPlaying(ctx context.Context) bool {
	return m.isPlaying && !m.paused
}

func (m *mockPlayer) Pause(ctx context.Context) error {
	m.pauseCalled = true
	m.paused = true
	return nil
}

func (m *mockPlayer) Resume(ctx context.Context) error {
	m.resumeCalled = true
	m.paused = false
	return nil
}

func (m *mockPlayer) TogglePause(ctx context.Context) error {
	m.toggleCalled = true
	m.paused = !m.paused
	return nil
}

func (m *mockPlayer) Seek(ctx context.Context, seconds float64) error {
	m.seekCalled = true
	return nil
}

func (m *mockPlayer) Status(ctx context.Context) (domain.PlaybackStatus, error) {
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

func (m *mockPlayer) Close(ctx context.Context) error {
	return nil
}

func (m *mockPlayer) SetSpeed(ctx context.Context, speed float64) error { return nil }
func (m *mockPlayer) GetSpeed(ctx context.Context) float64              { return 1.0 }

// mockRepo implements only the focused ports PlayerService consumes —
// EpisodeRepo and PodcastRepo (issue #17). It no longer stubs the ~9
// download-job methods the service never calls.
type mockRepo struct {
	episodes map[int64]*domain.Episode
	podcasts map[int64]*domain.Podcast
}

func (m *mockRepo) FindEpisodeByID(ctx context.Context, id int64) (*domain.Episode, error) {
	if ep, ok := m.episodes[id]; ok {
		return ep, nil
	}
	return nil, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id int64) (*domain.Podcast, error) {
	if p, ok := m.podcasts[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockRepo) UpdateEpisodePlaybackState(ctx context.Context, id int64, isPlayed bool) error {
	if ep, ok := m.episodes[id]; ok {
		ep.IsPlayed = isPlayed
	}
	return nil
}

func (m *mockRepo) Save(ctx context.Context, podcast *domain.Podcast) error { return nil }
func (m *mockRepo) FindAll(ctx context.Context) ([]domain.Podcast, error)   { return nil, nil }
func (m *mockRepo) Delete(ctx context.Context, id int64) error              { return nil }
func (m *mockRepo) UpdateFeedHeaders(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}
func (m *mockRepo) SaveEpisode(ctx context.Context, episode *domain.Episode) error {
	return nil
}
func (m *mockRepo) FindEpisodesByPodcastID(ctx context.Context, id int64) ([]domain.Episode, error) {
	return nil, nil
}
func (m *mockRepo) DeleteEpisode(ctx context.Context, id int64) error { return nil }
func (m *mockRepo) MarkEpisodeDownloaded(ctx context.Context, episodeID int64, localPath string) error {
	return nil
}

func TestPlayerServiceBroadcastsOnPlay(t *testing.T) {
	ctx := context.Background()
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

	svc := NewPlayerService(mr, mr, mp, mb, nil)

	err := svc.PlayEpisode(ctx, 1)
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
	ctx := context.Background()
	mp := &mockPlayer{isPlaying: true}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mr, mp, mb, nil)

	_ = svc.Pause(ctx)

	if mb.publishedState != domain.PlaybackStatePaused {
		t.Errorf("expected state Paused, got %v", mb.publishedState)
	}
}

func TestPlayerServiceBroadcastsOnStop(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{isPlaying: true}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mr, mp, mb, nil)

	_ = svc.StopPlayback(ctx)

	if mb.publishedState != domain.PlaybackStateStopped {
		t.Errorf("expected state Stopped, got %v", mb.publishedState)
	}
}

func TestPlayerServiceReplaysLastEpisode(t *testing.T) {
	ctx := context.Background()
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

	svc := NewPlayerService(mr, mr, mp, mb, nil)

	_ = svc.PlayEpisode(ctx, 1)

	mp.playCalled = false

	err := svc.Play(ctx, 0)
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
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{}

	svc := NewPlayerService(mr, mr, mp, mb, nil)

	err := svc.Play(ctx, 0)
	if err == nil {
		t.Error("expected error when Play(0) with no last episode")
	}
}

func TestPlayerServiceInboundControl(t *testing.T) {
	ctx := context.Background()
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

	svc := NewPlayerService(mr, mr, mp, mb, nil)
	_ = svc

	mb.controller.Play(ctx, 1)
	if !mp.playCalled {
		t.Error("expected player.Play to be called via inbound control")
	}

	mb.controller.Pause(ctx)
	if !mp.pauseCalled {
		t.Error("expected player.Pause to be called via inbound control")
	}

	mb.controller.PlayPause(ctx)
	if !mp.toggleCalled {
		t.Error("expected player.TogglePause to be called via inbound control")
	}

	mb.controller.Stop(ctx)
	if !mp.stopCalled {
		t.Error("expected player.Stop to be called via inbound control")
	}

	mb.controller.SeekTo(ctx, 60)
	if !mp.seekCalled {
		t.Error("expected player.Seek to be called via inbound control")
	}
}
