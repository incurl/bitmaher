package bindings

import (
	"reflect"
	"sort"
	"testing"
)

// expectedMethods is the exact set of methods the frontend may call.
//
// Every exported method on a bound service becomes a method in the
// generated TypeScript, so this list is the public API of the
// application. It is spelled out on purpose: adding a method is a
// deliberate act, and a method that returns an internal type is a
// mistake this test catches.
var expectedMethods = []string{
	"AddDownload",
	"AppName",
	"AppVersion",
	"CancelDownloadPreview",
	"ClearConsoleEntries",
	"DatabaseReset",
	"DeletePath",
	"DeletePreset",
	"DirectoryExists",
	"DiskFreeSpace",
	"DownloadFiles",
	"DownloadStat",
	"EngineCancel",
	"EngineRecoverRuntimeState",
	"EngineState",
	"ExportDiagnosticLogs",
	"FactoryReset",
	"FetchRemoteBytes",
	"GetBtPeerBlocklistStatus",
	"GetSettings",
	"ListConsoleEntries",
	"ListDirectoryFiles",
	"ListDownloads",
	"ListPresets",
	"NotificationAccessState",
	"OpenDefaultAppsSettings",
	"OpenExternalURL",
	"OpenNotificationSettings",
	"PathExists",
	"PathIsDirectory",
	"PauseDownload",
	"PauseDownloadAll",
	"PlatformPaths",
	"PreviewDownload",
	"PreviewSiteRules",
	"PurgeDownloads",
	"ReadLocalFile",
	"ReconcileBtPeerBlocklist",
	"RefreshRuntimeConfig",
	"RemoveDownload",
	"RenameCompletedFile",
	"ResolveFileCategory",
	"ResumeDownload",
	"SaveAria2",
	"SaveCredentials",
	"SavePreset",
	"SaveUI",
	"SelectDownloadFiles",
	"SendAppSystemNotification",
	"SetAppLogLevel",
	"ShowInFolder",
	"Shutdown",
	"StoreClear",
	"StoreDelete",
	"StoreKeys",
	"StoreRead",
	"StoreReadValue",
	"StoreWrite",
	"SyncBtPeerBlocklist",
	"SyncDownloads",
	"TaskFileStates",
	"UnsupportedCommand",
	"ValidateFileCategories",
	"WriteLocalFile",
	"WriteLog",
}

// TestAppServiceExposesExactlyTheIntendedMethods pins the bound surface.
//
// This is the regression test for the finding of the 2026-10-28 code
// review: JobManager, Console and Settings were bound as methods and
// returned *jobs.Manager, *console.Log and *pocketbase.SettingsStore.
// Those types hold unexported fields, so they serialise to an empty
// object, and the generated bindings offered three methods that could
// never work. Nothing called them, and nothing noticed.
func TestAppServiceExposesExactlyTheIntendedMethods(t *testing.T) {
	typ := reflect.TypeOf(&AppService{})

	var actual []string
	for i := 0; i < typ.NumMethod(); i++ {
		actual = append(actual, typ.Method(i).Name)
	}
	sort.Strings(actual)

	if len(actual) != len(expectedMethods) {
		t.Errorf("AppService has %d methods, want %d\n got: %v\nwant: %v",
			len(actual), len(expectedMethods), actual, expectedMethods)
		return
	}

	for i, name := range actual {
		if name != expectedMethods[i] {
			t.Errorf("method %d is %q, want %q\n got: %v\nwant: %v",
				i, name, expectedMethods[i], actual, expectedMethods)
			return
		}
	}
}

// TestNoBoundMethodReturnsAnInternalType verifies that no bound method
// hands back a type from an internal package.
//
// A returned value is serialised, so a type with only unexported fields
// arrives as an empty object. The frontend can do nothing with it.
func TestNoBoundMethodReturnsAnInternalType(t *testing.T) {
	typ := reflect.TypeOf(&AppService{})

	// The value types the frontend genuinely needs.
	allowed := map[string]bool{
		"bindings.SettingsView":          true,
		"bindings.DownloadView":          true,
		"bindings.FileView":              true,
		"bindings.Paths":                 true,
		"bindings.EngineState":           true,
		"bindings.Preview":               true,
		"bindings.SiteConnectionCap":     true,
		"bindings.BtPeerBlocklistStatus": true,
		"bindings.NotificationAccess":    true,
		"category.Category":              true,
		"jobs.Artifact":                  true,
		"jobs.Job":                       true,
		"jobs.Request":                   true,
		"console.Entry":                  true,
		"pocketbase.UI":                  true,
		"pocketbase.Aria2":               true,
		"downloads.Preset":               true,
		"aria2.GlobalStat":               true,

		// The plain types serialise to themselves, so the frontend can
		// read them. The platform commands answer with them: a path, a
		// version, a number of bytes.
		"string":       true,
		"bool":         true,
		"int64":        true,
		"uint8":        true,
		"interface {}": true,

		// The file states are the values of a map, and the frontend
		// reads them.
		"bindings.TaskFileState": true,
	}

	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		results := method.Type.NumOut()

		// Skip the error result; check the value result.
		for r := 0; r < results; r++ {
			out := method.Type.Out(r)

			// A pointer or a slice of a value type is fine: the
			// frontend receives the serialised element.
			for out.Kind() == reflect.Pointer || out.Kind() == reflect.Slice {
				out = out.Elem()
			}

			// A map is fine when its values are: the frontend
			// receives the serialised entries.
			if out.Kind() == reflect.Map {
				out = out.Elem()
				for out.Kind() == reflect.Pointer {
					out = out.Elem()
				}
			}

			if out.Kind() == reflect.Interface && out.String() != "interface {}" {
				continue // the error interface
			}

			name := out.String()
			if !allowed[name] {
				t.Errorf("%s returns %s, which the frontend cannot use", method.Name, name)
			}
		}
	}
}
