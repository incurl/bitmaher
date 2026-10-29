# Chapter 8 — Building and shipping

The toolchain is `wails3` with its Taskfiles. This chapter is about how the build fits together,
what it produces, and the things that break on a fresh machine.

## The one tool

```bash
wails3 task <name>
```

`wails3` runs the Taskfiles itself, so the `task` binary is not needed.

**`wails3` is not on the default PATH of a non-interactive shell.** It lives in `~/go/bin`:

```bash
export PATH="$HOME/go/bin:$PATH"
```

And `pnpm` lives in `~/.local/bin`, which also carries the Node it needs:

```bash
export PATH="$HOME/go/bin:/home/peter/.local/bin:$PATH"
```

This is the single most common source of "the task fails on my machine but works on yours".

## The tasks are namespaced

The Taskfiles are `include`d under `common:`, `linux:`, `windows:`, `darwin:`, `ios:` and
`android:`.

The bare names in the root `Taskfile.yml` work:

```bash
wails3 task build        # → linux:build
wails3 task package      # → linux:package
wails3 task run          # → linux:run
wails3 task dev          # → wails3 dev
```

But the tasks defined in `build/Taskfile.yml` are **only reachable through their namespace**:

```bash
wails3 task common:generate:bindings
wails3 task common:fetch:aria2
wails3 task common:install:frontend:deps
```

### `wails3 task --summary` is not a description

This trips people up:

```bash
wails3 task --summary common:generate:bindings   # EXECUTES the task
```

`--summary` **runs** the task and prints its summary line. It does not describe it.

To list:

```bash
wails3 task --list        # the tasks in the current scope
wails3 task --list-all    # every task, with names
```

To inspect without running:

```bash
wails3 task --dry common:generate:bindings
```

## The build, step by step

```bash
wails3 task build
```

which is:

```
build → linux:build → common:build → build:frontend → build:go
```

### `build:frontend`

```yaml
build:frontend:
  deps:
    - task: install:frontend:deps
    - task: generate:bindings
```

**The bindings are generated before the frontend is built**, because the frontend imports them. If
you change a Go method and the frontend does not see it, this is why: the bindings were stale.

The task has `run: once` and a `sources:`/`generates:` pair, so it does not run when nothing
changed. That is also what makes it safe to call `wails3 task build` repeatedly.

There is a comment in the Taskfile about a race that bit someone:

```yaml
# darwin:build:universal runs its per-arch builds as parallel deps, each of
# which depends on this task. Without run:once the two executions race:
# one regenerates frontend/bindings (-clean deletes it first) while the
# other's bundler is reading it, failing intermittently with
# 'Could not resolve "./bindings/<pkg>"' (#4637).
```

`-clean=true` deletes the bindings directory before writing it, so two concurrent generations would
delete each other's output. `run: once` prevents that.

### `generate:bindings`

```yaml
cmds:
  - wails3 generate bindings -f '{{.BUILD_FLAGS}}' -clean=true -ts -i
```

It needs `go mod tidy` first (it is a dependency), and it writes into `frontend/bindings/`, which is
**gitignored**.

### `build:go`

The Go build with the right tags, and the `//go:embed` payload:

```
go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" -o "bin/bitmaher"
```

The frontend payload is embedded:

```go
//go:embed all:frontend/dist
var assets embed.FS
```

So **the frontend must be built before the Go binary**, or the binary embeds the previous
`frontend/dist`.

## The artefacts

```bash
wails3 task build      # → bin/bitmaher
wails3 task package    # → the platform's package
```

| Platform | The package |
| --- | --- |
| Linux | AppImage, `.deb` |
| macOS | `.app`, `.dmg` |
| Windows | `.exe`, NSIS installer, `.msix` |

## The server build

```bash
wails3 task build:server
```

```bash
go build -tags server,production -trimpath -buildvcs=false -ldflags="-w -s" -o "bin/bitmaher-server"
```

The `server` tag drops the GUI and leaves an HTTP server. It is what the live checks drive, because
the runtime then answers over HTTP and a test can reach it from outside the process.

Two things to know about it:

- **It does not end on its own.** The server's run loop waits for a signal; the desktop build leaves
  when the window goes. This is a Wails behaviour, and Chapter 2 explains it.
- **It has no authentication.** The download collections are readable by anyone who can reach the
  port. The desktop app is in-process and unaffected; the container image binds `0.0.0.0` and is
  affected.

## The bundled engine

```bash
wails3 task common:fetch:aria2
```

This downloads the prebuilt aria2-next binaries for every supported platform into
`internal/aria2/assets/`, so that they can be embedded.

```
internal/aria2/assets/
├── linux-x86_64/aria2-next
├── linux-aarch64/aria2-next
├── darwin-x86_64/aria2-next
├── darwin-arm64/aria2-next
└── windows-x86_64/aria2-next.exe
```

**The binaries are not in Git.** They are large, and the GPL requires that their sources be offered
separately, which the upstream release already does.

A fresh clone therefore has none, and the application falls back to a system-installed
`aria2-next`. That is why the fetch task exists, and why it is worth running before the first
build.

The version is pinned in one place:

```yaml
# build/Taskfile.yml
ARIA2_VERSION: '3.0.9'
```

and it is checked at runtime:

```go
// internal/aria2/binary.go
```

The check:

```bash
go run ./tools/bundled-check
```

## The licence audit

```bash
python3 tools/license-audit.py
```

It reads the dependency trees of both languages and **fails on a copyleft dependency**. This is
the automation behind the licensing boundary: if a dependency with a copyleft licence ever appears
in the tree, the build stops.

```
Licence breakdown:
  MIT            45
  BSD-3-Clause   34
  Apache-2.0     9
  ISC            3
  UNKNOWN        3
  MPL-2.0        2

96 dependencies audited.

PASS: no copyleft dependency would be shipped.
```

The `UNKNOWN` entries are worth a look when they appear — an unrecognised licence is not a
licence, it is a question.

## The cross-compilation

```bash
wails3 task build GOOS=windows
```

The Windows cross-compile works.

**Cross-compiling to macOS needs CGO and a macOS toolchain**, because of Wails. This is a Wails
constraint, not a Go one.

There is also a Docker route for the cases that need a real toolchain:

```bash
wails3 task setup:docker      # ~800MB download
wails3 task build:docker
```

## The development loop

```bash
wails3 task dev
```

This runs `wails3 dev`, which:

- starts the Vite dev server on port 9245 (`WAILS_VITE_PORT`);
- watches the Go files and rebuilds;
- serves the frontend from Vite, so the changes are live.

The dev server binds `127.0.0.1` and uses `strictPort`, so a port clash is an error rather than a
silent move to another port.

## The data directory

```bash
BITMAHER_DATA_DIR=/tmp/my-test-data ./bin/bitmaher
```

Everything lives in `pb_data/` next to the executable by default:

```
pb_data/
├── data.db
├── storage/            the uploaded files
├── .bitmaher_key       (mode 0600)
└── aria2/
    ├── session.txt     the engine's session
    └── aria2-next.log  the engine's log
```

Relocating it is the way to run against an isolated database during development and testing, and
it is what every live check does.

**Never run two instances against the same data directory.** SQLite is a single-writer database,
and the second instance will fight the first.

## What to read next

[Chapter 9](09-testing.md) is about the tests: the unit tests, the live checks, and what each of
them can actually tell you.
