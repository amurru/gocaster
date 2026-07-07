//go:build linux

package system

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/domain"
)

// stubController is a PlaybackController whose Status blocks until released,
// letting a test hold the positionUpdater mid-call to exercise the Close()
// shutdown ordering.
type stubController struct {
	mu      sync.Mutex
	playing bool
	block   chan struct{}
}

func newStubController() *stubController {
	return &stubController{block: make(chan struct{})}
}

func (s *stubController) Play(context.Context, int64) error { return nil }
func (s *stubController) Pause(context.Context) error       { return nil }
func (s *stubController) Resume(context.Context) error      { return nil }
func (s *stubController) PlayPause(context.Context) error   { return nil }
func (s *stubController) Stop(context.Context) error        { return nil }
func (s *stubController) SeekTo(context.Context, float64) error {
	return nil
}
func (s *stubController) Status(context.Context) (domain.PlaybackStatus, error) {
	// Hold until the test releases the blocker, simulating a slow player.
	<-s.block
	return domain.PlaybackStatus{State: domain.PlaybackStatePlaying}, nil
}

// newTestBroadcaster builds a mprisBroadcaster without a real D-Bus connection
// so the shutdown lifecycle (done channel + WaitGroup + nil-conn guards) can be
// exercised under the race detector.
func newTestBroadcaster(ctrl domain.PlaybackController) *mprisBroadcaster {
	b := &mprisBroadcaster{
		running: true,
		done:    make(chan struct{}),
		state:   domain.PlaybackStatePlaying,
	}
	b.SetController(context.Background(), ctrl)
	b.wg.Add(1)
	go b.positionUpdater()
	return b
}

// TestMPRISBroadcaster_CloseWaitsForUpdater covers issue #9: Close must not
// return until positionUpdater has stopped. Here the updater's Status() call
// blocks on ctrl.block, so Close cannot complete until the test releases it.
// If Close returned before the updater exited (the old race), this test would
// finish instead of blocking until ctrl.block is closed.
func TestMPRISBroadcaster_CloseWaitsForUpdater(t *testing.T) {
	ctrl := newStubController()
	b := newTestBroadcaster(ctrl)

	// Let the updater tick at least once and enter Status() (blocked).
	time.Sleep(1200 * time.Millisecond)

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- b.Close(context.Background())
	}()

	// Close should be blocked because the updater is mid-Status() and wg hasn't
	// been decremented. Give it a moment, then verify it hasn't returned.
	select {
	case <-closeDone:
		t.Fatal("Close returned before the positionUpdater finished (WaitGroup not honored)")
	case <-time.After(300 * time.Millisecond):
		// expected: still waiting
	}

	// Release the updater; Close should now complete promptly.
	close(ctrl.block)

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after the updater was released")
	}

	b.mu.Lock()
	running := b.running
	b.mu.Unlock()
	if running {
		t.Fatal("expected running=false after Close")
	}
}

// TestMPRISBroadcaster_CloseIdempotent verifies that calling Close more than
// once is safe — the done channel is not closed twice (which would panic).
func TestMPRISBroadcaster_CloseIdempotent(t *testing.T) {
	ctrl := newStubController()
	b := newTestBroadcaster(ctrl)

	close(ctrl.block) // let the updater proceed so Close can finish quickly

	if err := b.Close(context.Background()); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := b.Close(context.Background()); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

// TestMPRISBroadcaster_PublishAfterCloseNoPanic verifies Publish* no-op safely
// after Close has nilled the connection (guards against post-close emits).
func TestMPRISBroadcaster_PublishAfterCloseNoPanic(t *testing.T) {
	ctrl := newStubController()
	b := newTestBroadcaster(ctrl)
	close(ctrl.block)
	if err := b.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// These must not panic on the nilled conn.
	if err := b.PublishState(context.Background(), domain.PlaybackStatePaused, domain.PlaybackMetadata{}); err != nil {
		t.Errorf("PublishState after Close: %v", err)
	}
	if err := b.PublishPosition(context.Background(), 10, 100); err != nil {
		t.Errorf("PublishPosition after Close: %v", err)
	}
}
