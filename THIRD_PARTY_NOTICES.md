# Third party notices

BitMaher is built on the following third party projects. Every licence
below is permissive; **no GPL, AGPL or other copyleft dependency is
present in the dependency tree**. This is verified automatically by
`tools/license-audit.py`, which exits non-zero if a copyleft licence is
introduced.

## The ported frontend

The interface of BitMaher is a port of the **Rayburst** frontend, the
interface of [Motrix Next](https://github.com/Rayburst/Motrix-Next),
which is licensed under the **MIT licence**.

| Project | Why it is here | Licence |
| --- | --- | --- |
| [Rayburst / Motrix Next](https://github.com/Rayburst/Motrix-Next) | The Vue 3 interface: the task list, the task detail, the add-task dialog, the settings and the design system | MIT |

The port is a derivative work, and it keeps the copyright notice of the
original:

```
MIT License

Copyright (c) 2025-present AnInsomniacy

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

What the port changed, and what it kept:

- The **interface** is Rayburst's: the components, the layout, the
  motion, the styles and the twenty-seven translations.
- The **backend** is BitMaher's: the Go services, the PocketBase
  database and the Aria2-Next sidecar. The Tauri commands of Rayburst
  are answered by a platform layer (`frontend/src/platform/`) that maps
  them onto the Wails bindings.
- The **push model** is Rayburst's: the Rust backend of Rayburst
  publishes a `tasks:snapshot` event, and the Go backend of BitMaher
  does the same, from `internal/pump`.

The MIT licence of Rayburst and the MIT licence of BitMaher are
compatible: both permit redistribution under the same terms, and the
notice above is carried in this file.

## The bundled download engine

BitMaher **ships a prebuilt binary of Aria2-Next**, which is licensed
under the **GNU General Public License, version 2**.

| Project | Why it is here | Licence |
| --- | --- | --- |
| [Aria2-Next](https://github.com/AnInsomniacy/aria2-next) | The download engine: the HTTP, BitTorrent and Metalink transfers | **GPL-2.0** |

This is deliberate, and the boundary around it is deliberate too:

- Aria2-Next runs as a **separate, independent child process**. BitMaher
  starts it, watches it, drives it over JSON-RPC and stops it again.
- Nothing in the BitMaher source tree links against Aria2-Next, imports
  its headers, vendors its sources or calls into it through CGO. The
  coupling is a loopback socket carrying a documented protocol, which is
  an interface between two independent programs rather than a link
  between two parts of one program.
- The binary is shipped as an opaque executable. Its sources are **not**
  included in this repository: they are offered by the upstream project,
  at <https://github.com/AnInsomniacy/aria2-next>, which is what the GPL
  requires of a binary distribution.
- The bundled binaries are fetched by `wails3 task common:fetch:aria2`
  and are not kept in the Git repository.

Because the engine is a separate work that BitMaher does not link
against, BitMaher's own code stays under its own licence. The engine's
GPL terms apply to the engine itself and to the binary as distributed.

The application can also be pointed at a **system-installed
aria2-next**, in which case nothing GPL is bundled at all. That choice
is in the settings, under "Engine".

## The direct dependencies

These are the projects BitMaher imports directly. The full transitive
tree is listed further down.

| Project | Why it is here | Licence |
| --- | --- | --- |
| [Wails v3](https://github.com/wailsapp/wails) | The desktop shell: the window, the asset server, the service bindings and the event bus | MIT |
| [PocketBase](https://github.com/pocketbase/pocketbase) | The embedded database: the schema, the migrations, the file storage and the REST API | MIT |
| [dbx](https://github.com/pocketbase/dbx) | The database query builder PocketBase is built on | MIT |
| [Vue](https://github.com/vuejs/core) | The UI | MIT |
| [Pinia](https://github.com/vuejs/pinia) | The stores: the task list, the settings, the engine state | MIT |
| [Vue Router](https://github.com/vuejs/router) | The routes: the task view, the settings, the about panel | MIT |
| [Vue I18n](https://github.com/intlify/vue-i18n) | The translations | MIT |
| [Naive UI](https://github.com/tusen-ai/naive-ui) | The widgets: the buttons, the inputs, the dialogs, the tables | MIT |
| [SortableJS](https://github.com/SortableJS/Sortable) | The reordering of the task list | MIT |
| [@vueuse/core](https://github.com/vueuse/vueuse) | The composables that watch the browser for the platform | MIT |
| [@vicons/ionicons5](https://github.com/viconsjs/vicons) | The icons | MIT |
| [@material/material-color-utilities](https://github.com/material-foundation/material-color-utilities) | The colour scheme of the interface | Apache-2.0 |
| [marked](https://github.com/markedjs/marked) | The release notes of the updater | MIT |
| [DOMPurify](https://github.com/cure53/DOMPurify) | The sanitising of the release notes | Apache-2.0 |
| [lodash-es](https://github.com/lodash/lodash) | The helpers of the ported code | MIT |
| [Vite](https://github.com/vitejs/vite) | The frontend build and the dev server | MIT |
| [Vitest](https://github.com/vitest-dev/vitest) | The frontend test runner | MIT |
| [@wailsio/runtime](https://github.com/wailsapp/wails) | The runtime the generated bindings call | MIT |


## The full dependency tree

### Go modules — Apache-2.0 (5)

- `appengine@v1.6.5`
- `cobra@v1.10.2`
- `mousetrap@v1.1.0`
- `pprof@v0.0.0-20260709232956-b9395ee17fa0`
- `yaml.v3@v3.0.1`

### Go modules — BSD-3-Clause (33)

- `bigfft@v0.0.0-20230129092748-24d4a6f8daec`
- `crypto@v0.57.0`
- `fexpr@v0.6.0`
- `fileutil@v1.4.0`
- `fsnotify@v1.10.1`
- `go-cmp@v0.7.0`
- `go-difflib@v1.0.0`
- `go-internal@v1.9.0`
- `goabi0@v0.2.0`
- `image@v0.45.0`
- `json@v0.0.0-20251027170946-4849db3c2f7e`
- `libc@v1.74.1`
- `mathutil@v1.7.1`
- `memory@v1.11.0`
- `mod@v0.41.0`
- `net@v0.60.0`
- `oauth2@v0.36.0`
- `opt@v0.2.0`
- `pflag@v1.0.10`
- `sortutil@v1.2.1`
- `sqlite@v1.55.0`
- `strutil@v1.2.1`
- `sync@v0.23.0`
- `sys@v0.48.0`
- `text@v0.42.0`
- `token@v1.1.0`
- `tools@v0.49.0`
- `uuid@v1.6.0`
- `v2@v2.6.5`
- `v3@v3.1.4`
- `v4@v4.29.0`
- `v4@v4.34.6`
- `v5@v5.2.2`

### Go modules — ISC (2)

- `go-spew@v1.1.1`
- `websocket@v1.8.14`

### Go modules — MIT (22)

- `cast@v1.10.0`
- `color@v1.19.0`
- `dbx@v1.12.0`
- `go-colorable@v0.1.15`
- `go-humanize@v1.0.1`
- `go-isatty@v0.0.23`
- `go-ole@v1.3.0`
- `go-strftime@v1.0.0`
- `govalidator@v0.0.0-20230301143203-a9d515a09cc2`
- `imaging@v1.6.2`
- `is@v1.4.1`
- `mimetype@v1.4.13`
- `pocketbase@v0.39.11`
- `pretty@v0.3.1`
- `quicktest@v1.14.6`
- `testify@v1.11.1`
- `text@v0.2.0`
- `v3@v3.0.0`
- `v3@v3.6.2`
- `v4@v4.3.0`
- `v5@v5.3.1`
- `xdg@v0.5.3`

### Go modules — MPL-2.0 (2)

- `mysql@v1.4.1`
- `v2@v2.0.7`

### npm packages — Apache-2.0 (4)
- `dompurify`
- `material-color-utilities`
- `typescript`
- `vite`

### npm packages — BSD-3-Clause (1)
- `marked`

### npm packages — ISC (1)
- `sanitize-filename`

### npm packages — MIT (20)
- `auto-animate`
- `core`
- `happy-dom`
- `intl-localematcher`
- `lodash-es`
- `naive-ui`
- `node`
- `picomatch`
- `pinia`
- `plugin-vue`
- `prettier`
- `sortablejs`
- `test-utils`
- `tsconfig`
- `unplugin-vue-i18n`
- `vitest`
- `vue`
- `vue-i18n`
- `vue-router`
- `vue-tsc`

### npm packages — UNKNOWN (3)
- `ionicons5`
- `marked-alert`
- `runtime`

## Adding a dependency

Before adding a dependency, check its licence:

```bash
python3 tools/license-audit.py
```

The audit fails when a GPL, AGPL, LGPL, SSPL, EUPL or OSL dependency
enters the tree, because a copyleft licence in a distributed desktop
application is a legal problem for the people who ship it.
