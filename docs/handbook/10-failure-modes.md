# Chapter 10 — The failure modes

Every entry here is a failure that actually happened in this repository. Each one has its symptom,
its cause, the fix, and the check that now prevents it.

This is the chapter to read when something is behaving strangely.

---

## 1. The window flashed and disappeared at start-up

**Symptom.** The application started, showed a window for a moment, and the window went away. The
process stayed alive. The engine kept running.

**How it was found.** The window existed on the X server with the right size, but unmapped:

```
0x6200004 MapState:IsUnMapped Width: 1440 Height: 900
```

**The cause.** A chain of three mistakes.

The frontend asks the backend whether the application was started by the autostart:

```ts
const isAutostart: boolean = await invoke('is_autostart_launch')
const silentPendingDeepLinks = await invoke<boolean>('peek_pending_deep_links_silent')
const shouldHide = (isAutostart && autoHide) || silentPendingDeepLinks
```

Those commands have no counterpart in BitMaher, so they fell through to `UnsupportedCommand`. The
Wails runtime answers a method that returns nothing with `{}` — and **`{}` is truthy in
JavaScript**. The frontend therefore believed it had been started by the autostart, took the
*silent* branch, and hid the window.

The hide was harmless at first, because the old platform layer implemented it by emitting an event
that nothing in the backend listened to:

```ts
await Events.Emit('window:hide' as never);   // a no-op that looked like it worked
```

When the platform layer was rewritten to call the runtime's real window, the hide became real — and
a latent bug became a visible one.

**The fix.** Three parts.

*Answer the questions falsely.* The commands that expect a boolean are answered by `NeutralBool`,
which returns `false`. The commands that expect a value are answered by `NeutralValue`, which
returns a **typed nil** — a nil of an interface would have become `{}` again and been truthy all
over again:

```go
// Neutral carries nothing. It exists so that the answer can be a
// typed nil.
type Neutral struct{}

func (s *AppService) NeutralValue() *Neutral { return nil }
```

*Route each command to the answer of its type.* The command map knows:

```ts
const NEUTRAL_COMMANDS: Record<string, 'boolean' | 'value'> = {
  is_autostart_launch: 'boolean',
  check_for_update: 'value',
  // ...
}
```

*Ask the truth about the window.* `isVisible()` returned a hardcoded `true`, which was the second
half of the bug — it made the hide fire even when the window was already hidden. The runtime does
not carry the question to the frontend, so the backend answers it:

```go
func (s *AppService) WindowIsVisible() bool {
    if s == nil || s.window == nil {
        return false
    }
    return s.window.IsVisible()
}
```

**The check that prevents it.** `tools/quit-check` asks the application, over the wire, for the
answers of the features it does not have, and fails if any of them is truthy.

**The lesson.** An empty object is true. A method that returns nothing is not a method that returns
"nothing" to JavaScript.

---

## 2. The quit did nothing

**Symptom.** The Quit action did not end the application. The sidecar kept running.

**The cause.** The frontend called `invoke('app_exit')`, and no backend answered that command. It
fell through to the catch-all of the command map and was only written to the log.

**The fix.** The end of the application is the Wails runtime's own quit:

```ts
export async function exit(code = 0): Promise<void> {
  // The promise is left hanging on purpose: the webview is torn down
  // before the answer could arrive.
  await Application.Quit();
}
```

The runtime reaches `globalApplication.Quit()` → `impl.destroy()` → `cleanup()` — which runs the
`OnShutdown` hooks, and that is where the sidecar is stopped.

**The check that prevents it.** `tools/quit-check`.

**The lesson.** A command with no backend is a command that silently does nothing. The command map
has to be complete.

---

## 3. The quit ended the application, but the sidecar lived on

**Symptom.** The application ended, and the engine kept running.

**The cause.** The engine is a child process, and the shutdown did not stop it. The `OnShutdown`
hook had to do it explicitly.

**The fix.** The shutdown stops the pump first — so that the pump is not reading an engine that is
on its way out — and then the engine:

```go
OnShutdown: func() {
    // The pump stops before the sidecar, so that it
    // does not read an engine that is on its way out.
    enginePump.Stop()
    // ...
    shutdownCtx, cancel := context.WithTimeout(context.Background(), sidecarShutdown)
    defer cancel()
    if err := sidecar.Stop(shutdownCtx); err != nil {
        consoleLog.Warn("aria2", "the sidecar did not stop cleanly: "+err.Error())
    }
},
```

**The check that prevents it.** `tools/leak-check`.

**The lesson.** A child process does not die with its parent by default. It has to be stopped, and
the check has to prove it.

---

## 4. The server build did not exit after the quit

**Symptom.** `quit-check` sent the runtime's quit over HTTP. The request was accepted (HTTP 200).
The shutdown ran, and the engine stopped. **The process was still there.**

**The cause.** This is a Wails behaviour, not a bug in the application. The server mode's run loop
waits for a signal or for its context, and the quit cancels the context *after* the loop has already
checked it. The desktop build has no such loop: it returns from `app.Run()` as soon as the GTK
main loop stops.

**The fix.** The check asserts what is actually true and valuable — that the shutdown runs and the
engine goes with it — and then stops the process itself:

```go
// The server build does not leave its process on its own, because
// the loop of the server waits for a signal. The desktop build
// leaves as soon as the window is gone. What matters here is the
// same in both: the quit runs the shutdown, and the shutdown stops
// the engine.
```

**The lesson.** "The process is still there" and "the quit does not work" are different claims.
Check whether the shutdown ran.

---

## 5. The window was held open forever

**Symptom.** The user closed the window, and nothing happened. The window did not go away.

**The cause.** The backend holds the close of the window so that it can ask the user what to do
with it. The hold is released when the user answers. But the **webview's** close callback does not
fire on every platform — on Linux with `decorations: false`, the taskbar close and the overview ×
do not trigger it.

So a frontend-only interception does not work.

**The fix.** The backend holds the close, using the one event that Wails maps every platform's
native close onto:

```go
closer.stop = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
    if !closer.held.Load() {
        return
    }
    event.Cancel()
    app.Event.Emit(EventWindowCloseRequest, nil)
})
```

**The lesson.** `common:WindowClosing` is the same event on every platform. Hooks run before
listeners, so `Cancel()` in a hook stops the default listener from destroying the window.

---

## 6. The window could not be closed once it was held

**Symptom.** After the hold was released, the window still did not go away.

**The cause.** `Window.Close()` **emits the closing event**. The hook saw it, saw that the close was
still held, and cancelled it again.

**The fix.** Release the hold first, then close:

```go
func (c *WindowCloser) Release() {
    c.held.Store(false)     // the hook now lets the close through
    if c.window != nil {
        c.window.Close()
    }
}
```

And the shutdown has to release a held close, or a quit during a held close leaves the window
hanging:

```go
if closer != nil {
    closer.Stop()
}
```

**The lesson.** `Close()` is not "destroy this now". It is a request, and your own hook sees it.

---

## 7. The relaunch test hung

**Symptom.** `go test -race ./internal/bindings/` never finished.

**The cause.** The test spawned the test binary with a `-test.run` filter that matched **itself**.
Every child started another child.

**The fix.** The child is the shell, not the test binary:

```go
// The child is the shell, not the test binary: the restart of the
// test binary would run the tests again.
shell, err := exec.LookPath("sh")
child := exec.Command(shell, "-c", "echo \"$"+EnvRelaunchParent+"\"")
```

**The lesson.** A test that starts a process has to make sure the process is not the test binary.

---

## 8. The second instance fought the first over the database

**Symptom.** Two instances of the application, the second one's writes failed with "database is
locked".

**The cause.** SQLite is a single-writer database, and the data directory is next to the
executable, so both instances used the same file.

**The fix.** Two parts.

The engine's port is picked per start, so two instances never collide on it:

```go
// The port is picked per start, so that two instances of the
// application never fight over the same one.
```

And a restart waits for the instance that started it to be gone, **before it touches the
database**:

```go
func run() error {
    // --- The restart ------------------------------------------------
    // A restart starts the new instance as the old one goes down. The
    // old one still holds the database when it does, so the new one
    // waits here, before it touches anything: the database belongs to
    // one process at a time.
    bindings.WaitForPreviousInstance()
```

**The lesson.** A restart that starts the new instance while the old one still holds the database
is a data race with a very long fuse.

---

## 9. The engine could not be removed while it was running

**Symptom.** Deleting a running download failed.

**The cause.** `aria2.remove` refuses to remove a download that is working.

**The fix.** Pause first, then remove. There is also `aria2.forceRemove` for the case where the
engine will not let it go politely.

**The lesson.** Read the engine's behaviour, not the upstream documentation. The two differ.

---

## 10. The engine failed to start on the first run

**Symptom.** The engine did not start. The application showed the error in the console.

**The cause.** The sidecar was started with `--input-file` pointing at a session file that did not
exist — because this was the first run. The engine refuses to start in that case.

**The fix.** The flag is only passed once the file is there.

**The lesson.** A flag that points at a file needs the file to exist. This one is easy to miss,
because the second run works fine.

---

## 11. The magnet's file list was empty

**Symptom.** The file selection of the add-task dialog showed nothing for a magnet.

**The cause.** A magnet carries no file list. The engine has to fetch the metadata from the
trackers first, which can take minutes.

**The fix.** The file selection is **offered once the files are known**. It is a UI rule, and it is
easy to forget when you build a panel that shows a list.

**The lesson.** Some of the engine's answers do not exist yet, and no amount of polling will make
them appear.

---

## 12. The file selection was refused

**Symptom.** Choosing which files of a torrent to download had no effect.

**The cause.** `select-file` is an option, and the engine only accepts it **while the download is
paused**.

**The fix.** The backend pauses, changes, and resumes — three steps that have to look like one to
the user.

**The lesson.** Some of the engine's options are only accepted in one state. Find out which.

---

## 13. The download came back with a different identity

**Symptom.** After a restart, the records and the engine's downloads did not line up.

**The cause.** Hypothetical — this did not happen, because the code accounts for it — but it is the
failure that the session file's design prevents.

**The fix.** The engine writes the **gid** into its session file, so a download that is picked up
again keeps its identity:

```go
// The session file carries the gid. A download survives a restart,
// with the same gid.
```

**The check that prevents it.** `tools/restart-check`:

```
4. The record after the restart: status=active completed=133120 gid=1c1b6d9f2a0a185e
```

**The lesson.** The identity of a record has to be the identity of the thing in the engine, not
something the application invents.

---

## 14. The list reordered itself after a restart

**Symptom.** The order of the downloads changed when the application was restarted.

**The cause.** The order was the order the engine happened to report the downloads in — which is
the order of the engine's own queues, and that is not stable.

**The fix.** The moment every download was added is written once, and never changed:

| What | Where |
| --- | --- |
| The moment every download was added | `task_birth` |

**The lesson.** If the user can see an order, the order has to be stored.

---

## 15. The interface froze on a wedged engine

**Symptom.** A call from the interface never came back.

**The cause.** The RPC call had no bound.

**The fix.** Two bounds, one on the context and one on the client:

```go
ctx, cancel := context.WithTimeout(context.Background(), ReadTimeout)
defer cancel()
```

```go
http: &http.Client{
    Timeout: DefaultTimeout,   // 10 seconds
},
```

**The lesson.** Every call that crosses a process boundary needs a deadline. The engine is a
separate program, and separate programs hang.

---

## 16. The engine was restarted by the watchdog on the way out

**Symptom.** Quitting the application left a running engine.

**The cause.** The watchdog restarts a sidecar that died. The application stops the sidecar
deliberately, and the watchdog could not tell the difference.

**The fix.** The watchdog remembers the `waitDone` channel it was watching. If the manager has
replaced it, the sidecar that died is not the one the watchdog was watching:

```go
// The shutdown already took the sidecar down on purpose, so there
// is nothing to restart.
if !stillOurs {
    return
}
```

And the shutdown stops the watchdog first, and waits for it.

**The lesson.** A supervisor needs to know the difference between a crash and a shutdown. The
easiest way to give it that knowledge is to make the two use different identities.

---

## 17. The secrets were readable by everyone

**Symptom.** The encryption key file had the wrong permissions.

**The cause.** `os.WriteFile` **does not change the mode of an existing file**. A key file that was
copied from a backup, or restored with a `0644` mode, kept its permissions.

**The fix.** The mode is set explicitly, every time:

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

**The lesson.** "I set the permissions when I created it" is not the same as "the permissions are
right".

---

## 18. The schema did not exist

**Symptom.** "Collection not found" at runtime.

**The cause.** The migrations register themselves in an `init()`, which only runs if the package is
imported. The import is blank:

```go
// Importing the migrations package registers the BitMaher schema
// migrations with PocketBase's own migration list.
_ "github.com/incurl/bitmaher/internal/pocketbase/migrations"
```

Without it, the migrations are never registered — and Go does not warn about an unused blank
import.

**The fix.** The import is there. The lesson is to know it is load-bearing.

**The lesson.** A blank import is invisible to the compiler's unused checks, and it can be the
difference between a working schema and none.

---

## 19. The caption buttons did nothing

**Symptom.** The minimise, maximise and close buttons in the interface had no effect.

**The cause.** The platform layer implemented the Tauri window API by **emitting events**:

```ts
await Events.Emit('window:minimize' as never);
```

Nothing in the backend listened to `window:minimize`. Every one of those calls was a no-op that
looked like it worked.

**The fix.** The runtime's own window:

```ts
await Window.Minimise();
await Window.Maximise();
await Window.ToggleMaximise();
await Window.Close();
```

**The lesson.** An event with no listener is a silent no-op. When a shim is supposed to *do*
something, it has to call something.

---

## 20. The selected item in a dropdown was blank

**Symptom.** The native select popup rendered with a light background while the option text stayed
light, so the selected item appeared empty.

**The cause.** WebKitGTK does not know the page is dark.

**The fix.** One line:

```css
:root {
  color-scheme: dark;
}
```

**The lesson.** A native widget is not styled by your stylesheet. It is styled by the platform, and
the platform has to be told.

---

## 21. The copy button did nothing

**Symptom.** Copying to the clipboard failed silently.

**The cause.** `navigator.clipboard` needs a **secure context** and a **user gesture**. A webview
does not always provide either.

**The fix.** The platform layer falls back to a hidden textarea and `document.execCommand('copy')`.

And the fallback has two details that are not optional:

- **It must not use `display: none`.** A hidden element cannot be selected, and the copy silently
  does nothing. It is positioned off-screen with `opacity: 0`.
- **A click that ends a text drag fires the row's click handler.** A selectable row checks
  `hasTextSelection()` first.

**The lesson.** The fallback needs a fallback's care. The obvious implementation is the broken one.

---

## 22. `go build ./...` failed

**Symptom.**

```
# github.com/incurl/bitmaher/build/ios
runtime.main_main·f: function main is undeclared in the main package
```

**The cause.** `build/ios` is a separate `main` package behind a `//go:build ios` tag. On Linux the
tag is not set, so the package has no `main` function.

**The fix.** Build the root package and `./internal/...` instead:

```bash
go build . ./internal/...
```

**The lesson.** A build tag that guards a `main` package needs the tag to be set in every build.

---

## 23. The interface was stale

**Symptom.** A value in the interface did not update for a while.

**The cause.** Almost certainly a poll added in the frontend to "fix" a stale list.

**The fix.** The pump is the reader, and it ticks. An action that changes the state wakes it:

```go
func (p *Pump) Wake() {
    select {
    case p.wake <- struct{}{}:
    default:
        // A tick is already pending. The one that is on its way will
        // pick the change up, so there is nothing to do.
    }
}
```

**The lesson.** If the data feels stale, the fix is in the pump — not another poll in the frontend.
Two readers mean two truths.

---

## 24. The graphics driver complained at the end

**Symptom.** The application printed this on the way out, and nothing else was wrong:

```
context mismatch in svga_surface_destroy
```

**The cause.** The message is **not** from this application, and not from GTK or WebKitGTK either. It
comes from the graphics driver: `svga_surface_destroy` is a function of the **VMware SVGA3D** Gallium
driver (Mesa's `libgallium`), and `context mismatch in %s` is its debug message. The driver destroys
the surface of the window on a thread of its own, and it notices that the surface belongs to another
context.

The machine is a virtual machine, and its only renderer is a software one:

```bash
$ glxinfo -B | grep -E 'renderer|Accelerated'
OpenGL renderer string: SVGA3D; build: RELEASE;  LLVM;
Accelerated: no
```

**The fix.** The webview is drawn on the CPU there, so that the driver is never asked to destroy a
surface. **Both** renderers have to be told, and it took two attempts to learn that:

```go
// internal/graphics/graphics_linux.go
func Configure() {
	// A machine with a real GPU keeps its rendering, and the setting
	// of the user wins over both.
	if hasHardwareRenderer() {
		return
	}

	// WebKit does not composite with the GPU.
	if os.Getenv(webkitDisableCompositing) == "" {
		_ = os.Setenv(webkitDisableCompositing, "1")
	}

	// GTK draws the window with a renderer of its own, and that one
	// creates the GL context. It is told to draw on the CPU as well.
	if os.Getenv(gtkSoftwareRenderer) == "" {
		_ = os.Setenv(gtkSoftwareRenderer, gtkCairoRenderer)
	}

	// The graphics stack is still loaded with the two settings above,
	// and it still opens the render node of the card. This one keeps
	// the card closed, so that the driver is never asked to destroy a
	// surface it did not make.
	if os.Getenv(alwaysSoftware) == "" {
		_ = os.Setenv(alwaysSoftware, "1")
	}
}
```

The first attempt set only `WEBKIT_DISABLE_COMPOSITING_MODE`, and the message stayed. The reason is
that the offending GL context is **not** WebKit's: it is GTK's own renderer, which draws the window
that the webview sits in. `WEBKIT_DISABLE_COMPOSITING_MODE` stops the web process from compositing,
and leaves GTK drawing through GL.

That was not enough either. The graphics stack is **still loaded**, and it still opens the render
node of the card, so the VMware driver is still asked to destroy a surface it never made:

```bash
$ ls -la /proc/$(pgrep -x zoom-check)/fd/ | grep -c renderD128
3
```

The card is closed as well, with `LIBGL_ALWAYS_SOFTWARE=1`. The render node is then never opened,
the driver is never engaged, and the function that printed the message cannot run:

```bash
$ ls -la /proc/$(pgrep -x zoom-check)/fd/ | grep -c renderD128
0
```

The three settings are independent, and a user may set any of them. They used to be one: the
setting of the first one stopped the second from being applied, and the message stayed.

The way to see which library is at fault is to read the maps of the running process:

```bash
$ grep -oE '/[^ ]*(gallium|drm|gbm|libGL)[^ ]*' /proc/$(pgrep -x zoom-check)/maps | sort -u
/usr/lib/x86_64-linux-gnu/libEGL_mesa.so.0.0.0
/usr/lib/x86_64-linux-gnu/libgallium-25.0.7-2+deb13u1.so
/usr/lib/x86_64-linux-gnu/libgbm.so.1.0.0
```

`libEGL_mesa` is what pulls the driver in, and it is what opens the render node.

The whole investigation, with every measurement that was taken and a prompt that guides a code
agent through the same failure in another project, is in
[`docs/troubleshooting-linux-webview-graphics.md`](../troubleshooting-linux-webview-graphics.md).
It applies to Tauri 2 and Wry as well, which draw their webview with the same WebKitGTK.

The renderer is found through the PCI bus, and through the DMI of the machine when there is no card to
ask:

```bash
$ ls /sys/bus/pci/devices/*/driver/module/drivers/
pci:vmwgfx
$ cat /sys/class/dmi/id/sys_vendor
VMware, Inc.
```

**The lesson.** A message that names no file of this repository is not a bug of this repository. Read
where the string comes from before changing anything:

```bash
$ strings /usr/lib/x86_64-linux-gnu/libgallium-*.so | grep -B 4 'context mismatch in %s'
```

The neighbours of the string are the answer: `SVGA_SWTNL_FSE` and `vmwgfx drm driver` put it in the
VMware driver.

---

## 25. Every scope showed every task

**Symptom.** The sidebar has four scopes — All, In Progress, Failed, Completed — and every one of
them showed the whole list. Selecting "Failed" left the finished downloads where they were.

**The cause.** Nothing was wrong with the scopes. The list was **frozen**: the last snapshot that
reached the interface intact was the one it drew on start-up, from the "All" scope, and no later
snapshot ever replaced it.

The snapshot publish threw, and the throw was swallowed by the event listener. One history record
carried the **string** `"null"` as its metadata:

```json
{"gid": "16e89ebcd6b6daee", "metaType": "string", "metaValue": "null"}
```

and the parse of that string is `null`, which is not an object:

```ts
// before
export function parseHistoryMeta(record: HistoryRecord): HistoryMeta {
  if (!record.meta) return {}
  try {
    return JSON.parse(record.meta) as HistoryMeta   // null, cast to an object
  } catch {
    return {}
  }
}
```

The first field read — `meta.completedLength` — threw `null is not an object`. The record could not
be turned into a task, so `mergeHistoryIntoTasks` threw, so `publishSnapshot` threw, so `taskList`
was never assigned again. Every scope kept showing the start-up list.

**The fix.** Two layers, because the string should never have been produced, and the parse should
never have trusted it.

The metadata column is a JSON one, and a value that was never written reads back as the literal
`null`. The decoder passed it on:

```go
// A JSON field that was never written holds the literal null. That
// is not metadata, and passing it on hands the frontend a string
// that parses to null, which is not an object: the record then
// fails to render, and every task of the list goes with it.
if trimmed := strings.TrimSpace(string(encoded)); trimmed == "null" {
	return ""
}
```

And the parse refuses anything that is not an object:

```ts
const parsed: unknown = JSON.parse(record.meta)
if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
return parsed as HistoryMeta
```

**How it was found.** Not by reading the code — by making the running application say what it
computed. The store wrote each publish to a file:

```
/tmp/scopeprobe/<time>-<scope>-<count>.json

all-6.json
all-6.json
...
```

and the files stopped at the moment of the first navigation and never resumed, while the route had
changed and no error appeared. Wrapping the function in a `try`/`catch` that wrote the error gave
the line:

```
MESSAGE: null is not an object (evaluating 'n.completedLength')
SCOPE: failed  PATH: /task/failed
```

**The lesson.** A union type cast with `as` is a promise, not a check. `JSON.parse` returns `unknown`
because it really can be anything, including `null`. And when a list "does not filter", ask whether
it is filtering and not being drawn — the difference is a swallowed exception.

---

## The patterns behind the failures

Look at the list again and a few shapes appear.

**The truthy empty object.** Failures 1 and 4 are both about a value that looked like something it
was not. `{}` is truthy; a nil that becomes `{}` is truthy.

**The no-op that looked like a call.** Failures 2, 19 and 23 are all calls that went nowhere and
looked fine. An event with no listener, a command with no backend, a poll in the wrong place.

**The order of two operations.** Failures 6, 8 and 16 are all about doing things in the wrong
order — closing before releasing, starting before the old instance is gone, restarting after a
deliberate stop.

**The difference between the documented and the real.** Failures 9, 10, 11 and 12 are all places
where upstream aria2's documentation does not describe Aria2-Next.

**The thing that is not where you expect.** Failures 13, 14 and 15 are about identity, order and
deadlines — the three things that have to be stored, stored and bounded.

## What to read next

[Chapter 11](11-glossary.md) is the glossary: the words this codebase uses in a particular way.
