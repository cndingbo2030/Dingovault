# Dingovault Upgrade Plan: Thinking-Doing Loop

## Executive Position

Dingovault should become the local-first workspace where a user can think in an outline, restructure that thinking as a mind map, execute work in a terminal, and write the result back into the same notes. Obsidian owns knowledge navigation but not execution. Wave owns terminal execution but not durable knowledge structure. Dingovault should own the loop between them.

The core architectural decision is to reuse existing Markdown-backed block data. The mind-map view should not introduce a second graph engine or second source of truth. It should be a second visual projection of the same `PageBlock` tree already used by the outliner.

## 1. Validated Code Model

### Source of Truth

The product assumption is correct: Markdown files on disk remain the source of truth. The backend resolves vault paths, reads indexed blocks from storage, and returns block data through the Wails bridge.

Validated files:

- `internal/bridge/app.go`
  - `GetPage(path)` resolves a Markdown vault path, loads domain blocks through `store.ListDomainBlocksBySourcePath`, and returns `buildPageTree(blocks)`.
  - `UpdateBlock`, `InsertBlockAfter`, `IndentBlock`, `OutdentBlock`, and `ReorderBlockBefore` delegate to `graph.Service`, then invalidate the page cache.
  - `GetWikiGraph()` returns a page/link graph from storage, not a block tree.
- `internal/bridge/page.go`
  - `PageBlock` embeds `domain.Block`.
  - `PageBlock.Children []PageBlock` is already serialized as `children`.
  - `buildPageTree` groups blocks by `ParentID`, sorts siblings by source line, and recursively builds a tree.
- `frontend/src/OutlineNode.svelte`
  - The recursive renderer consumes `node.children`.
  - It already supports collapse, selection, sibling drag reorder, indent/outdent, slash commands, TODO cycling, wiki navigation, and inline AI.
- `frontend/src/App.svelte`
  - `roots` are rendered by recursive `OutlineNode`.
  - `graphOpen` switches the center panel from outline to `PageGraph`.
  - `WorkspaceConsole` is mounted as the bottom execution panel.
- `frontend/src/PageGraph.svelte`
  - This is a d3-force relationship graph based on `{nodes, edges}` from `GetWikiGraph`.
  - It already has pan, zoom, node drag, degree sizing, semantic edge overlay, and force layout.
  - It is not a mind-map view and should not be repurposed for the outline tree.
- `internal/bridge/console.go`
  - `RunVaultCommand` is currently a one-shot command runner: 45 second timeout, merged stdout/stderr, returns after process completion.
  - It is implemented on `bridge.App`, but physically lives in `console.go`, not `app.go`.
- `frontend/src/WorkspaceConsole.svelte`
  - The current console calls `RunVaultCommand`, stores command history, and has a Wave handoff preset: `open -a Wave .`.
- `internal/bus/bus.go`, `internal/bus/topics.go`, `internal/graph/update_block.go`
  - Backend hooks exist for `before:block:save` and `after:block:indexed`.
  - The current plugin surface is useful, but it is still an in-process backend hook plus a simple frontend global API, not a full third-party plugin runtime.

### Key Insight

The existing outline tree is the correct data source for a mind-map view:

```text
Markdown file on disk
  -> parser/indexer
  -> SQLite/FTS block rows
  -> GetPage(path)
  -> PageBlock[] tree
  -> OutlineNode visual projection
  -> MindMapView visual projection
```

The relationship graph stays separate:

```text
Indexed pages and wikilinks
  -> GetWikiGraph()
  -> PageGraph d3-force relationship projection
```

Therefore:

- Do not build a new graph engine for the mind map.
- Do not store mind-map nodes separately.
- Do not fork the block model.
- Do not use the wiki graph as the mind-map model.
- Add a view-mode projection over `PageBlock[]`, preserving the existing edit operations.

### Assumptions That Need Correction

- `RunVaultCommand` is a bridge method, but its implementation is in `internal/bridge/console.go`, not `internal/bridge/app.go`.
- `PageGraph.svelte` already has pan, zoom, and node drag. The issue is not lack of graph interaction in the current code; the issue is that it is the wrong visualization for outline restructuring.
- There is no direct "append child block" bridge method today. `InsertBlockAfter` inserts a sibling. For "command output becomes child block", we need an explicit child append operation or a safe composite operation.
- The frontend plugin API is currently minimal: toolbar/sidebar registration. The backend hook system exists, but the product should not assume a mature Obsidian-like plugin system yet.

## 2. Terminal Strategy

### Option A: Real PTY Terminal Block

Use `creack/pty` on the Go side and `xterm.js` on the Svelte side. Each terminal block/session would be a real shell with streaming output, resize support, stdin, process lifecycle, and terminal escape handling.

Effort:

- High.
- Requires backend session registry, PTY lifecycle, stream events, resize events, cancellation, cleanup, security boundaries, and cross-platform behavior.
- Windows requires ConPTY support and careful testing.

Risk:

- High for a solo founder.
- Easy to sink time into terminal edge cases instead of the product loop.
- CI and packaging complexity increase.

Payoff:

- Very high.
- This is the strongest long-term differentiation from Obsidian.
- Enables real interactive workflows: REPLs, long-running dev servers, `vim`, `ssh`, test watchers, package installers, and AI agents.

Verdict:

- Make this the fast-follow once the product loop is proven.

### Option B: Wave Interop via CLI Handoff and Shared File Protocol

Keep Wave as an external terminal and provide a structured handoff: open Wave at the vault, pass selected page/block context, write command output to a known file, then let Dingovault ingest that file back into notes.

Effort:

- Low to medium.
- The current console already has `open -a Wave .`.
- A simple protocol could use `.dingovault/run-requests/*.json` and `.dingovault/run-results/*.md`.

Risk:

- Medium.
- Depends on Wave being installed and on its CLI/app behavior.
- The user experience is split across two apps.
- Dingovault loses the sense of owning the loop.

Payoff:

- Medium.
- Useful for power users who already like Wave.
- Good bridge during early commercial validation.

Verdict:

- Keep as an interop path, not the primary product strategy.

### Option C: Streaming and Persistent RunVaultCommand

Extend the existing command runner into named command sessions. It would still use normal process execution, not a PTY, but it would stream stdout/stderr through Wails events, support cancellation, keep command history, and attach results to a selected page/block.

Effort:

- Medium.
- Reuses the current `RunVaultCommand` shell resolution, cwd handling, and frontend console.
- Requires Wails event streaming, process registry, cancellation, and result append actions.

Risk:

- Medium-low.
- Not a true terminal; interactive tools will still be limited.
- But it is enough for commands like `git status`, `go test`, `npm run build`, `rg`, scripts, and one-shot AI/dev tasks.

Payoff:

- High for the first commercial loop.
- Directly enables "mind-map node -> command -> result child block".
- Much faster to ship than a PTY.

Recommendation:

- Primary: Option C.
- Fast-follow: Option A.
- Keep Option B as a lightweight compatibility bridge, but do not let Wave define the core architecture.

## 3. Named Loop Workflows

### Loop 1: Node Run Capture

User selects a mind-map node, runs a command in the vault context, and Dingovault appends the result as a child block under that node.

Flow:

1. Select block in `MindMapView`.
2. Open `WorkspaceConsole` with selected block context.
3. Run command.
4. Stream output live.
5. On completion, append a child block:
   - command
   - cwd
   - exit code
   - duration
   - stdout/stderr, folded or truncated with full output persisted if needed
6. Re-index the Markdown file.
7. The new child appears in outline, mind map, search, backlinks, and related views.

Why it matters:

- This is the simplest end-to-end thinking-doing loop.
- It turns terminal output into durable knowledge.

### Loop 2: Branch Refactor to Execution Plan

User restructures a branch in mind-map form, then turns selected nodes into an execution sequence.

Flow:

1. Open page as mind map.
2. Drag nodes to reorder sibling ideas.
3. Use indent/outdent gestures to reshape hierarchy.
4. Select a branch.
5. Run "Create execution plan".
6. Dingovault inserts TODO child blocks for each actionable node.
7. User runs commands from each TODO and captures results as children.

Why it matters:

- Mind-map restructuring is not decorative; it changes the Markdown outline.
- It links planning, hierarchy, and execution without leaving the note.

### Loop 3: Failure Triage Loop

User runs a command from a project node, captures failure output, then uses AI/search/related context to create next actions.

Flow:

1. Select a project or task block.
2. Run `go test`, `npm run build`, or a project script.
3. Dingovault streams output and detects non-zero exit.
4. The failure is appended as a child block with status metadata.
5. AI Chat receives the current page plus the failure child as context.
6. Suggested fixes are inserted as child TODO blocks.
7. After edits, user runs the command again and appends the passing result.

Why it matters:

- This creates an auditable work log.
- It connects code execution, AI assistance, and Markdown knowledge without a separate task system.

## 4. Architecture Proposal

### Mind Map View

Use the current `PageBlock[]` as the input and add a deterministic projection layer.

Proposed data flow:

```text
App.svelte roots
  -> mindMapProjection(roots)
  -> MindMapView.svelte
  -> existing block operations
     - UpdateBlock
     - InsertBlockAfter
     - IndentBlock
     - OutdentBlock
     - ReorderBlockBefore
```

Do not add a backend mind-map DTO unless the frontend projection becomes too expensive. The current page tree is already small enough for local projection in the first version.

View behavior:

- Root block starts near center-left.
- Children fan out by depth.
- Sibling order follows Markdown line order.
- Dragging a node within the same sibling group calls `ReorderBlockBefore`.
- Indent/outdent gestures call existing APIs.
- Editing text calls existing `UpdateBlock`.
- Selection is shared with outline and console context.

Important distinction:

- `MindMapView` is a tree projection for the current page.
- `PageGraph` remains a vault-level relationship projection.

### Terminal Loop

Upgrade the console in two layers.

Layer 1:

- Keep `RunVaultCommand` for compatibility and quick commands.
- Add streaming command sessions for long-running one-shot commands.
- Add a command-result-to-block operation.

Layer 2:

- Replace or augment streaming commands with a real PTY terminal block.
- Keep the same capture protocol so PTY output can also be appended into notes.

### Result Capture

The missing primitive is "append child block". This should be backend-owned because source line insertion and re-indexing are correctness-sensitive.

Proposed backend primitive:

```text
AppendChildBlock(parentBlockID, content)
```

It should:

- find the parent source line span
- find the end of the parent's subtree
- insert a new line with parent indentation + two spaces
- re-index the file
- invalidate page cache
- publish indexing hooks

This is better than doing `InsertBlockAfter` plus `IndentBlock` from the frontend because it avoids transient wrong structure and reduces failure cases.

## 5. Exact Files and Dependency Order

### This Step

Create:

1. `docs/UPGRADE_PLAN.md`
   - Written architecture plan only.
   - No product code changes.

### Future Implementation Order

#### Phase 1: Backend Child Append Primitive

Create:

1. `internal/graph/append_child_block.go`
   - Implement child insertion under an existing block.
2. `internal/graph/append_child_block_test.go`
   - Cover root child append, nested child append, source line order, indentation, and re-index behavior.

Modify:

3. `internal/bridge/app.go`
   - Expose `AppendChildBlock(parentBlockID, content)`.
   - Invalidate page cache on success.
4. `wailsjs/go/bridge/App.js`, `wailsjs/go/bridge/App.d.ts`, `wailsjs/go/models.ts`
   - Regenerate Wails bindings after bridge changes.

Dependency:

- This must land before command-result capture or mind-map execution workflows.

#### Phase 2: Mind Map Projection

Create:

1. `frontend/src/lib/mindMapProjection.js`
   - Pure transform from `PageBlock[]` to layout-ready tree nodes.
   - No persistence and no storage calls.
2. `frontend/src/MindMapView.svelte`
   - Renders the current page tree as a mind map.
   - Uses existing edit callbacks.

Modify:

3. `frontend/src/App.svelte`
   - Add view mode: `outline | mindmap | graph`.
   - Pass `roots`, selection state, and block operations into `MindMapView`.
   - Keep `PageGraph` behind the relationship graph mode.
4. `frontend/src/lib/i18n/en.json`
5. `frontend/src/lib/i18n/zh-CN.json`
   - Add labels for Mind Map, Run from Node, Capture Result, and view toggles.

Dependency:

- Can start after Phase 1, but should not need new backend data.

#### Phase 3: Streaming Command Sessions

Create:

1. `internal/bridge/terminal.go`
   - Session registry.
   - Start command.
   - Stream stdout/stderr via Wails events.
   - Cancel command.
   - Return final result.
2. `internal/bridge/terminal_test.go`
   - Cover streaming, cancellation, cwd, exit code, and timeout.
3. `frontend/src/lib/terminalEvents.js`
   - Frontend subscription helper for Wails events.

Modify:

4. `internal/bridge/console.go`
   - Keep `RunVaultCommand` as compatibility wrapper or simple non-streaming path.
5. `frontend/src/WorkspaceConsole.svelte`
   - Move from single awaited promise to streaming session UI.
   - Allow selected block context.
6. `frontend/src/App.svelte`
   - Track selected block context.
   - Pass selected block/page into `WorkspaceConsole`.
7. `frontend/src/lib/i18n/en.json`
8. `frontend/src/lib/i18n/zh-CN.json`
   - Add streaming/cancel/capture labels.
9. `wailsjs/go/bridge/App.js`, `wailsjs/go/bridge/App.d.ts`, `wailsjs/go/models.ts`
   - Regenerate Wails bindings.

Dependency:

- Depends on Phase 1 if command output capture is included in the same release.

#### Phase 4: Loop Workflows

Create:

1. `frontend/src/lib/commandResultBlocks.js`
   - Format command results as Markdown blocks.
2. `frontend/src/lib/loopWorkflows.js`
   - Orchestrate "run command then append result" from selected block context.

Modify:

3. `frontend/src/MindMapView.svelte`
   - Add node-level action: Run Command.
4. `frontend/src/WorkspaceConsole.svelte`
   - Add "Capture to selected node" and "Auto-capture on completion".
5. `frontend/src/AIChatPanel.svelte`
   - Allow command-result child block to become explicit context for failure triage.
6. `frontend/src/App.svelte`
   - Wire selected node, console, AI panel, and append APIs.

Dependency:

- Depends on Phase 1 and Phase 3.

#### Phase 5: PTY Fast-Follow

Create:

1. `internal/bridge/pty_terminal.go`
   - Real PTY sessions using `creack/pty` or platform-specific equivalent.
2. `frontend/src/PtyTerminal.svelte`
   - xterm.js shell surface.
3. `internal/bridge/pty_terminal_test.go`
   - Lifecycle and smoke tests where CI supports it.

Modify:

4. `frontend/package.json`
   - Add `xterm` and fit addon.
5. `frontend/src/WorkspaceConsole.svelte`
   - Host either streaming command mode or PTY mode.
6. `.github/workflows/test.yml`
   - Add targeted PTY-safe tests or skip platform-specific tests where needed.

Dependency:

- Should wait until Phase 1-4 prove the loop. Do not make PTY a prerequisite for the first commercial workflow.

## 6. Non-Goals

- Do not replace Markdown as source of truth.
- Do not introduce a mind-map database.
- Do not merge `PageGraph` and `MindMapView`.
- Do not build a full plugin platform before the loop works.
- Do not make Wave a hard dependency.
- Do not start with a real PTY if it delays command-result capture.

## 7. First Milestone Definition

The first milestone is successful when:

1. A user opens a Markdown page.
2. The same `PageBlock[]` can be viewed as outline or mind map.
3. A user selects a mind-map node.
4. A user runs a command in the vault context.
5. Output streams in the console.
6. The final result is appended as a child block under the selected node.
7. The file is re-indexed.
8. The new result appears in outline, mind map, search, and AI context.

That milestone proves the product thesis without building a second graph engine or a full PTY terminal.
