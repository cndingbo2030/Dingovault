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

  /** @type {{ blockId: string, relPath?: string, sourcePath?: string, preview: string, score: number }[]} */
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

  /** @param {string} p */
  function pageLabel(p) {
    const s = String(p || '').split(/[/\\]/).pop() || p
    return s.replace(/\.md$/i, '')
  }

  /** @param {number | undefined} score */
  function scoreLabel(score) {
    if (score == null || Number.isNaN(score)) return ''
    return `${Math.round(score * 100)}%`
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
        {@const targetPath = h.relPath || h.sourcePath || ''}
        <li>
          <button type="button" class="hit" disabled={!targetPath} on:click={() => onOpenPage(targetPath)}>
            <span class="hit-head">
              <span class="path" title={targetPath}>{pageLabel(targetPath)}</span>
              {#if scoreLabel(h.score)}
                <span class="score">{scoreLabel(h.score)}</span>
              {/if}
            </span>
            <span class="preview">{h.preview}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .semantic-related {
    margin-top: 0;
    padding: 0;
  }
  .title {
    margin: 0 0 8px;
    font-size: 0.76rem;
    letter-spacing: 0;
    opacity: 0.62;
    font-weight: 600;
  }
  .muted {
    margin: 0;
    font-size: 0.8rem;
    opacity: 0.56;
    line-height: 1.45;
  }
  .hits {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .hit {
    display: block;
    width: 100%;
    text-align: left;
    min-height: 0;
    padding: 8px 10px;
    margin: 0;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    font: inherit;
    touch-action: manipulation;
  }
  .hit:hover {
    border-color: color-mix(in srgb, var(--dv-accent) 20%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .hit:disabled {
    cursor: default;
    opacity: 0.55;
  }
  .hit-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }
  .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: color-mix(in srgb, var(--dv-accent) 76%, var(--dv-fg));
    font-size: 0.8rem;
    font-weight: 520;
  }
  .score {
    padding: 1px 5px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    color: var(--dv-muted);
    font-size: 0.66rem;
    font-family: var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif);
    flex-shrink: 0;
  }
  .preview {
    display: -webkit-box;
    margin: 5px 0 0;
    color: color-mix(in srgb, var(--dv-fg) 72%, var(--dv-muted));
    font-size: 0.78rem;
    line-height: 1.42;
    line-clamp: 3;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
