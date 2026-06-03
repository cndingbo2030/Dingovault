# Dingovault v1.5.0 Release Notes

Date: 2026-06-03

## Summary

v1.5.0 is a desktop-product release. It moves Dingovault toward an Obsidian-grade, office-ready workspace while keeping the local-first Markdown model intact.

## Highlights

- Obsidian/JetBrains-inspired desktop shell with compact titlebar, activity rail, files pane, right inspector, standalone settings, and workspace console.
- Page graph rebuilt for real interaction: wheel zoom, panning, node dragging, reset/zoom controls, hover emphasis, and better label density.
- Vault file browser can list Markdown, Office/WPS documents, PDFs, images, and CAD/DWG-style files.
- Markdown opens in-app; supported non-Markdown files open through the OS default app.
- Editor polish: quieter focus rings, clearer wikilink chips, less intrusive tag suggestions, and stale block recovery after reindexing.
- AI Chat, backlinks, and related-content panels are denser and clearer.

## Version Matrix

| Component | Version |
| --- | --- |
| Desktop / Wails product | 1.5.0 |
| Frontend package | 1.5.0 |
| Go embedded version | 1.5.0 |
| Android shell version | 1.5.0 |
| SDK package | tag-derived, published from `sdk/` on `v*` tags |

## Expected Release Artifacts

The existing release workflow is triggered by pushing a `v*` tag. For `v1.5.0`, it should produce the same artifact classes as previous releases:

- macOS desktop app artifacts for Apple Silicon and Intel naming paths.
- Windows 64-bit installer artifact.
- Linux desktop artifact.
- Android universal install artifacts from the Android shell.
- SaaS/server binaries.
- GHCR image `ghcr.io/cndingbo2030/dingovault:v1.5.0`.
- GitHub Packages npm stub `@cndingbo2030/dingovault-sdk`.

## Validation

Local validation performed before tagging:

- `npm run lint`
- `npm run build`
- `go test ./...`
- `/Users/dingbo/go/bin/wails build -clean`

Known non-blocking Wails binding warning:

- `Not found: time.Time`

The warning was already present in previous builds and does not block packaging.

## Upgrade Notes

- Existing Markdown vault files remain the source of truth.
- The app may rebuild the local SQLite index on launch or after the health reset action.
- If an indexed block ID becomes stale after external edits or reindexing, the editor now refreshes the page instead of showing a raw `lookup block: block not found` error.

## Commercial Readiness

This release keeps the core AGPL-licensed and local-first while laying groundwork for commercial workflows: denser office UI, stronger file handling, AI settings, release artifacts, and a clearer settings surface for future account/license/sync features.
