# Chapter 7 — The seams

The interesting parts of an application are not in its layers. They are in the places where the
layers meet, because that is where the assumptions of one technology collide with the assumptions of
another.

This chapter is about the six seams in BitMaher, and what each one is for.

## The list

| Seam | Between | Who owns it |
| --- | --- | --- |
| [The webview and the backend](#the-webview-and-the-backend) | Vue ↔ Wails | `internal/bindings` |
| [The backend and the engine](#the-backend-and-the-engine) | Go ↔ Aria2-Next | `internal/aria2` |
| [The backend and the database](#the-backend-and-the-database) | Go ↔ PocketBase | `internal/pocketbase` |
| [The interface and the platform](#the-interface-and-the-platform) | Vue ↔ the shim | `frontend/src/platform` |
| [The engine and the clock](#the-engine-and-the-clock) | Aria2-Next ↔ the pump | `internal/pump` |
| [The application and the operating system](#the-application-and-the-operating-system) | Wails ↔ the window manager | `internal/bindings/window.go` |

## The webview and the backend

**Vue ↔ Wails.**

The boundary is `internal/bindings`. Every method the frontend may call is on `AppService`, and the
bindings are generated from it.

### The rule: the frontend never talks to the database or the engine

Both are technically reachable — the collections are public-read, and the engine's RPC endpoint
listens on loopback. Neither is used.

The reason is that the invariants live in Go. The download record and the engine's state have to
agree, and only the process that owns both can guarantee it.

### What goes across

Three things, and only three:

1. **Calls** — the frontend asks the backend to do something.
2. **Events** — the backend tells the frontend what happened.
3. **The snapshot** — the backend tells the frontend the whole state.

Everything else is a detail of one side.

### The shape of a bound method

```go
// internal/bindings/downloads.go
func (s *AppService) PauseDownload(gid string) error {
    // ...
}
```

The rules for a method that the frontend calls:

- **It takes and returns plain values.** No engine types, no database records — the view types.
- **It returns an error** when something went wrong, and the runtime turns that into a rejected
  promise.
- **It is a method on `AppService`,** so the binding's name is
  `github.com/incurl/bitmaher/internal/bindings.AppService.<Name>`.

### The rule about setters

Not every function may be a method on the service. The binding generator exposes **every exported
method** of a bound service, so a helper that is not for the frontend must not be one.

That is why the setters are **functions**:

```go
// SetQuitter tells the service how to end the application.
//
// It is a function and not a method, because a method on the service
// would become a method of the frontend's API: the binding generator
// exposes every exported method of a bound service, and this one is
// not for the frontend.
func SetQuitter(service *AppService, quit func()) {
    if service == nil {
        return
    }
    service.quit = quit
}
```

The same applies to `ExitCodeOf`, `SetWindowCloser` and `InterceptWindowClose`.

**The test that enforces it** is `internal/bindings/surface_test.go`: it lists the methods the
frontend may see, and fails if the set changes without the list being updated. It is a tripwire for
"I added a helper and accidentally published it".

```bash
go test ./internal/bindings/
```

### The types the frontend may see

The same test checks the **return types**. A method that returns an internal type is useless to the
frontend, because the binding cannot marshal it into anything meaningful.

```go
allowed := map[string]bool{
    "bindings.SettingsView":          true,
    "bindings.DownloadView":          true,
    // ...
}
```

## The backend and the engine

**Go ↔ Aria2-Next.**

The boundary is `internal/aria2`, and it is the one that carries the licence.

### The vocabulary stops here

The engine says `tellStatus`, `getFiles`, `changeGlobalOption`. The domain says "download",
"status", "options". The translation happens in `internal/aria2/types.go` and the client's typed
methods.

```go
// The status mapping lives here, and the frontend never sees the
// engine's raw strings.
```

### The secret never leaves

The RPC secret authenticates the caller. Only the Go process knows it.

```go
// Secret returns the token the sidecar is started with.
//
// It is never sent to the frontend: the Go process is the only
// caller of the engine.
```

The frontend can ask *whether* a secret is set, and that is all.

### The failure modes have to be translated too

The engine's errors are its own. The application's errors are the user's. The translation is in the
domain layer, and it is where "the engine refused" becomes "that download is already finished".

### What must never happen

- An engine type in a binding's signature.
- An engine method name in the frontend.
- The engine's binary being linked, embedded, or reached through CGO.

The last one is checked by the licence audit:

```bash
python3 tools/license-audit.py
```

## The backend and the database

**Go ↔ PocketBase.**

The boundary is `internal/pocketbase`, and it holds the settings, the crypto and the migrations.

### The rules are the gate

PocketBase's access rules decide who may write. They are set in the migrations, and they are the
reason the frontend can read but not write.

### The secrets are write-only

The frontend never receives a secret's value. The encrypted envelope is in the database; the key is
in a `0600` file next to it.

### The configuration is stored whole

The backend does not interpret the frontend's configuration. That is a deliberate choice, and it is
what lets the frontend add a setting without touching Go.

### The exception: the `ui` record

The size of the interface and the theme are in a separate record, because the backend **owns** the
size of the interface and has to apply it to the window.

## The interface and the platform

**Vue ↔ the shim.**

The boundary is `frontend/src/platform/`.

### One direction of knowledge

The platform layer knows about Wails. The components know about Tauri. **Neither knows about the
other.**

```ts
// A component, unchanged from Rayburst:
import { getCurrentWindow } from '@tauri-apps/api/window'
```

```ts
// The shim, which knows about Wails:
import { Window } from '@wailsio/runtime';
```

### The shims must not lie

Every method that returns a value has to return **the truth**, or the neutral value, and the
difference matters.

This is the whole lesson of the bug in Chapter 2, and it deserves to be stated as a rule:

> A shim that returns a plausible-but-false answer is worse than one that returns nothing, because
> the caller cannot tell the difference.

Before my change, `isVisible()` returned `true` and `hide()` emitted an event nobody heard. Both
were individually harmless and jointly fatal.

### The map is the contract

`platform/commands.ts` is a **runtime** lookup, so TypeScript cannot check it. The `as unknown as
Promise<T>` in `invoke()` is where the type safety is given up.

The consequence: **nothing but a test will tell you when the map is wrong.**

```ts
// The commands of the features BitMaher does not have, and the type
// each of them answers.
const NEUTRAL_COMMANDS: Record<string, 'boolean' | 'value'> = { ... }
```

## The engine and the clock

**Aria2-Next ↔ the pump.**

This is the seam that has no counterpart in a normal web application, because the engine does not
push.

### The rule: one reader

```go
// The pump is the single reader of the engine's state. It reads the
// global statistics and the task queues on a tick, and publishes the
// result as one snapshot. The frontend never polls the engine: it
// waits for the snapshot.
```

If two things read the engine, the engine is asked twice and the two answers may differ.

### The rule: one snapshot

The snapshot carries the **whole state**, not a delta. The frontend replaces its list with it.

A delta would need an acknowledgement, an ordering guarantee, and a recovery path for a missed
delta. The whole state has none of those problems.

### The interval is the bound on staleness

```go
const (
    // VisibleInterval is the interval while the window is in front.
    VisibleInterval = 500 * time.Millisecond
    // BackgroundInterval is the interval when the window is hidden.
    BackgroundInterval = 2 * time.Second
)
```

Two intervals, because the cost of a tick is a set of RPC calls, and the benefit is a number the
user is looking at. When the window is hidden, the number is not being looked at.

### The wake-up

An action wakes the pump so that the change appears at once, without waiting for the tick.

This is the seam that makes the application feel responsive despite polling every 500 ms.

## The application and the operating system

**Wails ↔ the window manager.**

The seam is `internal/bindings/window.go`, and it is where the close of the window is held.

### The problem

The user closes the window. The application has to decide: does the window go away and the
application stay (minimise to tray), or does the application go too (quit)?

That question has to be asked **in the interface**, because the interface is where the user's
preference lives. But the close has to be **held in the backend**, because the webview's close
callback does not fire on every platform.

### The solution

```go
closer.stop = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
    if !closer.held.Load() {
        return
    }
    event.Cancel()
    app.Event.Emit(EventWindowCloseRequest, nil)
})
```

The backend holds the close and asks the frontend. The frontend answers, and the backend releases
the close.

### The three ways out

| The user chose | What happens |
| --- | --- |
| Minimise to tray | `invoke('minimize_window_to_tray')` → the backend hides the window |
| Quit | `exit(0)` → the runtime's quit → the shutdown runs → the engine stops |
| Cancel | Nothing. The close is still held, and the window stays. |

### The shutdown has to release it

```go
// main.go — OnShutdown
if closer != nil {
    closer.Stop()
}
```

A quit that arrives while a close is held would leave the window hanging, and the application
waiting for an answer that will not come.

## The rules that hold the seams together

There are four, and they are the ones to check when you are unsure.

1. **The frontend never talks to the database or the engine directly.** Every call goes through
   `internal/bindings` and the typed wrappers in `frontend/src/platform/`.

2. **The engine's vocabulary never leaves `internal/aria2`.**

3. **A command that has no backend must answer falsy.** `NeutralBool` and `NeutralValue`.

4. **The bindings are generated.** Change the Go service, run
   `wails3 task common:generate:bindings`, add a wrapper. Never hand-edit
   `frontend/bindings/`, never commit it.

## What to read next

[Chapter 8](08-building.md) is about the toolchain: the tasks, the artefacts, and the things that
break on a fresh machine.
