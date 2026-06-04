# Dingovault Architecture

## System Overview

Dingovault combines a local-first Markdown workflow with a structured graph index and a desktop UI.
The same backend stack can also run in SaaS/server mode via `cmd/dingovault -server`.

Core layers:

1. **Markdown Vault**: source of truth files on disk.
2. **Parser + Graph Service**: transforms Markdown into block/page/link structures.
3. **Storage Provider**: persists and queries index data (SQLite locally, remote provider for SaaS).
4. **API/Bridge Surface**: Wails bridge for desktop and HTTP handlers for server mode.
5. **Terminal Layer**: ephemeral PTY sessions for local execution inside the vault root.
6. **Frontend (Svelte)**: consumes bridge/API and renders editor, graph, terminal, search, and operations.

## Data Flow

### 1) Markdown -> Goldmark AST

- File scanner watches the vault directory and triggers indexing.
- Parser engine reads Markdown and builds an AST (Goldmark-backed pipeline).
- Parse output includes:
  - block hierarchy (IDs, parent-child, outline levels, line ranges),
  - page properties/frontmatter,
  - block properties from an explicit `properties:` region containing whole-line `key:: value` entries,
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

The current desktop shell is intentionally closer to an IDE than a marketing site. The UI is organized around reusable work surfaces:

- **Activity rail**: primary mode switches for files, graph, console, AI, and settings.
- **Files pane**: `ListVaultFiles` returns supported vault files with type metadata; Markdown routes to `GetPage`, while non-Markdown uses `OpenVaultFile` and the OS default handler.
- **Editor pane**: block operations remain file-backed through graph service mutations (`UpdateBlock`, `InsertBlockAfter`, indent/outdent, slash operations, reorder).
- **Inspector pane**: backlinks, semantically related blocks, and AI Chat share the current page context.
- **Graph pane**: `GetWikiGraph` produces page-level nodes and links; the frontend applies force layout, pan, wheel zoom, hover emphasis, and node dragging.
- **Workspace console**: keeps one-shot commands for quick checks, and hosts ephemeral PTY terminal blocks for interactive work.

This keeps the desktop UI dense and operational while preserving Markdown files as the source of truth.

## Terminal Layer and Thinking-Doing Loop

Dingovault's terminal layer is intentionally local-first and ephemeral. Markdown remains the durable source of truth; terminal sessions are working surfaces.

Backend:

- `internal/terminal.Manager` owns PTY sessions created with `github.com/creack/pty`.
- Session cwd is scoped to the vault root by default; vault-relative cwd values are resolved and paths that escape the root are rejected.
- PTY output is streamed through Wails events:
  - `terminal-session-started`,
  - `terminal-output`,
  - `terminal-exit`,
  - `terminal-error`.
- Wails bridge methods expose session start, stdin write, resize, close, and PTY-backed block command execution.
- `RunVaultCommand` remains available for bounded one-shot quick commands.

Frontend:

- `frontend/src/Terminal.svelte` wraps `@xterm/xterm` and `@xterm/addon-fit`.
- `WorkspaceConsole.svelte` treats terminal sessions as Wave-style blocks displayed as tabs. Sessions are not persisted and should not store secrets.
- Quick commands still render as compact history cards beside the terminal area.
- Wave Terminal interop is optional: `OpenInWave` detects `wave`/`waveterm` or macOS `open -a Wave`, then opens the vault cwd when available.

Loop workflows:

1. **Run block as command**: the user explicitly clicks a terminal action on an outline or mind-map node. Dingovault shows the exact command and confirms anything that is not a simple read-only command. The backend runs the command in a PTY, streams output to the console, and appends an immutable child Markdown history block with `properties:`, `runId::`, `source:: terminal`, `exitCode::`, `ranAt::`, `durationMs::`, `command::`, and fenced `text` output. Re-running the same command appends another explicit history block rather than replacing prior evidence; each result block is safe to delete by hand. Because these are block properties, `QueryBlocks` can find command history with queries like `source:terminal` or failures with `exitCode:1`.
2. **Run in node context**: the user opens a terminal from an outline or mind-map node. The node text is treated as a possible path/project hint; otherwise the current page folder is used. The terminal opens scoped to that cwd without executing anything automatically.
3. **Think -> restructure -> execute -> record**: users can plan in the outline, view/restructure the same page as a mind map, run selected execution steps in the console, then keep the result under the originating block so later graph/search/AI flows can reason over the outcome.

Run history read path:

- The sidebar Run history inspector uses the existing `QueryBlocks("source:terminal")` path, then filters by current page, vault scope, or non-zero `exitCode` in the frontend. It does not introduce a second query engine. Rows navigate back to the source block via the result block's `parentId`, and re-run uses the same `RunBlockCommand(blockID, command, cwd, confirmed)` bridge method plus the same frontend `classifyCommand` confirmation flow. Mind-map nodes derive their green/red status glyph from terminal-result children in the same `GetPage` tree.

Guardrail:

- Block content is data, not trusted code. Dingovault never auto-executes a block during navigation, rendering, indexing, or mind-map layout. Execution requires a user click, and non-read-only commands require a confirmation containing the exact command text.

Trust boundaries:

- `RunVaultCommand` executes commands the user types directly into the console; that path represents explicit user intent and remains an arbitrary command runner scoped to the vault root. `RunBlockCommand` executes text sourced from Markdown blocks, sync, AI output, or shared vault content; that text is untrusted data and is gated by the shared frontend/backend `ClassifyCommand` rules plus an explicit confirmation flag before the backend will execute anything non-read-only.

Block property syntax:

- Dingovault indexes block properties only inside an explicit `properties:` region in a block. Property lines use `key:: value`, continue until a non-property line, and are ignored inside fenced code blocks. Ordinary prose containing `::`, URLs, ratios, inline code, or CJK full-width punctuation is not treated as metadata. Reindexing reads Markdown into SQLite only; it does not reformat synced Markdown, so WebDAV/S3 sync does not create conflict churn for property-looking prose.

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
