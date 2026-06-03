# Dingovault Architecture

## System Overview

Dingovault combines a local-first Markdown workflow with a structured graph index and a desktop UI.
The same backend stack can also run in SaaS/server mode via `cmd/dingovault -server`.

Core layers:

1. **Markdown Vault**: source of truth files on disk.
2. **Parser + Graph Service**: transforms Markdown into block/page/link structures.
3. **Storage Provider**: persists and queries index data (SQLite locally, remote provider for SaaS).
4. **API/Bridge Surface**: Wails bridge for desktop and HTTP handlers for server mode.
5. **Frontend (Svelte)**: consumes bridge/API and renders editor, graph, search, and operations.

## Data Flow

### 1) Markdown -> Goldmark AST

- File scanner watches the vault directory and triggers indexing.
- Parser engine reads Markdown and builds an AST (Goldmark-backed pipeline).
- Parse output includes:
  - block hierarchy (IDs, parent-child, outline levels, line ranges),
  - page properties/frontmatter,
  - wikilinks and tags.

### 2) Goldmark AST -> SQLite Index

- `internal/graph.Service` applies parse results through the storage provider.
- Writes are source-file scoped: replace all indexed entities for one file atomically.
- Index tables include blocks, wikilinks, tags, page aliases, and page properties.
- SQLite runs with WAL mode for concurrent read-heavy access and resilient writes.

### 3) SQLite -> Wails Bridge / HTTP API

- Desktop mode: Wails bridge (`internal/bridge`) exposes backend operations to frontend JS.
- Server mode: HTTP handlers (`internal/server`) expose `/api/v1` endpoints.
- Both surfaces rely on the same graph/storage services, so behavior stays aligned.

### 4) Bridge/API -> Frontend

- Frontend invokes generated Wails bindings or REST endpoints.
- Returned data powers outline rendering, backlinks, queries, graph visualizations, and plugins.

## SaaS Provider Pattern

The key abstraction is `internal/storage.Provider`.

Why it matters:

- **Decouples graph logic from persistence details**: graph/parser code does not care whether data is local SQLite or remote.
- **Enables progressive SaaS adoption**: the same high-level operations run against `Store` (local) or `RemoteStore` (HTTP-backed).
- **Keeps call sites stable**: bridge and server handlers call provider methods, not SQL directly.

Provider responsibilities include:

- block reads and queries,
- full-source replacement and deletion,
- alias/property resolution,
- aggregated index stats,
- wiki graph extraction.

Current implementations:

- **`Store`**: local SQLite implementation.
- **`RemoteStore`**: SaaS API client implementation.

## Runtime Modes

- **Desktop mode (default)**:
  - local DB file,
  - file watching + continuous indexing,
  - Wails bridge UI.
- **Server mode (`-server`)**:
  - HTTP API with JWT auth paths,
  - optional vault scanning when `-notes` is provided,
  - suitable for shared SaaS-style deployments.

## Desktop Workspace Architecture

The v1.5.0 desktop shell is intentionally closer to an IDE than a marketing site. The UI is organized around reusable work surfaces:

- **Activity rail**: primary mode switches for files, graph, console, AI, and settings.
- **Files pane**: `ListVaultFiles` returns supported vault files with type metadata; Markdown routes to `GetPage`, while non-Markdown uses `OpenVaultFile` and the OS default handler.
- **Editor pane**: block operations remain file-backed through graph service mutations (`UpdateBlock`, `InsertBlockAfter`, indent/outdent, slash operations, reorder).
- **Inspector pane**: backlinks, semantically related blocks, and AI Chat share the current page context.
- **Graph pane**: `GetWikiGraph` produces page-level nodes and links; the frontend applies force layout, pan, wheel zoom, hover emphasis, and node dragging.
- **Workspace console**: bridge commands run inside the vault root with bounded output and tests around command execution.

This keeps the desktop UI dense and operational while preserving Markdown files as the source of truth.

## Migration and Integrity

- SQLite schema uses `PRAGMA user_version` and incremental migrations.
- On startup, migrations run to the expected schema version.
- This preserves user data across releases while enabling new features safely.

## Operational Debug Surface

`cmd/dingovault` provides debug subcommands to inspect runtime health:

- `debug graph` for index summary counts,
- `debug doctor` for filesystem, SQLite WAL, and JWT secret diagnostics,
- `debug migrate-redo` for migration replay in development.

These commands are intentionally lightweight to speed up local troubleshooting and CI triage.
