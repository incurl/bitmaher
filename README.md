# BitMaher

A desktop download manager for Linux, macOS and Windows.

BitMaher gives a clean, low-friction interface to the
[Aria2-Next](https://github.com/AnInsomniacy/aria2-next) download engine:
a list of the downloads with their progress, the details of each one, and
a dialog to add a link, a magnet or a torrent file.

It is built on [Wails v3](https://wails.io) (the desktop shell),
[PocketBase](https://pocketbase.io) (the embedded database) and React (the
interface).

## The download engine, and why it is a separate process

Aria2-Next is licensed under the **GPL**. BitMaher is not.

The two are kept apart on purpose:

- Aria2-Next runs as a **separate, independent child process**. BitMaher
  starts it, watches it, drives it over JSON-RPC and stops it again.
- Nothing in this repository links against Aria2-Next, imports its
  headers, vendors its sources or calls into it through CGO. The only
  coupling is a loopback socket carrying JSON-RPC.
- The binary is shipped as an opaque executable. Its sources are offered
  by the upstream project, which is what the GPL requires of a binary
  distribution. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

If that boundary is ever broken, the licensing of the whole application
changes. Keep it.

## Getting started

```bash
# The bundled aria2-next binaries (about 18 MB each, not in the repository).
PATH="$HOME/go/bin:$PATH" wails3 task common:fetch:aria2

# The frontend dependencies.
cd frontend && npm install --legacy-peer-deps && cd ..

# The development build.
PATH="$HOME/go/bin:$PATH" wails3 task dev
```

A system-installed `aria2-next` on the `PATH` is used when there is no
bundled binary, so the application also runs straight from a checkout.

### The production build

```bash
PATH="$HOME/go/bin:$PATH" wails3 task build      # -> bin/bitmaher
PATH="$HOME/go/bin:$PATH" wails3 task package     # the platform packages
```

## How it is put together

```
frontend/            the React interface
  src/features/downloads/   the list, the sidebar, the dialogs
  src/bindings/              the typed wrappers over the Wails bindings
internal/
  aria2/             the sidecar: the lifecycle and the JSON-RPC client
  downloads/         the domain: the records and the service
  pocketbase/        the embedded database and its migrations
  bindings/          the Wails service: the API of the application
```

The engine's vocabulary never reaches the frontend. `internal/aria2`
translates the six states Aria2-Next reports into the six the interface
shows, and turns its stringly typed numbers into integers.

### What the engine does and does not do

The engine does the transfers: HTTP, HTTPS, BitTorrent and Metalink.
BitMaher owns the interface, the persistence and the lifecycle of the
sidecar.

Two things are worth knowing about the engine, because they shape the
design:

- **It does not push its updates.** The WebSocket transport accepts a
  connection, but the engine never sends anything on it, and there is no
  `aria2.onDownloadStart`. The interface therefore polls the backend,
  which reads the engine's state and updates the records.
- **A torrent's files can only be chosen while the download is paused.**
  The engine refuses to change the selection of a running download, so
  the backend pauses it, applies the choice and resumes it.

## The data

Everything lives in `pb_data/` next to the executable:

| What | Where |
| --- | --- |
| The database | `pb_data/data.db` |
| The uploaded torrent files | `pb_data/storage/` |
| The encryption key | `pb_data/.bitmaher_key` (mode `0600`) |
| The engine's session file | `pb_data/aria2/session.txt` |
| The engine's log | `pb_data/aria2/aria2-next.log` |

`BITMAHER_DATA_DIR` relocates it, which is how the application is run
against an isolated database during development.

## The checks

```bash
# The backend
gofmt -l internal/ main.go main_test.go tools/   # must print nothing
go vet ./internal/... .
go test -race ./internal/... .

# The live checks, against a real engine
go run ./tools/e2e-check          # the whole pipeline, with a real sidecar
go run ./tools/restart-check      # a download survives a restart
go run ./tools/watchdog-check     # a crashed sidecar is started again
go run ./tools/bundled-check      # the bundled binary is found and run
go run ./tools/leak-check         # no sidecar outlives the application

# The frontend
cd frontend && npm run typecheck && npm test && npm run build

# The licences
python3 tools/license-audit.py
```

## The licence

The application is MIT licensed. See [LICENSE](LICENSE).

The bundled download engine is GPL-2.0 and is covered by
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
