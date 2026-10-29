package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// init registers the migration that creates the BitMaher history
// schema.
//
// The file name sorts after 1732000000_bitmaher_downloads.go, so it
// runs after the download schema is in place.
func init() {
	registerMigration("1733000000_bitmaher_history.go", func(app core.App) error {
		return createHistorySchema(app)
	})
}

// createHistorySchema creates the history collections.
//
// The history is what the user has already downloaded. It outlives the
// download records: a download that is removed from the list stays in
// the history, and the history is what the interface shows in its own
// panel.
func createHistorySchema(app core.App) error {
	// --- download_history --------------------------------------------
	// The downloads that have come to rest.
	//
	// The read rules are open, because the read is the user's own
	// data and the frontend reads it through the API. Every write
	// goes through the Go services, so the write rules are closed.
	history := core.NewBaseCollection("download_history")
	history.Fields.Add(
		// The engine's handle for the download. It is what links a
		// record to the engine, and it is unique.
		&core.TextField{Name: "gid", Required: true, Max: 100},
		// The display name of the download.
		&core.TextField{Name: "name", Max: 500},
		// The source: the URL or the magnet link.
		&core.TextField{Name: "uri", Max: 2000},
		// The directory the download was written to.
		&core.TextField{Name: "dir", Max: 1000},
		// The size of the download in bytes.
		&core.NumberField{Name: "total_length"},
		// The terminal status: "complete", "error" or "removed".
		&core.SelectField{
			Name:      "status",
			Values:    []string{"complete", "error", "removed"},
			MaxSelect: 1,
		},
		// The kind of the download: "http", "torrent" or "magnet".
		&core.TextField{Name: "task_type", Max: 20},
		// The moment the download was added. It never changes, and it
		// is what keeps the order of the list stable across a
		// restart.
		&core.TextField{Name: "added_at", Max: 40},
		// The moment the download finished.
		&core.TextField{Name: "completed_at", Max: 40},
		// The metadata of the download: the info hash of a torrent,
		// the files it holds, and what it shared.
		&core.JSONField{Name: "meta"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	history.ListRule = types.Pointer("")
	history.ViewRule = types.Pointer("")
	if err := app.Save(history); err != nil {
		return err
	}

	// --- task_birth ---------------------------------------------------
	// The moment every download was added.
	//
	// It is written when the download starts and never changed, so the
	// order of the list is the same after a restart as it was before
	// it. The history holds the same moment for the downloads that
	// have come to rest.
	births := core.NewBaseCollection("task_birth")
	births.Fields.Add(
		&core.TextField{Name: "gid", Required: true, Max: 100},
		&core.TextField{Name: "added_at", Max: 40},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	births.ListRule = types.Pointer("")
	births.ViewRule = types.Pointer("")
	if err := app.Save(births); err != nil {
		return err
	}

	return nil
}
