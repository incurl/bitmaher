# The BitMaher Handbook

A companion book for BitMaher: the nooks and crannies of the five technologies the application is
built on, written for the reader who has to change this code.

It is not a tutorial. It is the collection of things that are true about **Wails v3**, **Go**,
**Aria2-Next**, **PocketBase as a library** and **Vue 3** in this particular application, written
down so that the next person does not have to rediscover them the hard way.

## Who this is for

**If you are new to the stack**, read the chapters in order. Each one opens with what the thing
is, in the fewest words that are still true, and then spends its time on the parts that surprise
people. The surprises are the point: the API you already know is not quite the API that is here.

**If you are changing the application**, read the chapter for the layer you are touching, and then
the "Nooks and crannies" section at the end of it. Those are the traps that have already cost
someone an afternoon.

**If you are reviewing or debugging**, go straight to [Chapter 10: The failure modes](10-failure-modes.md#chapter-10--the-failure-modes).
Every entry there is a real failure that happened in this repository, with the reason and the fix.

## The reading order

| # | Chapter | What it is about |
| --- | --- | --- |
| 1 | [The shape of the application](01-the-shape.md#chapter-1--the-shape-of-the-application) | What the pieces are, and why they are cut this way |
| 2 | [Wails v3](02-wails.md#chapter-2--wails-v3) | The desktop runtime, its wire protocol, and its traps |
| 3 | [Go](03-go.md#chapter-3--go) | The parts of Go that this application leans on |
| 4 | [Aria2-Next](04-aria2.md#chapter-4--aria2-next) | The engine, its RPC surface, and what it does not do |
| 5 | [PocketBase as a library](05-pocketbase.md#chapter-5--pocketbase-as-a-library) | The database, the migrations, and the rules |
| 6 | [Vue 3](06-vue.md#chapter-6--vue-3) | The frontend, the platform layer, and the port |
| 7 | [The seams](07-the-seams.md#chapter-7--the-seams) | Where the five technologies meet, and who owns what |
| 8 | [Building and shipping](08-building.md#chapter-8--building-and-shipping) | The toolchain, the tasks, the artefacts |
| 9 | [Testing](09-testing.md#chapter-9--testing) | The unit tests, the live checks, and what each can tell you |
| 10 | [The failure modes](10-failure-modes.md#chapter-10--the-failure-modes) | The bugs that happened, and the traps that cause them |
| 11 | [The glossary](11-glossary.md#chapter-11--the-glossary) | The words this codebase uses in a particular way |

## How the claims were checked

Every claim about a library's behaviour in this book was checked against the version that is
pinned here, not against the documentation of that library:

- **Wails v3.0.0** — read in `~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0`
- **PocketBase v0.39.11** — the same, under `github.com/pocketbase/pocketbase@v0.39.11`
- **Aria2-Next 3.0.9** — asked directly, over JSON-RPC, with `system.listMethods`
- **Go 1.26** — the toolchain pinned in `go.mod`
- **Vue 3.5** and **Vite 6** — the versions in `frontend/package.json`

Where the upstream documentation of a library disagrees with what the pinned version does, the book
says so and gives the version's behaviour. The upstream docs are frequently written for a different
release, and the difference is usually the interesting part.

## The companion documents

This book is the **why** behind the code. The other documents carry the rest:

| Document | Read it for |
| --- | --- |
| [`../architecture.md`](../architecture.md) | Why the pieces are shaped the way they are, in the application's own terms |
| [`../porting.md`](../porting.md) | The checklist for taking this template to a new application |
| [`../troubleshooting.md`](../troubleshooting.md) | The failures that cost time, and what fixes them |
| [`../../AGENTS.md`](../../AGENTS.md) | The working rules, the commands that work, and the change routing |
| [`../../CONTEXT.md`](../../CONTEXT.md) | The vocabulary |

## A note on the licence

BitMaher is MIT. Aria2-Next is GPLv2. They are two programs that talk over a socket, and this book
is careful to keep them two programs — see [Chapter 7](07-the-seams.md#chapter-7--the-seams). Nothing in here
links against the engine, embeds its sources, or reaches it through CGO.
