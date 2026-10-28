# The bundled Aria2-Next binaries

BitMaher ships the Aria2-Next download engine as a prebuilt binary, one
per platform. It is embedded into the application with `go:embed` and
copied to a writable cache directory before it is started.

## The layout

```
linux-x86_64/aria2-next
linux-aarch64/aria2-next
darwin-x86_64/aria2-next
darwin-arm64/aria2-next
windows-x86_64/aria2-next.exe
```

The directory name is the pair of `runtime.GOOS` and `runtime.GOARCH`,
with `x86_64` in place of `amd64`, which is what the upstream releases
call it. The file name is `aria2-next`, with the `.exe` suffix on
Windows.

## Fetching the binaries

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:fetch:aria2
```

The binaries are **not** in the Git repository. They are around 18 MB
each, and they are GPLv2: the licence requires the sources to be offered
separately, which the upstream release already does. The `.gitignore`
keeps them out.

## Why this file exists

`go:embed` refuses to build when its pattern matches no files. A fresh
clone has no binaries, so this file keeps the pattern satisfied and the
build working. An application built from a fresh clone therefore reports
that no bundled binary is present, and expects a system-installed
`aria2-next` on the `PATH` instead — which is what the settings call
"external binary".

Nothing here is a placeholder binary: an empty or absent binary is
detected, and the application says what is missing rather than failing to
start.

## The licence boundary

Aria2-Next is GPLv2. It is run as a **separate, independent child
process**, driven over JSON-RPC on a loopback socket. Nothing links
against it. See `THIRD_PARTY_NOTICES.md` at the root of the repository.
