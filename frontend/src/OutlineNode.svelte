<script>
  import { onDestroy } from 'svelte'
  import { StartAIInlineStream, SuggestTagsForBlock } from '../wailsjs/go/bridge/App.js'
  import { messages, tr } from './lib/i18n/index.js'
  import { pushToast } from './toastStore.js'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @type {{ id: string, content: string, children?: any[], metadata?: any }} */
  export let node
  /** @type {number} */
  export let depth = 0
  /** @type {(id: string, text: string) => void} */
  export let onScheduleSave
  /** @type {(id: string, text: string) => void | Promise<void>} */
  export let onFlushSave
  /** @type {(id: string) => void | Promise<void>} */
  export let onInsertAfter
  /** @type {(target: string) => void} */
  export let onWikiNavigate
  /** @type {(id: string) => Promise<void>} */
  export let onIndent = async () => {}
  /** @type {(id: string) => Promise<void>} */
  export let onOutdent = async () => {}
  /** @type {(id: string) => Promise<void>} */
  export let onCycleTodo = async () => {}
  /** @type {(id: string, op: string) => Promise<void>} */
  export let onSlash = async () => {}
  /** @type {Record<string, boolean>} */
  export let collapsedMap = {}
  /** @type {(id: string) => void} */
  export let onToggleCollapse = () => {}
  /** @type {string[]} */
  export let selectedIds = []
  /** @type {(id: string, selected: boolean) => void} */
  export let onToggleSelect = () => {}
  /** @type {(movingId: string, beforeId: string) => Promise<void>} */
  export let onReorderBefore = async () => {}
  /** Mobile: swipe left on rail — cycle TODO */
  export let onSwipeTodo = async () => {}
  /** Mobile: swipe right on rail — clear block (confirm in parent) */
  export let onSwipeClear = async () => {}

  let local = node.content
  let saveTimer = 0
  let lastNodeId = ''
  /** @type {HTMLTextAreaElement | undefined} */
  let taEl

  let slashOpen = false
  let slashStart = 0
  let slashFilter = ''

  const slashCommands = [
    { op: 'ai', label: 'AI edit', match: 'ai' },
    { op: 'ai', label: 'Quick AI //', match: '/' },
    { op: 'todo', label: 'TODO', match: 'todo' },
    { op: 'today', label: 'Today', match: 'today' },
    { op: 'h1', label: 'Heading 1', match: 'h1' },
    { op: 'h2', label: 'Heading 2', match: 'h2' },
    { op: 'h3', label: 'Heading 3', match: 'h3' },
    { op: 'code', label: 'Code block', match: 'code' }
  ]

  let aiPanelOpen = false
  let aiInstruction = ''
  let aiStreaming = false
  /** @type {string} */
  let aiOpID = ''
  let aiBackup = ''
  /** @type {string[]} */
  let tagSuggestions = []
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  let tagSuggestTimer
  /** @type {((e: Event) => void) | undefined} */
  let winChunkHandler
  /** @type {((e: Event) => void) | undefined} */
  let winErrHandler
  /** @type {((e: Event) => void) | undefined} */
  let winDoneHandler

  $: if (node.id !== lastNodeId) {
    lastNodeId = node.id
    local = node.content
    slashOpen = false
    aiPanelOpen = false
    aiStreaming = false
    tagSuggestions = []
    unwireAIWin()
  }

  function newAIopID() {
    return typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `ai-${node.id}-${Date.now()}`
  }

  /** @param {unknown} err */
  function aiInlineToastMessage(err) {
    const s = String(err || '').toLowerCase()
    if (
      /connection refused|econnrefused|network|broken pipe|eof|connection reset|reset by peer|timeout|dial tcp|no such host|failed to fetch/.test(
        s
      )
    ) {
      return T('outline.aiConnectionLost')
    }
    return String(err || 'AI error')
  }

  function unwireAIWin() {
    if (winChunkHandler) window.removeEventListener('dv-ai-chunk', winChunkHandler)
    if (winErrHandler) window.removeEventListener('dv-ai-err', winErrHandler)
    if (winDoneHandler) window.removeEventListener('dv-ai-done', winDoneHandler)
    winChunkHandler = undefined
    winErrHandler = undefined
    winDoneHandler = undefined
  }

  function wireAIWin() {
    unwireAIWin()
    winChunkHandler = (e) => {
      const d = /** @type {CustomEvent} */ (e).detail
      if (!d || d.opID !== aiOpID) return
      const c = d.chunk != null ? String(d.chunk) : ''
      local = (local || '') + c
      queueSave()
    }
    winErrHandler = (e) => {
      const d = /** @type {CustomEvent} */ (e).detail
      if (!d || d.opID !== aiOpID) return
      pushToast(aiInlineToastMessage(d.message), 'error')
      local = aiBackup
      aiStreaming = false
      aiPanelOpen = false
      unwireAIWin()
    }
    winDoneHandler = (e) => {
      const d = /** @type {CustomEvent} */ (e).detail
      if (!d || d.opID !== aiOpID) return
      aiStreaming = false
      aiPanelOpen = false
      unwireAIWin()
      void onFlushSave(node.id, local)
    }
    window.addEventListener('dv-ai-chunk', winChunkHandler)
    window.addEventListener('dv-ai-err', winErrHandler)
    window.addEventListener('dv-ai-done', winDoneHandler)
  }

  async function submitAIInline() {
    const inst = aiInstruction.trim()
    if (!inst || aiStreaming) return
    aiBackup = local
    local = ''
    aiStreaming = true
    aiOpID = newAIopID()
    wireAIWin()
    try {
      await StartAIInlineStream(aiOpID, node.id, inst)
    } catch (e) {
      pushToast(aiInlineToastMessage(e), 'error')
      local = aiBackup
      aiStreaming = false
      unwireAIWin()
    }
  }

  function cancelAIPanel() {
    if (aiStreaming) return
    aiPanelOpen = false
    aiInstruction = ''
  }

  /** @param {string} tag */
  function appendSuggestedTag(tag) {
    const t = String(tag || '').trim().toLowerCase()
    if (!t) return
    const token = '#' + t
    if (local.includes(token)) return
    const pad = local.length && !/\s$/.test(local) ? ' ' : ''
    local = (local + pad + token).trimEnd()
    void onFlushSave(node.id, local)
  }

  async function loadTagSuggestions() {
    if (!local.trim()) {
      tagSuggestions = []
      return
    }
    try {
      const tags = await SuggestTagsForBlock(node.id)
      tagSuggestions = Array.isArray(tags) ? tags : []
    } catch {
      tagSuggestions = []
    }
  }

  onDestroy(() => {
    unwireAIWin()
    if (tagSuggestTimer) clearTimeout(tagSuggestTimer)
  })

  function queueSave() {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = /** @type {any} */ (setTimeout(() => {
      saveTimer = 0
      onScheduleSave(node.id, local)
    }, 500))
  }

  async function flush() {
    if (saveTimer) {
      clearTimeout(saveTimer)
      saveTimer = 0
    }
    await onFlushSave(node.id, local)
  }

  /** @param {HTMLTextAreaElement} el */
  function detectSlash(el) {
    const v = el.value
    const pos = el.selectionStart
    const lineStart = v.lastIndexOf('\n', pos - 1) + 1
    const upto = v.slice(lineStart, pos)
    const slashIdx = upto.lastIndexOf('/')
    if (slashIdx === -1) {
      slashOpen = false
      return
    }
    const after = upto.slice(slashIdx + 1)
    if (/[\s/]/.test(after)) {
      slashOpen = false
      return
    }
    slashOpen = true
    slashStart = lineStart + slashIdx
    slashFilter = after.toLowerCase()
  }

  /** @param {Event} e */
  function onInput(e) {
    queueSave()
    detectSlash(/** @type {HTMLTextAreaElement} */ (e.target))
  }

  $: filteredSlash = slashCommands.filter(
    (c) => slashFilter === '' || c.match.startsWith(slashFilter) || c.label.toLowerCase().includes(slashFilter)
  )

  /** @param {string} op */
  async function pickSlash(op) {
    if (!taEl) return
    const v = taEl.value
    const pos = taEl.selectionStart
    let endRemove = pos
    if (slashFilter === '/' && v.slice(slashStart, slashStart + 2) === '//') {
      endRemove = slashStart + 2
    }
    const before = v.slice(0, slashStart)
    const after = v.slice(endRemove)
    local = before + after
    slashOpen = false
    await flush()
    if (op === 'ai') {
      aiInstruction = ''
      aiPanelOpen = true
      return
    }
    await onSlash(node.id, op)
  }

  function closeSlash() {
    slashOpen = false
  }

  /** @param {KeyboardEvent} e */
  function onKeydown(e) {
    if (slashOpen && e.key === 'Escape') {
      e.preventDefault()
      closeSlash()
      return
    }
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      void onCycleTodo(node.id)
      return
    }
    if (e.key === 'Tab') {
      e.preventDefault()
      if (e.shiftKey) void onOutdent(node.id)
      else void onIndent(node.id)
      return
    }
    if (e.key !== 'Enter' || e.shiftKey) return
    const el = /** @type {HTMLTextAreaElement} */ (e.target)
    if (el.selectionStart !== el.value.length) return
    e.preventDefault()
    onInsertAfter(node.id)
  }

  const wikiRE = /\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g

  let dragOver = false

  $: collapsed = !!collapsedMap[node.id]
  $: selected = selectedIds.includes(node.id)
  $: hasKids = !!(node.children && node.children.length)

  /** @param {DragEvent} e */
  function onDragStartRow(e) {
    e.dataTransfer?.setData('application/x-dingovault-block', node.id)
    e.dataTransfer?.setData('text/plain', node.id)
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }

  /** @param {DragEvent} e */
  function onDragOverRow(e) {
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    dragOver = true
  }

  /** @param {DragEvent} e */
  function onDragLeaveRow() {
    dragOver = false
  }

  /** @param {DragEvent} e */
  async function onDropRow(e) {
    e.preventDefault()
    dragOver = false
    const moving =
      e.dataTransfer?.getData('application/x-dingovault-block') ||
      e.dataTransfer?.getData('text/plain') ||
      ''
    if (!moving || moving === node.id) return
    await onReorderBefore(moving, node.id)
  }

  let swipeStartX = 0
  let swipeStartY = 0

  /** @param {TouchEvent} e */
  function onTouchStartSwipe(e) {
    if (e.touches.length !== 1) return
    swipeStartX = e.touches[0].clientX
    swipeStartY = e.touches[0].clientY
  }

  /** @param {TouchEvent} e */
  async function onTouchEndSwipe(e) {
    if (e.changedTouches.length !== 1) return
    const dx = e.changedTouches[0].clientX - swipeStartX
    const dy = e.changedTouches[0].clientY - swipeStartY
    const min = 52
    if (Math.abs(dx) < min || Math.abs(dx) < Math.abs(dy) * 1.2) return
    if (dx < 0) await onSwipeTodo(node.id)
    else await onSwipeClear(node.id)
  }

  /** @param {string} text */
  function wikiLinks(text) {
    /** @type {{ target: string, label: string }[]} */
    const out = []
    if (!text) return out
    let m
    const re = new RegExp(wikiRE.source, 'g')
    while ((m = re.exec(text)) !== null) {
      out.push({ target: m[1].trim(), label: (m[2] || m[1]).trim() })
    }
    return out
  }
</script>

<div
  class="row"
  class:hasSlash={slashOpen || aiPanelOpen}
  class:dragOver
  style="--depth: {depth}; padding-left: {4 + depth * 14}px; border-left-width: {depth > 0 ? 1 : 0}px"
  role="group"
  on:dragover={onDragOverRow}
  on:dragleave={onDragLeaveRow}
  on:drop={onDropRow}
>
  <div class="row-inner">
    <div
      class="row-controls touch-actions"
      aria-hidden="false"
      on:touchstart|passive={onTouchStartSwipe}
      on:touchend|passive={onTouchEndSwipe}
    >
      <label class="sel">
        <input
          type="checkbox"
          checked={selected}
          on:change={(e) =>
            onToggleSelect(node.id, /** @type {HTMLInputElement} */ (e.target).checked)}
        />
      </label>
      {#if hasKids}
        <button
          type="button"
          class="fold"
          title={collapsed ? 'Expand' : 'Collapse'}
          aria-expanded={!collapsed}
          on:click={() => onToggleCollapse(node.id)}
        >
          {collapsed ? '▸' : '▾'}
        </button>
      {:else}
        <span class="fold-spacer"></span>
      {/if}
      <span
        class="drag-handle"
        draggable="true"
        title="Drag to reorder (siblings)"
        role="button"
        tabindex="0"
        on:dragstart={onDragStartRow}
        on:keydown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') e.preventDefault()
        }}
      >⠿</span>
    </div>
    <div class="edit">
    <textarea
      bind:this={taEl}
      class="ta"
      class:ai-generating={aiStreaming}
      rows="2"
      data-block-id={node.id}
      bind:value={local}
      on:input={(e) => {
        tagSuggestions = []
        onInput(e)
      }}
      on:blur={() => {
        flush()
        closeSlash()
        if (tagSuggestTimer) clearTimeout(tagSuggestTimer)
        tagSuggestTimer = window.setTimeout(() => void loadTagSuggestions(), 650)
      }}
      on:focus={() => {
        if (tagSuggestTimer) {
          clearTimeout(tagSuggestTimer)
          tagSuggestTimer = undefined
        }
      }}
      on:keydown={onKeydown}
    ></textarea>
    {#if slashOpen && filteredSlash.length}
      <div class="slash-menu" role="listbox" aria-label="Slash commands">
        {#each filteredSlash as cmd (cmd.match + cmd.label)}
          <button
            type="button"
            class="slash-item"
            on:mousedown={(e) => {
              e.preventDefault()
              void pickSlash(cmd.op)
            }}
          >
            <span class="slash-cmd">/{cmd.match}</span>
            <span class="slash-label">{cmd.label}</span>
          </button>
        {/each}
      </div>
    {/if}
    {#if wikiLinks(local).length}
      <div class="wikis">
        {#each wikiLinks(local) as w (w.target + w.label)}
          <button type="button" class="wiki" on:click={() => onWikiNavigate(w.target)}>[[{w.label}]]</button>
        {/each}
      </div>
    {/if}
    {#if tagSuggestions.length}
      <div class="tag-suggest" aria-label={T('outline.tagSuggestAria')}>
        <span class="tag-hint">{T('outline.tagHint')}</span>
        {#each tagSuggestions as tg (tg)}
          <button type="button" class="tag-chip" on:click={() => appendSuggestedTag(tg)}>#{tg}</button>
        {/each}
      </div>
    {/if}
    {#if aiPanelOpen}
      <div class="ai-pop" role="dialog" aria-label={T('outline.aiTitle')}>
        <p class="ai-title">{T('outline.aiTitle')}</p>
        <textarea
          class="ai-instruction"
          rows="2"
          bind:value={aiInstruction}
          placeholder={T('outline.aiPlaceholder')}
          disabled={aiStreaming}
          on:keydown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              void submitAIInline()
            }
            if (e.key === 'Escape') cancelAIPanel()
          }}
        />
        <div class="ai-actions">
          <button type="button" class="ai-run" disabled={aiStreaming || !aiInstruction.trim()} on:click={() => submitAIInline()}>
            {T('outline.aiRun')}
          </button>
          <button type="button" class="ai-cancel" disabled={aiStreaming} on:click={cancelAIPanel}>{T('outline.aiCancel')}</button>
        </div>
      </div>
    {/if}
    </div>
  </div>
</div>
{#if hasKids && !collapsed}
  {#each node.children || [] as child (child.id)}
    <svelte:self
      node={child}
      depth={depth + 1}
      {onScheduleSave}
      {onFlushSave}
      {onInsertAfter}
      {onWikiNavigate}
      {onIndent}
      {onOutdent}
      {onCycleTodo}
      {onSlash}
      {collapsedMap}
      {onToggleCollapse}
      {selectedIds}
      {onToggleSelect}
      {onReorderBefore}
      {onSwipeTodo}
      {onSwipeClear}
    />
  {/each}
{/if}

<style>
  .row {
    position: relative;
    margin-bottom: 8px;
    border-left: 0 solid var(--dv-rail, rgba(255, 255, 255, 0.08));
    border-radius: 8px;
    transition: background 0.12s ease;
  }
  .row.dragOver {
    background: rgba(100, 140, 255, 0.08);
  }
  .row-inner {
    display: flex;
    align-items: flex-start;
    gap: 6px;
  }
  .row-controls {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding-top: 6px;
    flex-shrink: 0;
    width: 28px;
    touch-action: pan-y;
  }
  @media (max-width: 640px) {
    .row-controls {
      width: 40px;
      min-height: 44px;
      padding-top: 8px;
      gap: 4px;
    }
    .sel input {
      width: 20px;
      height: 20px;
      min-width: 20px;
      min-height: 20px;
    }
    .fold,
    .fold-spacer {
      width: 32px;
      min-height: 28px;
      font-size: 0.9rem;
    }
    .drag-handle {
      font-size: 0.85rem;
      padding: 6px 0 8px;
      opacity: 0.5;
    }
  }
  .sel input {
    width: 14px;
    height: 14px;
    cursor: pointer;
    accent-color: rgba(120, 140, 255, 0.9);
  }
  .fold {
    border: none;
    background: transparent;
    color: var(--dv-muted, rgba(255, 255, 255, 0.45));
    cursor: pointer;
    font-size: 0.75rem;
    line-height: 1;
    padding: 2px 0;
    width: 22px;
  }
  .fold:hover {
    color: var(--dv-fg, #fff);
  }
  .fold-spacer {
    display: block;
    width: 22px;
    height: 14px;
  }
  .drag-handle {
    cursor: grab;
    font-size: 0.65rem;
    line-height: 1;
    letter-spacing: -0.12em;
    opacity: 0.35;
    user-select: none;
    padding: 2px 0 4px;
  }
  .drag-handle:active {
    cursor: grabbing;
  }
  .edit {
    position: relative;
    flex: 1;
    min-width: 0;
  }
  .ta {
    width: 100%;
    resize: vertical;
    min-height: 2.75rem;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: var(--dv-input, rgba(0, 0, 0, 0.2));
    color: inherit;
    font-family: var(--dv-font-mono, 'JetBrains Mono', ui-monospace, monospace);
    font-size: 0.92rem;
    line-height: 1.5;
    touch-action: manipulation;
  }
  @media (max-width: 640px) {
    .ta {
      font-size: 16px;
      min-height: 3rem;
      padding: 10px 12px;
    }
  }
  .ta:focus {
    outline: none;
    border-color: rgba(120, 160, 255, 0.4);
  }
  .slash-menu {
    position: absolute;
    left: 0;
    top: 100%;
    margin-top: 4px;
    z-index: 30;
    min-width: 220px;
    max-height: 240px;
    overflow-y: auto;
    border-radius: 10px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: var(--dv-panel, rgba(28, 28, 34, 0.98));
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
    padding: 4px;
  }
  .slash-item {
    display: flex;
    align-items: baseline;
    gap: 10px;
    width: 100%;
    text-align: left;
    padding: 8px 10px;
    margin: 0;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    font: inherit;
  }
  .slash-item:hover {
    background: rgba(255, 255, 255, 0.06);
  }
  .slash-cmd {
    font-size: 0.8rem;
    opacity: 0.55;
    font-family: ui-monospace, monospace;
  }
  .slash-label {
    font-size: 0.88rem;
  }
  .wikis {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 4px;
  }
  .wiki {
    font-size: 0.75rem;
    padding: 2px 8px;
    border-radius: 999px;
    border: 1px solid rgba(120, 160, 255, 0.35);
    background: rgba(80, 120, 255, 0.12);
    color: #b4c8ff;
  }
  .wiki:hover {
    background: rgba(80, 120, 255, 0.22);
  }
  .tag-suggest {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    padding: 6px 8px;
    border-radius: 8px;
    border: 1px dashed color-mix(in srgb, var(--dv-fg) 16%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
  }
  .tag-hint {
    font-size: 0.72rem;
    opacity: 0.5;
    margin-right: 4px;
  }
  .tag-chip {
    font-size: 0.72rem;
    padding: 2px 8px;
    border-radius: 999px;
    border: 1px solid rgba(160, 200, 140, 0.35);
    background: rgba(100, 160, 90, 0.12);
    color: #c8e6b8;
    cursor: pointer;
  }
  .tag-chip:hover {
    background: rgba(100, 160, 90, 0.22);
  }
  .ai-pop {
    position: absolute;
    left: 0;
    right: 0;
    top: 100%;
    margin-top: 6px;
    z-index: 40;
    padding: 10px 12px;
    border-radius: 10px;
    border: 1px solid rgba(120, 160, 255, 0.35);
    background: var(--dv-panel, rgba(28, 28, 34, 0.98));
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
  }
  .ai-title {
    margin: 0 0 8px;
    font-size: 0.78rem;
    font-weight: 600;
    opacity: 0.75;
  }
  .ai-instruction {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    min-height: 48px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: var(--dv-input, rgba(0, 0, 0, 0.2));
    color: inherit;
    font-family: inherit;
    font-size: 0.88rem;
    margin-bottom: 8px;
  }
  .ai-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
  .ai-run,
  .ai-cancel {
    padding: 6px 12px;
    border-radius: 8px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    font-size: 0.82rem;
    cursor: pointer;
    color: inherit;
  }
  .ai-run {
    background: rgba(80, 120, 255, 0.28);
  }
  .ai-run:disabled,
  .ai-cancel:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .ai-cancel {
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
  }
  .ta.ai-generating {
    background: linear-gradient(
      100deg,
      var(--dv-input, rgba(0, 0, 0, 0.2)) 0%,
      rgba(120, 160, 255, 0.12) 45%,
      var(--dv-input, rgba(0, 0, 0, 0.2)) 90%
    );
    background-size: 200% 100%;
    animation: dv-ai-shimmer 1.2s ease-in-out infinite;
  }
  @keyframes dv-ai-shimmer {
    0% {
      background-position: 100% 0;
    }
    100% {
      background-position: -100% 0;
    }
  }
</style>
