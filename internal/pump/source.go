package pump

import (
	"context"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
)

// Engine reads the state of the download engine.
//
// It is the adapter between the pump and the download service: the
// service knows how to talk to the engine and how to keep the records
// in step with it, and the pump only needs the result.
type Engine struct {
	// service drives the engine and keeps the records.
	service *downloads.Service
	// manager owns the sidecar's lifecycle.
	manager *aria2.Manager
}

// NewEngine creates a reader of the engine's state.
func NewEngine(service *downloads.Service, manager *aria2.Manager) *Engine {
	return &Engine{service: service, manager: manager}
}

// Snapshot reads the state of the engine.
//
// The download service syncs the records with the engine and returns
// the downloads whose state changed. The global statistics are read
// straight from the engine, because they are not a record.
func (e *Engine) Snapshot(ctx context.Context) (Snapshot, error) {
	if e == nil || e.service == nil {
		return Snapshot{}, aria2.ErrNotRunning
	}

	if _, err := e.service.Sync(ctx); err != nil {
		return Snapshot{}, err
	}

	// The snapshot carries every download, not only the ones that
	// changed: the frontend replaces its list with the snapshot, so a
	// partial list would make the downloads that did not change
	// disappear.
	jobs, err := e.service.List(taskLimit)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{Tasks: jobs}

	if e.manager != nil {
		if client := e.manager.Client(); client != nil {
			if stat, statErr := client.GlobalStat(ctx); statErr == nil {
				snapshot.Stat = stat
			}
		}
	}

	return snapshot, nil
}

// taskLimit is how many downloads a snapshot carries.
//
// The engine keeps the history of the finished downloads, and a long
// tail is of no use to the list.
const taskLimit = 500
