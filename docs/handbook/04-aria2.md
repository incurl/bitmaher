# Chapter 4 — Aria2-Next

Aria2-Next is the download engine. It does the transfers: HTTP, HTTPS, BitTorrent, Metalink. It is
fast, it is robust, and it has a JSON-RPC surface that this application drives.

It is also **GPLv2**, and BitMaher is MIT. That fact shapes everything in this chapter.

The version measured here is **3.0.9** (and 2.8.2, for the differences between them). Every claim
was checked by asking the running engine, not by reading the documentation of upstream aria2.

## The licensing boundary, and how it is held

The engine is a **separate process**. Not a library, not an embedded binary, not a Go package with
CGO. A child program, started by the application, watched by it, and stopped by it.

Four things keep the boundary intact:

1. **A separate process.** Its own address space, its own lifetime, its own licence.
2. **A loopback socket.** The only channel is HTTP carrying JSON-RPC on `127.0.0.1`.
3. **No linking.** Nothing in this repository imports aria2's headers, vendors its sources, or
   reaches it through CGO.
4. **An opaque executable.** The bundled binary is shipped as a file. The bundled binaries are not
   in Git — they are fetched by `wails3 task common:fetch:aria2`, because they are large and the
   GPL requires their sources to be offered separately.

The comment at the top of `internal/aria2/manager.go` says all of this, and it is not decoration.
If you ever find yourself wanting to `import` something from aria2, that is the moment to stop.

### What this means for the code

The engine's vocabulary **never leaves `internal/aria2`**. The rest of the application speaks in
the domain's words. When a binding needs the files of a torrent, it does not return aria2's
structure; it returns a `FileView`.

```go
// internal/aria2/rpcclient.go — the engine's own shape
func (c *Client) Files(ctx context.Context, gid string) ([]File, error)

// internal/bindings/downloads.go — the domain's shape
func (s *AppService) DownloadFiles(gid string) ([]FileView, error)
```

## The RPC surface, and where it disagrees with the docs

The engine speaks JSON-RPC 2.0 over HTTP, at `/jsonrpc`:

```json
{"jsonrpc":"2.0","id":"1","method":"aria2.addUri","params":["token:xxx",["https://..."]]}
```

The secret is always the **first parameter**. The client does that for you:

```go
// internal/aria2/rpcclient.go
// The secret token is always the first parameter.
params = append([]any{"token:" + c.token}, params...)
```

A request without a valid token is refused with `Unauthorized`.

### The names that differ from upstream aria2

This is the part that catches people, because the upstream aria2 documentation is the thing you
find first, and the engine does not follow it in three places:

| Upstream aria2 | Aria2-Next | Consequence |
| --- | --- | --- |
| `aria2.tellFiles` | **`aria2.getFiles`** | `tellFiles` does not exist |
| `aria2.setGlobalOption` | **`aria2.changeGlobalOption`** | `setGlobalOption` does not exist |
| `aria2.tellWaiting` / `tellStopped` take no arguments | **they take an offset and a count** | `tellActive` takes neither |

The code says so at the top of the client:

```go
// The names are the ones the engine actually implements. Several of
// them differ from the names the upstream aria2 documentation uses,
// which was verified against aria2-next 2.8.2 and 3.0.9:
//
//   - aria2.getFiles, not aria2.tellFiles.
//   - aria2.changeGlobalOption, not aria2.setGlobalOption.
//   - aria2.tellWaiting and aria2.tellStopped take an offset and a
//     count, while aria2.tellActive takes neither.
```

### The methods Aria2-Next adds

The engine has methods upstream aria2 does not. They were found by asking the engine for its own
list:

```go
// The methods Aria2-Next adds to the upstream aria2. They were
// verified against aria2-next 3.0.9 by asking the engine for its
// own list of methods: "system.listMethods".
methodProbe              = "aria2.probe"
methodInspectTorrent     = "aria2.inspectTorrent"
methodGetPeers           = "aria2.getPeers"
methodGetBtTrackers      = "aria2.getBtTrackers"
methodReplaceBtTrackers  = "aria2.replaceBtTrackers"
methodReplaceBtWebSeeds  = "aria2.replaceBtWebSeeds"
methodAddBtPeers         = "aria2.addBtPeers"
methodForceBtRecheck     = "aria2.forceBtRecheck"
methodChangeURI          = "aria2.changeUri"
methodResolveFilename    = "aria2.resolveFilename"
methodGetOption          = "aria2.getOption"
methodGetUris            = "aria2.getUris"
methodGetSessionInfo     = "aria2.getSessionInfo"
```

**`aria2.probe` is the interesting one.** It is what the preview asks: it answers with the size
and the name a download would get, without starting it.

Upstream aria2 has no equivalent. If you are porting something that assumed a HEAD request or a
`Content-Length` probe, this is the replacement.

**`aria2.inspectTorrent`** is what the file selection of the add-task dialog is built on: it
answers which files are inside a torrent, before the download starts.

## Nook 1: the engine pushes nothing

There is no `aria2.onDownloadStart`. There is no event to subscribe to. There is no notification of
a download that moved, finished or failed.

You can ask the engine questions. It will not tell you when the answers change.

Every design decision about the frontend follows from this. The application has to poll, and it
does so in exactly one place: the pump (Chapter 1).

## Nook 2: errors arrive as a body, whatever the status code

This one changed between versions and is worth stating exactly, because the code has to handle both.

- **aria2-next 2.8.2**: an error is an **HTTP 400** with a JSON body.
- **aria2-next 3.0.9**: an error is an **HTTP 200** with a JSON body.

The client therefore **parses the body first** and only uses the status code to decide whether
there is a body at all:

```go
// The engine reports an error with HTTP 400 and a body that still
// follows the JSON-RPC shape, so the body is parsed first and the
// status code only decides whether there is a body at all. Treating a
// 400 as a transport failure would hide the engine's own message,
// which is the part that tells the user what went wrong.
```

The consequence for you: **do not check the status code to decide whether the call failed.** Check
the JSON-RPC envelope.

```go
var decoded rpcResponse
if err := json.Unmarshal(body, &decoded); err != nil {
    return fmt.Errorf("aria2: the %s response is not JSON: %w", method, err)
}

if decoded.Error != nil {
    return decoded.Error
}
```

Treating a 400 as a transport failure would hide the engine's own message, which is the part that
tells the user what went wrong.

## Nook 3: a magnet has no file list until its metadata is fetched

A magnet link carries no file list. The engine has to fetch the metadata from the trackers first,
and only then does it know what it is downloading.

So the file selection of the add-task dialog is **offered once the files are known**:

```go
// A magnet has no file list until its metadata has been fetched. File
// selection is offered once the files are known.
```

This is a UI rule, not a backend rule, and it is easy to forget. If you build a panel that shows a
file list for a magnet, it will show an empty list for as long as the metadata takes — which can be
minutes.

## Nook 4: the file selection only works while paused

`aria2.changeOption` with a `select-file` option is refused unless the download is paused.

The backend therefore **pauses, changes, and resumes**:

```go
// select-file works only as an option, and only while paused. The
// backend pauses, changes, resumes.
```

This is a three-step operation that has to look like one to the user. The user clicks "download
only these files" and the panel does not flicker through a paused state.

## Nook 5: `--input-file` fails when the session file does not exist

The engine's session file is what lets a download survive a restart of the application: it holds
the gid of every download that was still running.

Passing `--input-file` to a session file that does not exist makes the engine **fail to start**.

So the flag is only passed once the file is there:

```go
// --input-file fails when the session file does not exist. It is
// only passed once the file is there.
```

You can see this in the running process:

```
aria2-next ... --save-session=.../session.txt --save-session-interval=5 \
    --input-file=.../session.txt
```

The `--input-file` is there only on the runs where the file exists.

## Nook 6: the session file carries the gid

This is what makes a restart work at all.

The engine writes its unfinished downloads into the session file, **with their gid**. When the
application starts the engine again with `--input-file`, the engine picks up the same downloads
**with the same gid**.

That is why the application's records still match after a restart: the record's key is the gid, and
the gid survived.

```go
// The session file carries the gid. A download survives a restart,
// with the same gid.
```

The check that proves it:

```bash
go run ./tools/restart-check
```

```
2. The application was closed (the sidecar stopped)
3. The application was started again
4. The record after the restart: status=active completed=133120 gid=1c1b6d9f2a0a185e
```

**If the gid changed, the records would not match.** They do.

## Nook 7: `aria2.remove` refuses a running download

The engine will not remove a download that is working:

```go
// Remove removes a download.
//
// The engine refuses to remove a running download, so the caller has
// to pause it first.
func (c *Client) Remove(ctx context.Context, gid string) error {
    return c.call(ctx, methodRemove, []any{gid}, nil)
}
```

There is also `aria2.forceRemove`, for the case where the engine will not let it go politely:

```go
// ForceRemove removes a download even when the engine is still working
// on it.
//
// It is the last resort, used when a download has to go and the engine
// would not let it go politely.
```

The right order for a user who deletes a running download is: pause, then remove. The backend does
that; the frontend does not.

### The other half of the rule: a finished download is not removed either

The engine only removes a download that is **still working**. A download that has come to rest is
refused, and the refusal says so:

```
aria2 error 1: Active download not found for GID 28cc7fd839fd7f53
```

That is measured against aria2-next 3.0.9, and it holds for `aria2.remove` and for
`aria2.forceRemove` alike. A download that has come to rest is taken out of the engine's own results
instead, which is what removes it:

```go
// RemoveResult removes the result of a finished download.
//
// It is the only way to take a download that has come to rest out of
// the engine.
func (c *Client) RemoveResult(ctx context.Context, gid string) error {
	return c.call(ctx, methodRemoveResult, []any{gid}, nil)
}
```

So the backend reads the state of the download once more before it removes it, and it picks the call
that the state allows.

### Every one of these calls is a race

The pause of a download is a request, not a fact: the engine answers it at once and pauses the
download a moment later. A download may also finish in between. The engine refuses both, and the
refusals carry no information the caller did not already have:

```
aria2 error 1: GID 0123456789abcdef cannot be paused now
aria2 error 1: GID 0123456789abcdef cannot be unpaused now
```

They are recognised as what they are:

```go
// IsNotPausableNow answers whether the engine refused the request
// because the download is no longer running.
func IsNotPausableNow(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), cannotBePausedNow)
}
```

A download that is paused already is not paused again, and a download that has come to rest is not
paused at all: there is nothing left to pause.

## Nook 8: the engine has to be verified before it is trusted

The manager checks that the binary it started is really aria2-next, because BitMaher is built
against the aria2-next RPC surface — and upstream aria2 would answer to `tellFiles`, not
`getFiles`.

```go
// Start starts the sidecar and waits until it answers.
//
// The engine is verified before it is marked as ready: the version
// call must answer and the engine must identify itself as aria2-next.
// A binary that is not aria2-next is refused, because BitMaher is
// built against the aria2-next RPC surface.
```

This is a real protection: a user who has upstream aria2 installed and points BitMaher at it would
otherwise get a confusing "unknown method" error much later.

The check that proves the bundled binary works:

```bash
go run ./tools/bundled-check
```

## Nook 9: the binary may not be where the application is

The engine binary is embedded with `go:embed` and copied to a writable cache directory before it
is started, because the install directory is not always writable — `/Applications` on macOS,
`C:\Program Files` on Windows.

So there are **three** possible sources of the engine, in the order they are tried:

1. The **bundled** binary, copied out of the embed and made executable.
2. The **external** binary the user chose in the settings.
3. A **system-installed** `aria2-next` on the `PATH`.

The third is the fallback for a fresh clone, because the bundled binaries are not in Git:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:fetch:aria2   # the bundled path
```

## Nook 10: the sidecar is started on a fresh port every time

The port is chosen per start:

```go
// The port is picked per start, so that two instances of the
// application never fight over the same one.
```

Two instances of the application therefore never collide on the engine's port. The cost is the race
described in Chapter 3, and the retry loop that answers it.

## Nook 11: the watchdog distinguishes a crash from a deliberate stop

The watchdog restarts a sidecar that died. But the application also stops the sidecar deliberately,
every time it quits — and a watchdog that cannot tell the difference will resurrect the engine on
the way out.

```go
// The shutdown already took the sidecar down on purpose, so there
// is nothing to restart.
if !stillOurs {
    return
}
```

The `stillOurs` check is the whole mechanism. The watchdog remembers the `waitDone` channel it was
watching; if the manager has replaced it, the sidecar that died is not the one the watchdog was
watching, and it stops.

There is a second guard: the shutdown **stops the watchdog first**, and waits for it:

```go
func (m *Manager) stopWatchdog() {
    // ...
    if cancel != nil {
        cancel()
    }
    m.wg.Wait()
}
```

Only one watchdog runs at a time. A second one would wait for the first at the shutdown, and the
first would never finish.

The check:

```bash
go run ./tools/watchdog-check
```

## Nook 12: the engine is stopped politely, then firmly

The shutdown has two steps and a timeout for each:

```go
const (
    // shutdownGrace is how long the engine may take to stop on its
    // own after it has been asked to.
    shutdownGrace = 5 * time.Second
    // killGrace is how long the engine may take after it has been
    // signalled, before it is killed.
    killGrace = 3 * time.Second
)
```

`aria2.shutdown` asks the engine to finish what it is doing and exit. If it does not, the process
is signalled, and if that does not work either, it is killed.

The engine is a child process. **It must not outlive the application.** That is not a preference:
an orphaned engine keeps downloading into a directory the user has closed the application for.

The check:

```bash
go run ./tools/leak-check
```

```
5. No sidecar outlived the application
```

## Nook 13: the status is normalised in exactly one place

The engine has six states. The application has six states, and they map one to one:

`active`, `waiting`, `paused`, `complete`, `error`, `removed`.

The mapping lives in `internal/aria2/types.go`, and the frontend **never sees the engine's raw
strings**. It sees numbers.

```go
// The backend normalises the engine's state into records with
// numbers; the ported components read the engine's own shape with
// strings. The adapter sits between them.
```

That is the `tasks.ts` adapter in the frontend. The ported components were written against strings,
because that is what the Tauri/Rust original used.

## Nook 14: the engine's log is a file, not the console

The engine writes to its own log file, in its own directory:

```
pb_data/aria2/aria2-next.log
```

The application captures the engine's **stderr** as well, and puts it into the debug console. Both
are useful, and they answer different questions: the file is the engine's own account of what it
did, and the console is the application's view of the conversation.

The engine is started with `--console-log-level=warn`, so its stdout is quiet. That is deliberate:
the console panel would otherwise fill with the engine's progress lines.

## The checks for this layer

```bash
go run ./tools/e2e-check       # the whole pipeline, with a real sidecar
go run ./tools/restart-check   # a download survives a restart
go run ./tools/watchdog-check  # a crashed sidecar is started again
go run ./tools/bundled-check   # the embedded binary is found and run
go run ./tools/leak-check      # no sidecar outlives the application
go run ./tools/torrent-check   # a torrent's files are listed and chosen
```

## What to read next

[Chapter 5](05-pocketbase.md) is about the other half of the backend: the database that runs as a
library inside the process, and the rules that keep it safe.
