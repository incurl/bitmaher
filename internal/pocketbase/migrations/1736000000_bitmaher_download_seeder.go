package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that adds the fields of a finished
// torrent.
//
// The file name sorts after 1735000000_bitmaher_ui_reconcile.go, so it
// runs last.
func init() {
	registerMigration("1736000000_bitmaher_download_seeder.go", func(app core.App) error {
		return addDownloadSeederFields(app)
	})
}

// addDownloadSeederFields adds the two fields that say that a download
// is over.
//
// The engine keeps a finished torrent running so that it seeds, and
// the status stays "active" for as long as it does. The flag and the
// state of the torrent are what say that the download itself is over,
// and the record had nowhere to keep them.
func addDownloadSeederFields(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("download_jobs")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		// The download is sharing the content it already has.
		&core.BoolField{Name: "seeder"},
		// The state of the torrent itself.
		&core.TextField{Name: "torrent_state", Max: 40},
	)

	return app.Save(collection)
}
