# Dingovault

**High-performance, local-first outliner with SaaS sync, built in Go.**

Dingovault is a block-based Markdown vault: fast full-text search (FTS5), wikilinks, YAML frontmatter, and a clean desktop shell. The same core runs **offline** against embedded **SQLite** or **online** against your **self-hosted or managed SaaS API**—switchable via a small `storage.Provider` abstraction.

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

- **Local FTS**: prefix-token queries over FTS5 are typically **sub‑millisecond** on modest vaults after warmup; the included benchmark (`make benchmark`) stress-tests large trees.
- **Page load**: serving a page’s block tree from SQLite is usually **well under a millisecond** of database work (UI and disk I/O add latency on top).

Exact numbers depend on hardware, vault size, and OS cache—run `make benchmark` on your machine for a reproducible report.

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

```bash
# Listens on :12030, isolated DB dingovault_saas.db in cwd unless you pass -db
DINGO_SERVER=1 go run ./cmd/dingovault -db=./dingovault_saas.db
```

Or only set **`DINGO_PORT`** (also enables HTTP):

```bash
DINGO_PORT=12030 go run ./cmd/dingovault -server -db=./dingovault_saas.db
```

Set a strong secret in production:

```bash
export DINGO_JWT_SECRET='at-least-16-chars-long'
```

### Docker (SaaS image)

```bash
make deploy-saas
docker run --rm -p 12030:12030 -v dingovault-data:/data dingovault-saas:latest
```

Health check: `GET http://localhost:12030/api/v1/health`

---

## Makefile targets

| Target | Purpose |
|--------|---------|
| `make dev` | Wails dev (desktop) |
| `make build` / `make release` | Production Wails build |
| `make benchmark` | Index + FTS stress benchmark |
| `make fmt` | `go fmt ./...` |
| `make lint-frontend` | Svelte / TS checks |
| `make dist` | Zip app bundle + help vault |
| **`make deploy-saas`** | Build **`dingovault-saas:latest`** Docker image |

---

## API overview (v1)

Public: `GET /api/v1/health`, `POST /api/v1/auth/token`  
Protected (`Authorization: Bearer …`): blocks, pages, search, backlinks, alias resolve, `POST /api/v1/pages/reindex` (markdown body), etc. See `internal/server/handlers.go`.

---

## Repository

Upstream: **[github.com/cndingbo2030/Dingovault](https://github.com/cndingbo2030/Dingovault)**  
Project metadata: `project.meta.json`.

---

## License

See repository files for license terms (add a `LICENSE` file if not yet present).
