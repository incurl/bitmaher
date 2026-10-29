# Chapter 2 — Wails v3

Wails v3 is the shell. It gives a Go program a window, a webview, and a channel to the code
running inside that webview.

This chapter is about the parts of it that are surprising, because the parts that work the way you
expect need no explanation.

The version is **3.0.0**. Every claim here was read out of
`~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0`.

## What it actually is

Three things:

1. A **native window** (GTK4 on Linux, AppKit on macOS, Win32 on Windows) with a webview in it.
   On Linux there is also a GTK3 variant, behind the `gtk3` build tag; the default is GTK4.
2. An **asset server** that serves the frontend over the `wails://` scheme.
3. A **message processor** that answers calls from the webview.

The third is the interesting one. There is no IPC channel, no socket, no bridge binary. The
frontend calls the backend with an HTTP `fetch`:

```ts
// @wailsio/runtime/dist/runtime.js
let url = new URL(runtimeURL());          // window.location.origin + "/wails/runtime"
let body = { object: objectID, method };
let response = await fetch(url, { method: 'POST', headers, body: JSON.stringify(body) });
```

On the desktop, `window.location.origin` is `wails://localhost` — and `http://wails.localhost` on
Windows, which has no custom scheme. The request is handled **in-process** by the asset server.
There is no listener on a port. That matters for testing: you cannot reach the desktop runtime
from outside the process with an HTTP client.

## The wire protocol

Two numbers address everything: an **object** and a **method**.

```ts
// @wailsio/runtime/src/runtime.ts
export const objectNames = Object.freeze({
    Call: 0,        Clipboard: 1,   Application: 2, Events: 3,
    ContextMenu: 4, Dialog: 5,      Window: 6,      Screens: 7,
    System: 8,      Browser: 9,     CancelCall: 10, IOS: 11, Android: 12,
});
```

So `Application.Quit()` is object 2, method 2. `Window.Hide()` is object 6, method 11. The method
ids are per-object and are listed in `pkg/application/messageprocessor_*.go`.

A call to a **bound Go method** looks like this:

```json
{
  "object": 0,
  "method": 0,
  "args": {
    "call-id": "m_Fw1k1gDyOVBVUBeR1nd",
    "methodName": "github.com/incurl/bitmaher/internal/bindings.AppService.StoreRead",
    "args": ["config.json"]
  }
}
```

Object 0 is `Call`, and its method 0 is `CallBinding`. The `methodName` is the **full import path**
of the service: the package path, a dot, the struct name, a dot, the method name. This is why the
generated bindings exist — they carry those ids, and hand-writing them is a maintenance trap.

The `call-id` is how the frontend matches a reply to a promise. The runtime keeps a map of them.

## Nook 1: an empty object is true

**This is the single most expensive trap in this codebase, and it has already cost a release.**

The runtime answers a Go method that returns **nothing** with `{}`:

```go
// pkg/application/transport_http.go
func (t *HTTPTransport) json(rw http.ResponseWriter, data any) {
    var jsonPayload = []byte("{}")
    var err error
    if data != nil {
        jsonPayload, err = json.Marshal(data)
        ...
    }
    rw.Write(jsonPayload)
}
```

In JavaScript, `{}` is **truthy**. So a method that returns `void` is, from the frontend's point of
view, a method that returns "yes".

This matters because the ported frontend asks about features BitMaher does not have, and reads the
answers as booleans:

```ts
const isAutostart: boolean = await invoke('is_autostart_launch')
const silentPendingDeepLinks = await invoke<boolean>('peek_pending_deep_links_silent')
const shouldHide = (isAutostart && autoHide) || silentPendingDeepLinks
if (!shouldHide) {
  await appWindow.show()
} else {
  if (await appWindow.isVisible()) await appWindow.hide()
}
```

Before the fix, `is_autostart_launch` fell through to `UnsupportedCommand`, which returned nothing,
which came back as `{}`, which was truthy. The frontend therefore believed it had been started by
the autostart, took the *silent* branch, and **hid the window**. The window flashed and
disappeared at start-up, and the process stayed alive.

The whole failure was invisible in the type checker, because the frontend cast the answer to
`boolean` and the backend returned `void`. The cast told the compiler what it wanted to hear.

### The rule

> A command that has no backend must answer **falsy**.

There are two answers, and they are not the same:

- **`NeutralBool`** returns `false`. For the commands that expect a boolean.
- **`NeutralValue`** returns a **typed nil**. For the commands that expect a value.

```go
// internal/bindings/platform.go
func (s *AppService) NeutralBool() bool { return false }

// Neutral is the shape of the answers of the features BitMaher does
// not have.
//
// It carries nothing. It exists so that the answer can be a typed nil:
// a nil of an interface is turned into an empty object by the runtime,
// and an empty object is true in JavaScript.
type Neutral struct{}

func (s *AppService) NeutralValue() *Neutral { return nil }
```

A **typed nil** is the whole trick. `json.Marshal` of a nil `*Neutral` is `null`:

```go
var p *Neutral
json.Marshal(p)   // → "null"
```

But the transport's check is `if data != nil` on the **interface**. A typed nil inside an interface
is not a nil interface, so it goes through `json.Marshal` and becomes `null` — the nothing the
frontend expects. A plain `nil` would have been converted to `{}` by the transport and been truthy
all over again.

The command map knows which answer each command wants:

```ts
// frontend/src/platform/commands.ts
const NEUTRAL_COMMANDS: Record<string, 'boolean' | 'value'> = {
  is_autostart_launch: 'boolean',
  peek_pending_deep_links_silent: 'boolean',
  // ...
  check_for_update: 'value',
  // ...
}
```

## Nook 2: the window close is one event on every platform

You would expect a window close to arrive differently on each platform. It does not.

Wails maps the native close of every platform onto **one** event, `common:WindowClosing`:

```go
// pkg/events/defaults.go
var defaultWindowEventMapping = map[string]map[WindowEventType]WindowEventType{
    "windows": { Windows.WindowClosing: Common.WindowClosing, ... },
    "darwin":  { Mac.WindowShouldClose:  Common.WindowClosing, ... },
    "linux":   { Linux.WindowDeleteEvent: Common.WindowClosing, ... },
}
```

The mapping is installed on every window in `setupEventMapping()`, and it uses the platform's own
mapping when the application provides one.

That single fact is what makes it possible to hold the close of the window in one place:

```go
// internal/bindings/window.go
closer.stop = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
    if !closer.held.Load() {
        return    // released: it goes on
    }
    event.Cancel()                                    // hold it
    app.Event.Emit(EventWindowCloseRequest, nil)     // ask the frontend
})
```

**Hooks run before listeners.** That ordering is what makes `Cancel()` work: `HandleWindowEvent`
runs the hooks first and returns early if one cancelled, so the default listener — the one that
destroys the window — never runs.

```go
// pkg/application/webview_window.go
func (w *WebviewWindow) HandleWindowEvent(id uint) {
    hooks := w.eventHooks[id]
    thisEvent := NewWindowEvent()
    for _, thisHook := range hooks {
        thisHook.callback(thisEvent)
        if thisEvent.IsCancelled() {
            return        // the default listener never sees it
        }
    }
    // ... the listeners
}
```

The frontend is told through an ordinary Wails event, because the webview's own close callback is
not reliable on every platform:

```ts
unlistenExitDialog = await listen('window:close-request', async () => { ... })
```

## Nook 3: `Window.Close()` emits the closing event too

`Close()` is not a "destroy this now" operation:

```go
// pkg/application/webview_window.go
func (w *WebviewWindow) Close() {
    if w.impl == nil || w.isDestroyed() { return }
    InvokeSync(func() {
        w.emit(events.Common.WindowClosing)
    })
}
```

It emits the event. Which means **your own hook sees it**.

This is a trap when you intercept the close: if the release path calls `Close()` while the close is
still held, the hook cancels it and the window never goes away.

The fix is the ordering. Release the hold *first*, then close:

```go
func (c *WindowCloser) Release() {
    c.held.Store(false)     // the hook now lets the close through
    if c.window != nil {
        c.window.Close()
    }
}
```

The same care is needed at the end of the application's life. A quit that arrives while a close is
being held would leave the window hanging:

```go
// main.go — OnShutdown
if closer != nil {
    closer.Stop()   // releases a held close
}
```

## Nook 4: the runtime does not expose everything the Go API has

`WebviewWindow` has `IsVisible()` in Go. The **runtime does not carry it to the frontend** — there
is no `WindowIsVisible` method id.

So a frontend that needs it has to ask the backend:

```go
// internal/bindings/window.go
func (s *AppService) WindowIsVisible() bool {
    if s == nil || s.window == nil {
        return false
    }
    return s.window.IsVisible()
}
```

This is the general pattern: the runtime's surface is *not* the Go API's surface. Before you write
a shim that reaches for something, check `pkg/application/messageprocessor_window.go` for the method
id. If it is not there, it has to go through a bound method.

The same applies to the frontend's `isMaximized()`, which the runtime **does** have
(`Window.IsMaximised()`, id 14), and which is therefore a direct runtime call.

## Nook 5: `Application.Quit()` tears down the webview

`Application.Quit()` reaches:

```go
// pkg/application/messageprocessor_application.go
case ApplicationQuit:
    globalApplication.Quit()
```

which is:

```go
// pkg/application/application.go
func (a *App) Quit() {
    if a.impl != nil {
        InvokeSync(a.impl.destroy)
    }
}
```

and `destroy()` runs `cleanup()`, which runs the `OnShutdown` hooks, and then calls
`appDestroy()` → `g_application_quit()` on Linux.

**The webview is gone by the time the call could answer.** So the promise in the frontend never
settles:

```ts
export async function exit(code = 0): Promise<void> {
  // The promise is left hanging on purpose: the webview is torn down
  // before the answer could arrive.
  await Application.Quit();
}
```

That is not a bug to fix. A frontend that awaits an answer that will never come is fine — the
process is on its way out. But it does mean the backend must do its work *before* the quit
completes, not after the frontend's call returns.

## Nook 6: server mode does not end the process

This one cost a debugging session and is worth stating plainly.

The `quit-check` tool runs the **server build** of the application, because that is the mode in
which the runtime answers over HTTP and a test can drive it from outside:

```go
// tools/quit-check/main.go
body := strings.NewReader(fmt.Sprintf(
    `{"object":%d,"method":%d}`, runtimeObjectApplication, runtimeMethodQuit,
))
```

The request is accepted, the shutdown runs, the sidecar stops. **The process stays alive.**

The reason is in the server's run loop: it waits for a signal or for its context, and the quit
cancels the context *after* the loop has already checked it. The desktop build has no such loop —
it returns from `app.Run()` as soon as the GTK main loop stops.

So the server build does not leave on its own. The check asserts what is actually true and
valuable — that the shutdown runs and the engine goes with it — and then stops the process itself.

**Do not read "the process is still there" as "the quit does not work."** Check whether the
shutdown ran.

## Nook 7: the bindings are generated, and they are gitignored

```
frontend/bindings/     ← generated, gitignored, never hand-edited
```

They are produced by `wails3 task common:generate:bindings` and carry the method ids and the
types. Two consequences:

- **Never hand-edit them.** Change the Go service and regenerate.
- **Never commit them.** They are in `.gitignore`.

The frontend is not supposed to import them directly either. There is a wrapper:

```ts
// frontend/src/api/aria2.ts
```

which wraps the generated modules behind a typed surface and normalises the nullable arrays and maps
the generator produces. The rule:

> Never import a generated binding file directly. Use `frontend/src/api/aria2.ts`.

## Nook 8: the window is frameless, and that is a Wails decision

`Frameless` is false on macOS and true on Windows and Linux.

```go
// main.go
Mac: application.MacOptions{
    ApplicationShouldTerminateAfterLastWindowClosed: true,
},
```

macOS is the other way round: the traffic lights are the native ones, and the interface draws
nothing in that corner. Wails **hides** the traffic lights when `Frameless` is true, so setting it
on macOS would remove the buttons entirely.

On Windows and Linux the interface draws its own caption buttons
(`frontend/src/components/layout/WindowControls.vue`), because a framed window would show the
native ones as well and the user would see two sets.

The **system title bar** preference (`systemTitleBar`, Linux only) turns the native frame back on,
for the window managers that force their own decorations. It calls `SetWindowDecorations`, which is
`SetFrameless` on the Wails window.

## Nook 9: the zoom is applied twice, on purpose

The size of the interface is a factor in the frontend (`uiScale`) and a percentage in the backend
(`ui.Zoom`). The store is where the two meet:

- the frontend applies it with `Window.SetZoom`, through the platform layer's `setZoom`;
- the backend applies it too, when the setting is written, so that the change shows at once while
  the user moves the slider;
- the backend reads it at start-up, so that the interface appears at the size the user chose.

Every size in the stylesheet is in `rem`, so the zoom scales the whole interface, text included.
The root font size is what moves:

```ts
// frontend/src/composables/useWindowScale.ts
const zoom = computed(() => preferenceStore.config.uiScale || 1)
```

## Nook 10: `color-scheme: dark` is not decoration

Without `color-scheme: dark` on `:root`, WebKitGTK renders the native select popup with a **light**
background while the option text stays light — so the selected item appears blank.

```css
:root {
  color-scheme: dark;
}
```

It is one line, and it is a bug fix, not a style.

## Nook 11: the clipboard needs a gesture, and a webview does not always have one

`navigator.clipboard` needs a **secure context** and a **user gesture**. A webview does not always
provide either.

The copy path therefore lives in the platform layer
(`frontend/src/platform/clipboard.ts`) and falls back to a hidden textarea plus
`document.execCommand('copy')`.

Two details of the fallback are not optional:

- **It must not use `display: none`.** A hidden element cannot be selected, and the copy silently
  does nothing. It is positioned off-screen with `opacity: 0` instead.
- **A click that ends a text drag fires the row's click handler.** A selectable row checks
  `hasTextSelection()` first.

And `user-select: none` on a copy button keeps the button's label out of the text the reader is
selecting.

## Nook 12: events the frontend emits are not events the backend hears

The old platform layer implemented the Tauri window API by emitting events:

```ts
await Events.Emit('window:hide' as never);
```

Nothing in the backend listened to `window:hide`. Every one of those calls was a **no-op that
looked like it worked**. The caption buttons did nothing at all.

The rewrite calls the runtime's own window:

```ts
await Window.Hide();
await Window.Minimise();
await Window.Maximise();
await Window.ToggleMaximise();
await Window.Focus();
```

**When you replace a shim that emitted events with one that calls the runtime, you are changing
behaviour, not just plumbing.** Every caller of the old shim now does something real. In this
repository that is exactly how a latent bug became a visible one: `isVisible()` had always returned
`true`, and `hide()` had always been harmless. Making `hide()` real turned the first into the
second.

## The checks for this layer

```bash
go run ./tools/quit-check        # the quit ends the application and the engine
go run ./tools/zoom-check        # the window carries the zoom of the interface
go run ./tools/leak-check        # no sidecar outlives the application
```

And the desktop build can be driven on a virtual display, which is how the window's visibility is
checked:

```bash
Xvfb :77 -screen 0 1440x900x24 &
DISPLAY=:77 ./bin/bitmaher
xwininfo -root -children | grep bitmaher
```

The `Map State` of the window is the answer. `IsUnMapped` means the window exists but is not on the
screen — which is what a "flashed and disappeared" bug looks like from the outside.

## What to read next

[Chapter 3](03-go.md) is about the language the backend is written in, and the handful of its
corners that this application depends on.
