# Architecture

Why Valkyrie is shaped the way it is. Each section states the decision and the reason, so that a later change does not undo it
by accident.

## The stack

| Layer | Choice | Why |
| --- | --- | --- |
| Shell | Wails v3 | A native window around a webview. The Go process owns the window and the backend; the webview owns the UI. |
| Backend | Go | One binary, one process, no sidecar. |
| Storage | PocketBase, as a library | Gives a real database, migrations, file storage and access rules without running a server. |
| UI | Vue 3 + Vite + Pinia + vue-i18n + Naive UI | Ported from Rayburst, the frontend of Motrix Next. MIT licensed, so it could be reused as it is. |

## PocketBase runs as a library, not as a process

`internal/pocketbase/service.go` calls `pocketbase.NewWithConfig(...)` and `Bootstrap()`, and **never** calls `Start()`. No HTTP
server is bound by PocketBase, and there is no second process to supervise.

Its REST and file API is mounted on the Wails asset server instead:

```go
Assets: application.AssetOptions{
    Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
},
```

`combineHandlers` routes `/api/*` to PocketBase and everything else to the built frontend. The consequence is the important
one: **a stored file is served from the same origin as the UI**. No CORS handling, no second port, no cookie or token
plumbing. The file route is `/api/files/<collection>/<recordId>/<filename>`, and it is the only way a file reaches the
frontend.

`combineHandlers` is covered by a test in `main_test.go`. That test is the guard: if the routing is broken, the files stop
loading and nothing else fails loudly.

## The job queue is generic on purpose

`internal/jobs` knows about jobs, artifacts and their lifecycle. It knows nothing about images, models, prompts or any other
domain. A job is a title, a prompt, a `map[string]any` of params and an arbitrary result.

The work is delegated to a `jobs.Runner`:

```go
type Runner interface {
    Run(ctx context.Context, job Job) (*Result, error)
}
```

This is the single most important design decision in the template. It means the queue, the persistence, the cancellation, the
events and the history are written once and never touched again, no matter what the application does. An application that
generates images, transcribes audio or scrapes a website changes `internal/runner/` and nothing else.

**Why a `map[string]any` for the params and the result, rather than a typed struct?** A typed struct would have to be
generified into `map[string]any` at the boundary anyway, and the generation would lose the field names. Keeping it loose means
the runner receives exactly what the frontend sent, and the frontend receives exactly what the runner returned. The cost is
that the shape is not checked at compile time; the price of that is paid in the runner, where the application knows its own
domain.

## The queue mechanics

- **Bounded concurrency.** `Manager.queue` is a buffered channel. A job acquires a slot before it runs. This is what stops a
  burst of submissions from flooding a rate-limited backend.
- **Cancellation is a context.** Each running job gets a `context.Context` whose cancel function is registered in
  `Manager.cancels`. `Cancel` calls it and marks the record `cancelled`. A runner that ignores the context will keep running
  until it finishes — the scaffold cannot force it to stop, so a runner **must** honour `ctx`.
- **A cancelled job is not a failed job.** When a runner returns an error after the context was cancelled, the manager checks
  `ctx.Err()` and keeps the `cancelled` status. The user asked for it; it is not an error.
- **Deletion requires rest.** `DeleteJob` refuses while a job is `pending` or `running`, because a running job would write to
  the database after its record was removed. It deletes the job's artifacts in the same call, so no orphan files are left in
  the storage directory.
- **A job has a time limit.** The default is 30 minutes. The runner receives a context that expires on its own, so a runner
  that never returns cannot hold a slot forever. `WithTimeout` changes the limit; zero removes it.
- **The manager owns a lifecycle.** Its root context is the parent of every job context, so `Shutdown` stops them all at once
  and waits for them. The Wails `OnShutdown` hook calls it, so quitting the application no longer kills a job mid-write.
- **The interrupted jobs are reconciled at startup.** A job that was pending or running when the process died is marked as
  failed with an explanation, so it appears in the history and the user can delete it.
- **The events are a push, and there is no polling.** The Go pump (`internal/pump`) is the single reader of the engine's
  state. It ticks every 500 ms and publishes the whole state as a `tasks:snapshot` event. The frontend waits for the snapshot
  and replaces its list with it. An action that changes a download wakes the pump, so the change shows up at once.

  This mirrors Rayburst, whose Rust backend pushes the same event. The engine itself pushes nothing — there is no
  `aria2.onDownloadStart` and no notification of a download that moved — so the pump is what turns the polling into a push.

## The backend answers the commands of the port

The interface is a port of Rayburst, and Rayburst's frontend calls the
Tauri commands of its Rust backend. BitMaher's backend is Go, so the
commands are answered by the `AppService` and mapped in
`frontend/src/platform/commands.ts`.

The commands that belong to a download manager are implemented: the file
system, the categories, the preview, the state of the files on the
disk, the recovery of the engine, the notifications, the BitTorrent
peer blocklist and the remote files.

The rest — the tray, the float window, the updater, the protocol
handler, the browser extension — answer with the neutral value. They
belong to features BitMaher does not have, and the components that call
them stay inert.

The categories are the ones Rayburst uses. The rules are in
`internal/downloads/category`, and the tests are the ones Rayburst
wrote for its own rules: the same download is classified the same way
by both applications.

The preview asks the engine. Aria2-Next has `aria2.probe`, which
answers with the size and the name a download would get, and the
backend uses it instead of making a request of its own.

## The secrets are write-only

PocketBase has no generic secret-encryption facility, so `internal/pocketbase/crypto.go` provides one: AES-256-GCM with a key
generated on first run and stored in a `0600` file in the data directory.

Two independent layers protect a secret:

1. The `app_settings` collection has **no** list or view rule, so PocketBase's own API never returns it. The Go backend reads it
   directly through the record API.
2. The value itself is encrypted before it is written.

The frontend therefore learns only whether a key is present (`has_api_key`), never the key. `SaveCredentials` keeps the stored
key when the submitted one is empty, so the UI never has to read back a secret it cannot see.

The debug console shows `console.Fingerprint(secret)` — a short, non-reversible digest — so that two keys can be told apart in a
log line without either being revealed.

## The migrations are the schema

The schema lives in `internal/pocketbase/migrations/`, not in a JSON file and not in the code that uses it. Each migration
registers itself with `core.AppMigrations` in an `init()`, and the blank import in `internal/pocketbase/service.go` makes the
package discoverable.

**The file name is the identity and the ordering key.** PocketBase sorts the migration list by name, so:

- a new migration must be named with a timestamp that sorts **after** the ones already applied;
- the name must be **stable** once applied, because PocketBase records it and will not re-run it under a different name;
- to make a migration idempotent, check whether the field already exists before adding it.

The shipped migrations are `1730000000_init.go`, `1732000000_bitmaher_downloads.go` and `1733000000_bitmaher_history.go`. A
new one would be `1734000000_add_notes.go`.

## The configuration of the frontend is one record

The ported frontend keeps its whole configuration in one value of a key-value store: 143 fields, from the colour scheme
to the speed limits. The backend stores that record as it is, under the `app.config` key of `app_settings`.

**The backend does not interpret it.** It does not know what `btMaxPeers` means, and it does not have to: the record is
stored whole and handed back whole, so a field the backend knows nothing about still survives a restart.

The size of the interface is the one exception, because the backend owns it: it is what the zoom of the window is. It is
therefore stored twice — in the `ui` record, as the percentage a person reads, and in the configuration, as the factor the
frontend works with. The two are written together and read back together, so they cannot drift apart.

The store used to keep only `uiScale` and `theme`, and drop the other 141 fields. Every setting the user had changed was
gone on the next start.

## The history outlives the download records

A download that is removed from the list is still a download the user did. The history is what the interface shows in its
own panel, and it is kept in two collections:

- `download_history` holds the downloads that have come to rest: their name, their size, their status, and the moment
  they finished.
- `task_birth` holds the moment every download was **added**, written once and never changed.

The moment of the birth is what holds the order of the list stable across a restart. Without it the order would follow the
order the engine happens to report the downloads in, and the list would shuffle every time the application started.

Both are written by `downloads.Sync`, which is the one reader of the engine's state, so there is a single place where a
download becomes part of the history.

## The frame of the window

The window is frameless on Windows and Linux, because the interface draws its own caption buttons there. A framed window
would show the native ones as well, and the user would see two sets of minimize, maximize and close.

macOS is the other way round: the traffic lights are the native ones, and the interface draws nothing in that corner.
`Frameless` is therefore false on macOS, because Wails hides the traffic lights when it is true. The macOS title bar is
the hidden-inset one, so the lights sit over the draggable chrome of the layout.

The **system title bar** preference turns the native frame back on, for the Linux window managers that force their own
decorations. It calls `SetWindowDecorations`, which is `SetFrameless` on the Wails window.

## The end of the application is the backend's business

The webview cannot end the process on its own. The Quit action of the interface asks the Wails runtime to quit, and the
runtime asks the application to end: the shutdown hook runs, the sidecar is stopped, the window is torn down, and the process
leaves. The order matters — the engine is a child process, and it must not outlive the application that started it.

The **close of the window** is a different thing, and it is held by the backend. Wails maps the native close of every platform
onto one event (`common:WindowClosing`), so a single hook catches the taskbar close, the overview cross and the traffic lights
alike. The hook cancels the close and asks the frontend what to do with it: the window goes away, or the application goes with
it. The frontend answers, and the backend releases the close.

The **restart** is the backend's business too, because the backend owns the sidecar and has to bring it up again. The new
instance is started by the old one as it goes down, in a process group of its own, and it waits for the old one to be gone
before it touches the database: the database belongs to one process at a time.

## The neutral answers of the features BitMaher does not have

The ported frontend asks for the features of Rayburst that BitMaher does
not have. Those calls are answered, so that the components that use them
stay inert and the gap is visible in the debug console.

The answer has to be of the type the frontend expects, and the runtime
makes that harder than it looks. A method that returns **nothing** is
answered with an **empty object**, and an empty object is **true** in
JavaScript. A question about a feature BitMaher does not have would
then be answered with a yes, and the frontend would act on it.

That is not a theory: the question about the autostart was answered with
a yes, the frontend took the branch that hides the window, and the
window flashed and disappeared at start-up.

The answers are therefore two:

- `NeutralBool` answers `false`, for the commands that expect a boolean.
- `NeutralValue` answers a **typed nil**, for the commands that expect a
  value. A typed nil is marshalled to `null`, which is the nothing the
  frontend expects. The nil of an interface would be turned into an
  empty object by the runtime, and would be true again.

The command map knows which of the two each command wants.

## PocketBase is the only record of truth

Every piece of metadata the application keeps is in the database. There
is no second place where it could be, and no file that holds it.

| What | Where | Written by |
| --- | --- | --- |
| The configuration of the frontend | `app_settings`, key `app.config` | `StoreWrite` |
| The size of the interface and the theme | `app_settings`, key `ui` | `StoreWrite` |
| The RPC secret of the engine | `app_settings`, key `app.aria2_rpc_secret` | `startSidecar` |
| The settings of the engine | `app_settings`, key `app.aria2` | `RefreshRuntimeConfig` |
| The key of the remote API | `app_settings`, key `app.api_key` | `SaveCredentials` |
| The saved HTTP credentials | `http_auth` | `HTTPAuthSave` |
| The downloads that are running | `download_jobs` | `downloads.Sync` |
| The downloads that have come to rest | `download_history` | `downloads.Sync` |
| The moment every download was added | `task_birth` | `downloads.Sync` |
| The saved presets | `bt_presets` | `SavePreset` |
| The log of a download | `job_logs` | `downloads` |

Two things are **not** in the database, and they are not metadata of
the application:

- **The encryption key** (`pb_data/.bitmaher_key`). It is the key of
  the database, and a database cannot hold its own key.
- **The session file of the engine** (`pb_data/aria2/session.txt`). It
  is the engine's own file, in the engine's own format. The application
  keeps the record of the downloads in `download_jobs`, and the engine
  keeps the record of what it is doing in the file it wrote.

### The saved HTTP credentials

They are the user name and the password that go with a site. The
password is encrypted at rest with the key of the application, and the
collection is **closed for reading** as well as for writing: the
credentials are the backend's business, and the frontend asks for the
one that matches a URL.

The saved credential is the **fallback**, not the override. A download
that the user gave a user name of uses that one; a download that did
not get one uses the saved credential of the site, if there is one.

## The access rules are the write gate

The scaffold sets `ListRule` and `ViewRule` to `""` (public read) on `jobs` and `artifacts`, and leaves `app_settings` without
any rule. The BitMaher collections — `download_jobs`, `job_logs`, `bt_presets`, `download_history` and `task_birth` — follow
the same shape: public read, closed write.

Read access is public because the application is a single-user desktop app: the local database is the user's own data. Write
access is **closed**, because every write goes through the Go services, where the invariants live. That is the shape the
frontend expects, and it is why the frontend never needs write access to the API.

`app_settings` gets no rule at all, which is what keeps it out of reach of the HTTP API.

## The queue knows about images, on purpose

`internal/jobs/media.go` decodes image headers, and the `artifacts` collection carries `width` and `height` columns. That is
image-specific knowledge inside a package whose headline claim is that the queue is generic.

It is a deliberate trade-off, and this is the honest statement of it: **images are the common case, and detecting the format
from the bytes is the thing that is easy to get wrong.** Storing a JPEG under a `.png` name produces a file that every tool
misidentifies. The scaffold does that detection once, correctly, rather than leaving it to each application.

The cost is that a queue storing audio, PDFs or JSON still runs `DimensionsOf`, still fails the decode, and still carries two
columns it never fills. The decode failure is ignored, so nothing breaks; the columns stay zero.

If an application is not image-based, the clean fix is to move the dimensions into the `artifacts.meta` JSON field, which
needs a migration. Until then, this is the trade-off, and it is not a bug.

## The listing is batched

`ListJobs` converts a whole page at once. It fetches the artifacts of the page with a single query, rather than letting each
job collect its own: without that, a page of N jobs costs N+1 queries, and the frontend polls the listing every five
seconds.

The query is a raw `dbx.In`, because the PocketBase search filter has no `IN` operator. It goes through `app.RecordQuery`
rather than `app.DB()`, so that the records are scanned with their collection context — a bare `core.Record` panics in
`PostScan` without one.

## The inputs and the outputs are separate

An artifact belongs to one job, and it is either an **input** (something the job consumed) or an **output** (something it
produced). A job tracks the two separately.

This matters because a retry re-uses the inputs of the job it repeats. Moving them would strip the original job of its
outputs and hand them to the retry, and deleting the retry would then delete files the original still refers to. So the
manager **copies** an artifact that already belongs to another job, and only claims one that belongs to no job.

## The frontend talks to Go, not to the database

Every backend call goes through the Wails bindings. The frontend never touches PocketBase directly, and never calls a remote
API directly. The path is:

```
React component
  → frontend/src/bindings/index.ts      (typed wrapper, normalises nullables)
    → frontend/bindings/.../appservice.ts   (generated)
      → Go: internal/bindings/service.go     (AppService)
        → internal/jobs/ or internal/pocketbase/
```

`frontend/src/bindings/index.ts` exists so that no component ever imports a generated file. It also normalises the nullable
arrays the generator produces: the backend always sends an array, but the generated type says `string[] | null`, so the
wrapper narrows it once, at the boundary, rather than in every component.

The generated files themselves are gitignored and are never edited by hand. Changing the Go service and re-running
`wails3 task common:generate:bindings` is the only way they change.

## The zoom scales the root font size

The application runs in a webview, where the user cannot change the browser's base font size. So the interface zoom scales the
**root font size** instead:

```ts
root.style.fontSize = `${(13 * factor) / 100}px`;
```

Every size in `frontend/src/styles/` is therefore expressed in `rem`, so the entire layout follows. A size in `px` does not
scale, and is a bug waiting to happen. The factor is also written as a custom property (`--zoom`) and a data attribute
(`data-zoom`) for the few things that cannot be expressed in `rem`.

The zoom is persisted in `app_settings` under the `ui` key, and restored as soon as the settings load. It is set from the
**Zoom** tab of the settings dialog, which is tabbed so that the credentials and the interface scale do not share one form.

## The clipboard needs a fallback

The webview does not always have a secure context, and `navigator.clipboard` needs one. So the Tauri-compatible clipboard API
lives in the platform layer (`frontend/src/platform/clipboard.ts`): it asks the Wails runtime first, and a hidden text area
plus `document.execCommand('copy')` takes over when the runtime refuses.

Two details are load-bearing, and they both come from the webview rather than from the design:

- **The fallback must not use `display: none`.** That makes the text area unselectable and breaks the copy command. It is
  positioned off-screen with `opacity: 0` instead.
- **A drag that ends in a click fires the row handler.** A selectable row checks `hasTextSelection()` before it acts, so
  selecting a message to read it does not collapse the row.

## Why there is no UI kit

The source project this was generalised from vendored a large component library. It worked, but it made the template heavy,
introduced a licence obligation, and made the "replace the parts you do not need" story much harder.

The scaffold therefore ships a plain stylesheet with a small set of CSS custom properties, and three runtime dependencies. The
layout is a grid; the panels are plain `div`s. Extending it means adding a panel and a class, not learning a component API.

## What the build tooling does

- `build/config.yml` is the Wails project configuration: the product name, the bundle identifier, the version and the dev-mode
  commands. The CLI binary is `wails3` and the config file is `build/config.yml`, never `wails.json`.
- `Taskfile.yml` at the root includes `build/Taskfile.yml` under `common:` and one Taskfile per platform. The root tasks
  (`build`, `package`, `run`, `dev`) dispatch to the platform by the `GOOS` variable.
- `wails3 task` is the runner. The standalone `task` binary is not needed.
- The Linux build needs a C compiler and the GTK/WebKit development packages. When they are missing, the Linux task falls back
  to a Docker cross-compilation image.
- `-tags server` produces the headless binary from the same source, with no GUI dependencies.

## Where to look first

| Question | File |
| --- | --- |
| How does a job run? | `internal/jobs/manager.go`, `internal/runner/` |
| What does the frontend call? | `internal/bindings/service.go`, `frontend/src/bindings/index.ts` |
| Where is the schema? | `internal/pocketbase/migrations/` |
| How are files served? | `internal/pocketbase/service.go` (`Handler`), `main.go` (`combineHandlers`) |
| How are secrets stored? | `internal/pocketbase/crypto.go`, `internal/pocketbase/settings.go` |
| How does the UI get live updates? | `internal/bindings/events.go`, `frontend/src/layouts/MainLayout.vue` |
