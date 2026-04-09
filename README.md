# Dingovault

[中文文档](README_zh.md) | English

[![Release](https://img.shields.io/github/v/release/cndingbo2030/dingovault?v=1.1.0)](https://github.com/cndingbo2030/dingovault/releases)
[![Test](https://github.com/cndingbo2030/dingovault/actions/workflows/test.yml/badge.svg?v=1.1.0)](https://github.com/cndingbo2030/dingovault/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cndingbo2030/dingovault?v=1.1.0)](https://goreportcard.com/report/github.com/cndingbo2030/dingovault)
[![Go mod](https://img.shields.io/github/go-mod/go-version/cndingbo2030/dingovault/main?label=go)](https://github.com/cndingbo2030/dingovault/blob/main/go.mod)
[![License](https://img.shields.io/github/license/cndingbo2030/dingovault?v=1.1.0)](https://github.com/cndingbo2030/dingovault/blob/main/LICENSE)
[![Stars](https://img.shields.io/github/stars/cndingbo2030/dingovault?v=1.1.0)](https://github.com/cndingbo2030/dingovault/stargazers)
[![Forks](https://img.shields.io/github/forks/cndingbo2030/dingovault?v=1.1.0)](https://github.com/cndingbo2030/dingovault/forks)
<!-- badge-refresh-2026-04-09 -->

**High-performance, local-first outliner with SaaS sync, built in Go.**

From source (CLI / server binary):

```bash
go install github.com/cndingbo2030/dingovault/cmd/dingovault@latest
```

Go module path: **`github.com/cndingbo2030/dingovault`**

Dingovault is a block-based Markdown vault: fast full-text search (FTS5), wikilinks, YAML frontmatter, and a clean desktop shell. The same core runs **offline** against embedded **SQLite** or **online** against your **self-hosted or managed SaaS API**—switchable via a small `storage.Provider` abstraction.

### Why teams pick Dingovault

- **Go performance that feels instant:** real-world benchmark runs are commonly around **~1ms FTS query p50** and **~0.2ms page load p50** on warm local storage (machine-dependent; run `make benchmark` to measure your hardware).
- **Secure by default:** optional **AES-256-GCM** encryption at rest (`DINGO_MASTER_KEY`) plus JWT-protected SaaS APIs for multi-user deployments.
- **Plugin-ready architecture:** backend hooks (`before:block:save`, `after:block:indexed`) and frontend plugin slots make it easy to extend without forking core logic.

---

## Maintainer & contact

| | |
|--|--|
| **Maintainer** | **cndingbo2030** |
| **Email** | **[cndingbo@outlook.com](mailto:cndingbo@outlook.com)** |
| **Repository** | [github.com/cndingbo2030/dingovault](https://github.com/cndingbo2030/dingovault) |

---

## Stack

| Layer | Technology |
|--------|------------|
| Runtime | **Go** (see `go.mod` for the exact toolchain) |
| Desktop | **[Wails v2](https://wails.io)** + webview |
| UI | **Svelte** + Vite |
| Local index | **SQLite** + **FTS5** (modernc.org/sqlite) |
| Markdown | **Goldmark** |
| SaaS API | `net/http`, **JWT** (HS256), REST under `/api/v1` |

---

## Architecture

### `storage.Provider`

All graph and UI persistence go through **`storage.Provider`** (`internal/storage/provider.go`):

- **`Store`** (`internal/storage/sqlite.go`) — local SQLite + triggers keeping FTS in sync.
- **`RemoteStore`** (`internal/storage/remote.go`) — HTTP client: `Authorization: Bearer <JWT>` on every call, mapping to the same REST routes the SaaS server exposes.

The **graph service** (`internal/graph`) and **Wails bridge** (`internal/bridge`) depend only on `Provider`, not on SQL or HTTP—so the desktop app can run in **local** or **cloud** mode without forking business logic.

### SaaS server

`cmd/dingovault` can run an HTTP server on **`DINGO_PORT`** (default **12030**), with routes registered in **`internal/server/handlers.go`**. Protected handlers read the tenant from the JWT (`sub`) and scope SQLite rows by `user_id`.

---

## Performance

- **Local FTS**: prefix-token queries over FTS5 are typically around **~1ms p50** on modest vaults after warmup.
- **Page load**: serving one page’s block tree from SQLite is typically around **~0.2ms p50** of database work on warm local cache.

Exact numbers depend on hardware, vault size, and OS cache—run `make benchmark` on your machine for a reproducible report.

**Encrypted stress + integrity:** `make benchmark-encrypted` sets `DINGO_MASTER_KEY` and runs the benchmark with `-verify` so decrypted block content is spot-checked after indexing.

---

## Schema migrations & plugins (v1.0+)

- **First launch:** If you start the desktop app with no `-notes` flag and no saved `vaultPath`, Dingovault unpacks the embedded **`demo-vault/`** into your OS cache and opens it so you can try the outliner immediately. Set **`DINGO_NO_DEMO_VAULT=1`** to disable that behavior.
- **SQLite `user_version`:** On open, `internal/storage/migrate.go` runs incremental migrations so upgrades can add tables/columns without wiping existing notes. Bump `CurrentSchemaVersion` and add a new step when the schema changes.
- **`before:block:save`:** Register on the graph service’s `bus` with `RegisterBeforeBlockSave` to mutate markdown before it is written (e.g. auto-formatting, AI hooks). Errors abort the save.
- **`after:block:indexed`:** Pub/sub topic `after:block:indexed` fires after a source is reindexed (alongside the existing file reindex topic).
- **Reference AI plugin:** `internal/plugins/summarizer` subscribes to `after:block:indexed`; when a block includes `#summarize`, it appends a generated child summary and reindexes through `storage.Provider`.
- **Desktop UI:** External scripts can call `window.__DINGOVAULT__.registerToolbarButton` / `registerSidebarSection` (see `frontend/src/pluginRegistry.js`). Missing images get a placeholder via `initImageFallback()`.

---

## Production hardening (SaaS)

| Variable | Purpose |
|----------|---------|
| **`DINGO_ENV=production`** | Enables strict JWT rules: **`DINGO_JWT_SECRET` is required** and must **not** be the built-in development default. The Docker image sets this by default. |
| **`DINGO_JWT_SECRET`** | HS256 signing key (minimum **16 characters**). Generate a long random string for production. |
| **`ALLOWED_ORIGINS`** | Optional comma-separated **exact** browser origins allowed for CORS (e.g. `https://app.example.com,http://localhost:5173`). If unset, no `Access-Control-Allow-Origin` is sent (non-browser / same-origin clients unaffected). |
| **`DINGO_MASTER_KEY`** | Optional passphrase: **AES-256-GCM** encryption for block **`content`** in SQLite (`dv1:` prefix). **FTS body search is ineffective** on ciphertext. Losing or changing the key makes encrypted rows unreadable. |
| **`DINGO_BLOB_BACKEND`** | `filesystem` (default) or `s3` / `minio` for `POST /api/v1/assets`. |
| **S3 / MinIO** | `DINGO_S3_BUCKET`, `DINGO_S3_REGION` (default `us-east-1`), optional `DINGO_S3_ENDPOINT`, `DINGO_S3_PREFIX`, **`DINGO_S3_PUBLIC_BASE`** (no trailing slash; markdown links), `DINGO_S3_USE_PATH_STYLE=1`. Auth: `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` or `DINGO_S3_ACCESS_KEY` / `DINGO_S3_SECRET_KEY`. |

### Health & metrics

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /api/v1/health` | Public | Liveness JSON `{"status":"ok"}`. |
| `GET /api/v1/sys/stats` | **JWT** | Global index stats: `blockCount`, `pageCount`, `tenantCount` (distinct `user_id` in `blocks`; tenants with no blocks are not counted). |
| `POST /api/v1/capture` | **JWT** | Quick capture: JSON `{"text":"…","sourcePath":"Inbox.md"}` (default path `Inbox.md`). Appends a bullet under the vault page and reindexes. **Requires `-notes` / vault path on the server.** |
| `POST /api/v1/assets` | **JWT** | Multipart field **`file`**. **Filesystem** mode: `vault/assets/` (needs vault path). **S3** mode: `DINGO_BLOB_BACKEND=s3` + bucket env (no local vault required for uploads). Returns `path`, `markdown`, `bytes`. |
| `GET /api/v1/graph/wiki` | **JWT** | Page-level graph: `nodes` (`id` = absolute path, `label`) and `edges` (`source` → `target`) from resolved wikilinks. **Requires vault path** for alias resolution. |

### Mobile / automation

- **Siri Shortcuts / widgets**: `POST /api/v1/capture` with a Bearer token is enough to append to an inbox page without loading the block tree.
- **CORS**: set `ALLOWED_ORIGINS` so browser-based or extension clients can call the API from your SPA origin.

### Desktop UX (Phase 14)

- **Fold**: parent blocks can be collapsed; state is stored in **`localStorage`** per page (`dingovault-collapse:<path>`), so large outlines stay manageable without changing Markdown yet.
- **Drag ⋮⋮**: reorder **sibling** blocks (same parent) via the Wails binding **`ReorderBlockBefore`** (file-backed, same rules as indent/outdent).
- **Graph** button: force-directed **page link graph** (resolved wikilinks) using **d3-force**.

### Mobile & privacy (Phase 15)

- **Responsive UI**: touch-sized controls, `16px` textarea font (reduces iOS zoom), safe-area padding, stacked toolbar on narrow screens.
- **Swipe on the left rail**: **left** = cycle TODO (same as desktop shortcut); **right** = clear block text (confirm). Gestures use the gutter so vertical scrolling in the editor stays natural.
- **Cloud assets**: enable **`DINGO_BLOB_BACKEND=s3`** to store uploads in S3-compatible storage; markdown uses **`DINGO_S3_PUBLIC_BASE`** URLs.
- **Releases**: push a tag `v1.2.3` to run **`.github/workflows/release.yml`** (Go tests, `dingovault-server-*` CLI matrix, Wails builds for Linux / macOS / Windows). Adjust runner packages if a platform fails in your fork.

---

## Usage

### Prerequisites

- **Go** (matching `go.mod`)
- **Node.js** + npm (for the frontend)
- **[Wails CLI v2](https://wails.io/docs/gettingstarted/installation)** for desktop builds

### Desktop (local SQLite)

```bash
make dev
# or
wails dev
```

Point `-notes` at your vault directory (saved in user config after first run).

### Desktop (cloud / SaaS index)

1. Run a SaaS server (see below) and obtain a JWT, e.g.  
   `POST /api/v1/auth/token` with `{"userId":"your-id"}` (dev-style; replace with real auth in production).
2. Enable cloud mode via **config** (`~/.config/dingovault/config.json` on Linux):

   ```json
   {
     "vaultPath": "/path/to/your/markdown/vault",
     "cloudMode": true,
     "cloudApiUrl": "http://127.0.0.1:12030",
     "cloudToken": "<paste JWT here>"
   }
   ```

   Or use environment variables (override config):

   - `DINGO_CLOUD_MODE=1`
   - `DINGO_CLOUD_URL=http://127.0.0.1:12030`
   - `DINGO_CLOUD_TOKEN=<jwt>`

3. Start the desktop app as usual (`wails dev` / `wails build`). The vault folder remains the **source of truth on disk**; indexing **pushes** parsed content to the API on full scan, watcher events, and edits.

### SaaS API server (CLI)

**Development** (default dev JWT secret allowed):

```bash
DINGO_SERVER=1 go run ./cmd/dingovault -db=./dingovault_saas.db
```

**Production** (strict secret):

```bash
export DINGO_ENV=production
export DINGO_JWT_SECRET='your-unique-secret-at-least-16-chars'
export ALLOWED_ORIGINS='https://your-spa.example.com'
DINGO_PORT=12030 go run ./cmd/dingovault -server -db=./dingovault_saas.db
```

Or only set **`DINGO_PORT`** (also enables HTTP):

```bash
DINGO_PORT=12030 go run ./cmd/dingovault -server -db=./dingovault_saas.db
```

### Docker (SaaS image)

The image sets **`DINGO_ENV=production`**; you **must** pass a real JWT secret at run time:

```bash
make deploy-saas
docker run --rm -p 12030:12030 \
  -e DINGO_JWT_SECRET='your-unique-secret-at-least-16-chars' \
  -e ALLOWED_ORIGINS='https://app.example.com' \
  -v dingovault-data:/data \
  dingovault-saas:latest
```

Health: `GET http://localhost:12030/api/v1/health`  
Stats (JWT): `GET http://localhost:12030/api/v1/sys/stats`

For **`/api/v1/capture`**, **`/api/v1/assets`**, and **`/api/v1/graph/wiki`**, run the binary with **`-notes` / a mounted vault directory** (same as non-Docker SaaS). The stock Docker example is API-oriented; mount your Markdown tree and pass `-notes=/vault` (or extend the image entrypoint) if you need those routes.

---

## Makefile targets

| Target | Purpose |
|--------|---------|
| `make dev` | Wails dev (desktop) |
| `make build` / `make release` | Production Wails build |
| `make benchmark` | Index + FTS stress benchmark |
| `make benchmark-encrypted` | Same with `DINGO_MASTER_KEY` + `-verify` (integrity spot-check) |
| `make fmt` | `go fmt ./...` |
| `make lint-frontend` | Svelte / TS checks |
| `make dist` | Zip app bundle + help vault |
| **`make deploy-saas`** | Build **`dingovault-saas:latest`** Docker image |
| **Git tag `v*`** | Triggers **release** workflow: tests, cross-platform **server** binaries, **Wails** desktop artifacts (see `.github/workflows/release.yml`). |

---

## API overview (v1)

**Public:** `GET /api/v1/health`, `POST /api/v1/auth/token`  

**Protected** (`Authorization: Bearer …`): blocks, pages, search, backlinks, alias resolve, `POST /api/v1/pages/reindex`, **`GET /api/v1/sys/stats`**, **`POST /api/v1/capture`**, **`POST /api/v1/assets`**, **`GET /api/v1/graph/wiki`**, etc. See `internal/server/handlers.go` and `internal/server/phase14_handlers.go`.

---

## Repository metadata

Upstream: **[github.com/cndingbo2030/dingovault](https://github.com/cndingbo2030/dingovault)**  
Also see `project.meta.json`.

---

## License

This project is released under the **MIT License** — see [`LICENSE`](LICENSE).
