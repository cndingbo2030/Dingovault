<script>
  import { GetBacklinks } from '../wailsjs/go/bridge/App.js'
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
  /** Bumps when the index changes (any file) to refresh backlinks in place. */
  export let indexEpoch = 0
  /** @param {string} rel */
  export let onOpenPage = async (rel) => {}

  /** @type {any[]} */
  let items = []
  let loading = false

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

  /** @param {string} content */
  function preview(content) {
    const s = (content || '').replace(/\s+/g, ' ').trim()
    return s.length > 160 ? s.slice(0, 157) + '…' : s
  }

  let seq = 0
  async function refresh() {
    const p = pagePath.trim()
    if (!p) {
      items = []
      return
    }
    const my = ++seq
    loading = true
    try {
      const blocks = await GetBacklinks(p)
      if (my !== seq) return
      items = blocks || []
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
      if (!p) {
        items = []
        loading = false
      } else {
        void refresh()
      }
    }
  }
</script>

<section class="backlinks" aria-label={T('backlinks.aria')}>
  <h2 class="title">{T('backlinks.title')}</h2>
  {#if loading}
    <p class="muted">{T('backlinks.loading')}</p>
  {:else if items.length === 0}
    <div class="bl-empty">
      <div class="bl-empty-icon" aria-hidden="true">
        <svg viewBox="0 0 56 48" width="48" height="40"><path d="M12 16h32M12 26h22M12 36h28" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" opacity="0.3"/><path d="M38 12 L44 22 L32 22 Z" fill="currentColor" opacity="0.12"/></svg>
      </div>
      <p class="bl-empty-title">{T('backlinks.emptyTitle')}</p>
      <p class="bl-hint">{T('backlinks.emptyHint')}</p>
    </div>
  {:else}
    <ul class="list">
      {#each items as b (b.id)}
        <li class="row">
          <button
            type="button"
            class="link"
            on:click={() => onOpenPage(relFromAbs(b.metadata?.sourcePath || ''))}
          >
            <span class="path">{relFromAbs(b.metadata?.sourcePath || '')}</span>
            <span class="snippet">{preview(b.content)}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .backlinks {
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
  .bl-empty {
    text-align: center;
    padding: 12px 8px 8px;
  }
  .bl-empty-icon {
    color: inherit;
    opacity: 0.45;
    margin-bottom: 6px;
  }
  .bl-empty-title {
    margin: 0 0 6px;
    font-size: 0.9rem;
    font-weight: 500;
    opacity: 0.8;
  }
  .bl-hint {
    margin: 0;
    font-size: 0.78rem;
    line-height: 1.45;
    opacity: 0.5;
    max-width: 40ch;
    margin-left: auto;
    margin-right: auto;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row + .row {
    border-top: 1px solid color-mix(in srgb, var(--dv-fg) 7%, transparent);
  }
  .link {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 10px;
    margin: 0;
    border: 1px solid transparent;
    background: transparent;
    color: inherit;
    cursor: pointer;
    border-radius: 5px;
  }
  .link:hover {
    border-color: color-mix(in srgb, var(--dv-accent) 20%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .path {
    display: block;
    color: color-mix(in srgb, var(--dv-accent) 72%, var(--dv-fg));
    font-size: 0.78rem;
    font-weight: 520;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-bottom: 4px;
  }
  .snippet {
    display: -webkit-box;
    color: color-mix(in srgb, var(--dv-fg) 76%, var(--dv-muted));
    font-size: 0.78rem;
    line-height: 1.42;
    line-clamp: 3;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
