# Contributing to Dingovault

Thank you for helping improve Dingovault. This document covers **local development** and **plugins** that extend the app without forking the core UI.

## Building & testing

- **Desktop (Wails):** `make dev` or `wails dev` after `npm ci` in `frontend/`.
- **Go:** `go test ./...` from the repository root.
- **Frontend checks:** `make lint-frontend`.
- **Benchmarks:** `make benchmark` and `make benchmark-encrypted` (see main README).

Release builds embed the app version via `-ldflags` (see `Makefile` `APP_VERSION` and `internal/version`).

## Built-in Demo Vault

First-time desktop launches with **no** `-notes` flag and **no** saved `vaultPath` materialize the **Demo Vault** from the embedded `demo-vault/` tree into the OS cache (`demo-vault` under the user cache directory, next to a `.bundle-version` marker).

- To force the old “must pass `-notes`” behavior in automation, set **`DINGO_NO_DEMO_VAULT=1`**.
- Bump **`internal/onboarding.DemoBundleVersion`** when you change demo content so existing installs refresh the cache.

## Go plugins: event bus & save hooks

The graph layer uses an internal **`bus.Bus`** (`internal/bus`) for cross-cutting concerns.

### `before:block:save` (interceptor chain)

This is **not** a pub/sub topic. Register a hook on the **same** `*bus.Bus` instance attached to `*graph.Service`:

```go
b := bus.New()
graphSvc.SetBus(b)

b.RegisterBeforeBlockSave(func(ctx context.Context, d bus.BeforeBlockSaveData) (string, error) {
    // Return transformed markdown before it hits disk / SQLite (auto-format, AI rewrite, etc.).
    return strings.TrimSpace(d.Content), nil
})
```

- Hooks run in **registration order**. Each handler receives the **latest** string from the previous hook.
- Return **`("", err)`** to **abort** the save (the error is surfaced to the client as a save failure).
- See `BeforeBlockSaveData` in `internal/bus/bus.go` for path, block ID, and content fields.

### `after:block:indexed` (pub/sub)

After a source file is replaced in the index, the service publishes **`after:block:indexed`** (`internal/bus/topics.go`). Subscribe on the same `*bus.Bus` with **`b.Subscribe(bus.TopicAfterBlockIndexed, func(ctx context.Context, payload any) { ... })`** (see `internal/bus/bus.go`). The payload type is `*bus.AfterBlockIndexedPayload`.

Keep plugin-like code in a **separate module or build tag** if it should not ship in the default binary.

## Desktop UI plugins: Svelte slots

The web frontend exposes a small **`window.__DINGOVAULT__`** API (`frontend/src/pluginRegistry.js`), initialized from `main.js`:

- **`registerToolbarButton({ id, label, run })`** — append a button to the main toolbar (`run` is a no-arg function).
- **`registerSidebarSection({ id, title, body })`** — append a card below backlinks.

External scripts can be loaded by advanced users (e.g. via a desktop webview preload or future official loader). Keep IDs unique to avoid duplicate entries.

Image elements that fail to load are swapped for a **placeholder** when `initImageFallback()` runs (also from `main.js`).

## Pull requests

- Keep changes focused; match existing **Go** and **Svelte** style.
- Run **`go test ./...`** and **`npm run build`** (in `frontend/`) before submitting.

## License

By contributing, you agree your contributions are licensed under the same terms as this repository — **AGPL-3.0** (see [`LICENSE`](LICENSE)).
