<script>
  import { GetSemanticRelatedForPage } from '../wailsjs/go/bridge/App.js'
  import { messages, tr } from './lib/i18n/index.js'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @type {string} */
  export let pagePath = ''
  export let indexEpoch = 0
  /** @param {string} rel */
  export let onOpenPage = async (rel) => {}

  /** @type {{ blockId: string, relPath: string, preview: string, score: number }[]} */
  let items = []
  let loading = false
  let seq = 0

  async function refresh() {
    const p = (pagePath || '').trim()
    if (!p) {
      items = []
      return
    }
    const my = ++seq
    loading = true
    try {
      const rows = await GetSemanticRelatedForPage(p, 12)
      if (my !== seq) return
      items = Array.isArray(rows) ? rows : []
    } catch {
      if (my === seq) items = []
    } finally {
      if (my === seq) loading = false
    }
  }

  let prevPath = ''
  let prevEpoch = -1
  $: {
    const p = (pagePath || '').trim()
    const ep = indexEpoch
    if (p !== prevPath || ep !== prevEpoch) {
      prevPath = p
      prevEpoch = ep
      void refresh()
    }
  }
</script>

<section class="semantic-related" aria-label={T('semantic.aria')}>
  <h2 class="title">{T('semantic.title')}</h2>
  {#if loading}
    <p class="muted">{T('semantic.loading')}</p>
  {:else if !items.length}
    <p class="muted">{T('semantic.empty')}</p>
  {:else}
    <ul class="hits">
      {#each items as h (h.blockId + h.relPath)}
        <li>
          <button type="button" class="hit" on:click={() => onOpenPage(h.relPath || h.sourcePath || '')}>
            <span class="path">{h.relPath || h.sourcePath}</span>
            <span class="score">{h.score != null ? h.score.toFixed(2) : ''}</span>
          </button>
          <p class="preview">{h.preview}</p>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .semantic-related {
    margin-top: 20px;
    padding: 14px 16px;
    border-radius: 10px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
  }
  .title {
    margin: 0 0 10px;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
  }
  .muted {
    margin: 0;
    font-size: 0.85rem;
    opacity: 0.55;
  }
  .hits {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .hit {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    width: 100%;
    text-align: left;
    padding: 6px 8px;
    margin: 0;
    border: none;
    border-radius: 6px;
    background: rgba(120, 160, 255, 0.08);
    color: #b4c8ff;
    cursor: pointer;
    font: inherit;
  }
  .hit:hover {
    background: rgba(120, 160, 255, 0.16);
  }
  .path {
    font-size: 0.82rem;
    word-break: break-all;
  }
  .score {
    font-size: 0.72rem;
    opacity: 0.55;
    font-family: var(--dv-font-mono, ui-monospace, monospace);
    flex-shrink: 0;
  }
  .preview {
    margin: 4px 0 0;
    font-size: 0.8rem;
    opacity: 0.72;
    line-height: 1.45;
  }
</style>
