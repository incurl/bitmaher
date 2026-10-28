// Package pump pushes the state of the download engine to the frontend.
//
// The engine does not push anything: there is no event to subscribe to
// and no notification of a download that moved. The frontend used to ask
// for the state on a timer, which meant that the list could be stale and
// that every panel polled on its own.
//
// The pump is the single reader of the engine's state. It reads the
// global statistics and the task queues on a tick, and publishes the
// result as one snapshot. The frontend never polls the engine: it waits
// for the snapshot.
//
// This mirrors the architecture of Rayburst, whose Rust backend pushes a
// `tasks:snapshot` event. The name and the payload shape are the same, so
// that the ported store works unchanged.
package pump

import (
	"context"
	"sync"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
)

// EventSnapshot is the name of the event the snapshots are published on.
const EventSnapshot = "tasks:snapshot"

// The intervals between two ticks.
//
// The engine does not push, so the interval is what bounds how stale the
// speeds and the progress may get. The visible one is short, because the
// user is looking at the numbers.
const (
	// VisibleInterval is the interval while the window is in front.
	VisibleInterval = 500 * time.Millisecond
	// BackgroundInterval is the interval when the window is hidden.
	BackgroundInterval = 2 * time.Second
	// ReadTimeout bounds a single read of the engine.
	ReadTimeout = 10 * time.Second
)

// Snapshot is the state of the engine at one moment.
//
// The tasks are the active, the waiting and the stopped ones, without
// the downloads that were removed.
type Snapshot struct {
	// Stat is the global state of the engine.
	Stat aria2.GlobalStat `json:"stat"`
	// Tasks are the downloads, in the order the engine reports them.
	Tasks []downloads.Job `json:"tasks"`
}

// Emitter publishes a snapshot to the frontend.
type Emitter interface {
	// EmitSnapshot publishes a snapshot.
	EmitSnapshot(snapshot Snapshot)
}

// Source is the reader of the engine's state.
//
// It is an interface, so that the pump can be tested without an engine.
type Source interface {
	// Snapshot reads the state of the engine.
	Snapshot(ctx context.Context) (Snapshot, error)
}

// Pump reads the engine's state and publishes it.
//
// The pump runs for as long as the application does. It is started once
// the Wails application exists, and stopped when it closes.
type Pump struct {
	source  Source
	emitter Emitter

	// wake is closed and replaced to ask for an immediate tick.
	wake chan struct{}

	mu sync.Mutex
	// running holds the stop channel while the pump is up.
	running chan struct{}
	// stopped is closed when the pump has finished.
	stopped chan struct{}
}

// New creates a pump.
//
// The source is the reader of the engine's state and the emitter is the
// publisher. Either may be nil: the pump then does nothing.
func New(source Source, emitter Emitter) *Pump {
	return &Pump{
		source:  source,
		emitter: emitter,
		wake:    make(chan struct{}),
	}
}

// Start begins the ticks.
//
// It returns immediately: the ticks run in their own goroutine, so that
// the caller is not blocked.
func (p *Pump) Start() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running != nil {
		return
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	p.running = stop
	p.stopped = done

	go p.loop(stop, done)
}

// Stop ends the ticks and waits for the last one to finish.
//
// It is safe to call it more than once, and on a pump that was never
// started.
func (p *Pump) Stop() {
	if p == nil {
		return
	}

	p.mu.Lock()
	stop := p.running
	done := p.stopped
	p.running = nil
	p.stopped = nil
	p.mu.Unlock()

	if stop == nil {
		return
	}

	close(stop)
	<-done
}

// SetEmitter attaches the publisher once it becomes available.
//
// The emitter is the Wails application, which only exists after the
// backend has been bootstrapped, so the pump is created before it.
func (p *Pump) SetEmitter(emitter Emitter) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.emitter = emitter
}

// Wake asks for an immediate tick.
//
// The engine notifies nothing, so an action that changes the state of a
// download would otherwise wait for the next tick. The caller wakes the
// pump, and the change shows up at once.
func (p *Pump) Wake() {
	if p == nil {
		return
	}

	p.mu.Lock()
	wake := p.wake
	p.mu.Unlock()

	select {
	case wake <- struct{}{}:
	default:
		// A tick is already pending. The one that is on its way will
		// pick the change up, so there is nothing to do.
	}
}

// loop runs the ticks until the pump is stopped.
func (p *Pump) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(VisibleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		case <-p.wake:
		}

		p.tick()
	}
}

// tick reads the state of the engine and publishes it.
func (p *Pump) tick() {
	if p.source == nil || p.emitter == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ReadTimeout)
	defer cancel()

	snapshot, err := p.source.Snapshot(ctx)
	if err != nil {
		// The engine may be restarting. The next tick will try again,
		// and the frontend keeps the last snapshot it had.
		return
	}

	p.emitter.EmitSnapshot(snapshot)
}
