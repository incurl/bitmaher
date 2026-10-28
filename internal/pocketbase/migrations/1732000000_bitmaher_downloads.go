package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// init registers the migration that creates the BitMaher download
// schema.
//
// The file name sorts after 1730000000_init.go, so it runs after the
// scaffold's own schema is in place.
func init() {
	registerMigration("1732000000_bitmaher_downloads.go", func(app core.App) error {
		return createDownloadSchema(app)
	})
}

// createDownloadSchema creates the download collections.
//
// The collections are created in dependency order, so that the
// relation field of the log can point at the downloads.
func createDownloadSchema(app core.App) error {
	// --- download_jobs ----------------------------------------------
	// The metadata of every download the application knows about.
	//
	// The read rules are open, because the read is the user's own
	// data and the frontend reads it through the API. Every write
	// goes through the Go services, so the write rules are closed.
	downloads := core.NewBaseCollection("download_jobs")
	downloads.Fields.Add(
		// The engine's handle for the download. It is what links a
		// record to the engine, and it is unique.
		&core.TextField{Name: "gid", Required: true, Max: 100},
		&core.SelectField{
			Name:      "kind",
			Values:    []string{"http", "torrent", "magnet"},
			MaxSelect: 1,
		},
		&core.SelectField{
			Name:      "status",
			Values:    []string{"active", "waiting", "paused", "complete", "error", "removed"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "title", Max: 500},
		&core.TextField{Name: "uri", Max: 2000},
		&core.TextField{Name: "dir", Max: 1000},
		&core.NumberField{Name: "total_length"},
		&core.NumberField{Name: "completed_length"},
		&core.NumberField{Name: "upload_length"},
		&core.NumberField{Name: "download_speed"},
		&core.NumberField{Name: "upload_speed"},
		&core.NumberField{Name: "connections"},
		&core.NumberField{Name: "seeders"},
		&core.NumberField{Name: "peers"},
		&core.TextField{Name: "info_hash", Max: 100},
		&core.NumberField{Name: "error_code"},
		&core.TextField{Name: "error_message", Max: 2000},
		// The .torrent file the user uploaded. It is a PocketBase
		// file field, so it is served by the built-in file route and
		// inherits the collection's rules.
		&core.FileField{Name: "torrent_file", MaxSelect: 1, MaxSize: 10 << 20},
		// The one-based indexes of the files chosen inside the
		// torrent.
		&core.JSONField{Name: "selected_files"},
		// The engine options the download was started with.
		&core.JSONField{Name: "options"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	downloads.ListRule = types.Pointer("")
	downloads.ViewRule = types.Pointer("")
	if err := app.Save(downloads); err != nil {
		return err
	}

	// --- job_logs ---------------------------------------------------
	// The log of a download: what the application did and what the
	// engine said.
	logs := core.NewBaseCollection("job_logs")
	logs.Fields.Add(
		&core.RelationField{
			Name:          "job",
			CollectionId:  downloads.Id,
			MaxSelect:     1,
			CascadeDelete: true,
		},
		&core.SelectField{
			Name:      "level",
			Values:    []string{"info", "warn", "error", "raw"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "message", Max: 2000},
		&core.TextField{Name: "detail", Max: 20000},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	logs.ListRule = types.Pointer("")
	logs.ViewRule = types.Pointer("")
	if err := app.Save(logs); err != nil {
		return err
	}

	// --- bt_presets --------------------------------------------------
	// The saved download presets.
	presets := core.NewBaseCollection("bt_presets")
	presets.Fields.Add(
		&core.TextField{Name: "name", Required: true, Max: 200},
		&core.SelectField{
			Name:      "kind",
			Values:    []string{"http", "torrent", "magnet"},
			MaxSelect: 1,
		},
		&core.JSONField{Name: "options"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	presets.ListRule = types.Pointer("")
	presets.ViewRule = types.Pointer("")
	if err := app.Save(presets); err != nil {
		return err
	}

	return nil
}
