# The gap between BitMaher and Rayburst

**What this document is.** A comparison of BitMaher against the interface it was ported from,
[Rayburst](https://github.com/Rayburst/Motrix-Next), the frontend of Motrix Next. It records
what the port carries, what it left out, and what it left out **in a way the user can see**.

**How it was made.** Rayburst's source is not reachable from this machine (the repository is not
public), so the comparison is against **the port itself**: the command map
(`frontend/src/platform/commands.ts`) is the complete inventory of what Rayburst's components
call, and the backend is the complete inventory of what BitMaher answers. Every command was
traced to the component that calls it and to the backend method that answers it. Nothing here is
inferred from a feature list; each finding was read out of the code.

**The rule the port follows.** A command with no backend answers the *neutral value* — `false`,
or a typed nil that marshals to `null`. The components that call it stay inert. That is the
correct answer for a feature the application does not have. It is the **wrong** answer for a
feature the application claims to have, and the findings below are grouped by that distinction.

---

## 1. The shape of the port

| | Rayburst | BitMaher |
| --- | --- | --- |
| Shell | Tauri (Rust) | Wails v3 (Go) |
| Engine | aria2, in-process, via a Rust binding | aria2-next, a **separate GPLv2 process** over JSON-RPC |
| Store | SQLite, through the Rust side | PocketBase v0.39.11, as a library |
| Push | Rust publishes `tasks:snapshot` | `internal/pump` publishes `tasks:snapshot` |
| Commands | 94 | 94 — 61 mapped, 33 neutral |

The interface is Rayburst's: the components, the layout, the motion, the styles and the
twenty-seven translations are its own. The backend is BitMaher's.

**The command count is identical, and that is the point.** No command was dropped. Every one of
Rayburst's commands is either answered by BitMaher or answered neutrally. The gap is therefore
not in the frontend — it is in the *depth* of the answers.

---

## 2. The findings, by what the user sees

### 2.1 The neutral answer that lies — a false success

These are the worst findings, because the interface reports a success that did not happen.

#### The extension API port — `restart_http_api`

`frontend/src/components/preference/Advanced.vue:194`

```ts
const appliedPort = await invoke<number>('restart_http_api', { port: newPort })
if (appliedPort !== newPort) { … }
message.success(t('preferences.extension-api-port-applied', { port: appliedPort }))
```

The command is neutral, so it answers `null`. `null !== newPort` is true, so the form is
corrected back to `null`, and then the success message is shown with `port: null`. The user is
told the extension API moved to a port that does not exist. BitMaher has **no HTTP API at all** —
there is no `ListenAndServe` anywhere in the repository.

#### The autostart — `set_autostart`

`frontend/src/components/preference/General.vue:144`

The General panel has a working "open at login" switch. It is saved to the config, and the
backend is asked to register a login item. There is no login-item code in the backend — no
`autostart`, no `login-item`, nothing. The switch reports a state that does not exist.

The same applies to `is_autostart_launch`, which is asked at start-up by `main.ts:355` and by
`MainLayout.vue:448` to decide whether the window should be shown at all. The neutral `false`
means the window is always shown. This one is *safe by accident*: the alternative would have been
a window that never appears.

#### The move of a download — `aria2_move_download`

`frontend/src/components/task/detail/MoveDownload.vue:30`

The button opens a directory picker, shows a progress message, and then calls a command that
does nothing. The success message appears; the files stay where they were.

#### The global options of the engine — `aria2_change_global_option`

This one is the deepest, because it is not a preference panel but the **hot-reload path of the
whole preference system**.

`frontend/src/composables/usePreferenceForm.ts:145` computes which changed options the engine
can accept at runtime (`filterHotReloadableKeys`, over the 93 keys of `aria2Options.json`) and
calls `changeGlobalOption`. The wrapper in `frontend/src/api/aria2.ts:526` is a stub:

```ts
export async function changeGlobalOption(options: Partial<AppConfig>): Promise<void> {
  logger.debug('api.changeGlobalOption', 'the backend owns the global options of the engine');
  void options;
}
```

It resolves, and the preference is saved. The engine is never told. **Every hot-reloadable
engine option in BitMaher is a no-op.** The backend does own the global options — but only
through the flags it passes at start-up (`internal/aria2/manager.go:339`), and those are the
ones the sidecar was started with, never changed afterwards.

**The plumbing is already there, and it is already dead.** `internal/aria2/rpcclient.go:362`
has `ChangeGlobalOption`, the method the engine actually has — AGENTS.md records that it is
`aria2.changeGlobalOption`, not `aria2.setGlobalOption`, and that this was measured against
aria2-next 2.8.2 and 3.0.9. The method is **never called**: a search of `internal/` and
`tools/` for `ChangeGlobalOption` returns only the definition. There is not even a test for it.

So the gap is not the engine's method, and not the RPC client's method. It is the two lines that
would join them to `AppService`, and the frontend wrapper that would stop discarding the
options.

### 2.2 The setting that is stored and never applied

#### The per-site rules — stored, shown, never used

`frontend/src/components/preference/SiteRulesEditor.vue` is a full editor: a host pattern, a
connection count per server, and a proxy choice per site. The rules are saved with the
configuration and survive a restart.

Nothing reads them. `internal/bindings/remote.go:151` has `PreviewSiteRules`, which *reports*
what the rules say — but the rules themselves never reach the engine. There is no
`max-connection-per-server` and no per-site proxy anywhere in `internal/aria2` or
`internal/downloads`.

This is the sharpest example of the pattern in this document: the interface is complete, the
persistence is complete, and the **effect** is missing. The user configures a site to use a
proxy, and every download from that site ignores it.

#### The proxy — configured for the app, never given to the engine

`frontend/src/components/preference/Network.vue` has the proxy settings, and `aria2Options.json`
lists `all-proxy`, `all-proxy-user` and `all-proxy-passwd` as engine options. The backend uses a
proxy in exactly one place: `internal/bindings/remote.go:91`, for the requests the **application**
makes itself.

The engine is never given one. There is no `all-proxy` in the flags of `internal/aria2/manager.go`.
Every download goes out directly, whatever the proxy setting says.

#### The system proxy detection — a dead panel

`frontend/src/composables/useSystemProxyDetect.ts:43` asks for `get_system_proxy` and is neutral,
so the detection always ends in `onNotFound()`. The panel that offers to import the system proxy
is therefore always empty. This one is *safe*: the detection failing is the honest answer.

#### The path of the engine's own configuration — an empty field

`frontend/src/components/preference/Advanced.vue:224` reads `get_engine_conf_path` and shows it
in a readonly field, with a copy button and a "reveal in folder" button beside it. The command
is neutral, so the field is **empty**, and the two buttons act on an empty string. The engine
has no configuration file of its own — its settings are the flags of `internal/aria2/manager.go`
— so there is no path to show. The field is honest by accident, but the buttons are not.

#### The float window — a preference for a window that does not exist

`frontend/src/components/preference/General.vue:612` offers the float window as a mode. The
mode is saved, and the backend is never asked to open a second window — there is exactly one
`WebviewWindow` in the application (`main.go:245`), and no code creates another. The setting is
inert, and the app keeps one window whatever the mode says.

### 2.3 The features that are absent, and whose interface was removed with them

These are the neutral commands whose components are properly hidden. They cost the user nothing
today, and they are the price of the port.

| The area | The commands | What Rayburst has | What BitMaher has |
| --- | --- | --- | --- |
| **The float window** | `add_window_open`, `add_window_prepare`, `float_window_ready`, `float_window_dismiss`, `float_window_needs_relaunch` | A separate always-on-top window with its own task list | Nothing. `offerFloatWindowRelaunch` is inert. |
| **The tray** | `minimize_to_tray`, `update_tray_menu_labels` | A tray icon with a menu of the downloads | `MinimiseWindowToTray` hides the window, but there is no icon and no menu. |
| **The updater** | `check_for_update`, `apply_update`, `cancel_update`, `download_update` | A self-update | Nothing. `UpdateDialog.vue` is in the tree but has no backend. |
| **The deep links** | `take_pending_deep_links`, `peek_pending_deep_links_silent`, `take_pending_external_inputs`, `peek_pending_external_inputs_silent`, `take_pending_frontend_actions` | A `rayburst://` scheme that opens the app | Nothing. The protocol handler of the OS is not registered. |
| **The protocol association** | `get_association_status`, `set_default_protocol_client` | The app asks the OS to be the handler of `magnet:`, `thunder:`, `ed2k:` | Nothing. `PreferenceAssociations.vue` always shows "not associated". |
| **The extension stores** | — | A panel that installs browser extensions | `ExtensionPanel.vue` renders; its `install()` opens a URL. |
| **The clipboard offer** | `clipboard_offer` | The clipboard watcher offers a download | Nothing — the watcher is the frontend's, and the offer is inert. |
| **The ED2K** | `get_ed2k_bootstrap_status`, `sync_ed2k_bootstrap_files` | An ED2K network with its own bootstrap | Nothing. `Ed2k.vue` is a preference panel for a network that does not exist. |

The ED2K one deserves a note: `needRestartKeys` in `frontend/src/shared/configKeys.ts:19` lists
five `ed2k-*` keys, and the engine options file lists more. The frontend has the **whole ED2K
vocabulary** and the backend has **none of it**.

### 2.4 The one that is neither: the HTTP API

`restart_http_api` is grouped with the false successes above, but it is worth saying what it
really is: Rayburst has an **HTTP API for browser extensions**, on a configurable port, with an
API key. BitMaher has neither the API nor the key. The port `extensionApiPort` is a
configuration field that leads nowhere.

---

## 3. What BitMaher has that Rayburst does not

The port is not a copy with holes in it. BitMaher has things Rayburst does not.

| | BitMaher | Rayburst |
| --- | --- | --- |
| **The licensing of the engine** | aria2-next is a **separate GPLv2 process**. The application is MIT, and stays so. | The engine is in the Rust binary. |
| **The database** | PocketBase, with a real migration history, encrypted secrets at rest, and a closed `app_settings` collection. | SQLite through the Rust side. |
| **The credential store** | `http_auth` is encrypted with AES-256-GCM and closed for reading *and* writing. | — |
| **The engine's own session** | `pb_data/aria2/session.txt` carries the gid, so a download survives a restart of the application. | — |
| **The diagnostics** | `export_diagnostic_logs`, `history_check_integrity`, `database_schema_version`, `factory_reset`. | — |
| **The blocklist** | `get_bt_peer_blocklist_status`, `sync_bt_peer_blocklist`, `reconcile_bt_peer_blocklist` — three commands, and the rules are Rayburst's own. | — |
| **The watchdog** | A crashed sidecar is started again (`tools/watchdog-check`). | — |
| **The software-rendering path** | `internal/graphics` moves the webview to the CPU on a machine whose only renderer is software. | — |
| **The category rules** | `internal/downloads/category` — Rayburst's rules, and Rayburst's own tests. | Rayburst's rules, in Rust. |

The last row is the shape of the whole port: **the logic is Rayburst's, the machinery is
BitMaher's.**

---

## 4. The summary, by the number that matters

Of the 94 commands:

- **61 are answered properly** — the task list, the actions, the files, the categories, the
  preview, the notifications, the blocklist, the history, the lifecycle, the store.
- **33 are neutral**, and those 33 fall into three groups:
  - **5 report a success that did not happen** — §2.1: `restart_http_api`, `set_autostart`,
    `is_autostart_launch`, `aria2_move_download`, `aria2_change_global_option`. These are bugs.
  - **4 are settings that are stored and never applied** — §2.2: the proxy and the site rules
    (`get_system_proxy`, `normalize_proxy_bypass`, `preview_site_rules`), the float window, and
    the engine's own path (`get_engine_conf_path`).
  - **the remaining 24 belong to features that are properly inert** — §2.3. The interface for them
    is either hidden or honestly empty. They are the tray, the updater, the deep links, the
    protocol association, the ED2K network, the float window's own commands, the clipboard offer,
    the tracker sources, and the pending-action queues.

---

## 5. What to do about it

In the order the cost argues for.

**First, the false successes (§2.1).** A user who turns on "open at login" is told it worked.
A user who moves a download is told it moved. A user who changes the extension API port is told
the new port is applied. The neutral answer was right for a feature that is absent; it is wrong
for a feature whose **interface is present**. The fix is to make the interface honest: either
remove the control, or give it a backend.

- `set_autostart` / `is_autostart_launch` — a login item is a small, well-understood thing on
  each platform (a `.desktop` file, a registry value, a login item). This is the smallest gap
  that the user notices every day.
- `aria2_move_download` — `internal/aria2/rpcclient.go:351` already has `ChangeOption`, and
  `internal/downloads/service.go:884` already uses it for the file selection, with the
  pause/change/resume dance the AGENTS.md documents. A move is the same shape with `dir`
  instead of `select-file`, and the files would have to be moved on disk as well as told to.
- `aria2_change_global_option` — `internal/aria2/rpcclient.go:362` already has
  `ChangeGlobalOption`, and the preference system already computes the changed hot-reloadable
  keys. This is the **largest** gap in effect: 93 engine options are silently inert today, and
  the two halves that are missing are a bound method and a wrapper.
- `restart_http_api` — the honest answer is to hide the control, because the HTTP API is a whole
  feature (a server, an API key, an authentication rule), not a method.

**Second, the settings that do nothing (§2.2).** The proxy and the site rules need to reach the
engine. The engine takes `all-proxy` and `max-connection-per-server` as options, so the path
exists — it is the *wiring* that is missing. Note that this also interacts with the first item:
the hot-reload path is how both would be applied.

**Third, the absent features (§2.3).** These are choices, not defects. The float window and the
tray are the two a user of Rayburst would miss most. The deep links and the protocol
association are the two that would need the OS to be asked, which is a different kind of work
on each platform.

**What is not recommended.** Do not wire the neutral commands to a "success" without the
behaviour behind them. A neutral answer that reads as a success is worse than an honest
failure, and the failure mode is exactly the one already found in this audit: the history
metadata that was parsed as `null` and froze the task list, because the code assumed the
answer was good.

---

## 6. How to keep this document honest

The command map is the instrument. A command that is added to `commands.ts` and answered
neutrally is a **decision**, and it belongs in §2.3 with its reason. A command that is answered
neutrally but whose interface is visible belongs in §2.1, and it is a bug.

The test that tells the two apart is in `internal/bindings/surface_test.go`. It asserts two
things about the service: that its exported methods are **exactly** the intended set
(`TestAppServiceExposesExactlyTheIntendedMethods`), and that none of them returns a type from an
internal package (`TestNoBoundMethodReturnsAnInternalType`) — the second one is the regression
test for the finding of the 2026-10-28 review, where three methods returned a struct with only
unexported fields, serialised to `{}`, and nothing noticed.

What it does not assert is the thing this document is about: that a command whose **interface is
visible** has a real backend behind it. `TestNoBoundMethodReturnsAnInternalType` catches a method
that returns something unusable; it does not catch a command that was never given one. That
assertion is the one worth adding when the first of §2.1 is fixed.
