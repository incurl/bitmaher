package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that creates the collection of the
// saved HTTP credentials.
//
// The file name sorts after 1733000000_bitmaher_history.go, so it runs
// after the history schema is in place.
func init() {
	registerMigration("1734000000_bitmaher_http_auth.go", func(app core.App) error {
		return createHttpAuthSchema(app)
	})
}

// createHttpAuthSchema creates the collection of the saved HTTP
// credentials.
//
// The credentials are the ones a site asks for: the user name and the
// password that go with a URL. They are used for the downloads that
// need them, and they are saved so that the user does not have to type
// them again.
//
// The password is encrypted before it is written, with the same key
// that protects the other secrets of the application. The collection
// is closed for reading as well as for writing: the credentials are
// the backend's business, and the frontend asks for the one that
// matches a URL.
func createHttpAuthSchema(app core.App) error {
	credentials := core.NewBaseCollection("http_auth")
	credentials.Fields.Add(
		// The origin the credentials belong to. It is the URL
		// without its path, and it is what a download is matched
		// against.
		&core.TextField{Name: "origin", Required: true, Max: 2000},
		// The user name.
		&core.TextField{Name: "username", Max: 500},
		// The password, encrypted with the key of the application.
		&core.TextField{Name: "password", Max: 4000},
		// The moment the credentials were last used. It is what the
		// panel shows, and it is not a secret.
		&core.DateField{Name: "last_used_at"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)

	// The read is closed: the credentials are the backend's
	// business. The frontend asks for the one that matches a URL,
	// and the backend answers with it.
	//
	// A rule that is not set at all is a closed one: PocketBase
	// refuses the request for anybody who is not a superuser.
	credentials.ListRule = nil
	credentials.ViewRule = nil

	return app.Save(credentials)
}
