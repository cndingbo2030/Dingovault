# Dingovault

**High-performance, local-first outliner with SaaS sync, built in Go.**

Dingovault is a block-based Markdown vault: fast full-text search (FTS5), wikilinks, YAML frontmatter, and a clean desktop shell. The same core runs **offline** against embedded **SQLite** or **online** against your **self-hosted or managed SaaS API**—switchable via a small `storage.Provider` abstraction.

---

## Maintainer & contact

| | |
|--|--|
| **Maintainer** | **cndingbo2030** |
| **Email** | **[cndingbo@outlook.com](mailto:cndingbo@outlook.com)** |
| **Repository** | [github.com/cndingbo2030/Dingovault](https://github.com/cndingbo2030/Dingovault) |

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

## Production hardening (SaaS)

| Variable | Purpose |
|----------|---------|
| **`DINGO_ENV=production`** | Enables strict JWT rules: **`DINGO_JWT_SECRET` is required** and must **not** be the built-in development default. The Docker image sets this by default. |
| **`DINGO_JWT_SECRET`** | HS256 signing key (minimum **16 characters**). Generate a long random string for production. |
| **`ALLOWED_ORIGINS`** | Optional comma-separated **exact** browser origins allowed for CORS (e.g. `https://app.example.com,http://localhost:5173`). If unset, no `Access-Control-Allow-Origin` is sent (non-browser / same-origin clients unaffected). |

### Health & metrics

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /api/v1/health` | Public | Liveness JSON `{"status":"ok"}`. |
| `GET /api/v1/sys/stats` | **JWT** | Global index stats: `blockCount`, `pageCount`, `tenantCount` (distinct `user_id` in `blocks`; tenants with no blocks are not counted). |

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

**Public:** `GET /api/v1/health`, `POST /api/v1/auth/token`  

**Protected** (`Authorization: Bearer …`): blocks, pages, search, backlinks, alias resolve, `POST /api/v1/pages/reindex`, **`GET /api/v1/sys/stats`**, etc. See `internal/server/handlers.go`.

---

## Repository metadata

Upstream: **[github.com/cndingbo2030/Dingovault](https://github.com/cndingbo2030/Dingovault)**  
Also see `project.meta.json`.

---

## License

This project is released under the **MIT License** — see [`LICENSE`](LICENSE).
