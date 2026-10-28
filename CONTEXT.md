# BitMaher vocabulary

The terms below are shared between the Go backend, the React frontend and the documentation. The same terms appear in code as
collection names (`download_jobs`, `job_logs`, `bt_presets`, `app_settings`) and as JSON field names.

Use the **Avoid** synonyms as a signal that the term is not what you want.

## The language

**Download**:
A transfer the application knows about: an HTTP or HTTPS link, a magnet link or a torrent file. It carries the engine's handle,
a normalised status, the counters, the error and the source.
_Avoid_: Job, Task, Transfer, Item

**Job**:
The stored record of a download, in the `download_jobs` collection. "Download" is what the user sees; "job" is what the record
is called in the code and in the collection name.
_Avoid_: Download record, Entry, Row

**GID**:
The engine's own handle for a download, a short hex string such as `d668352f1df77284`. It is what every RPC call refers to, and
it is what links a record to the engine. It survives a restart, because the engine writes it into its session file.
_Avoid_: ID (that is the record's), Handle, Key

**Engine / sidecar**:
The Aria2-Next process. "Engine" is what it does; "sidecar" is how it is packaged: a separate child process that the
application starts, watches and stops. Both words are used, and they mean the same thing.
_Avoid_: Server, Backend, Daemon, Worker

**Status**:
The normalised state of a download: `active`, `waiting`, `paused`, `complete`, `error` or `removed`. It is the engine's own
six states, mapped one to one. The frontend never sees the engine's raw strings.
_Avoid_: State, Phase, Stage

**Kind**:
Where a download came from: `http`, `magnet` or `torrent`. It decides which RPC call starts it and whether a file list exists.
_Avoid_: Type, Source, Mode

**Session file**:
The engine's own record of what it was doing, `pb_data/aria2/session.txt`. It carries the gid of every unfinished download,
which is what lets a download be picked up again after the application restarts.
_Avoid_: State file, Resume file, Cache

**Watchdog**:
The goroutine that notices the engine has died and starts it again, on a new port. It is on by default and can be turned off in
the settings.
_Avoid_: Supervisor, Monitor, Guard

**Reconcile**:
The pass that brings the records in step with the engine after a restart: the engine has picked up its session, and the records
are updated to match. It is what makes the list show the truth rather than what it showed before the restart.
_Avoid_: Recover, Replay, Sync

**Sync**:
Reading the engine's state and updating the records. It is what the frontend polls, because the engine pushes nothing.
_Avoid_: Refresh, Poll (that is what the frontend does), Update

**Selection**:
The one-based indexes of the files inside a torrent that the user chose. The engine only accepts the choice while the download
is paused.
_Avoid_: Files, Filter, Subset

**Preset**:
A saved set of engine options, in the `bt_presets` collection. It is the "download the same way again".
_Avoid_: Profile, Template, Default

## The licensing boundary

**Separate process**:
How the engine runs. It has its own address space, its own lifetime and its own licence. The only coupling is a loopback socket
carrying JSON-RPC. "Separate" is the operative word: it is what keeps the two programs two programs.
_Avoid_: Subprocess wrapper, Embedded engine, Sandbox

**Boundary**:
The line between BitMaher's own code, which is MIT, and the engine, which is GPLv2. It is held by four things: a separate
process, no linking, no vendored sources, and a binary shipped as an opaque executable.
_Avoid_: Interface, Layer, Abstraction

## The infrastructure

**Settings**:
The stored preferences, in the `app_settings` collection. The collection has **no** access rule, so the HTTP API cannot reach it
and the Go backend is the only reader.
_Avoid_: Config, Preferences, Options

**Secret**:
A value the backend reads and the frontend never sees: the engine's RPC secret and the API key. It is encrypted with AES-256-GCM
before it is written, and the key is a `0600` file in the data directory. The frontend only ever learns *whether* one is set.
_Avoid_: Token, Key, Password

**RPC secret**:
The token the engine authenticates its caller with. It is generated once, on the first run, and stored encrypted. It never leaves
the machine and is never sent to the frontend.
_Avoid_: Password, Auth token, API key

**Console**:
The in-memory debug log at the bottom of the window, toggled with `Ctrl+Shift+I`. It holds the last 500 entries and is not
persisted: it is a debugging aid, not an audit trail.
_Avoid_: Log, Output, Terminal

**Bundle**:
The engine binary shipped inside the application. It is embedded with `go:embed` and copied to a writable cache directory before
it is started, because the install directory is not always writable.
_Avoid_: Binary, Payload, Asset
