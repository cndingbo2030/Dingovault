<script>
  import { QueryBlocks } from '../wailsjs/go/bridge/App.js'
  import { messages, tr } from './lib/i18n/index.js'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @type {string} */
  export let notesRoot = ''
  /** @type {string} */
  export let pagePath = ''
  export let indexEpoch = 0
  /** @param {string} rel @param {string} blockId */
  export let onOpenBlock = async (rel, blockId) => {}
  /** @param {string} blockId @param {string} command @param {string} rel */
  export let onRerunCommand = async (blockId, command, rel) => {}

  /** @type {'page' | 'vault'} */
  let scope = 'page'
  let failedOnly = false
  /** @type {any[]} */
  let rows = []
  let loading = false
  let seq = 0

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

  /** @param {string} value */
  function normPath(value) {
    return String(value || '').replace(/\\/g, '/').replace(/^\//, '').toLowerCase()
  }

  /** @param {string | number | undefined} value */
  function exitNumber(value) {
    const n = Number(value)
    return Number.isFinite(n) ? n : 0
  }

  /** @param {string | undefined} value */
  function timeMs(value) {
    const n = value ? Date.parse(value) : Number.NaN
    return Number.isFinite(n) ? n : 0
  }

  /** @param {any} block */
  function historyRow(block) {
    const props = block?.properties || {}
    const rel = relFromAbs(block?.metadata?.sourcePath || '')
    const exitCode = exitNumber(props.exitCode)
    const command = String(props.command || '').trim()
    return {
      id: String(block?.id || ''),
      parentId: String(block?.parentId || ''),
      rel,
      command,
      exitCode,
      ranAt: String(props.ranAt || ''),
      ranAtMs: timeMs(props.ranAt),
      durationMs: Number(props.durationMs || 0),
      runId: String(props.runId || block?.id || ''),
      content: String(block?.content || '')
    }
  }

  /** @param {any[]} blocks */
  function normalizeRows(blocks) {
    return (blocks || [])
      .map(historyRow)
      .filter((row) => row.id && row.command)
      .sort((a, b) => (b.ranAtMs || 0) - (a.ranAtMs || 0) || b.id.localeCompare(a.id))
  }

  async function refresh() {
    const my = ++seq
    loading = true
    try {
      const blocks = await QueryBlocks('source:terminal')
      if (my !== seq) return
      rows = normalizeRows(blocks)
    } catch {
      if (my === seq) rows = []
    } finally {
      if (my === seq) loading = false
    }
  }

  /** @param {string} command */
  function commandPreview(command) {
    const s = String(command || '').replace(/\s+/g, ' ').trim()
    if (!s) return T('runHistory.commandFallback')
    return s.length > 92 ? s.slice(0, 91) + '…' : s
  }

  /** @param {number} exitCode */
  function exitLabel(exitCode) {
    return exitCode === 0 ? T('runHistory.ok') : T('runHistory.exitCode', { code: exitCode })
  }

  /** @param {string} rel */
  function pageLabel(rel) {
    const name = String(rel || '').split(/[/\\]/).pop() || rel || T('runHistory.unknownPath')
    return name.replace(/\.md$/i, '')
  }

  /** @param {number} ms */
  function relativeTime(ms) {
    if (!ms) return ''
    const diff = Math.max(0, Date.now() - ms)
    const minute = 60 * 1000
    const hour = 60 * minute
    const day = 24 * hour
    if (diff < minute) return T('runHistory.now')
    if (diff < hour) return `${Math.floor(diff / minute)}m`
    if (diff < day) return `${Math.floor(diff / hour)}h`
    return `${Math.floor(diff / day)}d`
  }

  /** @param {number} ms */
  function durationLabel(ms) {
    if (!ms) return ''
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(ms < 10000 ? 1 : 0)}s`
  }

  /** @param {any} row */
  async function openRow(row) {
    await onOpenBlock(row.rel, row.parentId || row.id)
  }

  /** @param {any} row */
  async function rerunRow(row) {
    await onRerunCommand(row.parentId || row.id, row.command, row.rel)
    await refresh()
  }

  function toggleFailedRuns() {
    if (failedOnly) {
      failedOnly = false
      return
    }
    scope = 'vault'
    failedOnly = true
  }

  let prevEpoch = -1
  $: {
    const ep = indexEpoch
    if (ep !== prevEpoch) {
      prevEpoch = ep
      void refresh()
    }
  }

  $: currentRows = rows.filter((row) => normPath(row.rel) === normPath(pagePath))
  $: scopedRows = scope === 'page' ? currentRows : rows
  $: vaultFailedRows = rows.filter((row) => row.exitCode !== 0)
  $: visibleRows = failedOnly ? vaultFailedRows : scopedRows
</script>

<section class="run-history" aria-label={T('runHistory.aria')}>
  <div class="head">
    <div>
      <h2>{T('runHistory.title')}</h2>
      <p>{T('runHistory.subtitle')}</p>
    </div>
    <button type="button" class="refresh" title={T('runHistory.refresh')} aria-label={T('runHistory.refresh')} on:click={() => void refresh()}>
      ↻
    </button>
  </div>

  <div class="filters" role="toolbar" aria-label={T('runHistory.filters')}>
    <button type="button" class:active={scope === 'page'} on:click={() => (scope = 'page')}>{T('runHistory.scopePage')}</button>
    <button type="button" class:active={scope === 'vault'} on:click={() => (scope = 'vault')}>{T('runHistory.scopeVault')}</button>
    <button type="button" class="failed" class:active={failedOnly} on:click={toggleFailedRuns}>
      {T('runHistory.failedOnly')} <span>{vaultFailedRows.length}</span>
    </button>
  </div>

  {#if loading}
    <p class="muted">{T('runHistory.loading')}</p>
  {:else if visibleRows.length === 0}
    <div class="empty">
      <p class="empty-title">{failedOnly ? T('runHistory.emptyFailedTitle') : T('runHistory.emptyTitle')}</p>
      <p>{failedOnly ? T('runHistory.emptyFailedHint') : T('runHistory.emptyHint')}</p>
    </div>
  {:else}
    <ul class="runs">
      {#each visibleRows as row (row.runId)}
        <li>
          <button type="button" class="run-row" on:click={() => void openRow(row)}>
            <span class="topline">
              <span class="page" title={row.rel}>{pageLabel(row.rel)}</span>
              <span class="status" class:failed={row.exitCode !== 0}>{exitLabel(row.exitCode)}</span>
            </span>
            <span class="command" title={row.command}>{commandPreview(row.command)}</span>
            <span class="meta">
              {#if relativeTime(row.ranAtMs)}
                <span>{relativeTime(row.ranAtMs)}</span>
              {/if}
              {#if durationLabel(row.durationMs)}
                <span>{durationLabel(row.durationMs)}</span>
              {/if}
              <span>{T('runHistory.openSource')}</span>
            </span>
          </button>
          <button type="button" class="rerun" title={T('runHistory.rerun')} aria-label={T('runHistory.rerun')} on:click|stopPropagation={() => void rerunRow(row)}>
            ▶
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .run-history {
    margin: 0;
    padding: 0;
  }
  .head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 10px;
  }
  h2 {
    margin: 0;
    font-size: 0.76rem;
    letter-spacing: 0;
    opacity: 0.68;
    font-weight: 650;
  }
  .head p {
    margin: 3px 0 0;
    color: var(--dv-muted);
    font-size: 0.72rem;
    line-height: 1.35;
  }
  .refresh {
    width: 28px;
    height: 26px;
    border: 1px solid var(--dv-border);
    border-radius: 6px;
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    color: var(--dv-muted);
    cursor: pointer;
    flex-shrink: 0;
  }
  .filters {
    display: grid;
    grid-template-columns: 1fr 1fr 1.1fr;
    gap: 4px;
    margin-bottom: 10px;
  }
  .filters button {
    min-width: 0;
    min-height: 28px;
    border: 1px solid var(--dv-border);
    border-radius: 6px;
    background: transparent;
    color: var(--dv-muted);
    font: inherit;
    font-size: 0.72rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .filters button.active {
    color: var(--dv-fg);
    border-color: color-mix(in srgb, var(--dv-accent) 34%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-accent) 10%, transparent);
  }
  .filters .failed span {
    margin-left: 4px;
    color: color-mix(in srgb, var(--dv-danger, #d64f4f) 70%, var(--dv-muted));
  }
  .muted {
    margin: 0;
    color: var(--dv-muted);
    font-size: 0.8rem;
    line-height: 1.45;
  }
  .empty {
    padding: 18px 8px;
    text-align: center;
    color: var(--dv-muted);
    font-size: 0.78rem;
    line-height: 1.45;
  }
  .empty-title {
    margin: 0 0 5px;
    color: var(--dv-fg);
    font-size: 0.86rem;
    font-weight: 560;
  }
  .empty p:last-child {
    margin: 0;
  }
  .runs {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  li {
    position: relative;
  }
  .run-row {
    display: block;
    width: 100%;
    min-height: 74px;
    padding: 8px 34px 8px 9px;
    border: 1px solid transparent;
    border-radius: 7px;
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .run-row:hover {
    border-color: color-mix(in srgb, var(--dv-accent) 24%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
  }
  .topline {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }
  .page {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: color-mix(in srgb, var(--dv-accent) 70%, var(--dv-fg));
    font-size: 0.76rem;
    font-weight: 620;
  }
  .status {
    flex-shrink: 0;
    padding: 1px 5px;
    border-radius: 999px;
    background: color-mix(in srgb, #1f9d66 14%, transparent);
    color: color-mix(in srgb, #1f9d66 75%, var(--dv-fg));
    font-size: 0.66rem;
    font-weight: 650;
  }
  .status.failed {
    background: color-mix(in srgb, #d64f4f 14%, transparent);
    color: color-mix(in srgb, #d64f4f 78%, var(--dv-fg));
  }
  .command {
    display: block;
    margin-top: 6px;
    color: var(--dv-fg);
    font-family: var(--dv-mono, 'SFMono-Regular', Consolas, monospace);
    font-size: 0.76rem;
    line-height: 1.35;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    color: var(--dv-muted);
    font-size: 0.68rem;
  }
  .meta span + span::before {
    content: '·';
    margin-right: 6px;
    opacity: 0.55;
  }
  .rerun {
    position: absolute;
    right: 6px;
    top: 28px;
    width: 25px;
    height: 25px;
    border: 1px solid var(--dv-border);
    border-radius: 6px;
    background: color-mix(in srgb, var(--dv-panel) 92%, transparent);
    color: var(--dv-muted);
    font-size: 0.72rem;
    cursor: pointer;
    opacity: 0.82;
  }
  li:hover .rerun,
  .rerun:hover {
    opacity: 1;
    color: var(--dv-fg);
    border-color: color-mix(in srgb, var(--dv-accent) 30%, var(--dv-border));
  }
</style>
