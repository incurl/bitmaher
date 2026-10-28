package pump

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeSource is a reader of the engine's state that the test controls.
type fakeSource struct {
	mu sync.Mutex
	// snapshot is what the source answers.
	snapshot Snapshot
	// err is the failure the source answers with.
	err error
	// reads counts the reads.
	reads int
	// changed is closed when the source is read.
	changed chan struct{}
}

func newFakeSource() *fakeSource {
	return &fakeSource{changed: make(chan struct{})}
}

func (s *fakeSource) Snapshot(context.Context) (Snapshot, error) {
	s.mu.Lock()
	s.reads++
	snapshot, err := s.snapshot, s.err
	s.mu.Unlock()

	select {
	case <-s.changed:
	default:
		close(s.changed)
	}

	return snapshot, err
}

func (s *fakeSource) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

// fakeEmitter records the snapshots it is given.
type fakeEmitter struct {
	mu        sync.Mutex
	snapshots []Snapshot
	// published is closed when a snapshot arrives.
	published chan struct{}
}

func newFakeEmitter() *fakeEmitter {
	return &fakeEmitter{published: make(chan struct{})}
}

func (e *fakeEmitter) EmitSnapshot(snapshot Snapshot) {
	e.mu.Lock()
	e.snapshots = append(e.snapshots, snapshot)
	e.mu.Unlock()

	select {
	case <-e.published:
	default:
		close(e.published)
	}
}

func (e *fakeEmitter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.snapshots)
}

// TestThePumpPublishesTheSnapshot verifies that the pump reads the state
// of the engine and publishes it.
func TestThePumpPublishesTheSnapshot(t *testing.T) {
	source := newFakeSource()
	source.snapshot = Snapshot{Tasks: nil}
	emitter := newFakeEmitter()

	pump := New(source, emitter)
	pump.Start()
	defer pump.Stop()

	select {
	case <-emitter.published:
	case <-time.After(3 * time.Second):
		t.Fatal("the pump did not publish a snapshot")
	}

	if emitter.count() == 0 {
		t.Error("the emitter received no snapshot")
	}
	if source.count() == 0 {
		t.Error("the source was not read")
	}
}

// TestThePumpKeepsTheLastSnapshotOnAFailure verifies that a failing read
// does not publish anything.
//
// The engine may be restarting, and the frontend keeps the last snapshot
// it had.
func TestThePumpKeepsTheLastSnapshotOnAFailure(t *testing.T) {
	source := newFakeSource()
	source.err = errors.New("the engine is not running")
	emitter := newFakeEmitter()

	pump := New(source, emitter)
	pump.Start()
	defer pump.Stop()

	// The pump reads at least once. Nothing is published, because the
	// read failed.
	time.Sleep(200 * time.Millisecond)

	if emitter.count() != 0 {
		t.Errorf("the emitter received %d snapshots, want 0", emitter.count())
	}
}

// TestThePumpCanBeWoken verifies that an action can ask for an immediate
// tick.
//
// The engine notifies nothing, so a change would otherwise wait for the
// next tick.
func TestThePumpCanBeWoken(t *testing.T) {
	source := newFakeSource()
	emitter := newFakeEmitter()

	pump := New(source, emitter)
	pump.Start()
	defer pump.Stop()

	// Let the first tick happen.
	time.Sleep(100 * time.Millisecond)
	before := source.count()

	pump.Wake()

	// The wake asks for a tick at once.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if source.count() > before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Error("the wake did not ask for a tick")
}

// TestThePumpStops verifies that the pump stops reading.
func TestThePumpStops(t *testing.T) {
	source := newFakeSource()
	emitter := newFakeEmitter()

	pump := New(source, emitter)
	pump.Start()
	time.Sleep(100 * time.Millisecond)

	pump.Stop()

	after := source.count()
	time.Sleep(300 * time.Millisecond)

	if source.count() != after {
		t.Error("the pump kept reading after it was stopped")
	}
}

// TestThePumpCanBeStoppedTwice verifies that stopping a pump that was
// already stopped does not panic.
//
// The shutdown may stop the pump twice, and a panic there would take
// the application down.
func TestThePumpCanBeStoppedTwice(t *testing.T) {
	pump := New(newFakeSource(), newFakeEmitter())
	pump.Start()
	pump.Stop()
	pump.Stop()
}

// TestThePumpCanBeStoppedWithoutBeingStarted verifies that stopping a
// pump that never ran does not panic.
func TestThePumpCanBeStoppedWithoutBeingStarted(t *testing.T) {
	pump := New(newFakeSource(), newFakeEmitter())
	pump.Stop()
}

// TestThePumpWithoutASourceDoesNothing verifies that a pump without a
// reader stays quiet.
//
// A headless run has no engine, and the pump must not complain about
// it.
func TestThePumpWithoutASourceDoesNothing(t *testing.T) {
	emitter := newFakeEmitter()

	pump := New(nil, emitter)
	pump.Start()
	defer pump.Stop()

	time.Sleep(200 * time.Millisecond)

	if emitter.count() != 0 {
		t.Errorf("the emitter received %d snapshots, want 0", emitter.count())
	}
}
