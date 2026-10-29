# Chapter 11 — The glossary

The words this codebase uses, and what they do **not** mean.

`CONTEXT.md` carries the vocabulary of the application itself. This chapter is the vocabulary of the
**stack** — the words that come from Wails, Go, Aria2-Next, PocketBase and Vue, and that mean
something slightly different here than they do in their own documentation.

## The application

**Download**:
A transfer the application knows about: an HTTP or HTTPS link, a magnet link or a torrent file.
_Avoid_: Job, Task, Transfer, Item

**Job**:
The stored record of a download, in `download_jobs`. "Download" is what the user sees; "job" is
what the record is called in the code.
_Avoid_: Download record, Entry, Row

**GID**:
The engine's own handle for a download, a short hex string such as `d668352f1df77284`. It links a
record to the engine, and it survives a restart because the engine writes it into its session file.
_Avoid_: ID, Handle, Key

**Status**:
The normalised state: `active`, `waiting`, `paused`, `complete`, `error`, `removed`. The engine's
six states, mapped one to one. The frontend never sees the engine's raw strings.
_Avoid_: State, Phase, Stage

**Kind**:
Where a download came from: `http`, `magnet` or `torrent`. It decides which RPC call starts it and
whether a file list exists.
_Avoid_: Type, Source, Mode

**Selection**:
The one-based indexes of the files inside a torrent that the user chose. The engine only accepts the
choice while the download is paused.
_Avoid_: Files, Filter, Subset

**History**:
The downloads that have come to rest, in `download_history`. It outlives the download records.
_Avoid_: Archive, Log, Journal

**Birth**:
The moment a download was added, in `task_birth`. Written once, never changed, and what holds the
order of the list stable across a restart.
_Avoid_: Creation time, Timestamp, Added at

## The engine

**Engine / sidecar**:
The Aria2-Next process. "Engine" is what it does; "sidecar" is how it is packaged: a separate
child process the application starts, watches and stops.
_Avoid_: Server, Backend, Daemon, Worker

**Session file**:
The engine's own record of what it was doing, `pb_data/aria2/session.txt`. It carries the gid of
every unfinished download.
_Avoid_: State file, Resume file, Cache

**Watchdog**:
The goroutine that notices the engine has died and starts it again, on a new port.
_Avoid_: Supervisor, Monitor, Guard

**Reconcile**:
The pass that brings the records in step with the engine after a restart.
_Avoid_: Recover, Replay, Sync

**Sync**:
Reading the engine's state and updating the records.
_Avoid_: Refresh, Poll, Update

**Probe**:
`aria2.probe` — the engine's answer to "what would this download be?", with its size and its name,
without starting it.
_Avoid_: Preview, Head, Inspect

## The runtime

**The pump**:
The single reader of the engine's state. It ticks every 500 ms and publishes the whole state as one
snapshot.
_Avoid_: Poller, Refresher, Ticker, Watcher

**Snapshot**:
The state of the engine at one moment: the global statistics and every download. Published as the
`tasks:snapshot` event.
_Avoid_: State, Sync, Update, Delta

**The platform layer**:
The implementation of the Tauri API on top of the Wails runtime, in `frontend/src/platform/`. The
only place in the frontend that knows Wails exists.
_Avoid_: The shim, The adapter, The bridge

**The command map**:
The table that maps Rayburst's Tauri command names onto Wails service methods.
_Avoid_: The registry, The router, The dispatch table

**The bindings**:
The generated TypeScript that carries the method ids and the types. Gitignored, never hand-edited.
_Avoid_: The generated API, The stubs

**The surface**:
The set of methods and types the frontend may see. Enforced by `surface_test.go`.
_Avoid_: The API, The contract

## The window

**Hold**:
The interception of the close of the window, so that the user can be asked what to do with it.
_Avoid_: Prevent, Block, Intercept

**Release**:
Letting a held close go on, so that the window closes.
_Avoid_: Allow, Accept, Confirm

**The tray**:
The system tray icon and its menu. Its quit action goes through the same path as the menu's.
_Avoid_: The notification area, The status bar

## The database

**Settings**:
The stored preferences, in `app_settings`. The collection has **no** access rule, so the HTTP API
cannot reach it.
_Avoid_: Config, Preferences, Options

**Configuration**:
The whole configuration of the frontend, as one record under the `app.config` key. Stored as it is
given; the backend does not interpret it.
_Avoid_: Settings, Preferences

**Secret**:
A value the backend reads and the frontend never sees: the engine's RPC secret and the API key.
Encrypted with AES-256-GCM; the key is a `0600` file in the data directory.
_Avoid_: Token, Key, Password

**RPC secret**:
The token the engine authenticates its caller with. Generated once, stored encrypted, never sent to
the frontend.
_Avoid_: Password, Auth token, API key

**Migration**:
A Go function that changes the schema. The file name is the identity *and* the ordering key.
_Avoid_: Schema change, DDL, Patch

**The rules**:
The access rules of a collection. The write gate.
_Avoid_: Permissions, ACL, Policy

## The licensing boundary

**Separate process**:
How the engine runs. Its own address space, its own lifetime, its own licence. "Separate" is the
operative word.
_Avoid_: Subprocess wrapper, Embedded engine, Sandbox

**Boundary**:
The line between BitMaher's own code, which is MIT, and the engine, which is GPLv2. Held by four
things: a separate process, no linking, no vendored sources, and an opaque binary.
_Avoid_: Interface, Layer, Abstraction

**Bundle**:
The engine binary shipped inside the application. Embedded with `go:embed` and copied to a
writable cache before it is started.
_Avoid_: Binary, Payload, Asset

## The words that carry a warning

These are the ones where the everyday meaning is wrong here.

**"Void"** — a Go method that returns nothing is answered by the Wails runtime with `{}`, and `{}`
is truthy in JavaScript. "Nothing" is not "nothing" on the other side of the boundary.

**"Event"** — in the platform layer, an event with no listener is a no-op. The old window shim
emitted events that nothing heard, and every one of those calls looked like it worked.

**"Poll"** — the frontend never polls the engine. The pump is the reader; it ticks.

**"Window close"** — in the Wails runtime, `Close()` is a *request* that emits the closing event.
Your own hook sees it.

**"Empty"** — an empty object is true. An empty string is not. The difference has cost a release.

**"Restart"** — the new instance is started *by the old one*, as the old one goes down, and it
waits for the old one to be gone before it touches the database.

## The commands, for reference

The commands the backend answers, by area:

| Area | The commands |
| --- | --- |
| The files | `check_path_is_dir`, `delete_path`, `list_dir_files`, `rename_completed_file`, `read_local_file`, `write_local_file` |
| The categories | `resolve_file_category`, `validate_file_categories` |
| The preview | `preview_download`, `cancel_download_preview` |
| The state of the files | `task_file_states` |
| The engine | `engine_recover_runtime_state`, `engine_cancel`, `engine_supervisor_state` |
| The notifications | `send_app_system_notification`, `notification_access_state`, `open_notification_settings` |
| The BitTorrent blocklist | `get_bt_peer_blocklist_status`, `sync_bt_peer_blocklist`, `reconcile_bt_peer_blocklist` |
| The remote files | `fetch_remote_bytes`, `preview_site_rules` |
| The application | `set_app_log_level`, `export_diagnostic_logs`, `factory_reset`, `database_reset` |
| The lifecycle | `app_exit`, `app_relaunch`, `release_window_close`, `minimize_window_to_tray` |

And the two answers for the commands that have no backend:

| Command | The answer |
| --- | --- |
| A command that expects a boolean | `NeutralBool` — `false` |
| A command that expects a value | `NeutralValue` — a typed nil, which marshals to `null` |

## The end

That is the book. The short version of everything in it:

- **The engine pushes nothing**, so one thing reads it, and the frontend waits for the snapshot.
- **An empty object is true**, so a command with no backend answers falsy.
- **The engine is a separate process**, so its vocabulary never leaves `internal/aria2`, and it
  never outlives the application.
- **The bindings are generated**, so change the Go and regenerate.
- **The frontend does not poll**, so if the data is stale, the fix is in the pump.
