<script context="module">
  /** @param {string} abs @param {string} root */
  export function toRelPath(abs, root) {
    if (!abs || !root) return abs || ''
    const a = abs.replace(/\\/g, '/')
    const r = root.replace(/\\/g, '/').replace(/\/?$/, '/')
    if (a.length >= r.length && a.slice(0, r.length).toLowerCase() === r.toLowerCase()) {
      return a.slice(r.length)
    }
    return abs
  }
</script>

<script>
  import { onMount, tick } from 'svelte'
  import {
    NotesRoot,
    GetPage,
    UpdateBlock,
    InsertBlockAfter,
    IndentBlock,
    OutdentBlock,
    CycleBlockTodo,
    ApplySlashOp,
    EnsurePage,
    ResolveWikilink,
    GetTheme,
    SetTheme,
    GetWikiGraph,
    ReorderBlockBefore
  } from '../wailsjs/go/bridge/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import OutlineNode from './OutlineNode.svelte'
  import PageGraph from './PageGraph.svelte'
  import Backlinks from './Backlinks.svelte'
  import CommandPalette from './CommandPalette.svelte'
  import ToastStack from './ToastStack.svelte'
  import { touchRecentPage } from './recentPages.js'
  import { pushToast } from './toastStore.js'

  let notesRoot = ''
  let pagePath = 'README.md'
  /** @type {any[]} */
  let roots = []
  let paletteOpen = false
  let err = ''
  let lastFileEvent = ''
  let indexEpoch = 0

  /** @type {Record<string, boolean>} */
  let collapsedState = {}
  /** @type {string[]} */
  let selectedIds = []
  let graphOpen = false
  /** @type {{ nodes: { id: string, label: string }[], edges: { source: string, target: string }[] }} */
  let graphData = { nodes: [], edges: [] }

  /** @type {Record<string, number>} */
  let saveTimers = {}

  /** @type {'dark' | 'light'} */
  let theme = 'dark'
  $: document.documentElement.dataset.theme = theme

  /** @param {unknown} e */
  function notifyErr(e) {
    const m = String(e)
    err = m
    pushToast(m, 'error')
  }

  /** @param {string} root */
  function vaultBasename(root) {
    if (!root) return 'Vault'
    const p = root.replace(/[/\\]+$/, '')
    const parts = p.split(/[/\\]/).filter(Boolean)
    return parts.length ? parts[parts.length - 1] : 'Vault'
  }

  $: breadcrumbSegments = pagePath.split('/').filter(Boolean)

  function collapseStorageKey() {
    return `dingovault-collapse:${pagePath}`
  }

  function loadCollapsedFromStorage() {
    try {
      const raw = localStorage.getItem(collapseStorageKey())
      collapsedState = raw ? JSON.parse(raw) : {}
    } catch {
      collapsedState = {}
    }
  }

  $: pagePath, loadCollapsedFromStorage()

  function toggleCollapse(id) {
    collapsedState = { ...collapsedState, [id]: !collapsedState[id] }
    try {
      localStorage.setItem(collapseStorageKey(), JSON.stringify(collapsedState))
    } catch {
      /* ignore quota */
    }
  }

  /** @param {string} id @param {boolean} on */
  function toggleSelect(id, on) {
    if (on) {
      if (!selectedIds.includes(id)) selectedIds = [...selectedIds, id]
    } else {
      selectedIds = selectedIds.filter((x) => x !== id)
    }
  }

  function clearSelection() {
    selectedIds = []
  }

  async function copySelectedMarkdown() {
    const lines = []
    for (const id of selectedIds) {
      const el = document.querySelector(`textarea[data-block-id="${id}"]`)
      if (el && el instanceof HTMLTextAreaElement) lines.push(el.value)
    }
    const text = lines.join('\n\n')
    try {
      await navigator.clipboard.writeText(text)
      pushToast(`Copied ${lines.length} block(s)`, 'info')
    } catch {
      pushToast('Clipboard failed', 'error')
    }
  }

  async function openGraph() {
    err = ''
    try {
      graphData = await GetWikiGraph()
      graphOpen = true
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} movingId @param {string} beforeId */
  async function handleReorderBefore(movingId, beforeId) {
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await ReorderBlockBefore(movingId, beforeId)
      await loadPage(pagePath)
    } catch (e) {
      notifyErr(e)
    }
  }

  onMount(() => {
    document.documentElement.style.setProperty(
      '--dv-font',
      'ui-sans-serif, system-ui, "Inter", "Segoe UI", sans-serif'
    )

    GetTheme()
      .then((t) => {
        theme = t === 'light' ? 'light' : 'dark'
      })
      .catch(() => {
        theme = 'dark'
      })

    NotesRoot()
      .then((p) => {
        notesRoot = p
        return loadPage(pagePath)
      })
      .catch((e) => notifyErr(e))

    EventsOn('file-updated', async (payload) => {
      indexEpoch++
      const abs = payload && typeof payload === 'object' && 'path' in payload ? /** @type {any} */ (payload).path : ''
      lastFileEvent = String(abs)
      const relEvt = toRelPath(String(abs), notesRoot).replace(/^\//, '')
      const relCur = pagePath.replace(/^\//, '')
      const norm = (/** @type {string} */ x) => x.replace(/\\/g, '/').toLowerCase()
      if (!abs || norm(relEvt) !== norm(relCur)) return

      const ae = document.activeElement
      if (ae && ae.tagName === 'TEXTAREA' && ae.closest('.outliner-panel')) {
        return
      }
      await loadPage(pagePath)
    })

    /** @param {KeyboardEvent} e */
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        paletteOpen = !paletteOpen
      }
      if (e.key === 'Escape') paletteOpen = false
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  /**
   * @param {string} rel
   * @param {{ focusBlockId?: string, caretOffset?: number }} [opts]
   */
  async function loadPage(rel, opts) {
    const focusId = opts?.focusBlockId
    const caret = opts?.caretOffset
    err = ''
    try {
      roots = await GetPage(rel)
      pagePath = rel
      selectedIds = []
      touchRecentPage(rel)
      if (focusId) {
        await tick()
        requestAnimationFrame(() => {
          const el = document.querySelector(`textarea[data-block-id="${focusId}"]`)
          if (el && el instanceof HTMLTextAreaElement) {
            el.focus()
            const n = caret != null ? Math.min(Math.max(0, caret), el.value.length) : el.value.length
            el.setSelectionRange(n, n)
          }
        })
      }
    } catch (e) {
      notifyErr(e)
      roots = []
    }
  }

  async function openOrCreate() {
    err = ''
    try {
      await EnsurePage(pagePath)
      await loadPage(pagePath)
    } catch (e) {
      notifyErr(e)
    }
  }

  async function toggleTheme() {
    const prev = theme
    const next = prev === 'dark' ? 'light' : 'dark'
    theme = next
    try {
      await SetTheme(next)
    } catch (e) {
      theme = prev
      notifyErr(e)
    }
  }

  /** @param {string} id @param {string} text */
  function scheduleSave(id, text) {
    if (saveTimers[id]) clearTimeout(saveTimers[id])
    saveTimers[id] = window.setTimeout(async () => {
      delete saveTimers[id]
      try {
        await UpdateBlock(id, text)
      } catch (e) {
        notifyErr(e)
      }
    }, 500)
  }

  /** @param {string} id @param {string} text */
  async function flushSave(id, text) {
    if (saveTimers[id]) {
      clearTimeout(saveTimers[id])
      delete saveTimers[id]
    }
    try {
      await UpdateBlock(id, text)
    } catch (e) {
      notifyErr(e)
    }
  }

  async function syncAllBlocksFromDOM() {
    const els = document.querySelectorAll('.outliner-panel textarea[data-block-id]')
    for (const el of els) {
      const id = el.getAttribute('data-block-id')
      if (!id) continue
      if (saveTimers[id]) {
        clearTimeout(saveTimers[id])
        delete saveTimers[id]
      }
      await UpdateBlock(id, /** @type {HTMLTextAreaElement} */ (el).value)
    }
  }

  /** @param {string} id */
  async function handleInsertAfter(id) {
    try {
      await syncAllBlocksFromDOM()
      await InsertBlockAfter(id, '')
      await loadPage(pagePath)
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id */
  function caretForBlock(id) {
    const ae = document.activeElement
    if (ae && ae instanceof HTMLTextAreaElement && ae.getAttribute('data-block-id') === id) {
      return ae.selectionStart
    }
    return undefined
  }

  /** @param {string} id */
  async function handleIndent(id) {
    err = ''
    const caret = caretForBlock(id)
    try {
      await syncAllBlocksFromDOM()
      await IndentBlock(id)
      await loadPage(pagePath, { focusBlockId: id, caretOffset: caret })
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id */
  async function handleOutdent(id) {
    err = ''
    const caret = caretForBlock(id)
    try {
      await syncAllBlocksFromDOM()
      await OutdentBlock(id)
      await loadPage(pagePath, { focusBlockId: id, caretOffset: caret })
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id */
  async function handleCycleTodo(id) {
    err = ''
    const caret = caretForBlock(id)
    try {
      await syncAllBlocksFromDOM()
      await CycleBlockTodo(id)
      await loadPage(pagePath, { focusBlockId: id, caretOffset: caret })
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id @param {string} op */
  async function handleSlash(id, op) {
    err = ''
    const caret = caretForBlock(id)
    try {
      await syncAllBlocksFromDOM()
      await ApplySlashOp(id, op)
      await loadPage(pagePath, { focusBlockId: id, caretOffset: caret })
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} target */
  async function openWiki(target) {
    err = ''
    try {
      const abs = await ResolveWikilink(target)
      const rel = toRelPath(abs, notesRoot)
      const tree = await GetPage(rel)
      if (!tree.length) {
        if (!confirm(`Create page "${rel}"?`)) return
        await EnsurePage(rel)
      }
      await loadPage(rel)
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {any} h */
  async function openBlockHit(h) {
    const rel = toRelPath(h.sourcePath, notesRoot)
    await loadPage(rel || pagePath)
  }
</script>

<main class="layout zen">
  <nav class="breadcrumbs" aria-label="Breadcrumb">
    <span class="crumb vault">{vaultBasename(notesRoot)}</span>
    {#if breadcrumbSegments.length > 1}
      {#each breadcrumbSegments.slice(0, -1) as seg}
        <span class="sep" aria-hidden="true">›</span>
        <span class="crumb">{seg}</span>
      {/each}
    {/if}
    {#if breadcrumbSegments.length}
      <span class="sep" aria-hidden="true">›</span>
      <span class="crumb current">{breadcrumbSegments[breadcrumbSegments.length - 1]}</span>
    {/if}
  </nav>

  <header class="top">
    <h1>Dingovault</h1>
    <p class="meta">{notesRoot || '…'}</p>
    {#if lastFileEvent}
      <p class="event">Last index event: <code>{lastFileEvent}</code></p>
    {/if}
  </header>

  <div class="toolbar">
    <input class="path-input" bind:value={pagePath} placeholder="path/to/page.md" />
    <button type="button" class="btn" on:click={() => loadPage(pagePath)}>Open</button>
    <button type="button" class="btn secondary" on:click={openOrCreate}>Ensure page</button>
    <button type="button" class="btn secondary" on:click={toggleTheme} title="Toggle light/dark">
      {theme === 'dark' ? 'Light' : 'Dark'} mode
    </button>
    <button type="button" class="btn secondary" on:click={openGraph}>Graph</button>
  </div>

  {#if selectedIds.length > 0}
    <div class="bulk-bar" role="toolbar" aria-label="Multi-select">
      <span class="bulk-count">{selectedIds.length} selected</span>
      <button type="button" class="btn secondary sm" on:click={copySelectedMarkdown}>Copy text</button>
      <button type="button" class="btn secondary sm" on:click={clearSelection}>Clear</button>
    </div>
  {/if}

  {#if graphOpen}
    <section class="graph-panel">
      <div class="graph-head">
        <h2>Page graph</h2>
        <button type="button" class="btn secondary sm" on:click={() => (graphOpen = false)}>Close</button>
      </div>
      <PageGraph graph={graphData} />
    </section>
  {/if}

  {#if err}
    <p class="err">{err}</p>
  {/if}

  <section class="outliner-panel">
    <h2>Outline</h2>
    {#if roots.length === 0}
      <p class="empty">No blocks — open a Markdown file or create one.</p>
    {:else}
      {#each roots as r (r.id)}
        <OutlineNode
          node={r}
          depth={0}
          onScheduleSave={scheduleSave}
          onFlushSave={flushSave}
          onInsertAfter={handleInsertAfter}
          onWikiNavigate={openWiki}
          onIndent={handleIndent}
          onOutdent={handleOutdent}
          onCycleTodo={handleCycleTodo}
          onSlash={handleSlash}
          collapsedMap={collapsedState}
          onToggleCollapse={toggleCollapse}
          {selectedIds}
          onToggleSelect={toggleSelect}
          onReorderBefore={handleReorderBefore}
        />
      {/each}
    {/if}
  </section>

  <Backlinks {notesRoot} {pagePath} indexEpoch={indexEpoch} onOpenPage={(rel) => loadPage(rel)} />

  <p class="hint">
    <kbd>Ctrl</kbd>+<kbd>K</kbd> palette · <kbd>Tab</kbd> / <kbd>Shift</kbd>+<kbd>Tab</kbd> indent ·
    <kbd>Cmd</kbd>+<kbd>Enter</kbd> TODO cycle · <kbd>/</kbd> commands · <kbd>Enter</kbd> at end adds sibling ·
    drag <span class="hint-mono">⠿</span> to reorder siblings · fold <span class="hint-mono">▾</span> · checkboxes
    multi-select
  </p>
</main>

<CommandPalette
  open={paletteOpen}
  {notesRoot}
  onSelectPage={(rel) => loadPage(rel)}
  onSelectBlockHit={openBlockHit}
  onClose={() => (paletteOpen = false)}
/>

<ToastStack />

<style>
  :global(html) {
    background: transparent;
    --dv-fg: #e8e8ec;
    --dv-muted: rgba(255, 255, 255, 0.55);
    --dv-border: rgba(255, 255, 255, 0.12);
    --dv-input: #121216;
    --dv-panel: rgba(28, 28, 34, 0.96);
    --dv-hit-hover: rgba(255, 255, 255, 0.06);
    --dv-toast-bg: rgba(30, 30, 36, 0.96);
    --dv-toast-border: rgba(255, 255, 255, 0.12);
  }

  :global(html[data-theme='light']) {
    --dv-fg: #141414;
    --dv-muted: rgba(0, 0, 0, 0.5);
    --dv-border: rgba(0, 0, 0, 0.12);
    --dv-input: #ffffff;
    --dv-panel: rgba(255, 255, 255, 0.94);
    --dv-hit-hover: rgba(0, 0, 0, 0.05);
    --dv-toast-bg: rgba(255, 252, 248, 0.98);
    --dv-toast-border: rgba(0, 0, 0, 0.1);
  }

  :global(body) {
    margin: 0;
    min-height: 100vh;
    background: transparent;
    color: var(--dv-fg);
    font-family: var(--dv-font, system-ui, sans-serif);
    -webkit-font-smoothing: antialiased;
  }

  .layout {
    max-width: 800px;
    width: 100%;
    margin: 0 auto;
    padding: 20px max(16px, env(safe-area-inset-left)) 56px max(16px, env(safe-area-inset-right));
    box-sizing: border-box;
  }
  .breadcrumbs {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: center;
    text-align: center;
    gap: 4px 2px;
    font-size: 0.78rem;
    opacity: 0.55;
    margin-bottom: 18px;
    letter-spacing: 0.02em;
  }
  .breadcrumbs .sep {
    margin: 0 4px;
    opacity: 0.45;
  }
  .breadcrumbs .crumb.current {
    opacity: 0.95;
    font-weight: 500;
  }
  .top h1 {
    margin: 0 0 4px;
    font-size: 1.5rem;
    font-weight: 650;
  }
  .meta,
  .event {
    margin: 0;
    font-size: 0.85rem;
    opacity: 0.75;
  }
  .event code {
    font-size: 0.8rem;
  }
  .toolbar {
    display: flex;
    gap: 8px;
    margin-top: 16px;
    flex-wrap: wrap;
    align-items: center;
  }
  .path-input {
    flex: 1;
    min-width: 200px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
  }
  .btn {
    padding: 8px 14px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: rgba(80, 120, 255, 0.25);
    color: var(--dv-fg);
  }
  .btn.secondary {
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
  }
  .err {
    color: #f87171;
    font-size: 0.9rem;
  }
  .outliner-panel {
    margin-top: 20px;
    padding: 16px;
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    border-radius: 10px;
    border: 1px solid var(--dv-border);
  }
  .outliner-panel h2 {
    margin: 0 0 12px;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
  }
  .empty {
    opacity: 0.55;
    font-size: 0.9rem;
  }
  .bulk-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid rgba(120, 160, 255, 0.25);
    background: rgba(80, 120, 255, 0.08);
    font-size: 0.85rem;
  }
  .bulk-count {
    font-weight: 500;
    margin-right: 4px;
  }
  .btn.sm {
    padding: 4px 10px;
    font-size: 0.8rem;
  }
  .graph-panel {
    margin-top: 16px;
    padding: 14px 16px 8px;
    border-radius: 10px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
  }
  .graph-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 4px;
  }
  .graph-head h2 {
    margin: 0;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
  }
  .hint {
    margin-top: 24px;
    font-size: 0.8rem;
    opacity: 0.5;
  }
  .hint-mono {
    font-family: ui-monospace, monospace;
    font-size: 0.72rem;
    opacity: 0.75;
  }
  kbd {
    font-size: 0.75rem;
    padding: 2px 6px;
    border-radius: 4px;
    border: 1px solid rgba(255, 255, 255, 0.15);
    background: rgba(0, 0, 0, 0.25);
  }
</style>
