# Chapter 5 — PocketBase as a library

PocketBase is a database with an HTTP API attached. Normally you run it as a server. Here it runs
**as a library inside the Go process**, and that changes several things about how you work with it.

The version is **v0.39.11**.

## What "as a library" means

The difference is one method:

```go
// internal/pocketbase/service.go
func (s *Service) Bootstrap() error {
    if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
        return fmt.Errorf("create data dir: %w", err)
    }

    s.app = pocketbase.NewWithConfig(pocketbase.Config{
        DefaultDataDir: s.dataDir,
    })

    if err := s.app.Bootstrap(); err != nil {
        return fmt.Errorf("bootstrap pocketbase: %w", err)
    }

    // Apply the Go migrations that create the BitMaher schema.
    if err := s.app.RunAppMigrations(); err != nil {
        return fmt.Errorf("run migrations: %w", err)
    }

    return nil
}
```

**`Start()` is never called.** That is the whole difference. `Start()` is what binds a port and
runs the HTTP server; without it, PocketBase is a set of Go objects over a SQLite file.

There is no PocketBase process. There is no port. There is no `pocketbase serve` in a systemd unit
or a Docker command.

### But the API is still there

The library can still serve HTTP, and BitMaher wants it to. PocketBase's own REST API — including
its **file route** — is mounted onto the Wails asset server:

```go
// internal/pocketbase/service.go
func (s *Service) Handler() (http.Handler, error) {
    router, err := apis.NewRouter(s.app)
    if err != nil {
        return nil, fmt.Errorf("create pocketbase router: %w", err)
    }
    handler, err := router.BuildMux()
    if err != nil {
        return nil, fmt.Errorf("build pocketbase mux: %w", err)
    }
    return handler, nil
}
```

and in `main.go`:

```go
Assets: application.AssetOptions{
    Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
},
```

**Why this matters:** every file the application stores is served from **the same origin as the
interface**. One port, no CORS, no second process, and a `<img src>` or a `<video>` in the webview
just works.

The `combineHandlers` is a two-line router:

```go
// combineHandlers routes /api/* to the PocketBase handler and
// everything else to the static frontend.
func combineHandlers(apiHandler, staticHandler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.HasPrefix(r.URL.Path, "/api/") {
            apiHandler.ServeHTTP(w, r)
            return
        }
        staticHandler.ServeHTTP(w, r)
    })
}
```

## The schema is Go code

There is no `pb_migrate` CLI, no `collections.json` exported from an admin UI. The collections are
created by **Go migrations** that PocketBase tracks and applies.

```go
// internal/pocketbase/migrations/1732000000_bitmaher_downloads.go
func init() {
    registerMigration("1732000000_bitmaher_downloads.go", func(app core.App) error {
        return createDownloadSchema(app)
    })
}
```

The file name is **both the identity and the ordering key**, because the list is sorted by name:

```
1730000000_init.go                    ← the scaffold's own schema
1732000000_bitmaher_downloads.go      ← the downloads
1733000000_bitmaher_history.go        ← the history and the birth times
```

A new migration must be named with a timestamp that sorts after the ones already applied. The
timestamps are not dates — they are **ordering keys**, and they have to be unique.

### The `init()` and the blank import

The migrations register themselves in an `init()`, which means they only run if the package is
imported. The import is blank, and it lives in the service:

```go
// internal/pocketbase/service.go
import (
    // Importing the migrations package registers the BitMaher schema
    // migrations with PocketBase's own migration list.
    _ "github.com/incurl/bitmaher/internal/pocketbase/migrations"
)
```

**The blank import is load-bearing.** Without it, the migrations are never registered, and the
collections never exist. The symptom is a "collection not found" error at runtime, not at compile
time — Go does not warn about an unused blank import.

This is the one place in the codebase where a missing import produces a runtime failure rather than
a compile failure. It is worth remembering when the schema looks wrong.

## Nook 1: the access rules are the write gate

Every collection has rules that say who may read it and who may write it. In PocketBase a rule is
a **string** that is evaluated as an expression; the empty string means "anybody".

```go
// internal/pocketbase/migrations/1732000000_bitmaher_downloads.go
downloads.ListRule = types.Pointer("")
downloads.ViewRule = types.Pointer("")
```

That is an **open read**. The write rules are left unset, which means **closed**: nobody may write
through the API.

The reasoning:

- **The read is open** because the application is a single-user desktop app. The local database is
  the user's own data. The frontend reads it through the API.
- **The write is closed** because every write goes through the Go services, where the invariants
  live.

That is the shape the frontend expects, and it is why the frontend never needs write access to the
API.

### The one collection with no rule at all

```go
// internal/pocketbase/settings.go — the settings record
```

`app_settings` has **no rule at all** — not an open one, not a closed one. That is stronger than
"closed writes": the collection cannot be read through the API at all, so the secrets in it are
out of reach of anything that can reach the HTTP endpoint.

**Do not open `app_settings.** It holds the RPC secret and the API key.

## Nook 2: the saved HTTP credentials

The user name and the password that go with a site are metadata, and
they are in the database like everything else.

The collection is `http_auth`, and it is **closed for reading** as well
as for writing. The credentials are the backend's business, and the
frontend asks for the one that matches a URL:

```go
// A rule that is not set at all is a closed one: PocketBase
// refuses the request for anybody who is not a superuser.
credentials.ListRule = nil
credentials.ViewRule = nil
```

The password is encrypted at rest, with the same key that protects the
other secrets. The frontend receives it in the clear, because the caller
is the download that has to authenticate itself.

The saved credential is the **fallback**, not the override:

```go
// The options are left alone when the frontend already sent a user
// name: what the user typed is what the download uses.
if request.Options["http-user"] != "" {
    return
}
```

The origin is reduced to the site before it is stored, so that a
credential saved for `https://example.com` is found for
`https://example.com/downloads/file.zip`:

```go
// The path, the query and the fragment are dropped: a credential is
// for a site, not for a page of it.
return scheme + "://" + host
```

## Nook 3: the settings are write-only from the UI's point of view

The secrets are encrypted before they are written, and the frontend only ever learns *whether* one
is set — never its value.

```go
// The value of a secret is never returned. The frontend asks whether
// a secret is set, and the backend answers.
```

The encryption is AES-256-GCM, and the key lives in a `0600` file in the data directory:

```go
// internal/pocketbase/crypto.go
const keyFileName = ".bitmaher_key"
```

```go
key = make([]byte, 32)
if _, err := rand.Read(key); err != nil {
    return nil, fmt.Errorf("generate encryption key: %w", err)
}
if err := os.WriteFile(keyPath, key, 0o600); err != nil {
    return nil, fmt.Errorf("write encryption key: %w", err)
}
```

There is a detail in there that is easy to miss and worth copying:

```go
} else {
    // The mode of an existing file is left alone by WriteFile, and
    // the file may have been copied or restored with looser
    // permissions. The key must not be readable by anyone else.
    if err := os.Chmod(keyPath, 0o600); err != nil {
        return nil, fmt.Errorf("secure the encryption key: %w", err)
    }
}
```

**`os.WriteFile` does not change the mode of an existing file.** A key file that was copied from a
backup, or restored with a `0644` mode, would stay readable. The `Chmod` is the fix.

### The envelope is self-describing

The nonce is prepended to the ciphertext, and the whole thing is base64:

```go
nonce := make([]byte, c.aead.NonceSize())
if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
    return "", fmt.Errorf("generate nonce: %w", err)
}

sealed := c.aead.Seal(nonce, nonce, plaintext, nil)
return base64.StdEncoding.EncodeToString(sealed), nil
```

`c.aead.Seal(nonce, nonce, plaintext, nil)` — the first `nonce` is the *destination* buffer, the
second is the *additional data*. Passing the nonce as additional data authenticates it, so a
tampered nonce is detected rather than silently producing garbage.

### The mutex in the cipher

```go
type Cipher struct {
    aead cipher.AEAD
    mu   sync.Mutex
    key  []byte
}
```

The lock is around the whole operation. `cipher.AEAD` implementations are safe for concurrent use
**when the nonce is unique**, and the nonce comes from `rand.Reader`, so it is. The lock is
defensive: it makes the whole encrypt-or-decrypt operation atomic with respect to the key.

## Nook 4: a collection is not a table

PocketBase's `core.Collection` is a **schema object**, not a table. The fields are typed and
validated:

```go
downloads.Fields.Add(
    &core.TextField{Name: "gid", Required: true, Max: 100},
    &core.SelectField{
        Name:      "status",
        Values:    []string{"active", "waiting", "paused", "complete", "error", "removed"},
        MaxSelect: 1,
    },
    &core.NumberField{Name: "total_length"},
    &core.JSONField{Name: "selected_files"},
    &core.FileField{Name: "torrent_file", MaxSelect: 1, MaxSize: 10 << 20},
    &core.AutodateField{Name: "created", OnCreate: true},
    &core.AutodateField{Name: "updated", OnUpdate: true},
)
```

Things worth noticing:

- **`SelectField` with `MaxSelect: 1`** is an enum. The `status` field can only hold one of those
  six values, and the database will refuse anything else. The status mapping therefore cannot
  write an invalid status — the database catches it.
- **`AutodateField`** with `OnCreate` and `OnUpdate` gives the timestamps without the code having
  to set them.
- **`JSONField`** holds structured data that the database does not need to understand. The engine
  options and the selected file indexes live there.
- **`FileField`** is a PocketBase file, served by the built-in file route. The `.torrent` file the
  user uploaded is one, and it inherits the collection's rules.

### The relation field cascades

```go
&core.RelationField{
    Name:          "job",
    CollectionId:  downloads.Id,
    MaxSelect:     1,
    CascadeDelete: true,
},
```

`CascadeDelete: true` means deleting the download deletes its logs. The collections are created in
**dependency order** so that the relation can point at an existing collection:

```go
// The collections are created in dependency order, so that the
// relation field of the log can point at the downloads.
```

## Nook 5: `app.Save` is how the schema changes

Not `app.RunAppMigrations` — that is for the migration files. A collection is created or changed
with:

```go
if err := app.Save(downloads); err != nil {
    return err
}
```

`Save` both creates and updates. PocketBase compares the collection with what is in the database and
writes the difference.

## Nook 6: the JSON field is not a bag of anything

The `options` field holds the engine options, and the engine's options are **strings**:

```go
// internal/aria2 — the engine's own options are strings
func (c *Client) ChangeOption(ctx context.Context, gid string, options map[string]string) error
```

So the JSON field holds a `map[string]string`, not a `map[string]any`. The conversion happens in
the domain layer, and it is deliberate: the engine's vocabulary should not leak into the database.

## Nook 7: the configuration is stored whole, and not interpreted

The frontend keeps all of its configuration in one value of the key-value store. The backend stores
it as one JSON blob under `app.config`:

```go
// The configuration is stored whole, so that a field the backend
// knows nothing about survives a restart.
if err := s.settings.SetAppConfig(values); err != nil {
    return err
}
```

**The backend does not interpret it.** A field the frontend adds, and the backend has never heard
of, survives a restart untouched.

This is a deliberate design decision, and it is worth preserving. The alternative — the backend
knowing the shape of the frontend's configuration — would make every change to the frontend's
settings a change to the backend.

The `ui` record is the exception, because the backend **owns** the size of the interface and has to
apply it to the window:

```go
// The size of the interface is stored as a percentage and handed
// to the frontend as the factor it works with.
config["uiScale"] = float64(ui.Zoom) / 100
```

## Nook 8: `ResetBootstrapState` is the way to close it

```go
// Close releases the resources PocketBase holds: the database
// connections and the cron ticker.
//
// It is only needed in tests and in the server mode, where the process
// outlives the request that created the service.
func (s *Service) Close() error {
    if s.app == nil {
        return nil
    }
    return s.app.ResetBootstrapState()
}
```

Two things to notice:

1. **The nil check.** `s.app` is nil until `Bootstrap` runs, and `Close` may be called on a service
   that never booted.
2. **The comment says when it is needed.** In the desktop build, the process exits and the OS
   reclaims everything. In the tests and in the server mode, the process outlives the service, so
   the connections have to be released by hand.

## Nook 9: the database is SQLite, and it is locked

Everything lives in `pb_data/` next to the executable:

```
pb_data/
├── data.db            the database
├── storage/           the uploaded files
├── .bitmaher_key      the encryption key (mode 0600)
├── aria2/
│   ├── session.txt    the engine's session
│   └── aria2-next.log the engine's log
└── .notify/           the notification outbox
```

SQLite is a **single-writer** database. Two processes writing to the same file will fight, and the
loser gets `database is locked`.

This is why the restart waits for the old instance to be gone (Chapter 3), and it is why the data
directory can be relocated for development:

```bash
BITMAHER_DATA_DIR=/tmp/my-test-data ./bin/bitmaher
```

**Never run two instances against the same data directory.** The second one will corrupt the
first's writes, or fail to start.

## Nook 10: `DefaultDataDir` is next to the executable

```go
func DefaultDataDir() string {
    if exe, err := os.Executable(); err == nil {
        return filepath.Join(filepath.Dir(exe), "pb_data")
    }
    return "pb_data"
}
```

Next to the executable, not in the home directory. That is what makes a portable installation
portable: copy the directory, and the application brings its data with it.

It is also why `bin/pb_data` exists in this repository — the binary is in `bin/`, so its data is
in `bin/pb_data/`.

## Nook 11: the collection names are part of the vocabulary

The names in the code, in the migrations, in the JSON, and in the documentation are the same:

| Collection | Holds |
| --- | --- |
| `download_jobs` | The downloads that are known |
| `job_logs` | What the application did, and what the engine said |
| `bt_presets` | The saved download presets |
| `download_history` | The downloads that have come to rest |
| `task_birth` | The moment every download was added |
| `http_auth` | The saved HTTP credentials: the user name and the password of a site |
| `app_settings` | The settings and the configuration |

`CONTEXT.md` has the full vocabulary, including the words to avoid. The rule there — the **Avoid**
synonyms — is a signal that the term is not what you want.

## Nook 12: the download record is keyed by the gid

```go
&core.TextField{Name: "gid", Required: true, Max: 100},
```

The gid is the engine's handle, and it is what links the record to the engine. It is **not** the
record's own id — PocketBase has its own id, and the two are different things.

The gid survives a restart because the engine writes it into its session file (Chapter 4). That is
the whole reason a download survives a restart with its record intact.

## The checks for this layer

```bash
go test ./internal/pocketbase/...    # the settings, the crypto, the migrations
go run ./tools/e2e-check              # the round trip through the database
```

The e2e check is the one that proves the whole thing:

```
5. The history holds the download: payload.1.bin (added at 2026-10-29 06:07:39.648Z)
6. The configuration of the frontend survived the round trip
```

## What to read next

[Chapter 6](06-vue.md) is about the frontend: the port, the platform layer, and the traps of Vue 3
in a desktop webview.
