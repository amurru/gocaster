package application

import (
	"context"
	"fmt"
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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)

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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)

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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)

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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)

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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)

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

	svc := NewPlayerService(mr, mr, mp, mb, nil, nil)
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

type mockQueueRepo struct {
	items []domain.QueueItem
	state domain.QueueState
}

func (m *mockQueueRepo) SaveQueueItem(_ context.Context, item *domain.QueueItem) error {
	if item.ID == 0 {
		item.ID = int64(len(m.items) + 100)
	}
	for i, existing := range m.items {
		if existing.ID == item.ID {
			m.items[i] = *item
			return nil
		}
	}
	m.items = append(m.items, *item)
	return nil
}

func (m *mockQueueRepo) FindAllQueueItems(_ context.Context) ([]domain.QueueItem, error) {
	out := make([]domain.QueueItem, len(m.items))
	copy(out, m.items)
	return out, nil
}

func (m *mockQueueRepo) DeleteQueueItem(_ context.Context, id int64) error {
	for i, item := range m.items {
		if item.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("item %d not found", id)
}

func (m *mockQueueRepo) ClearQueue(_ context.Context) error {
	m.items = nil
	return nil
}

func (m *mockQueueRepo) GetQueueState(_ context.Context) (*domain.QueueState, error) {
	s := m.state
	return &s, nil
}

func (m *mockQueueRepo) SaveQueueState(_ context.Context, state *domain.QueueState) error {
	m.state = *state
	return nil
}

func testRepoWithQueue(currentIndex int) (*mockRepo, *mockQueueRepo) {
	episodes := map[int64]*domain.Episode{
		10: {ID: 10, PodcastID: 1, Title: "Ep 10", AudioURL: "https://example.com/10.mp3"},
		20: {ID: 20, PodcastID: 1, Title: "Ep 20", AudioURL: "https://example.com/20.mp3"},
		30: {ID: 30, PodcastID: 1, Title: "Ep 30", AudioURL: "https://example.com/30.mp3"},
	}
	podcasts := map[int64]*domain.Podcast{
		1: {ID: 1, Title: "Test Podcast"},
	}

	mr := &mockRepo{episodes: episodes, podcasts: podcasts}

	qr := &mockQueueRepo{
		items: []domain.QueueItem{
			{ID: 1, EpisodeID: 10, Position: 0},
			{ID: 2, EpisodeID: 20, Position: 1},
			{ID: 3, EpisodeID: 30, Position: 2},
		},
		state: domain.QueueState{CurrentIndex: currentIndex, RepeatMode: domain.RepeatNone},
	}

	return mr, qr
}

func newPlayerSvcWithQueue(mr *mockRepo, mp *mockPlayer, mb *mockBroadcaster, qr *mockQueueRepo) (*PlayerService, *QueueService) {
	qsvc := NewQueueService(qr, mr, mr, mp, mb, nil)
	psvc := NewPlayerService(mr, mr, mp, mb, nil, qsvc)
	return psvc, qsvc
}

// ---------------------------------------------------------------------------
// Queue-aware PlayerService tests
// ---------------------------------------------------------------------------

func TestNextAdvancesToNextEpisode(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(0)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	if err := svc.Next(ctx); err != nil {
		t.Fatalf("Next: %v", err)
	}

	want := "https://example.com/20.mp3"
	if mp.source != want {
		t.Errorf("expected source %q, got %q", want, mp.source)
	}

	if svc.currentEpisode == nil || svc.currentEpisode.ID != 20 {
		t.Errorf("expected currentEpisode.ID == 20, got %v", svc.currentEpisode)
	}
}

func TestNextAtEndOfQueueReturnsError(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(2)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	err := svc.Next(ctx)
	if err == nil {
		t.Fatal("expected error at end of queue without repeat")
	}
}

func TestNextWithRepeatAllWrapsAround(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(2)

	svc, qsvc := newPlayerSvcWithQueue(mr, mp, mb, qr)

	_ = qsvc.ToggleRepeat(ctx)
	_ = qsvc.ToggleRepeat(ctx)

	if err := svc.Next(ctx); err != nil {
		t.Fatalf("Next with RepeatAll: %v", err)
	}

	want := "https://example.com/10.mp3"
	if mp.source != want {
		t.Errorf("expected wrap-around to %q, got %q", want, mp.source)
	}
}

func TestPreviousGoesBack(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(1)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	if err := svc.Previous(ctx); err != nil {
		t.Fatalf("Previous: %v", err)
	}

	want := "https://example.com/10.mp3"
	if mp.source != want {
		t.Errorf("expected source %q, got %q", want, mp.source)
	}

	if svc.currentEpisode == nil || svc.currentEpisode.ID != 10 {
		t.Errorf("expected currentEpisode.ID == 10, got %v", svc.currentEpisode)
	}
}

func TestPreviousAtStartStaysAtZero(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(0)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	if err := svc.Previous(ctx); err != nil {
		t.Fatalf("Previous at start: %v", err)
	}

	want := "https://example.com/10.mp3"
	if mp.source != want {
		t.Errorf("expected source %q (stay), got %q", want, mp.source)
	}
}

func TestNextWithEmptyQueueReturnsError(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{episodes: map[int64]*domain.Episode{}, podcasts: map[int64]*domain.Podcast{}}
	qr := &mockQueueRepo{}

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	err := svc.Next(ctx)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestPreviousWithEmptyQueueReturnsError(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr := &mockRepo{episodes: map[int64]*domain.Episode{}, podcasts: map[int64]*domain.Podcast{}}
	qr := &mockQueueRepo{}

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	err := svc.Previous(ctx)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestHasNextHasPrevious(t *testing.T) {
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}

	t.Run("middle", func(t *testing.T) {
		mr, qr := testRepoWithQueue(1)
		_, qsvc := newPlayerSvcWithQueue(mr, mp, mb, qr)
		if !qsvc.HasNext() {
			t.Error("expected HasNext() == true in middle of queue")
		}
		if !qsvc.HasPrevious() {
			t.Error("expected HasPrevious() == true in middle of queue")
		}
	})

	t.Run("end", func(t *testing.T) {
		mr, qr := testRepoWithQueue(2)
		_, qsvc := newPlayerSvcWithQueue(mr, mp, mb, qr)
		if qsvc.HasNext() {
			t.Error("expected HasNext() == false at end of queue without repeat")
		}
		if !qsvc.HasPrevious() {
			t.Error("expected HasPrevious() == true at end of queue")
		}
	})

	t.Run("start", func(t *testing.T) {
		mr, qr := testRepoWithQueue(0)
		_, qsvc := newPlayerSvcWithQueue(mr, mp, mb, qr)
		if !qsvc.HasNext() {
			t.Error("expected HasNext() == true at start of queue")
		}
		if qsvc.HasPrevious() {
			t.Error("expected HasPrevious() == false at start of queue without repeat")
		}
	})
}

func TestNextSyncsPlayerServiceState(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(0)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	_ = svc.PlayEpisode(ctx, 10)
	if svc.lastEpisodeID != 10 {
		t.Fatalf("setup: expected lastEpisodeID=10, got %d", svc.lastEpisodeID)
	}

	_ = svc.Next(ctx)

	if svc.lastEpisodeID != 20 {
		t.Errorf("expected lastEpisodeID=20 after Next, got %d", svc.lastEpisodeID)
	}
	if svc.currentEpisode == nil || svc.currentEpisode.ID != 20 {
		t.Errorf("expected currentEpisode.ID=20, got %v", svc.currentEpisode)
	}
	if svc.currentPodcast == nil || svc.currentPodcast.ID != 1 {
		t.Errorf("expected currentPodcast.ID=1, got %v", svc.currentPodcast)
	}
}

func TestBroadcastMetadataIncludesCanGoNextPrevious(t *testing.T) {
	ctx := context.Background()
	mp := &mockPlayer{}
	mb := &mockBroadcaster{}
	mr, qr := testRepoWithQueue(1)

	svc, _ := newPlayerSvcWithQueue(mr, mp, mb, qr)

	_ = svc.PlayEpisode(ctx, 20)

	if !mb.publishedMeta.CanGoNext {
		t.Error("expected CanGoNext=true in middle of queue")
	}
	if !mb.publishedMeta.CanGoPrevious {
		t.Error("expected CanGoPrevious=true in middle of queue")
	}
}
