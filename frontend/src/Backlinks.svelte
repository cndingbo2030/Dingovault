<script>
  import { GetBacklinks } from '../wailsjs/go/bridge/App.js'

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

<section class="backlinks" aria-label="Linked references">
  <h2 class="title">Linked references</h2>
  {#if loading}
    <p class="muted">Loading…</p>
  {:else if items.length === 0}
    <p class="muted">No other pages link here yet.</p>
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
    margin-top: 28px;
    padding: 16px;
    background: rgba(255, 255, 255, 0.02);
    border-radius: 10px;
    border: 1px solid rgba(255, 255, 255, 0.06);
  }
  .title {
    margin: 0 0 12px;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
    font-weight: 600;
  }
  .muted {
    margin: 0;
    font-size: 0.88rem;
    opacity: 0.5;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row + .row {
    border-top: 1px solid rgba(255, 255, 255, 0.06);
  }
  .link {
    display: block;
    width: 100%;
    text-align: left;
    padding: 10px 4px;
    margin: 0;
    border: none;
    background: transparent;
    color: inherit;
    cursor: pointer;
    border-radius: 6px;
  }
  .link:hover {
    background: rgba(255, 255, 255, 0.04);
  }
  .path {
    display: block;
    font-size: 0.72rem;
    opacity: 0.55;
    word-break: break-all;
    margin-bottom: 4px;
  }
  .snippet {
    display: block;
    font-size: 0.88rem;
    line-height: 1.45;
    opacity: 0.92;
  }
</style>
