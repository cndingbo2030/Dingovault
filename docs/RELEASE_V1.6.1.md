# Dingovault v1.6.1 Release Notes

v1.6.1 is a security and capability patch for the v1.6.0 thinking-doing loop. It hardens block-derived command execution, makes command results queryable knowledge, and improves large mind-map and export behavior.

## Security Fix

- Block-derived terminal commands are now classified by a shared frontend/backend read-only allowlist.
- Commands containing shell control syntax or non-read-only operations require explicit user confirmation.
- The backend now refuses unconfirmed non-read-only block commands even if a frontend bug or compromised UI tries to call the bridge directly.
- Commands typed directly into the console remain explicit user intent and continue to run through the existing vault-scoped console path.

## Capability Patch

- Terminal result blocks now write structured Markdown with queryable block properties:
  - `source:: terminal`
  - `exitCode:: <code>`
  - `ranAt:: <timestamp>`
  - `durationMs:: <milliseconds>`
  - `command:: <command>`
- Command output is fenced as `text`, so output remains readable and does not pollute metadata parsing.
- Users can query command history with `source:terminal` and failed runs with `exitCode:1`.
- Large page mind maps now cull minor deep labels until zoom/hover and auto-collapse deep branches on first open.
- Mind-map SVG/PNG export now inlines themed colors and a CJK-capable font stack.
- xterm is lazy-loaded when a terminal opens; the initial frontend bundle is now below the Vite 500KB warning threshold.

## Version Matrix

| Component | Version |
| --- | --- |
| Desktop / Wails product | 1.6.1 |
| Frontend package | 1.6.1 |
| Go embedded version | 1.6.1 |
| Android shell version | 1.6.1 |

## Expected Release Artifacts

The existing release workflow is triggered by pushing the `v1.6.1` tag. It should produce:

- macOS desktop archives for Apple Silicon and Intel.
- Windows desktop installer.
- Linux desktop archive and server binary.
- Android APK, AAB, and AAR artifacts.
- GHCR image `ghcr.io/cndingbo2030/dingovault:v1.6.1`.
- GitHub Packages SDK stub from `sdk/`.

## Notes

- Markdown files on disk remain the source of truth for outline, mind-map, and command-result mutations.
- Terminal sessions remain ephemeral and do not persist secrets.
- Stored commands are still treated as untrusted data when re-used by future UI flows.
