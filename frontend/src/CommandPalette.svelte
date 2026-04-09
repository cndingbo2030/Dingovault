<script>
  import { tick } from 'svelte'
  import { fly, fade } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { ListVaultPages, SearchBlocks } from '../wailsjs/go/bridge/App.js'
  import { rankPagePaths } from './fuzzyMatch.js'
  import { readRecentPages } from './recentPages.js'
  import { messages, tr } from './lib/i18n/index.js'

  /** @type {boolean} */
  export let open = false
  /** @type {string} */
  export let notesRoot = ''
  /** @param {string} rel */
  export let onSelectPage = async (rel) => {}
  /** @param {any} hit */
  export let onSelectBlockHit = async (hit) => {}
  export let onClose = () => {}

  let query = ''
  /** @type {string[]} */
  let pageIndex = []
  let indexLoading = false
  /** @type {any[]} */
  let blockHits = []
  let blocksLoading = false
  let blockErr = ''
  let selectedIdx = 0
  let prevOpen = false
  /** @type {HTMLInputElement | undefined} */
  let inputEl

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @param {string} p */
  function pageTitle(p) {
    const seg = (p || '').split(/[/\\]/).pop() || p
    return seg.replace(/\.md$/i, '')
  }

  /** @param {string} abs */
  function relFromAbs(abs) {
    if (!abs || !notesRoot) return abs || ''
    const a = abs.replace(/\\/g, '/')
    const r = notesRoot.replace(/\\/g, '/').replace(/\/?$/, '/')
    if (a.length >= r.length && a.slice(0, r.length).toLowerCase() === r.toLowerCase()) {
      return a.slice(r.length).replace(/^\//, '')
    }
    return abs
  }

  /** @param {string} s */
  function escapeSnippet(s) {
    if (!s) return ''
    return s
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('«', '<mark>')
      .replaceAll('»', '</mark>')
  }

  /** @type {{ kind: 'page'|'block', path?: string, hit?: any, badge: string }[]} */
  $: rows = (() => {
    const q = query.trim()
    if (!q) {
      const recent = readRecentPages()
      return recent.slice(0, 14).map((path) => ({
        kind: /** @type {'page'} */ ('page'),
        path,
        badge: T('palette.badgeRecent')
      }))
    }
    const ranked = rankPagePaths(q, pageIndex, 10)
    const pageRows = ranked.map((r) => ({
      kind: /** @type {'page'} */ ('page'),
      path: r.path,
      badge: T('palette.badgePage')
    }))
    const blockRows = blockHits.slice(0, 18).map((h) => ({
      kind: /** @type {'block'} */ ('block'),
      hit: h,
      badge: T('palette.badgeBlock')
    }))
    return [...pageRows, ...blockRows]
  })()

  $: if (selectedIdx >= rows.length) selectedIdx = Math.max(0, rows.length - 1)

  async function loadPageIndex() {
    indexLoading = true
    try {
      pageIndex = await ListVaultPages()
    } catch {
      pageIndex = []
    } finally {
      indexLoading = false
    }
  }

  let ftsTimer = 0
  let ftsSeq = 0

  /** @param {string} q */
  function scheduleBlockSearch(q) {
    if (!open) return
    if (ftsTimer) clearTimeout(ftsTimer)
    if (!q.trim()) {
      blockHits = []
      blocksLoading = false
      blockErr = ''
      return
    }
    blocksLoading = true
    blockErr = ''
    const my = ++ftsSeq
    ftsTimer = window.setTimeout(async () => {
      try {
        const hits = await SearchBlocks(q)
        if (my !== ftsSeq) return
        blockHits = hits || []
      } catch (e) {
        if (my !== ftsSeq) return
        blockErr = String(e)
        blockHits = []
      } finally {
        if (my === ftsSeq) blocksLoading = false
      }
    }, 90)
  }

  $: {
    if (open && !prevOpen) {
      prevOpen = true
      query = ''
      selectedIdx = 0
      blockHits = []
      blocksLoading = false
      blockErr = ''
      void (async () => {
        await loadPageIndex()
        await tick()
        inputEl?.focus()
        inputEl?.select()
      })()
    } else if (!open) {
      prevOpen = false
    }
  }

  $: {
    if (open) scheduleBlockSearch(query.trim())
    else {
      if (ftsTimer) clearTimeout(ftsTimer)
      blockHits = []
      blocksLoading = false
      blockErr = ''
    }
  }

  /** @param {KeyboardEvent} e */
  function onInputKeydown(e) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (rows.length) selectedIdx = (selectedIdx + 1) % rows.length
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (rows.length) selectedIdx = (selectedIdx - 1 + rows.length) % rows.length
    } else if (e.key === 'Enter') {
      e.preventDefault()
      void activateRow(rows[selectedIdx])
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }

  /** @param {typeof rows[0] | undefined} row */
  async function activateRow(row) {
    if (!row) return
    if (row.kind === 'page' && row.path) {
      await onSelectPage(row.path)
      onClose()
      return
    }
    if (row.kind === 'block' && row.hit) {
      await onSelectBlockHit(row.hit)
      onClose()
    }
  }
</script>

{#if open}
  <div
    class="palette-backdrop"
    role="presentation"
    tabindex="-1"
    transition:fade={{ duration: 140 }}
    on:click={onClose}
    on:keydown|stopPropagation
  ></div>
  <div
    class="palette"
    role="dialog"
    aria-modal="true"
    aria-label={T('palette.aria')}
    transition:fly={{ y: -10, duration: 220, easing: cubicOut }}
  >
    <div class="palette-head">
      <span class="kbd-hint">{T('palette.hint')}</span>
    </div>
    <input
      bind:this={inputEl}
      class="palette-input"
      placeholder={T('palette.placeholder')}
      bind:value={query}
      on:keydown={onInputKeydown}
      autocomplete="off"
      spellcheck="false"
    />
    {#if indexLoading}
      <div class="palette-status">{T('palette.indexingPages')}</div>
    {/if}
    {#if blocksLoading && query.trim()}
      <div class="palette-status subtle">{T('palette.searchingBlocks')}</div>
    {/if}
    {#if blockErr && query.trim()}
      <div class="palette-err">{blockErr}</div>
    {/if}

    <ul class="hits" role="listbox" aria-label="Results">
      {#each rows as row, i (row.kind === 'page' ? 'p:' + row.path : 'b:' + (row.hit?.id ?? i))}
        <li class="hit-li" role="none">
          <button
            type="button"
            class="hit"
            class:active={i === selectedIdx}
            role="option"
            aria-selected={i === selectedIdx}
            on:mouseenter={() => (selectedIdx = i)}
            on:click={() => void activateRow(row)}
          >
            <span class="badge">{row.badge}</span>
            {#if row.kind === 'page'}
              <div class="hit-main">
                <span class="hit-title">{pageTitle(row.path || '')}</span>
                <span class="hit-path">{row.path}</span>
              </div>
            {:else if row.hit}
              <div class="hit-main">
                <span class="hit-title">{@html escapeSnippet(row.hit.snippet)}</span>
                <span class="hit-path">{relFromAbs(row.hit.sourcePath)}</span>
              </div>
            {/if}
          </button>
        </li>
      {/each}
    </ul>

    {#if !indexLoading && rows.length === 0 && !query.trim()}
      <div class="palette-empty palette-empty-rich">
        <div class="pe-icon" aria-hidden="true">
          <svg viewBox="0 0 48 48" width="40" height="40"><path d="M14 20h20M14 28h14" stroke="currentColor" stroke-width="2" stroke-linecap="round" opacity="0.35"/><circle cx="34" cy="16" r="3" fill="currentColor" opacity="0.15"/></svg>
        </div>
        <p class="pe-title">{T('palette.emptyRecents')}</p>
      </div>
    {/if}
    {#if query.trim() && !blocksLoading && rows.length === 0 && !blockErr}
      <div class="palette-empty palette-empty-rich">
        <div class="pe-icon" aria-hidden="true">
          <svg viewBox="0 0 48 48" width="40" height="40"><circle cx="22" cy="22" r="9" stroke="currentColor" stroke-width="2" fill="none" opacity="0.35"/><path d="M32 32l8 8" stroke="currentColor" stroke-width="2" stroke-linecap="round" opacity="0.35"/></svg>
        </div>
        <p class="pe-title">{T('palette.emptySearchTitle')}</p>
        <p class="pe-hint">{T('palette.emptySearchHint')}</p>
      </div>
    {/if}
  </div>
{/if}

<style>
  .palette-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 40;
  }
  .palette {
    position: fixed;
    left: 50%;
    top: 16%;
    transform: translateX(-50%);
    width: min(580px, calc(100vw - 24px));
    max-width: calc(100vw - 24px);
    max-height: 72vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    z-index: 50;
    background: var(--dv-panel, #1a1a20);
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.1));
    border-radius: 12px;
    padding: 10px 12px 12px;
    box-shadow: 0 28px 90px rgba(0, 0, 0, 0.5);
  }
  .palette-head {
    margin-bottom: 8px;
  }
  .kbd-hint {
    font-size: 0.68rem;
    opacity: 0.45;
    letter-spacing: 0.02em;
  }
  .palette-input {
    width: 100%;
    box-sizing: border-box;
    padding: 11px 12px;
    border-radius: 8px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: var(--dv-input, #121218);
    color: inherit;
    outline: none;
    font-size: 0.95rem;
  }
  .palette-input:focus {
    border-color: rgba(120, 160, 255, 0.45);
  }
  .palette-status {
    margin-top: 8px;
    font-size: 0.8rem;
    opacity: 0.65;
  }
  .palette-status.subtle {
    opacity: 0.45;
    font-size: 0.72rem;
  }
  .palette-err {
    margin-top: 8px;
    font-size: 0.8rem;
    color: #f87171;
  }
  .palette-empty {
    margin-top: 12px;
    font-size: 0.85rem;
    opacity: 0.5;
    padding: 8px 4px;
  }
  .palette-empty-rich {
    text-align: center;
    padding: 16px 12px;
    opacity: 1;
    border-radius: 10px;
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    border: 1px dashed color-mix(in srgb, var(--dv-fg) 12%, transparent);
  }
  .pe-icon {
    color: var(--dv-fg);
    opacity: 0.5;
    margin-bottom: 8px;
  }
  .pe-title {
    margin: 0;
    font-size: 0.88rem;
    font-weight: 500;
    opacity: 0.85;
  }
  .pe-hint {
    margin: 8px 0 0;
    font-size: 0.78rem;
    line-height: 1.45;
    opacity: 0.55;
  }
  .hits {
    list-style: none;
    margin: 10px 0 0;
    padding: 0;
    overflow-y: auto;
    max-height: min(52vh, 420px);
  }
  .hit-li {
    margin: 0;
    padding: 0;
  }
  .hit {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    width: 100%;
    box-sizing: border-box;
    padding: 9px 8px;
    border-radius: 8px;
    cursor: pointer;
    border: 1px solid transparent;
    margin: 0;
    text-align: left;
    font: inherit;
    color: inherit;
    background: transparent;
  }
  .hit:hover,
  .hit.active {
    background: var(--dv-hit-hover, rgba(255, 255, 255, 0.06));
  }
  .hit.active {
    border-color: rgba(120, 160, 255, 0.25);
  }
  .badge {
    flex-shrink: 0;
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.4;
    margin-top: 3px;
    min-width: 2.8rem;
  }
  .hit-main {
    flex: 1;
    min-width: 0;
  }
  .hit-title {
    display: block;
    font-size: 0.88rem;
    line-height: 1.35;
    word-break: break-word;
  }
  .hit-path {
    display: block;
    font-size: 0.7rem;
    opacity: 0.5;
    margin-top: 3px;
    word-break: break-all;
  }
  .hit-title :global(mark) {
    background: rgba(250, 204, 21, 0.22);
    color: inherit;
    padding: 0 2px;
    border-radius: 2px;
  }
</style>
