# Chapter 1 — The shape of the application

Before the details of five libraries, a map. You cannot hold "why does this file exist" without
knowing which layer it belongs to.

## What BitMaher is

A desktop download manager. It shows a list of transfers, lets you add them, pause them, and watch
them. It stores what it has to remember. It runs a separate program — the download engine — and
drives it.

That is the whole product. Everything else is arrangement.

## The five pieces

```
                      ┌──────────────────────────────────────────┐
                      │  The webview (Vue 3)                      │
                      │  the interface the user sees              │
                      └────────────────┬─────────────────────────┘
                                       │  Wails runtime
                                       │  (HTTP fetch to /wails/runtime)
                      ┌────────────────▼─────────────────────────┐
                      │  The Go process                          │
                      │                                          │
                      │  internal/bindings   ← the API surface   │
                      │  internal/downloads  ← the domain        │
                      │  internal/pump       ← the reader        │
                      │  internal/pocketbase ← the persistence   │
                      └───────┬───────────────────┬──────────────┘
                              │ JSON-RPC          │ in-process
                      ┌───────▼────────┐  ┌───────▼──────────────┐
                      │ aria2-next     │  │ PocketBase           │
                      │ (child process)│  │ (a library, not a    │
                      │ GPLv2          │  │  process)            │
                      └────────────────┘  └──────────────────────┘
```

Five technologies, five responsibilities:

| Technology | Its job | Its rule |
| --- | --- | --- |
| **Vue 3** | The interface | Never talks to the database or the engine directly |
| **Wails v3** | The channel between the two | The only way across |
| **Go** | The domain | Owns the sidecar and the persistence |
| **Aria2-Next** | The transfers | A separate process, always |
| **PocketBase** | The memory | A library, never a process |

The two rules in the last column are the ones that are easy to break and expensive to notice.

## The direction of every call

Almost every design decision in this codebase follows from one fact: **the engine pushes nothing**.

The engine has no `onDownloadStart`, no event to subscribe to, no notification of a download that
moved. You can ask it questions; it will not tell you when the answers change.

So the application has to ask. And because asking is expensive and the interface must not stutter,
the asking is done in **exactly one place**.

That place is `internal/pump`. It ticks every 500 milliseconds, reads the whole state of the
engine, and publishes it as one event:

```
internal/pump/pump.go
    →  aria2.tellActive / tellWaiting / tellStopped / getGlobalStat
    →  pump.Snapshot { stat, tasks }
    →  Wails event "tasks:snapshot"
    →  the frontend replaces its list
```

The frontend **never polls the engine**. It waits for the snapshot. This is the single most
important structural fact about the frontend, and Chapter 6 explains what it means for the Vue
code.

The cost is that the interface is up to 500 ms stale. That is the trade, and it is a good one: the
alternative — every panel polling on its own timer — is how you get a list that flickers, a
database that is hammered, and an engine that is asked the same question twelve times a second.

### The one exception

When the user does something that changes the engine's state — pause a download, add one, remove
one — the caller *wakes* the pump rather than waiting for the tick:

```go
// internal/pump/pump.go
func (p *Pump) Wake() {
    // A tick that is already pending will pick the change up.
    select {
    case p.wake <- struct{}{}:
    default:
    }
}
```

So an action feels instant, and the idle cost stays at one read per tick. The `default` branch is
the important part: if a tick is already queued, the one on its way will see the change. There is
no need to queue a second one, and queueing one would mean the pump reads the engine twice for one
user action.

## The licensing boundary

This is the constraint that shapes the packaging, and it is worth stating plainly because it is
easy to erode by accident.

**Aria2-Next is GPLv2. BitMaher is MIT.** The two stay separate because of how they are run:

- the engine is started as **its own process**, with its own address space;
- the only channel is a **loopback HTTP socket** carrying JSON-RPC;
- nothing links against it, imports its headers, vendors its sources, or reaches it through CGO;
- the binary is shipped as an **opaque executable**, and the bundled binaries are not in Git.

Break any one of those and the licensing of the whole application changes. `internal/aria2/manager.go`
and `main.go` both carry a note about it, and `THIRD_PARTY_NOTICES.md` explains it to a reader.

The practical consequence for your day-to-day work: **the engine's vocabulary never leaves
`internal/aria2`**. The rest of the application speaks in the domain's words — "download", "status",
"kind" — and a thin layer translates. When you find yourself wanting to use `aria2.getFiles` in a
binding, stop: the translation belongs in `internal/aria2`, and the binding should see a `FileView`.

## The persistence

PocketBase runs **as a library inside the Go process**. It is never a separate process, and it
never binds a port of its own.

Two consequences shape the code:

**The schema is Go code.** Collections are created by migrations in
`internal/pocketbase/migrations/*.go`, each registered in an `init()`. The file name is both the
identity and the ordering key, because the list is sorted by name:

```
1730000000_init.go                    ← the scaffold's own schema
1732000000_bitmaher_downloads.go      ← the downloads
1733000000_bitmaher_history.go        ← the history and the birth times
```

A new migration must be named with a timestamp that sorts after the ones already applied.

**The API is mounted, not started.** PocketBase's own REST and file routes are mounted onto the
Wails asset server, so every file the application stores is served from the same origin as the
interface. One port, no CORS, no second process:

```go
// main.go
Assets: application.AssetOptions{
    Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
},
```

## The front and the back do not talk directly

This is the rule that the whole frontend architecture follows from, and it is enforced by
convention rather than by the type system:

> **The frontend never talks to PocketBase or to the engine directly. Every call goes through
> `internal/bindings` and the typed wrappers in `frontend/src/platform/`.**

The frontend *could* read the database — the collections are public-read — and it could call the
engine's RPC endpoint, which listens on loopback. Neither is done, and neither should be.

Why not? Because the invariants live in Go. The download record and the engine's state have to
agree; the only place that can guarantee it is the one that owns both. A frontend that wrote to
the database would break that agreement the moment it computed a different status mapping.

When you need new backend behaviour:

1. add a method to `AppService` in `internal/bindings/`;
2. regenerate the bindings (`wails3 task common:generate:bindings`);
3. add a wrapper in `frontend/src/platform/`.

## The platform layer

The frontend was ported from **Rayburst**, the frontend of Motrix Next, which is written against the
**Tauri** API. Rather than rewrite every component, the port gives them the API they expect:

```ts
// A ported component, unchanged:
import { getCurrentWindow } from '@tauri-apps/api/window'

// The Vite alias points that import at the platform layer:
'@tauri-apps/api/window': platform('window.ts')
```

`frontend/src/platform/` implements the Tauri API on top of the Wails runtime. It is the **only**
place in the frontend that knows Wails exists. A component calls `invoke()`, `listen()` or
`getCurrentWindow()`; the platform layer turns it into a Wails call.

The command map (`platform/commands.ts`) is the heart of it. Rayburst calls Tauri commands by
name; each one is mapped onto a Wails service method:

```ts
case 'app_exit':
  return { method: APP_SERVICE + '.AppExit', args: [exitCodeOf(args)] }
```

Chapter 6 explains how to keep this map honest, and Chapter 10 explains what happens when it is
not.

## What is persisted, and where

Everything the user would miss if it were lost survives a restart, and each thing has its own
place:

| What | Where | Written by |
| --- | --- | --- |
| The configuration of the frontend | `app_settings`, key `app.config` | `StoreWrite` |
| The size of the interface and the theme | `app_settings`, key `ui` | `StoreWrite` |
| The downloads that are running | `download_jobs` | `downloads.Sync` |
| The downloads that have come to rest | `download_history` | `downloads.Sync` |
| The moment every download was added | `task_birth` | `downloads.Sync` |

Two of these deserve a note.

**The configuration is one record.** The ported frontend keeps all of it in one value of the
key-value store, so the backend stores it as one JSON blob under `app.config`. The backend does not
interpret it: a field it knows nothing about still survives a restart. That is deliberate — the
frontend owns the shape of its own configuration.

**The birth time never changes.** It is written the first time the engine is seen, and it is what
holds the order of the list stable across a restart. Without it, the order would follow the order
the engine happens to report the downloads in — which is the order the engine's own queues happen
to be in, and that is not stable.

## The checks

The repository carries a set of live checks under `tools/`. They are not unit tests: they start
the real application, with a real engine, and watch what happens.

```
tools/e2e-check      the whole pipeline, with a real sidecar
tools/pump-check     the pump publishes the snapshots
tools/zoom-check     the window carries the zoom of the interface
tools/commands-check the commands the port added
tools/restart-check  a download survives a restart
tools/watchdog-check a crashed sidecar is started again
tools/bundled-check  the embedded binary is found and run
tools/leak-check     no sidecar outlives the application
tools/torrent-check  a torrent's files are listed and chosen
tools/quit-check     the quit ends the application and the engine
```

Chapter 9 explains what each one can and cannot tell you. The short version: a unit test proves the
code does what it says; a live check proves the *system* does.

## What to read next

If you are about to change the desktop shell — the window, the quit, the tray — read
[Chapter 2](02-wails.md) next. It is the layer with the most surprises in it.

If you are about to change the domain — the downloads, the statuses, the persistence — read
[Chapter 4](04-aria2.md) and [Chapter 5](05-pocketbase.md).

If you are about to change the interface, read [Chapter 6](06-vue.md).
