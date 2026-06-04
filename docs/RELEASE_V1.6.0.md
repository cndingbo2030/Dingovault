# Dingovault v1.6.0 Release Notes

v1.6.0 turns Dingovault into a tighter thinking-doing workspace: outline thinking, mind-map restructuring, terminal execution, and results written back into Markdown.

## Highlights

- **Live page Mind Map:** renders the same `PageBlock` tree returned by `GetPage`, not a separate graph engine. It supports pan, wheel zoom, branch collapse, top-level branch coloring, inline node editing, child creation, drag re-parenting, SVG/PNG export, and Markdown outline copy.
- **Real terminal blocks:** the console now has PTY-backed xterm.js sessions with streamed output over Wails events, stdin, resize, close, and multiple ephemeral terminal blocks.
- **Thinking-doing loop:** outline and mind-map blocks can be explicitly run as terminal commands, then stdout is appended back to the page as a child Markdown block with terminal metadata.
- **Wave interop:** the current vault can be handed off to Wave Terminal when installed, while Dingovault remains the local-first thinking layer.
- **Architecture and tests:** new backend primitives for child insertion, subtree movement, PTY session lifecycle, and command-result capture are covered by focused tests.

## Version Matrix

| Component | Version |
| --- | --- |
| Desktop / Wails product | 1.6.0 |
| Frontend package | 1.6.0 |
| Go embedded version | 1.6.0 |
| Android shell version | 1.6.0 |

## Expected Release Artifacts

The existing release workflow is triggered by pushing a `v*` tag. For `v1.6.0`, it should produce:

- macOS desktop archives for Apple Silicon and Intel.
- Windows desktop installer.
- Linux desktop archive and server binary.
- Android APK, AAB, and AAR artifacts.
- GHCR image `ghcr.io/cndingbo2030/dingovault:v1.6.0`.
- GitHub Packages SDK stub from `sdk/`.

## Notes

- Terminal sessions are intentionally ephemeral and do not persist secrets.
- Block text is never auto-executed; running a block requires an explicit user action and higher-risk commands require confirmation.
- Markdown files on disk remain the source of truth for all outline, mind-map, and command-result mutations.
