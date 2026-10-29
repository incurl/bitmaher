# Chapter 9 — Testing

This repository has two kinds of tests, and they answer different questions.

| | Unit tests | The live checks |
| --- | --- | --- |
| Where | `*_test.go`, `__tests__/` | `tools/*-check/` |
| What they start | Nothing | The real application, with a real engine |
| How long | Twelve seconds | A minute each |
| What they prove | The code does what it says | The **system** does |

Neither is a substitute for the other. The unit tests cannot find a bug in the way two processes
talk to each other. The live checks cannot tell you *which* line is wrong.

## The unit tests

```bash
go test -race ./internal/... .
cd frontend && npm test
```

### The race detector is not optional

```bash
go test -race ./internal/... .
```

The whole application is a set of goroutines talking to a child process. The mistakes that matter
are the ones that only show up under load, on a different machine, weeks later.

The race detector finds them at test time, on your machine, today.

### The surface test is a tripwire

`internal/bindings/surface_test.go` is not a test of behaviour. It is a **contract**:

```go
var expectedMethods = []string{
    "AddDownload",
    "AppExit",
    // ...
}
```

It fails when the set of methods the frontend can see changes without the list being updated.

That is deliberate. The binding generator publishes **every exported method** of `AppService`, so
adding a helper by mistake publishes it to the frontend. The test makes that a compile-time-shaped
event rather than a surprise.

The same file checks the **return types**: a method that returns an internal type is useless to
the frontend, because the binding cannot marshal it.

### The pure logic is tested on its own

The convention:

> Keep the pure logic (the helpers, the formatters) in a separate file next to the component and
> unit test it — `shared/utils/task.ts` and its test are the reference.

So the frontend's 1180 tests run in twelve seconds, because most of them are plain function tests
that mount no component.

### The frontend tests mock the runtime, not the backend

```ts
const bridge = vi.hoisted(() => ({ quit: vi.fn(), invoke: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Application: { Quit: bridge.quit } }))
```

This tests the **shim's contract**: "the hide calls the runtime's hide". The backend's behaviour is
covered by the Go tests and the live checks.

### The typecheck covers the tests

```bash
npm run typecheck   # vue-tsc --noEmit -p tsconfig.test.json
```

The `-p tsconfig.test.json` is what makes the test files part of the type check.

## The live checks

Each one is a `main.go` under `tools/`, and each one starts the real thing.

```bash
go run ./tools/e2e-check
```

They are not unit tests and they do not pretend to be. They are **integration checks that happen to
be executable**.

### The list

| Check | What it proves |
| --- | --- |
| `e2e-check` | The whole pipeline: bootstrap, download, persist, history, restart |
| `pump-check` | The pump publishes the snapshots |
| `zoom-check` | The window carries the zoom of the interface |
| `commands-check` | The commands the port added work against a real engine |
| `restart-check` | A download survives a restart of the application |
| `watchdog-check` | A crashed sidecar is started again |
| `bundled-check` | The embedded binary is found, copied out, and run |
| `leak-check` | No sidecar outlives the application |
| `torrent-check` | A torrent's files are listed, chosen, and the choice is honoured |
| `quit-check` | The quit ends the application and the engine |

### What they look like

They print what they are doing, in a numbered list, and the numbers are the assertions:

```
1. PocketBase bootstrapped, schema applied
   the engine binary is /tmp/bitmaher-e2e-binary/aria2-next
2. The sidecar started on port 41425 (aria2-next 3.0.9)
3. The download was added: payload.bin (gid c15daf5fce1247ed)
4. The record was updated: status=complete size=524288
   the file was written to the download folder
5. The history holds the download: payload.1.bin (added at 2026-10-29 06:07:39.648Z)
6. The configuration of the frontend survived the round trip
7. The engine answered: 1 stopped, 0 active
8. The sidecar stopped

END TO END: PASS
```

A check that fails says which number failed, and prints the application's own output.

### They use an isolated data directory

Every check runs against a temporary directory:

```go
dir, err := os.MkdirTemp("", "bitmaher-quit")
```

so they never touch the user's data. They also point at a copy of the engine binary, so a failing
check cannot damage a real installation.

### They clean up after themselves

This is worth stating, because it is a property of the checks that is easy to lose:

```bash
go run ./tools/leak-check
```

```
5. No sidecar outlived the application

NO LEAK: PASS
```

A check that leaves a child process behind is worse than no check, because it makes the next run
fail for the wrong reason.

## The two checks that reach further

### `quit-check` — the one that found the window bug

```bash
go run ./tools/quit-check
```

```
1. The application is /home/peter/GolandProjects/bitmaher/bin/bitmaher-server
   the engine binary is /tmp/bitmaher-check-engine/aria2-next
2. Started, pid 75242
3. The server is listening
4. The engine is running, pid 75255
5. The answers of the features BitMaher does not have are falsy
   asking the application to quit
6. The engine was stopped by the shutdown
7. The application ended

The quit works: the shutdown runs, and the engine goes with it.
```

Step 5 was added after the window bug. It asks the application, **over the wire**, for the answers
of the features it does not have, and fails if any of them is truthy.

That is the check that would have caught the bug before it shipped:

```go
// tools/quit-check/main.go
func theNeutralAnswersAreFalsy() bool {
	for _, method := range []string{"NeutralBool", "UnsupportedCommand"} {
		// ...
		if answer != "false" {
			fmt.Printf("   FAIL: %s answered %q, want false\n", method, answer)
			return false
		}
	}
	// The value answer has to be nothing, and not an empty object.
	// ...
}
```

### The desktop check — the one that needs a display

The desktop binary serves its runtime over the `wails://` scheme, in-process. An external HTTP
client cannot reach it.

So the window's visibility is checked with a virtual display:

```bash
Xvfb :77 -screen 0 1440x900x24 &
DISPLAY=:77 ./bin/bitmaher
```

and the window's state is read from X:

```bash
xwininfo -root -children | grep bitmaher
xwininfo -id 0x6200004 | grep 'Map State'
```

```
0x6200004 MapState:IsViewable Width: 1440 Height: 900
```

**`IsUnMapped` is the signature of a "flashed and disappeared" bug.** The process is alive, the
window exists, and it is not on the screen.

This check is not in `tools/` yet, because it needs an X server and would fail on a headless
machine. It is worth knowing how to run it when a window behaves strangely.

## The order to run them in

While iterating, run the narrowest thing:

```bash
go test ./internal/downloads/       # the domain
go test ./internal/aria2/           # the engine's RPC surface
npm test                             # the frontend
```

Before finishing, run the checks that match what you changed — the routing table in `AGENTS.md`
has the mapping.

The full sweep:

```bash
# the backend
gofmt -l internal/ main.go main_test.go tools/   # must print nothing
go vet ./internal/... .
go test -race ./internal/... .

# the live checks, against a real engine
go run ./tools/e2e-check
go run ./tools/pump-check
go run ./tools/zoom-check
go run ./tools/commands-check
go run ./tools/restart-check
go run ./tools/watchdog-check
go run ./tools/bundled-check
go run ./tools/leak-check
go run ./tools/torrent-check
go run ./tools/quit-check

# the frontend
cd frontend && npm test && npm run typecheck && npm run build

# the licences
python3 tools/license-audit.py
```

## The verification policy

From `AGENTS.md`, and it is the rule to follow:

- Run the narrowest relevant checks while iterating.
- Before finishing, run the checks that match the changed surface.
- **A change to the engine layer is not done until `go run ./tools/e2e-check` passes against a real
  binary.**
- If a required check cannot run locally, report the reason and the exact command that remains.

That last one matters. A check that could not run is a check that did not pass, and the difference
has to be stated rather than glossed over.

## What to read next

[Chapter 10](10-failure-modes.md) is the collection of failures that happened in this repository,
each with its cause and its fix.
