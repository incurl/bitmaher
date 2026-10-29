# Chapter 6 — Vue 3

The frontend is a Vue 3 application with Pinia, vue-i18n and Naive UI. It was **ported** from
Rayburst, the frontend of Motrix Next, which is written against the **Tauri** API.

This chapter is about the port, the platform layer that made it possible, and the Vue corners that
this application leans on.

The versions are Vue **3.5**, Vite **6**, Pinia **3**, vitest **2**.

## The port, and why it shaped everything

The components were not rewritten. They were taken from Rayburst and made to work against the Wails
runtime.

That decision has two consequences that run through the whole frontend.

**The components speak Tauri.** They import `@tauri-apps/api/window`, they call `invoke()` with
Tauri command names, they expect the Tauri event envelope.

**The platform layer is the adapter.** `frontend/src/platform/` implements the Tauri API on top of
Wails. It is the only place that knows Wails exists.

```ts
// frontend/vite.config.ts
resolve: {
  alias: {
    '@tauri-apps/api/core': platform('core.ts'),
    '@tauri-apps/api/event': platform('event.ts'),
    '@tauri-apps/api/window': platform('window.ts'),
    // ...
    '@tauri-apps/plugin-process': platform('process.ts'),
    // ...
  },
},
```

Note that the aliases are listed **one by one**, not as a `@tauri-apps/*` prefix. Vite matches an
alias exactly unless it ends with a slash — a prefix alias would swallow packages that do not
exist in the platform layer.

## The command map

Rayburst calls Tauri commands by name. `platform/commands.ts` maps each of them onto a Wails
service method:

```ts
case 'app_exit':
  return { method: APP_SERVICE + '.AppExit', args: [exitCodeOf(args)] }
case 'aria2_pause_task':
  return { method: APP_SERVICE + '.PauseDownload', args: [stringOf(args, 'gid')] }
```

The method name is the **full import path** of the service:

```ts
const APP_SERVICE = 'github.com/incurl/bitmaher/internal/bindings.AppService';
```

because that is what the Wails runtime addresses a bound method by.

### Three rules about the map

**One.** Every command the frontend calls has to be in the map. A missing one falls through to
`UnsupportedCommand`, which logs a warning and answers `false`. The feature it belongs to stays
inert — which is the intent for the features BitMaher does not have, and a bug for the ones it does.

**Two.** The arguments are **positional** in the Wails binding and **named** in Tauri. The map
converts:

```ts
case 'store_write':
  return {
    method: APP_SERVICE + '.StoreWrite',
    args: [stringOf(args, 'name'), stringOf(args, 'key'), args.value],
  };
```

The `stringOf`, `numberOf`, `stringsOf` helpers at the bottom of the file exist because the
conversion has to be **total**: a command that receives the wrong shape must not throw.

**Three.** A command with no backend must answer **falsy**. This is the rule from Chapter 2, and
it is worth repeating here because the command map is where you would break it.

## Nook 1: the frontend never polls the engine

This is the rule that surprises people coming from a normal web application.

> The Go pump pushes the snapshots; the frontend never polls the engine. The `tasks:snapshot` event
> carries the whole state, and `internal/pump` publishes it.

So the frontend's store does not have a `setInterval`. It has a **listener**:

```ts
// The whole state arrives in one event, and the list is replaced by it.
await listen('tasks:snapshot', ({ payload }) => { ... })
```

The temptation is strong. You see a value that is one tick old, and you add a poll to "fix" it.
**Do not.** The pump is the reader, and it ticks. A poll in the frontend means:

- the engine is asked the same question twice;
- two sources of truth exist, and they disagree;
- the list flickers, because one poll lands between two snapshots.

If a value feels stale, the fix is in the pump — a shorter interval, or a `Wake()` after an action.

### The `Wake()` case

An action that changes the engine's state does not wait for the next tick:

```ts
// The change is picked up at once.
await invoke('aria2_pause_task', { gid })
// and the pump is asked to publish it now.
```

The backend's `Wake()` makes the next tick immediate. The frontend does not need to know.

## Nook 2: the event envelope is different

Tauri's `listen` gives the handler an envelope:

```ts
{ event: string, id: number, payload: T }
```

The Wails runtime gives it the payload directly. The platform layer rebuilds the envelope, so the
ported components do not change:

```ts
// frontend/src/platform/event.ts
export function listen<T>(
  eventName: string,
  handler: (event: Event<T>) => void,
): Promise<UnlistenFn> {
  return Promise.resolve(
    Events.On(eventName as never, (event: { data: T }) => {
      handler({ event: eventName, id: 0, payload: event.data });
    }),
  );
}
```

## Nook 3: the window events carry no payload

The Wails window events are **notifications**, not data. `common:WindowDidResize` says that the
window changed its size; it does not say to what.

So the platform layer reads the values itself:

```ts
async onResized(handler) {
  // The Wails event carries no payload either: it says that the
  // window changed its size. The size is read here, so that the
  // caller gets the same shape as under Tauri.
  return listenEvent<{ width: number; height: number }>('common:WindowDidResize', () =>
    handler({ payload: { width: window.innerWidth, height: window.innerHeight } }),
  );
}
```

The same applies to the scale:

```ts
async onScaleChanged(handler) {
  // The Wails event carries no payload: it says that the scale of the
  // screen changed, and not by how much.
  return listenEvent<number>('common:WindowDPIChanged', () =>
    handler({ payload: window.devicePixelRatio }),
  );
}
```

## Nook 4: `getCurrentWindow()` returns the same object every time

```ts
// frontend/src/platform/window.ts
const mainWindow: WindowHandle = { ... };

export function getCurrentWindow(): WindowHandle {
  return mainWindow;
}
```

BitMaher runs in a **single window**. The parts of the Tauri API that belong to the multi-window
features of Rayburst — the float window, the add window — answer with the neutral value.

The alternative — a registry of windows — would be more faithful to Tauri, and would carry a
question the application cannot answer: which window is "the current" one? The single object makes
the answer obvious.

## Nook 5: the shims must not lie

A shim that returns a plausible-but-false answer is worse than a shim that returns nothing, because
the caller cannot tell the difference.

This is why `isVisible()` was changed to ask the backend:

```ts
async isVisible() {
  // The runtime does not carry the question to the frontend, so the
  // backend answers it. The answer has to be the truth: the frontend
  // hides the window when it is asked to, and a window that was
  // already hidden does not need hiding again.
  return invoke<boolean>('window_is_visible');
}
```

It used to return a hardcoded `true`. That was harmless while `hide()` was a no-op, and it became
the second half of a bug when `hide()` became real.

**The general rule: when you replace an event-emitting shim with one that calls the runtime, you are
changing behaviour. Every caller of the old shim now does something real.** Audit them.

## Nook 6: `v-model` on a store is not reactive on its own

Pinia stores are reactive, but a destructured value loses its reactivity:

```ts
// NOT reactive after the destructuring:
const { config } = usePreferenceStore()
```

Use `storeToRefs` when you need the reactivity and the destructuring:

```ts
const { config } = storeToRefs(usePreferenceStore())
```

Or just use the store directly, which is what the ported code mostly does.

## Nook 7: the pure logic lives next to the component

The convention in this codebase:

> One panel per concern under `frontend/src/components/`. Keep the pure logic (the helpers, the
> formatters) in a separate file next to the component and unit test it — `shared/utils/task.ts` and
> its test are the reference.

So `shared/utils/task.ts` has `shared/utils/__tests__/task.test.ts`. The component imports the
helper; the test imports the helper. Neither imports the other.

This is what makes the logic testable without mounting a component, and it is why the repository has
1180 tests that run in twelve seconds.

## Nook 8: every size is in `rem`

The interface has a zoom factor, and every size in the stylesheet is in `rem`, so that the zoom
scales the whole interface — text included.

```css
.panel {
  padding: 0.75rem;
  font-size: 0.875rem;
}
```

The root font size is what moves. This is not a preference: **pixels do not scale with the zoom**, so
a stylesheet written in `px` would have text that stayed the same size while everything around it
grew.

## Nook 9: the zoom has three moving parts

The size of the interface is a factor in the frontend (`uiScale`) and a percentage in the backend
(`ui.Zoom`), and the store is where they meet:

1. the frontend applies it with `Window.SetZoom`;
2. the backend applies it too when the setting is written, so the change shows at once while the
   user moves the slider;
3. the backend reads it at start-up, so the interface appears at the size the user chose.

Point 2 is the one that is easy to forget. Without it, the slider does nothing until the
application is restarted.

## Nook 10: the system scale is not the zoom

The screen's own scale (the GNOME text-scaling factor, the Windows text size) and the application's
zoom are **different things**, and the stylesheet has to account for both.

```ts
// frontend/src/composables/useWindowScale.ts
// Before the page is zoomed, its device pixel ratio over the
// window's scale factor gives that system part, which holds even
// while a hidden WebKitGTK window has no viewport to measure.
const scale = (window.devicePixelRatio || 1) / factor
```

The subtlety the comment is about: **a hidden window has no viewport to measure.** So the system
part is computed once, before the page is zoomed, and held.

## Nook 11: `document.visibilityState` in a hidden window

Related to the above, and worth knowing: when the window is hidden, the page stops painting. Any
code that waits for a paint — a transition, an animation — will never see it happen.

That is why the reveal helpers exist:

```ts
// A window that was hidden resumes painting a few frames after it is
// shown. A transition started at once would spend those frames unseen
// and appear to jump; it starts instead once the shown page has
// painted its start state.
export async function afterReveal(): Promise<void> {
  await whenVisible()
  await nextFrame()
  await nextFrame()
}
```

## Nook 12: the `painted()` helper has a timeout for a reason

```ts
export function painted(): Promise<void> {
  return Promise.race([
    nextFrame().then(nextFrame),
    new Promise<void>((resolve) => setTimeout(resolve, PAINT_TIMEOUT_MS)),
  ])
}
```

The window is transparent, and it is shown once the layout has painted, so the user sees the content
rather than an empty frame. But a page that paints no frames — because the window was hidden, or
because the renderer is busy — would wait forever. The timeout is the escape.

## Nook 13: the focus event carries a payload, the closing event does not

Not all the Wails events are the same. Some carry data, some do not:

```ts
async onFocusChanged(handler) {
  return listenEvent<boolean>('common:WindowFocus', (event) => handler({ payload: event.payload }));
}
```

The focus event has a payload (whether the window gained or lost focus). The closing event does
not, because it is a notification that the close was requested.

## Nook 14: `preventDefault` on the close does nothing by itself

The ported component does this:

```ts
unlistenCloseRequested = await appWindow.onCloseRequested(async (event) => {
  // The close is held by the backend. This call is the belt to its
  // braces: the window must not go away before the user has been
  // asked.
  event.preventDefault()
})
```

But the **backend** is what actually holds the close. The frontend's `preventDefault` is the belt to
the backend's braces — because the webview's close callback does not fire on every platform.

The comment says so, and it is the reason the backend exists:

```ts
// The backend holds the close of the window and asks the frontend
// what to do with it. This is the path that works on every platform:
// the close never reaches the frontend as a webview event on Linux
// with decorations:false, where the taskbar close and the overview
// × do not trigger the webview callback.
```

## Nook 15: the store is a view onto the database

The Tauri store plugin keeps a JSON file. BitMaher keeps the settings in PocketBase. The platform
layer's `load()` is therefore not a file read — it is a **call to the backend**:

```ts
export async function load(name = 'settings'): Promise<Store> {
  const values = new Map<string, unknown>();

  try {
    const stored = await invoke<Record<string, unknown>>('store_read', { name });
    for (const [key, value] of Object.entries(stored ?? {})) {
      values.set(key, value);
    }
  } catch {
    // A store that cannot be read is an empty one. The settings the
    // backend holds are the fallback.
  }
  // ...
}
```

Two details:

- **The `catch` turns a failed read into an empty store**, not an exception. A store that cannot be
  read is an empty one.
- **The store keeps a local copy** (`values`), so a `get` of a key that was just written does not
  need a round trip.

## Nook 16: the plugin-store name does not matter

Every store in Rayburst maps onto the same settings record in BitMaher. The `name` is kept for the
caller's sake:

```ts
// The name is kept for the caller's sake: every store of Rayburst
// maps onto the same settings record of BitMaher, and the values are
// keyed by their name.
```

So `load('config.json')` and `load('settings')` are the same store, and the values are keyed by
*their* name, not by the store's.

## Nook 17: `invoke` spreads the arguments

```ts
export function invoke<T>(command: string, args?: Record<string, unknown> | null): Promise<T> {
  const mapped = resolveCommand(command, args ?? {});
  return Call.ByName(mapped.method, ...mapped.args) as unknown as Promise<T>;
}
```

The Tauri signature is preserved — a command name and an optional payload — and the Wails services
take positional arguments, so the payload is spread into them.

**The `as unknown as Promise<T>` is a cast.** The Wails binding is typed, but the command map is a
runtime lookup, so TypeScript cannot check it. The cast is where the type safety of the boundary
is given up, and it is why the **command map has to be complete and correct**: nothing else will
tell you when it is wrong.

## Nook 18: the tests mock the runtime, not the backend

The platform layer's tests mock `@wailsio/runtime`:

```ts
const bridge = vi.hoisted(() => ({ quit: vi.fn(), invoke: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Application: { Quit: bridge.quit } }))
vi.mock('@tauri-apps/api/core', () => ({ invoke: bridge.invoke }))
```

That is the right level: it tests **the shim's contract** — "the hide calls the runtime's hide" —
rather than the backend's behaviour. The backend's behaviour is covered by the Go tests and the
live checks.

## Nook 19: vitest needs `--legacy-peer-deps`

```bash
npm install --legacy-peer-deps   # npm 9 cannot resolve the vitest 4 peer tree
```

Or use a newer npm. The `packageManager` field says pnpm, and pnpm is the package manager this
project uses:

```bash
# pnpm lives in ~/.local/bin and needs the Node from there
export PATH="$HOME/.local/bin:$PATH"
```

## Nook 20: the typecheck covers the tests

```bash
npm run typecheck   # vue-tsc --noEmit -p tsconfig.test.json
```

The `-p tsconfig.test.json` is what makes the test files part of the type check. Without it, a test
that calls a function with the wrong signature would pass the typecheck and fail at runtime.

## Nook 21: the locale is loaded, not bundled

vue-i18n loads the messages asynchronously, and the locale is resolved at start-up:

```ts
// The locale is resolved from the stored preference, or from the
// system, and the closest supported one is chosen.
const resolvedLocale = resolveSystemLocale(raw, SUPPORTED_LOCALES)
```

The supported locales are a closed list, and `resolveSystemLocale` picks the closest match — so a
system set to `de-AT` gets `de`, and a system set to something unsupported gets `en-US`.

## Nook 22: the components are unchanged, and that is the point

The strongest statement this codebase makes about the port is that **Rayburst's components were
not modified**. They were dropped in, and the platform layer made them work.

When you find a component that behaves oddly, check whether the behaviour comes from Rayburst or
from the shim. Most surprises come from the shim — because the shim is where the two worlds meet,
and the shim is where the assumptions differ.

## The checks for this layer

```bash
npm test                # vitest
npm run typecheck       # tsc --noEmit, includes the test files
npm run build           # tsc && vite build -> frontend/dist
```

And the Go side, because the boundary is where the bugs are:

```bash
go run ./tools/quit-check
go run ./tools/commands-check
```

## What to read next

[Chapter 7](07-the-seams.md) is about the places where the five technologies meet — and who owns
what when they disagree.
