# The Linux webview and the graphics driver

A detailed record of the `svga_surface_destroy` failure, what actually causes it, and what
actually fixes it — measured on the machine, not assumed. It is written so that it can be used
outside this repository: the same failure hits **Tauri 2**, **Wry**, **Wails** and anything else
that draws a **WebKitGTK** webview on Linux under a virtual machine.

---

## 1. The symptom

```
context mismatch in svga_surface_destroy
```

The line is printed by the **driver**, not by the application. On this machine it comes from the
VMware SVGA3D Gallium driver (`vmwgfx`), and it appears **four times** during a single window's
lifetime — on the way in and on the way out.

The application itself is fine: the window is created, it paints, it closes, and the exit code is
`0`. The message is noise.

It is not always noise. The same race that produces the message has been observed to leave a
window that does not repaint, and on a machine whose driver is broken in a different way it can
take the whole web process down. **Treat it as a real defect, and silence it.**

### The environment it was measured on

| | |
| --- | --- |
| Machine | `VMware, Inc.` — a virtual machine |
| The card | `0000:00:0f.0`, class `0x030000`, driver **`pci:vmwgfx`** |
| The render node | `/dev/dri/renderD128` (mode `crw-rw----`, group `render`) |
| The renderer | `SVGA3D; build: RELEASE; LLVM` — Mesa 25.0.7-2+deb13u1 |
| `glxinfo -B` | `Accelerated: no` |
| GTK | 4.18.6 |
| WebKitGTK | 2.54.0 (both `webkit2gtk-4.1` and `webkitgtk-6.0`) |
| Desktop | SparkyLinux 8.4, **X11** (`DISPLAY=:0`, no Wayland) |

---

## 2. The cause

The VMware SVGA3D driver tears a surface down on a thread that does not own the GL context that
created it. The driver's own check fails, and it says so.

The important consequence is that **the message is a symptom, not the cause**. The cause is that
the graphics stack engaged the VMware driver at all. The fix is not to suppress the message — it
is to keep the driver out of the picture.

### What the message does *not* tell you

The offending GL context is **not** the webview's. This is the trap, and it is what makes the
first two attempts fail:

- `WEBKIT_DISABLE_COMPOSITING_MODE=1` stops the **web process** from compositing with the GPU.
- But the window is drawn by **GTK's own renderer**, which creates its own GL context. That
  context is the one the driver complains about.
- `GSK_RENDERER=cairo` stops **GTK** from drawing with the GPU.
- But the **graphics stack is still loaded**, and it still opens the render node of the card. The
  driver is still engaged, and it still destroys the surface it never made.

---

## 3. The measurements

Every row below was measured on the machine above, with the application built **without** its
own fix, so that the environment variable under test is the only thing that changes.

The count is the number of `context mismatch in svga_surface_destroy` lines printed during one
full run of the application.

| The environment | Messages | The render node |
| --- | --- | --- |
| nothing set | **4** | open |
| `WEBKIT_DISABLE_COMPOSITING_MODE=1` | **4** | open |
| `GSK_RENDERER=cairo` | **4** | open |
| `GDK_BACKEND=x11` | **4** | open |
| `WEBKIT_DISABLE_DMABUF_RENDERER=1` | **0** | closed |
| **`LIBGL_ALWAYS_SOFTWARE=1`** | **0** | **closed** |

**Only two settings silence it.** The three that people reach for first do nothing, and the two
that work both work by the same mechanism: **the render node of the card is never opened**, so
the VMware driver is never engaged and cannot destroy anything.

### The render node is the ground truth

The renderer string tells you what the stack *would* use; the render node tells you what it
*did*. This is the measurement that settles every argument:

```bash
# is the card's render node open in this process?
ls -la /proc/$(pgrep -x <your-app>)/fd/ | grep -c renderD128
```

```bash
# 1 = the driver is engaged.  0 = it is not.
```

To compare two settings, run a long-lived process under each and sample it. `glxinfo` is too
short-lived to sample reliably:

```bash
probe() {
  local label="$1"; shift
  "$@" >/dev/null 2>&1 &
  local PID=$!
  sleep 1
  echo "$label: $(ls -la /proc/$PID/fd/ 2>/dev/null | grep -c renderD128)"
  kill $PID 2>/dev/null; wait $PID 2>/dev/null
}

probe "default          " glxinfo -B
probe "LIBGL_ALWAYS_SOFT" env LIBGL_ALWAYS_SOFTWARE=1 glxinfo -B
```

```text
default          : 1
LIBGL_ALWAYS_SOFT: 0
```

### Which renderer is being used

```bash
glxinfo -B | grep -E 'OpenGL renderer|Accelerated'
```

```text
OpenGL renderer string: SVGA3D; build: RELEASE; LLVM;
    Accelerated: no
```

`SVGA3D` is the driver that misbehaves. With the fix, the same command answers differently:

```bash
LIBGL_ALWAYS_SOFTWARE=1 glxinfo -B | grep 'OpenGL renderer'
```

```text
OpenGL renderer string: llvmpipe (LLVM 19.1.7, 256 bits)
```

`llvmpipe` is Mesa's software rasteriser. It is slower, and it is correct.

### Which library pulls the driver in

When the message is present, the graphics stack is loaded into the process. This shows it:

```bash
grep -oE '/[^ ]*(gallium|drm|gbm|libGL|libEGL)[^ ]*' /proc/$(pgrep -x <your-app>)/maps | sort -u
```

```text
/usr/lib/x86_64-linux-gnu/libEGL_mesa.so.0.0.0
/usr/lib/x86_64-linux-gnu/libgallium-25.0.7-2+deb13u1.so
/usr/lib/x86_64-linux-gnu/libgbm.so.1.0.0
```

`libEGL_mesa` is what opens the render node. With the fix in place, the render node count is
zero and this list is shorter.

---

## 4. The fix

Two settings work. **Both are needed as a pair if the machine might have a real GPU**, because
they work at different levels:

| Setting | Level | What it does |
| --- | --- | --- |
| `WEBKIT_DISABLE_COMPOSITING_MODE=1` | the web process | stops WebKit from compositing with the GPU |
| `GSK_RENDERER=cairo` | GTK | stops GTK's renderer from drawing with the GPU |
| **`LIBGL_ALWAYS_SOFTWARE=1`** | **the graphics stack** | **stops Mesa from opening the render node of the card** |
| `WEBKIT_DISABLE_DMABUF_RENDERER=1` | the web process | stops WebKit from using the DMA-BUF renderer |

The third one is the one that matters. The first two stop the *drawing*; they do not stop the
*driver* from being engaged.

### The three that do nothing

**Do not spend time on these.** They are the usual first suggestions, and they were measured here
and did nothing:

- `WEBKIT_DISABLE_COMPOSITING_MODE=1` — 4 messages, unchanged.
- `GSK_RENDERER=cairo` — 4 messages, unchanged.
- `GDK_BACKEND=x11` — 4 messages, unchanged. (This machine is already X11.)

There is a second trap here, and it is the one that makes the fix look intermittent: **if the
settings are applied in a chain where each one guards the next, the first one disables the
rest.** A user who sets one variable by hand and the others by code gets half the fix and no
message explaining why. **Make the settings independent.** Each one is checked on its own, and
each one is left alone when the user has already set it.

### Where to set them

The environment is read when the webview is made, so the setting has to be in place **before the
window is created** — not after, and not from a `main` that has already shown a window.

**A desktop application** sets them on its own process, at the very top of `main`, before the
GUI library is initialised:

```rust
// Tauri 2 — src-tauri/src/main.rs, before tauri::Builder::default()
fn main() {
    if std::env::var("LIBGL_ALWAYS_SOFTWARE").is_err() {
        std::env::set_var("LIBGL_ALWAYS_SOFTWARE", "1");
    }
    if std::env::var("GSK_RENDERER").is_err() {
        std::env::set_var("GSK_RENDERER", "cairo");
    }
    if std::env::var("WEBKIT_DISABLE_COMPOSITING_MODE").is_err() {
        std::env::set_var("WEBKIT_DISABLE_COMPOSITING_MODE", "1");
    }

    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

> **Rust 2024 note.** `std::env::set_var` is `unsafe` in Rust 2024. In an edition 2024 crate,
> either keep the crate on edition 2021, or wrap the call in an `unsafe` block with a comment
> explaining that it happens before any thread is spawned.

**Go** does the same, before `wails3` is touched:

```go
func main() {
    if os.Getenv("LIBGL_ALWAYS_SOFTWARE") == "" {
        _ = os.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")
    }
    if os.Getenv("GSK_RENDERER") == "" {
        _ = os.Setenv("GSK_RENDERER", "cairo")
    }
    if os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE") == "" {
        _ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
    }

    app := NewApp()
    // … the webview is created from here on
}
```

**To try it out without changing any code**, set the variables in the shell:

```bash
LIBGL_ALWAYS_SOFTWARE=1 GSK_RENDERER=cairo WEBKIT_DISABLE_COMPOSITING_MODE=1 ./your-app
```

### Deciding whether to apply it at all

The fix costs performance, so it should not be applied to a machine with a real GPU. The
renderer is found in three places, in the order that keeps the answer the most certain:

1. **the environment** — `WEBKIT_RENDERER`, which a user sets to override everything else;
2. **the graphics card** — the PCI bus, class `0x03xxxx`, and the driver bound to it;
3. **the DMI of the machine** — `sys_vendor`, which names the virtual machine.

```bash
# 2. the card and its driver
for f in /sys/bus/pci/devices/*/class; do
  c=$(cat "$f" 2>/dev/null)
  case "$c" in 0x03*) echo "$f -> $c -> $(ls "$(dirname "$f")/driver/module/drivers/" 2>/dev/null)";; esac
done

# 3. the machine
cat /sys/class/dmi/id/sys_vendor
```

The software renderers are the ones that misbehave: `llvmpipe`, `softpipe`, `swrast`, `svga3d`,
`vmwgfx`, `vmware`. The virtual machine vendors are `vmware`, `virtualbox`, `innotek`, `qemu`,
`kvm`, `parallels`, `bochs`, `oracle`.

**When nothing is known, leave the machine alone.** A wrong guess costs the user the smoothness
of the interface, and the error it avoids is only a message.

---

## 5. The other WebKitGTK setting that matters

`WEBKIT_DISABLE_DMABUF_RENDERER=1` also silenced the message — 0 messages, render node closed.

This one is worth knowing about on its own, because it is the setting people reach for when
they see a **blank window**, a **window that never paints**, or **GPU hangs** under a virtual
machine, and it is *not* the same knob as the compositing one. WebKitGTK 2.54.0 has it:

```bash
strings /usr/lib/x86_64-linux-gnu/libwebkit2gtk-4.1.so.0 | grep WEBKIT_DISABLE
```

```text
WEBKIT_DISABLE_COMPOSITING_MODE
WEBKIT_DISABLE_DMABUF_RENDERER
WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS
```

The DMA-BUF renderer hands the frames to the compositor through a GPU buffer, and that path is
what a broken virtual GPU cannot carry. Disabling it makes WebKit fall back to a shared-memory
path.

**Both** settings are worth having in a virtual machine:

- `WEBKIT_DISABLE_DMABUF_RENDERER=1` — the webview's own rendering path.
- `LIBGL_ALWAYS_SOFTWARE=1` — the graphics stack underneath it.

They fix the same symptom by different routes, and either alone was enough here.

---

## 6. A note on Wayland

This was measured on **X11**. On Wayland the failure looks different, and the fix that works is
different too:

- Wayland lets no window place itself, so a compositor that requires it needs the application to
  run under XWayland.
- `GSK_RENDERER=cairo` is still the correct answer for the GTK renderer.
- The `GDK_BACKEND=x11` fallback is what forces the XWayland path — and on this machine it made
  **no difference** to the message, because the machine was already on X11.

If the message persists under Wayland with `LIBGL_ALWAYS_SOFTWARE=1` set, the remaining suspect is
the compositor's own GL handling, not the webview's.

---

## 7. How to check that it worked

Four checks, in the order they should be run.

**1. The message is gone.**

```bash
./your-app 2>&1 | grep -c 'context mismatch'
```

```text
0
```

**2. The render node is closed.** Start the application, and while it is running:

```bash
ls -la /proc/$(pgrep -x your-app)/fd/ | grep -c renderD128
```

```text
0
```

**3. The renderer is the software one.**

```bash
LIBGL_ALWAYS_SOFTWARE=1 glxinfo -B | grep 'OpenGL renderer'
```

```text
OpenGL renderer string: llvmpipe (LLVM 19.1.7, 256 bits)
```

**4. The application still works.** The window paints, the interface is responsive, and the
frames are not torn. The fix moves the drawing to the CPU, and on a machine that was already
using a software renderer (`Accelerated: no`) **nothing is lost at all**.

---

## 8. The prompt

The prompt below is for a code agent that has to solve this on a Tauri 2 application, or on any
other project with the same symptom. It states the symptom, the measurements, and the traps, so
that the agent does not repeat the three dead ends.

---

> **The task.**
>
> A Linux desktop application built with **Tauri 2** prints the following four times while it
> starts and closes, and the window sometimes fails to repaint:
>
> ```
> context mismatch in svga_surface_destroy
> ```
>
> The application itself works — the window opens, it paints, and the exit code is `0`. The
> message comes from the **VMware SVGA3D driver** (`vmwgfx`), not from the application. It is
> printed when the driver destroys a surface on a thread that does not own the GL context that
> created it.
>
> **The machine** is a VMware virtual machine. The card is `0000:00:0f.0`, class `0x030000`,
> driver `pci:vmwgfx`. `glxinfo -B` reports `OpenGL renderer string: SVGA3D` and
> `Accelerated: no`. There is a render node at `/dev/dri/renderD128`.
>
> **What has already been measured on this machine**, with the application built so that the
> environment is the only variable:
>
> | The environment | Messages |
> | --- | --- |
> | nothing set | **4** |
> | `WEBKIT_DISABLE_COMPOSITING_MODE=1` | **4** — no effect |
> | `GSK_RENDERER=cairo` | **4** — no effect |
> | `GDK_BACKEND=x11` | **4** — no effect |
> | `WEBKIT_DISABLE_DMABUF_RENDERER=1` | **0** — fixes it |
> | `LIBGL_ALWAYS_SOFTWARE=1` | **0** — fixes it |
>
> **The mechanism.** The offending GL context is **not** the webview's — it belongs to GTK's own
> renderer. Turning off the compositing of WebKit, or of GTK's renderer, stops the *drawing* with
> the GPU, but the **graphics stack is still loaded and still opens the render node of the card**.
> The VMware driver is therefore still engaged, and still destroys the surface it never made. Both
> settings that work do so by the same route: **the render node is never opened**, so the driver
> is never engaged.
>
> **What to do.**
>
> 1. **Do not** try the three settings that were measured to do nothing. They are the usual first
>    suggestions, and they were all ruled out by measurement on this exact machine.
> 2. Apply **`LIBGL_ALWAYS_SOFTWARE=1`** and **`WEBKIT_DISABLE_DMABUF_RENDERER=1`**. Apply
>    `GSK_RENDERER=cairo` as well if the machine is known to have no real GPU.
> 3. Set them **before the webview is created** — in Rust, at the top of `main`, before
>    `tauri::Builder::default().run(...)`; the environment is read when the webview is made, and
>    setting it afterwards has no effect.
> 4. Make the settings **independent**. If each one guards the next, a user who sets one of them
>    by hand silently disables the others, and the fix looks intermittent. Check each variable on
>    its own, and leave it alone when the user has already set it.
> 5. **Only apply it where it is needed.** The fix moves the drawing to the CPU. Detect the
>    software renderer first: the `WEBKIT_RENDERER` environment variable, then the PCI bus (class
>    `0x03xxxx` and the driver bound to it), then `/sys/class/dmi/id/sys_vendor`. Treat `llvmpipe`,
>    `softpipe`, `swrast`, `svga3d`, `vmwgfx` and `vmware` as software, and `vmware`, `virtualbox`,
>    `innotek`, `qemu`, `kvm`, `parallels`, `bochs` and `oracle` as virtual machines. **When
>    nothing is known, leave the machine alone** — a wrong guess costs the smoothness of the
>    interface, and the error it avoids is only a message.
> 6. If the crate is on **Rust edition 2024**, `std::env::set_var` is `unsafe`. Handle it.
>
> **How to prove it worked.** The message is gone (`./app 2>&1 | grep -c 'context mismatch'` → `0`);
> the render node is closed (`ls -la /proc/$(pgrep -x app)/fd/ | grep -c renderD128` → `0`); and
> the renderer is the software one (`glxinfo -B` → `llvmpipe`). The window must still paint and
> the interface must stay responsive.
>
> **Do not** suppress the message in the application's own output. The message is the driver's,
> and it is emitted on a path the application does not control. Silence it by keeping the driver
> out of the picture.

---

## 9. The implementation in this repository

The fix lives in `internal/graphics`:

| The file | What it holds |
| --- | --- |
| `internal/graphics/graphics_linux.go` | `Configure()` — the three settings, each independent |
| `internal/graphics/renderer_linux.go` | the detection of the software renderer |
| `internal/graphics/graphics_other.go` | the empty `Configure()` for the other platforms |

`Configure()` is called at the top of `main.go`, before the application is created, and by the
live check that draws a window (`tools/zoom-check`). The tests are in
`internal/graphics/graphics_linux_test.go`.

The three settings are:

```go
if os.Getenv(webkitDisableCompositing) == "" {
    _ = os.Setenv(webkitDisableCompositing, "1")
}
if os.Getenv(gtkSoftwareRenderer) == "" {
    _ = os.Setenv(gtkSoftwareRenderer, gtkCairoRenderer)
}
if os.Getenv(alwaysSoftware) == "" {
    _ = os.Setenv(alwaysSoftware, "1")
}
```

Each one is checked on its own. That independence was a bug fix, not a style choice: the settings
used to be one, and setting the first one stopped the second from being applied.

---

## 10. What is still open

- **Wayland.** Measured on X11 only. The Wayland path has its own failure (a window that cannot
  place itself) and its own fix (`GDK_BACKEND=x11` to force XWayland), and the two interact.
- **Other drivers.** The measurements are for the VMware SVGA3D driver. VirtualBox, QEMU and
  KVM have their own software renderers, and the same mechanism is expected to hold — but it has
  not been measured on them.
- **The performance cost.** On a machine that reports `Accelerated: no`, the fix costs nothing,
  because the renderer was already software. On a machine with a real GPU, applying the fix
  unnecessarily is a real regression. The detection avoids it, and the detection is the part that
  deserves its own tests.

---

## 11. The sources of the failure modes

- The driver is the **VMware SVGA3D Gallium driver** (`vmwgfx`), and the message is its own.
- The renderer string `SVGA3D; build: RELEASE; LLVM` is Mesa's, and the software rasteriser it
  falls back to is `llvmpipe`.
- The setting `GSK_RENDERER=cairo` is **GTK 4's** own documented software fallback;
  `GSK_RENDERER=help` lists the available renderers.
- `WEBKIT_DISABLE_COMPOSITING_MODE` and `WEBKIT_DISABLE_DMABUF_RENDERER` are **WebKitGTK's** own
  environment variables.
- `LIBGL_ALWAYS_SOFTWARE` is **Mesa's** own environment variable, and it is the one that keeps
  the render node closed.
