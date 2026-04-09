<script>
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

  let local = node.content
  let saveTimer = 0
  let lastNodeId = ''
  /** @type {HTMLTextAreaElement | undefined} */
  let taEl

  let slashOpen = false
  let slashStart = 0
  let slashFilter = ''

  const slashCommands = [
    { op: 'todo', label: 'TODO', match: 'todo' },
    { op: 'today', label: 'Today', match: 'today' },
    { op: 'h1', label: 'Heading 1', match: 'h1' },
    { op: 'h2', label: 'Heading 2', match: 'h2' },
    { op: 'h3', label: 'Heading 3', match: 'h3' },
    { op: 'code', label: 'Code block', match: 'code' }
  ]

  $: if (node.id !== lastNodeId) {
    lastNodeId = node.id
    local = node.content
    slashOpen = false
  }

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
    const before = v.slice(0, slashStart)
    const after = v.slice(pos)
    local = before + after
    slashOpen = false
    await flush()
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
  class:hasSlash={slashOpen}
  class:dragOver
  style="--depth: {depth}; padding-left: {4 + depth * 14}px; border-left-width: {depth > 0 ? 1 : 0}px"
  role="group"
  on:dragover={onDragOverRow}
  on:dragleave={onDragLeaveRow}
  on:drop={onDropRow}
>
  <div class="row-inner">
    <div class="row-controls" aria-hidden="false">
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
      rows="2"
      data-block-id={node.id}
      bind:value={local}
      on:input={onInput}
      on:blur={() => {
        flush()
        closeSlash()
      }}
      on:keydown={onKeydown}
    ></textarea>
    {#if slashOpen && filteredSlash.length}
      <div class="slash-menu" role="listbox" aria-label="Slash commands">
        {#each filteredSlash as cmd (cmd.op)}
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
    font-family: var(--dv-font, system-ui, sans-serif);
    font-size: 0.95rem;
    line-height: 1.45;
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
</style>
