# Changelog

All notable user-facing changes for Dingovault are listed here. We describe what you gain in daily use, not internal implementation details.

## Repository — AGPL-3.0, GHCR, GitHub Packages npm (2026-04-10)

- **License:** project is now **AGPL-3.0** (see root `LICENSE`).
- **Container:** tagged releases build and push **`ghcr.io/cndingbo2030/dingovault:<tag>`** and **`:latest`** (SaaS server `Dockerfile`).
- **npm stub:** **`@cndingbo2030/dingovault-sdk`** published from **`sdk/`** to **GitHub Packages** on each **`v*`** tag (placeholder for future plugin APIs).

---

## v1.4.1 — Release workflow healing & semantic download names (2026-04-10)

### CI / Android

GitHub Actions **release** builds set **ANDROID_HOME**, **ANDROID_NDK_HOME**, **NDK_HOME**, pin **cmdline-tools** for `setup-android`, install the **NDK** via the action’s package list, add a **`ndk-bundle` symlink** for gomobile, and run **`gomobile init` only after** the NDK is present. **Gradle `gradlew`** is explicitly **`chmod +x`**. The module pins **`golang.org/x/mobile`** (via `tools/tools.go`) so **gobind** can resolve **`golang.org/x/mobile/bind`** on the runner.

### Downloads

Release assets use **long, self-describing filenames** (for example **Apple Silicon** vs **Intel**, **Linux Desktop** `.tar.gz`, **Windows 64-bit Installer**, **Android Mobile Phone Tablet** `.apk`) so the right file is obvious. See the **`Makefile`** `RELEASE_*` variables and **`make release-names`** for the canonical list.

### UI

**Viewport-fit=cover**, stronger **safe-area** padding for notches and **Android gesture bars**, and a **chrome layout** mode that adjusts the **header** for tablet-landscape vs phone portrait.

---

## v1.4.0 — Android build pipeline & mobile-ready UI (2026-04-10)

### Android (gomobile)

Tagged releases now attach a **`.aar`** built with **gomobile bind** from `cmd/dingovault-android/mobile`, plus a minimal shell app that produces a **universal APK** and **AAB** (CI uses the debug keystore for attachable artifacts — sign release builds for Play yourself). The library exposes version and a **scoped-storage vault path** helper for hosts that pass `Context.getExternalFilesDir(null)`.

### Desktop / webview UI

The Svelte shell uses **dynamic viewport units** where it matters for mobile height, **48px** minimum targets on primary controls, a **bottom tab bar** on narrow screens to switch between outline, semantic related, and backlinks/AI, and an automatic **three-column** layout on wide viewports.

### Tooling

- `golangci-lint` now includes **ineffassign** alongside govet, staticcheck, and revive.

---

## v1.3.2 — S3-compatible sync & richer LAN pairing (2026-04-10)

### Object storage sync

Sync your vault to **Amazon S3** or any **S3-compatible** endpoint (for example MinIO) with the same bidirectional Markdown rules as WebDAV: newer-or-larger wins, and true conflicts become a `*.conflict.md` file next to the original.

### LAN pairing carries more settings

When you pair with a 4-digit PIN on a trusted Wi‑Fi, the other device can now receive **WebDAV and S3** fields you have configured, so multi-cloud setups propagate in one step.

---

## v1.3.1 — Stable sync & LAN discovery (2026-04-10)

### Keep the same vault on every device

Connect Dingovault to a **WebDAV** folder (Nextcloud, ownCloud, a NAS, or any standards-compliant server). One action syncs your Markdown notes both ways. If two copies diverge in meaningful ways, Dingovault keeps **both**: your version is saved next to the main file as a `*.conflict.md` sibling so nothing is silently lost.

### Find teammates on Wi‑Fi

On a trusted local network, Dingovault can **announce itself** and **discover other desktops** running the app. Pair with a short **4-digit PIN** to copy WebDAV sync settings from one machine to another—handy when you would rather not re-type URLs and passwords.

### Polish

- Cleaner AI provider setup code and small parser/readability tweaks.
- `gofmt -s` and linter-driven cleanups for a smoother Go Report Card experience.

---

## v1.3.0 — AI writing & smart links (2026-04-10)

### Real-time AI writing

See the assistant’s words appear as they are composed when you use inline AI on a bullet. The experience feels like a collaborator typing beside you instead of waiting for a single block of text at the end.

### Smarter answers from your vault

Ask questions in the AI chat sidebar and get answers that take the **current page** and **related notes** into account. Dingovault surfaces passages from elsewhere in your vault that are genuinely similar in meaning—even when you never linked them—so follow-ups and research stay grounded in what you already wrote.

### Instant brain link

The **Semantically related** panel suggests other blocks that match the spirit of what you are reading. It helps you rediscover past notes, connect ideas, and avoid duplicate work without manually hunting through filenames.

### Tag suggestions that understand content

When you edit a block, suggested **#tags** reflect how your note reads, not just spelling. Picking one is a fast way to keep tagging consistent across the vault.

### Graph: meaning, not only links

The page graph can show **semantic connections** between notes—visual hints for “these pages belong together” based on content similarity, alongside classic wikilink edges.

### Calmer when the AI server drops

If the local model or API stops mid-stream, the app shows a clear **connection lost** message, restores your text, and stops spinning—instead of leaving the editor stuck.

### Under the hood (for the curious)

- Stress-tested concurrent search while hundreds of embeddings are written, to keep the database responsive during heavy indexing.
- Tighter code structure across AI, search, and graph paths for long-term stability.

---

## Earlier releases

See [GitHub Releases](https://github.com/cndingbo2030/dingovault/releases) for prior binaries and notes.
