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
  import { fly, fade } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
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
    GetSemanticGraphEdges,
    ReorderBlockBefore,
    GetAppVersion,
    GetLocale,
    SetLocale,
    IsAIReachable
  } from '../wailsjs/go/bridge/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { locale, messages, tr, detectBrowserLocale, normalizeLocaleTag } from './lib/i18n/index.js'
  import OutlineNode from './OutlineNode.svelte'
  import PageGraph from './PageGraph.svelte'
  import Backlinks from './Backlinks.svelte'
  import SemanticRelated from './SemanticRelated.svelte'
  import AIChatPanel from './AIChatPanel.svelte'
  import CommandPalette from './CommandPalette.svelte'
  import ToastStack from './ToastStack.svelte'
  import { touchRecentPage } from './recentPages.js'
  import { pushToast } from './toastStore.js'
  import { toolbarEntries, sidebarEntries } from './pluginRegistry.js'

  let notesRoot = ''
  let pagePath = 'README.md'
  /** @type {any[]} */
  let roots = []
  let paletteOpen = false
  let err = ''
  let lastFileEvent = ''
  let indexEpoch = 0
  let pageLoading = false
  let indexPulse = false
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  let pulseTimer

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @type {Record<string, boolean>} */
  let collapsedState = {}
  /** @type {string[]} */
  let selectedIds = []
  let graphOpen = false
  let aboutOpen = false
  /** @type {'backlinks' | 'ai'} */
  let sideTab = 'backlinks'
  /** @type {'outline' | 'related' | 'side'} */
  let mobilePanel = 'outline'
  let sideSheetOpen = false
  /** @type {string[]} */
  let navStack = []
  /** @type {'default' | 'phone-portrait' | 'phone-land' | 'tablet-land' | 'small-tablet'} */
  let chromeMode = 'default'

  $: showMobileChrome =
    chromeMode === 'phone-portrait' ||
    chromeMode === 'phone-land' ||
    chromeMode === 'small-tablet'

  /** @type {boolean} */
  let aiReachable = true

  async function refreshAIReach() {
    try {
      aiReachable = await IsAIReachable()
    } catch {
      aiReachable = false
    }
  }

  $: if (typeof window !== 'undefined' && sideTab === 'ai') {
    void refreshAIReach()
  }

  function syncChromeMode() {
    if (typeof window === 'undefined') return
    const w = window.innerWidth
    const h = window.innerHeight
    const land = w >= h
    if (land && w >= 900) {
      chromeMode = 'tablet-land'
    } else if (w >= 600 && w < 900) {
      chromeMode = 'small-tablet'
    } else if (!land && w <= 599) {
      chromeMode = 'phone-portrait'
    } else if (land && w < 600) {
      chromeMode = 'phone-land'
    } else {
      chromeMode = 'default'
    }
  }

  $: if (chromeMode !== 'small-tablet') sideSheetOpen = false
  /** @type {string} */
  let appVersion = ''
  /** @type {{ nodes: { id: string, label: string }[], edges: { source: string, target: string }[] }} */
  let graphData = { nodes: [], edges: [] }
  /** @type {{ source: string, target: string, score: number }[]} */
  let graphSemanticEdges = []
  let graphSemanticOn = false

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
    if (!root) return T('app.vault')
    const p = root.replace(/[/\\]+$/, '')
    const parts = p.split(/[/\\]/).filter(Boolean)
    return parts.length ? parts[parts.length - 1] : T('app.vault')
  }

  /** @param {string} code */
  async function setLanguage(code) {
    err = ''
    const n = normalizeLocaleTag(code)
    try {
      await SetLocale(n === 'zh-CN' ? 'zh-CN' : 'en')
    } catch (e) {
      notifyErr(e)
      return
    }
    locale.set(n)
    document.documentElement.lang = n === 'zh-CN' ? 'zh-CN' : 'en'
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
      pushToast(T('app.copiedBlocks', { count: lines.length }), 'info')
    } catch {
      pushToast(T('app.clipboardFailed'), 'error')
    }
  }

  async function openGraph() {
    err = ''
    try {
      graphData = await GetWikiGraph()
      graphSemanticEdges = []
      graphSemanticOn = false
      graphOpen = true
    } catch (e) {
      notifyErr(e)
    }
  }

  async function toggleSemanticGraph() {
    if (!graphSemanticOn) {
      graphSemanticEdges = []
      return
    }
    err = ''
    try {
      graphSemanticEdges = await GetSemanticGraphEdges()
    } catch (e) {
      graphSemanticOn = false
      graphSemanticEdges = []
      notifyErr(e)
    }
  }

  async function openAbout() {
    err = ''
    try {
      appVersion = await GetAppVersion()
    } catch {
      appVersion = ''
    }
    aboutOpen = true
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

  /** @param {string} id */
  async function handleSwipeTodo(id) {
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await CycleBlockTodo(id)
      await loadPage(pagePath, { focusBlockId: id })
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id */
  async function handleSwipeClear(id) {
    if (typeof window !== 'undefined' && !window.confirm(T('app.confirmClearBlock'))) return
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await UpdateBlock(id, '')
      await loadPage(pagePath, { focusBlockId: id })
    } catch (e) {
      notifyErr(e)
    }
  }

  function goBackAndroid() {
    if (paletteOpen) {
      paletteOpen = false
      return true
    }
    if (aboutOpen) {
      aboutOpen = false
      return true
    }
    if (graphOpen) {
      graphOpen = false
      return true
    }
    if (chromeMode === 'small-tablet' && sideSheetOpen) {
      sideSheetOpen = false
      return true
    }
    if (navStack.length > 1) {
      const prev = navStack[navStack.length - 2]
      navStack = navStack.slice(0, -1)
      void loadPage(prev, { skipHistory: true })
      return true
    }
    return false
  }

  onMount(() => {
    document.documentElement.style.setProperty('--dv-font', "var(--dv-font-sans, 'Inter', system-ui, sans-serif)")

    syncChromeMode()
    const ro =
      typeof ResizeObserver !== 'undefined' ? new ResizeObserver(() => syncChromeMode()) : null
    ro?.observe(document.documentElement)
    window.addEventListener('orientationchange', syncChromeMode)

    void (async () => {
      try {
        let loc = await GetLocale()
        if (!loc) {
          loc = detectBrowserLocale()
          await SetLocale(loc)
        }
        const n = normalizeLocaleTag(loc)
        locale.set(n)
        document.documentElement.lang = n === 'zh-CN' ? 'zh-CN' : 'en'
      } catch {
        const fb = detectBrowserLocale()
        locale.set(fb)
        document.documentElement.lang = fb === 'zh-CN' ? 'zh-CN' : 'en'
      }
    })()

    try {
      const cachedTheme = localStorage.getItem('dingovault-theme')
      if (cachedTheme === 'light' || cachedTheme === 'dark') {
        theme = cachedTheme
      }
    } catch {
      // Ignore storage errors.
    }

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
        void refreshAIReach()
        return loadPage(pagePath)
      })
      .catch((e) => notifyErr(e))
      .finally(() => {
        const w = typeof window !== 'undefined' ? /** @type {any} */ (window) : null
        if (w && typeof w.__dingoMarkFrontendReady === 'function') w.__dingoMarkFrontendReady()
      })

    EventsOn('ai-inline-chunk', (/** @type {any} */ payload) => {
      window.dispatchEvent(new CustomEvent('dv-ai-chunk', { detail: payload }))
    })
    EventsOn('ai-inline-error', (/** @type {any} */ payload) => {
      window.dispatchEvent(new CustomEvent('dv-ai-err', { detail: payload }))
    })
    EventsOn('ai-inline-done', (/** @type {any} */ payload) => {
      window.dispatchEvent(new CustomEvent('dv-ai-done', { detail: payload }))
    })

    EventsOn('file-updated', async (payload) => {
      indexEpoch++
      if (pulseTimer) clearTimeout(pulseTimer)
      indexPulse = true
      pulseTimer = setTimeout(() => {
        indexPulse = false
      }, 1200)
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
      await loadPage(pagePath, { skipHistory: true, softNav: true })
    })

    if (typeof window !== 'undefined') {
      const w = /** @type {any} */ (window)
      w.__dingoConsumeAndroidBack = () => goBackAndroid()
    }

    /** @param {FocusEvent} e */
    const onFocusIn = (e) => {
      const t = e.target
      if (t instanceof HTMLTextAreaElement && t.closest('.outliner-panel')) {
        requestAnimationFrame(() => {
          t.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
        })
      }
    }
    document.addEventListener('focusin', onFocusIn)

    const vv = typeof window !== 'undefined' ? window.visualViewport : null
    const onVV = () => {
      if (!vv) return
      const inset = Math.max(0, window.innerHeight - vv.height - vv.offsetTop)
      document.documentElement.style.setProperty('--dv-keyboard-inset', `${inset}px`)
    }
    if (vv) {
      onVV()
      vv.addEventListener('resize', onVV)
      vv.addEventListener('scroll', onVV)
    }

    /** @param {KeyboardEvent} e */
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        paletteOpen = !paletteOpen
      }
      if (e.key === 'Escape') paletteOpen = false
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('orientationchange', syncChromeMode)
      document.removeEventListener('focusin', onFocusIn)
      if (vv) {
        vv.removeEventListener('resize', onVV)
        vv.removeEventListener('scroll', onVV)
      }
      if (typeof window !== 'undefined') {
        const w = /** @type {any} */ (window)
        if (w.__dingoConsumeAndroidBack) delete w.__dingoConsumeAndroidBack
      }
      ro?.disconnect()
    }
  })

  /**
   * @param {string} rel
   * @param {{ focusBlockId?: string, caretOffset?: number, skipHistory?: boolean, replaceTop?: boolean, softNav?: boolean }} [opts]
   */
  async function loadPage(rel, opts) {
    const focusId = opts?.focusBlockId
    const caret = opts?.caretOffset
    const skipHist = opts?.skipHistory
    const replaceTop = opts?.replaceTop
    const softNav = !!opts?.softNav && rel === pagePath && roots.length > 0
    if (!skipHist) {
      if (navStack.length === 0) {
        navStack = [rel]
      } else if (replaceTop) {
        navStack = [...navStack.slice(0, -1), rel]
      } else if (navStack[navStack.length - 1] !== rel) {
        navStack = [...navStack, rel]
      }
    }
    err = ''
    if (!softNav) pageLoading = true
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
    } finally {
      pageLoading = false
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
      localStorage.setItem('dingovault-theme', next)
    } catch {
      // Ignore storage errors.
    }
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
        if (!confirm(T('app.createPage', { path: rel }))) return
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

<main
  class="layout zen"
  class:side-sheet-open={sideSheetOpen && chromeMode === 'small-tablet'}
  data-chrome-mode={chromeMode}
>
  <nav class="breadcrumbs" class:index-pulse={indexPulse} aria-label={T('app.breadcrumb')}>
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
    <h1>{T('app.title')}</h1>
    <p class="meta">{notesRoot || '…'}</p>
    {#if lastFileEvent}
      <p class="event">{T('app.lastIndex')}: <code>{lastFileEvent}</code></p>
    {/if}
    {#if !aiReachable}
      <p class="ai-offline-pill" role="status">{T('app.aiOffline')}</p>
    {/if}
  </header>

  <div class="toolbar">
    {#if chromeMode === 'small-tablet'}
      <button
        type="button"
        class="btn secondary hamburger-btn"
        aria-expanded={sideSheetOpen}
        on:click={() => (sideSheetOpen = !sideSheetOpen)}
      >
        ☰ {T('app.mobileNavSide')}
      </button>
    {/if}
    <input class="path-input" bind:value={pagePath} placeholder={T('app.pathPlaceholder')} />
    <button type="button" class="btn" on:click={() => loadPage(pagePath, { replaceTop: true })}
      >{T('app.open')}</button
    >
    {#if showMobileChrome}
      <button
        type="button"
        class="btn secondary icon-btn"
        on:click={openOrCreate}
        aria-label={T('app.ensurePage')}
        title={T('app.ensurePage')}
      >
        <svg class="ico" viewBox="0 0 24 24" aria-hidden="true"
          ><path
            fill="currentColor"
            d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm4 18H6V4h7v5h5v11zM12 11H8v2h4v4h2v-4h4v-2h-4V7h-2v4z"
          /></svg
        >
      </button>
      <button
        type="button"
        class="btn secondary icon-btn"
        on:click={toggleTheme}
        aria-label={theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
        title={theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
      >
        {#if theme === 'dark'}
          <svg class="ico" viewBox="0 0 24 24" aria-hidden="true"
            ><path
              fill="currentColor"
              d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10zm0-5h2v3h-2V2zm0 19h2v3h-2v-3zM2 11h3v2H2v-2zm19 0h3v2h-3v-2zM4.22 4.22l2.12 2.12-1.41 1.41L2.81 5.64 4.22 4.22zm12.73 12.73 2.12 2.12-1.41 1.41-2.12-2.12 1.41-1.41zM19.78 4.22l-2.12 2.12-1.41-1.41 2.12-2.12 1.41 1.41zM7.05 18.95l-2.12 2.12-1.41-1.41 2.12-2.12 1.41 1.41z"
            /></svg
          >
        {:else}
          <svg class="ico" viewBox="0 0 24 24" aria-hidden="true"
            ><path
              fill="currentColor"
              d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
            /></svg
          >
        {/if}
      </button>
    {:else}
      <button type="button" class="btn secondary" on:click={openOrCreate}>{T('app.ensurePage')}</button>
      <button
        type="button"
        class="btn secondary"
        on:click={toggleTheme}
        title={theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
      >
        {theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
      </button>
    {/if}
    <span class="lang-toolbar" role="group" aria-label={T('app.langLabel')}>
      <button
        type="button"
        class="btn secondary lang-btn"
        class:active={$locale === 'en'}
        on:click={() => setLanguage('en')}>{T('app.langEn')}</button>
      <button
        type="button"
        class="btn secondary lang-btn"
        class:active={$locale === 'zh-CN'}
        on:click={() => setLanguage('zh-CN')}>{T('app.langZh')}</button>
    </span>
    {#if !showMobileChrome}
      <button type="button" class="btn secondary" on:click={openGraph}>{T('app.graph')}</button>
      <button type="button" class="btn secondary" on:click={openAbout}>{T('app.about')}</button>
    {/if}
    {#each $toolbarEntries as p (p.id)}
      <button
        type="button"
        class="btn secondary plugin-tb"
        on:click={() => p.run?.()}
      >{p.label}</button>
    {/each}
  </div>

  {#if selectedIds.length > 0}
    <div class="bulk-bar" role="toolbar" aria-label={T('app.multiSelect')}>
      <span class="bulk-count">{T('app.selectedCount', { count: selectedIds.length })}</span>
      <button type="button" class="btn secondary sm" on:click={copySelectedMarkdown}>{T('app.copyText')}</button>
      <button type="button" class="btn secondary sm" on:click={clearSelection}>{T('app.clear')}</button>
    </div>
  {/if}

  {#if aboutOpen}
    <div
      class="about-backdrop"
      role="presentation"
      transition:fade={{ duration: 160 }}
      on:click|self={() => (aboutOpen = false)}
    >
      <div
        class="about-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="about-title"
        transition:fly={{ y: 16, duration: 220, easing: cubicOut }}
      >
        <div class="about-brand" aria-hidden="true">
          <div class="about-logo">D</div>
        </div>
        <h2 id="about-title">{T('app.title')}</h2>
        <p class="about-ver">{appVersion || 'v1.4.5'}</p>
        <p class="about-copy">
          {T('app.aboutBody')}
        </p>
        <button type="button" class="btn secondary" on:click={() => (aboutOpen = false)}>{T('app.close')}</button>
      </div>
    </div>
  {/if}

  {#if graphOpen}
    <section
      class="graph-panel"
      in:fly={{ y: 14, duration: 240, easing: cubicOut }}
      out:fade={{ duration: 160 }}
    >
      <div class="graph-head">
        <h2>{T('app.pageGraph')}</h2>
        <div class="graph-head-actions">
          <label class="graph-semantic-toggle">
            <input
              type="checkbox"
              bind:checked={graphSemanticOn}
              on:change={() => toggleSemanticGraph()}
            />
            <span>{T('app.graphSemantic')}</span>
          </label>
          <button type="button" class="btn secondary sm" on:click={() => (graphOpen = false)}>{T('app.close')}</button>
        </div>
      </div>
      <PageGraph graph={graphData} semanticEdges={graphSemanticEdges} semanticOn={graphSemanticOn} />
    </section>
  {/if}

  {#if err}
    <p class="err">{err}</p>
  {/if}

  {#if chromeMode === 'small-tablet' && sideSheetOpen}
    <div
      class="side-sheet-backdrop"
      role="presentation"
      transition:fade={{ duration: 140 }}
      on:click={() => (sideSheetOpen = false)}
    ></div>
  {/if}

  <div class="layout-grid" data-mobile-panel={mobilePanel}>
  <section class="col-main outliner-panel">
    <h2>{T('app.outline')}</h2>
    {#if pageLoading}
      <div class="skeleton-stack" aria-busy="true">
        {#each [88, 92, 78, 85, 70] as w, i (i)}
          <div class="sk-line" style="width: {w}%"></div>
        {/each}
      </div>
    {:else if roots.length === 0}
      <div class="empty-state">
        <div class="empty-svg" aria-hidden="true">
          <svg viewBox="0 0 120 100" width="120" height="100">
            <rect x="12" y="18" width="96" height="64" rx="10" fill="none" stroke="currentColor" stroke-opacity="0.2" stroke-width="1.5"/>
            <path d="M28 38h64M28 52h48M28 66h56" stroke="currentColor" stroke-opacity="0.25" stroke-width="2" stroke-linecap="round"/>
            <circle cx="88" cy="30" r="6" fill="currentColor" fill-opacity="0.12"/>
          </svg>
        </div>
        <p class="empty-title">{T('app.emptyTitle')}</p>
        <p class="empty-sub">{T('app.emptySubtitle')}</p>
        <p class="empty-tip"><strong>{T('app.emptyCta')}</strong> {T('app.emptyCtaBody')}</p>
      </div>
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
          onSwipeTodo={handleSwipeTodo}
          onSwipeClear={handleSwipeClear}
        />
      {/each}
    {/if}
    </section>

  <div class="col-related-wrap">
    <SemanticRelated {pagePath} indexEpoch={indexEpoch} onOpenPage={(rel) => loadPage(rel)} />
  </div>

  <aside class="dv-sidebar col-side" aria-label={T('sidebar.aria')}>
    <div class="side-tabs" role="tablist" aria-label={T('sidebar.tablist')}>
      <button
        type="button"
        role="tab"
        class="side-tab"
        class:active={sideTab === 'backlinks'}
        aria-selected={sideTab === 'backlinks'}
        id="tab-backlinks"
        on:click={() => (sideTab = 'backlinks')}
      >
        {T('sidebar.tabBacklinks')}
      </button>
      <button
        type="button"
        role="tab"
        class="side-tab"
        class:active={sideTab === 'ai'}
        aria-selected={sideTab === 'ai'}
        id="tab-ai"
        on:click={() => (sideTab = 'ai')}
      >
        {T('sidebar.tabAI')}
      </button>
    </div>
    <div
      class="side-panel"
      role="tabpanel"
      aria-labelledby={sideTab === 'backlinks' ? 'tab-backlinks' : 'tab-ai'}
    >
      {#if sideTab === 'backlinks'}
        <Backlinks {notesRoot} {pagePath} indexEpoch={indexEpoch} onOpenPage={(rel) => loadPage(rel)} />
      {:else}
        <AIChatPanel {pagePath} />
      {/if}
    </div>
  </aside>
  </div>

  {#if $sidebarEntries.length}
    <aside
      class="plugin-sidebar"
      aria-label="Plugin sidebar"
      in:fly={{ x: 18, duration: 260, easing: cubicOut }}
      out:fade={{ duration: 140 }}
    >
      {#each $sidebarEntries as s (s.id)}
        <section class="plugin-card">
          <h3 class="plugin-card-title">{s.title}</h3>
          <p class="plugin-card-body">{s.body}</p>
        </section>
      {/each}
    </aside>
  {/if}

  <p class="hint">
        {T('app.hint')}
  </p>
</main>

<nav class="mobile-tabbar" aria-label={T('app.mobileNavAria')}>
  <button
    type="button"
    class="mobile-tab-btn"
    class:active={mobilePanel === 'outline' && !graphOpen && !aboutOpen}
    on:click={() => {
      graphOpen = false
      aboutOpen = false
      mobilePanel = 'outline'
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><path fill="currentColor" d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h10v2H4v-2z" /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavOutline')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn"
    class:active={mobilePanel === 'related' && !graphOpen && !aboutOpen}
    on:click={() => {
      graphOpen = false
      aboutOpen = false
      mobilePanel = 'related'
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><path
          fill="currentColor"
          d="M12 2l3 6 6 .9-4.5 4.4L18 20l-6-3.2L6 20l1.5-6.7L3 8.9 9 8z"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavRelated')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn"
    class:active={mobilePanel === 'side' && !graphOpen && !aboutOpen}
    on:click={() => {
      graphOpen = false
      aboutOpen = false
      mobilePanel = 'side'
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><path
          fill="currentColor"
          d="M3.9 12c0-1.7 1.4-3.1 3.1-3.1h4V7H7c-2.8 0-5 2.2-5 5s2.2 5 5 5h4v-1.9H7c-1.7 0-3.1-1.4-3.1-3.1zm4.1 1h8v-2H8v2zm9-6h-4v2h4c1.7 0 3.1 1.4 3.1 3.1s-1.4 3.1-3.1 3.1h-4V14h4c2.8 0 5-2.2 5-5s-2.2-5-5-5z"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavSide')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn mobile-tab-iconish"
    class:active={graphOpen}
    aria-label={T('app.mobileNavGraph')}
    title={T('app.mobileNavGraph')}
    on:click={() => {
      aboutOpen = false
      void openGraph()
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><circle cx="6" cy="6" r="2.5" fill="currentColor" /><circle cx="18" cy="8" r="2.5" fill="currentColor" /><circle
          cx="9"
          cy="18"
          r="2.5"
          fill="currentColor"
        /><path
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          d="M8 7.5l8 1.5M10 17l6-6M6 8.5L9 16.5"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavGraph')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn mobile-tab-iconish"
    class:active={aboutOpen}
    aria-label={T('app.mobileNavAbout')}
    title={T('app.mobileNavAbout')}
    on:click={() => {
      graphOpen = false
      void openAbout()
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><path
          fill="currentColor"
          d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavAbout')}</span>
  </button>
</nav>

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
    min-height: 100dvh;
    background: transparent;
    color: var(--dv-fg);
    font-family: var(--dv-font, var(--dv-font-sans, 'Inter', system-ui, sans-serif));
    font-size: 15px;
    line-height: 1.55;
    -webkit-font-smoothing: antialiased;
  }

  @keyframes dv-reindex-pulse {
    0%,
    100% {
      box-shadow: 0 0 0 0 rgba(120, 160, 255, 0);
    }
    45% {
      box-shadow: 0 0 0 4px rgba(120, 160, 255, 0.18);
    }
  }

  .breadcrumbs.index-pulse {
    border-radius: 8px;
    animation: dv-reindex-pulse 0.9s ease-in-out 2;
  }

  .lang-toolbar {
    display: inline-flex;
    gap: 4px;
    margin: 0 2px;
  }
  .lang-btn.active {
    border-color: rgba(120, 160, 255, 0.45);
    background: rgba(120, 160, 255, 0.12);
  }

  .skeleton-stack {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 8px 0 4px;
  }
  .sk-line {
    height: 1.05em;
    border-radius: 6px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--dv-fg) 8%, transparent) 0%,
      color-mix(in srgb, var(--dv-fg) 14%, transparent) 50%,
      color-mix(in srgb, var(--dv-fg) 8%, transparent) 100%
    );
    background-size: 200% 100%;
    animation: dv-shimmer 1.1s ease-in-out infinite;
  }
  @keyframes dv-shimmer {
    0% {
      background-position: 100% 0;
    }
    100% {
      background-position: -100% 0;
    }
  }

  .empty-state {
    text-align: center;
    padding: 28px 16px 20px;
    border-radius: 12px;
    border: 1px dashed color-mix(in srgb, var(--dv-fg) 18%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
  }
  .empty-svg {
    color: var(--dv-fg);
    opacity: 0.45;
    margin-bottom: 12px;
  }
  .empty-title {
    margin: 0 0 8px;
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: -0.02em;
  }
  .empty-sub {
    margin: 0 0 12px;
    font-size: 0.9rem;
    opacity: 0.72;
    line-height: 1.5;
    max-width: 36ch;
    margin-left: auto;
    margin-right: auto;
  }
  .empty-tip {
    margin: 0;
    font-size: 0.82rem;
    opacity: 0.55;
    line-height: 1.45;
  }
  .empty-tip strong {
    font-weight: 600;
    opacity: 0.85;
  }

  .layout {
    max-width: 800px;
    width: 100%;
    margin: 0 auto;
    padding: max(20px, env(safe-area-inset-top, 0px)) max(16px, env(safe-area-inset-left, 0px))
      max(56px, calc(12px + env(safe-area-inset-bottom, 0px))) max(16px, env(safe-area-inset-right, 0px));
    box-sizing: border-box;
  }
  main[data-chrome-mode='tablet-land'].layout {
    max-width: min(100%, 1440px);
  }
  .top {
    padding-left: max(0px, env(safe-area-inset-left, 0px));
    padding-right: max(0px, env(safe-area-inset-right, 0px));
    padding-top: max(0px, env(safe-area-inset-top, 0px));
    box-sizing: border-box;
  }
  main[data-chrome-mode='tablet-land'] .top {
    min-height: 4.75rem;
    padding-bottom: 10px;
  }
  main[data-chrome-mode='tablet-land'] .toolbar {
    margin-top: 14px;
  }
  main[data-chrome-mode='tablet-land'] .top h1 {
    font-size: 1.62rem;
    line-height: 1.22;
  }
  main[data-chrome-mode='phone-portrait'] .top {
    min-height: unset;
    padding-bottom: 4px;
  }
  main[data-chrome-mode='phone-portrait'] .toolbar {
    margin-top: 10px;
  }
  main[data-chrome-mode='phone-land'] .top {
    min-height: 3.25rem;
    padding-bottom: 6px;
  }
  main[data-chrome-mode='phone-land'] .toolbar {
    margin-top: 10px;
  }
  @media (min-width: 900px) {
    .layout {
      max-width: min(100%, 1680px);
    }
  }
  .layout-grid {
    display: block;
  }
  main[data-chrome-mode='small-tablet'] .layout-grid .dv-sidebar {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    max-height: 52vh;
    z-index: 70;
    transform: translateY(110%);
    transition: transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
    overflow: auto;
    margin-top: 0;
    border-radius: 14px 14px 0 0;
  }
  main[data-chrome-mode='small-tablet'].side-sheet-open .layout-grid .dv-sidebar {
    transform: translateY(0);
  }
  .side-sheet-backdrop {
    position: fixed;
    inset: 0;
    z-index: 65;
    background: rgba(0, 0, 0, 0.42);
    -webkit-backdrop-filter: blur(2px);
    backdrop-filter: blur(2px);
  }
  .hamburger-btn {
    flex-shrink: 0;
  }

  @media (min-width: 900px) {
    main:not([data-chrome-mode='tablet-land']) .layout-grid {
      display: grid;
      grid-template-columns: minmax(200px, 24%) minmax(300px, 1fr) minmax(240px, 26%);
      gap: 16px;
      align-items: start;
    }
    main:not([data-chrome-mode='tablet-land']) .layout-grid .col-related-wrap :global(.semantic-related) {
      margin-top: 0;
    }
    main:not([data-chrome-mode='tablet-land']) .layout-grid .dv-sidebar {
      margin-top: 0;
    }
    main:not([data-chrome-mode='tablet-land']) .layout-grid .col-main.outliner-panel {
      margin-top: 0;
    }
  }
  main[data-chrome-mode='tablet-land'] .layout-grid {
    display: grid;
    grid-template-columns: minmax(260px, 32%) minmax(0, 1fr);
    gap: 16px;
    align-items: start;
  }
  main[data-chrome-mode='tablet-land'] .layout-grid .dv-sidebar.col-side {
    grid-column: 1;
    grid-row: 1 / span 2;
  }
  main[data-chrome-mode='tablet-land'] .layout-grid .col-main.outliner-panel {
    grid-column: 2;
    grid-row: 1;
  }
  main[data-chrome-mode='tablet-land'] .layout-grid .col-related-wrap {
    grid-column: 2;
    grid-row: 2;
  }
  main[data-chrome-mode='tablet-land'] .layout-grid .col-related-wrap :global(.semantic-related) {
    margin-top: 0;
  }
  .mobile-tabbar {
    display: none;
  }
  @media (max-width: 899px) {
    main.layout:not([data-chrome-mode='tablet-land']) {
      max-width: 100%;
      padding: max(12px, env(safe-area-inset-top, 0px)) max(12px, env(safe-area-inset-left, 0px))
        calc(12px + 56px + max(env(safe-area-inset-bottom, 0px), 12px)) max(12px, env(safe-area-inset-right, 0px));
    }
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='outline'] .col-related-wrap,
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='outline'] .col-side {
      display: none;
    }
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='related'] .col-main,
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='related'] .col-side {
      display: none;
    }
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='side'] .col-main,
    main.layout:not([data-chrome-mode='tablet-land']) .layout-grid[data-mobile-panel='side'] .col-related-wrap {
      display: none;
    }
    main.layout:not([data-chrome-mode='tablet-land']) .mobile-tabbar {
      display: flex;
      position: fixed;
      z-index: 60;
      left: 0;
      right: 0;
      bottom: 0;
      min-height: calc(48px + max(env(safe-area-inset-bottom, 0px), 8px));
      padding: 4px max(8px, env(safe-area-inset-left, 0px)) max(4px, max(env(safe-area-inset-bottom, 0px), 8px))
        max(8px, env(safe-area-inset-right, 0px));
      gap: 12px;
      justify-content: stretch;
      align-items: stretch;
      border-top: 1px solid var(--dv-border);
      background: color-mix(in srgb, var(--dv-panel) 96%, transparent);
      -webkit-backdrop-filter: blur(10px);
      backdrop-filter: blur(10px);
    }
    main.layout:not([data-chrome-mode='tablet-land']) .mobile-tab-btn {
      flex: 1;
      min-width: 0;
      min-height: 48px;
      padding: 6px 4px;
      border-radius: 10px;
      border: 1px solid var(--dv-border);
      background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
      color: inherit;
      font-size: 0.68rem;
      font-weight: 600;
      touch-action: manipulation;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 2px;
      line-height: 1.15;
    }
    main.layout:not([data-chrome-mode='tablet-land']) .mobile-tab-btn.active {
      border-color: rgba(120, 160, 255, 0.45);
      background: rgba(80, 120, 255, 0.14);
    }
    .top h1 {
      font-size: 1.35rem;
    }
    .toolbar {
      flex-direction: column;
      align-items: stretch;
    }
    .path-input {
      min-width: 0;
      width: 100%;
      font-size: 16px;
      min-height: 48px;
    }
    .btn {
      min-height: 48px;
      font-size: 1rem;
      touch-action: manipulation;
    }
    .outliner-panel {
      padding: 12px;
    }
    .breadcrumbs {
      font-size: 0.72rem;
    }
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
    gap: 12px;
    margin-top: 16px;
    flex-wrap: wrap;
    align-items: center;
  }
  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 48px;
    padding-left: 12px;
    padding-right: 12px;
  }
  .icon-btn .ico {
    width: 22px;
    height: 22px;
    display: block;
  }
  .ai-offline-pill {
    display: inline-block;
    margin: 8px 0 0;
    padding: 4px 12px;
    font-size: 0.78rem;
    border-radius: 999px;
    border: 1px solid color-mix(in srgb, var(--dv-fg) 16%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    opacity: 0.9;
  }
  .mobile-tab-ico svg {
    width: 22px;
    height: 22px;
    display: block;
    opacity: 0.92;
  }
  .mobile-tab-lbl {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    min-height: 48px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: rgba(80, 120, 255, 0.25);
    color: var(--dv-fg);
    touch-action: manipulation;
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
    padding-bottom: calc(16px + var(--dv-keyboard-inset, 0px));
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    border-radius: 10px;
    border: 1px solid var(--dv-border);
    scroll-margin-bottom: max(24px, var(--dv-keyboard-inset, 0px));
  }
  .outliner-panel h2 {
    margin: 0 0 12px;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
  }
  .bulk-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
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
    padding: 6px 12px;
    min-height: 40px;
    font-size: 0.8rem;
  }
  @media (max-width: 899px) {
    main.layout:not([data-chrome-mode='tablet-land']) .btn.sm {
      min-height: 48px;
    }
  }
  .about-backdrop {
    position: fixed;
    inset: 0;
    z-index: 80;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    background: rgba(0, 0, 0, 0.45);
    -webkit-backdrop-filter: blur(4px);
    backdrop-filter: blur(4px);
  }
  .about-card {
    max-width: 380px;
    width: 100%;
    padding: 20px 22px;
    border-radius: 12px;
    border: 1px solid var(--dv-border);
    background: var(--dv-panel);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
  }
  .about-card h2 {
    margin: 0 0 4px;
    font-size: 1.25rem;
  }
  .about-brand {
    display: flex;
    justify-content: center;
    margin-bottom: 10px;
  }
  .about-logo {
    width: 64px;
    height: 64px;
    border-radius: 16px;
    display: grid;
    place-items: center;
    font-size: 2rem;
    font-weight: 700;
    color: #ecfeff;
    background:
      radial-gradient(circle at 30% 25%, rgba(255, 255, 255, 0.28), transparent 45%),
      linear-gradient(145deg, #0ea5e9, #fb7185);
    box-shadow: 0 10px 24px rgba(6, 58, 100, 0.35);
  }
  .about-ver {
    margin: 0 0 12px;
    font-family: var(--dv-font-mono, 'JetBrains Mono', monospace);
    font-size: 0.85rem;
    opacity: 0.65;
  }
  .about-copy {
    margin: 0 0 16px;
    font-size: 0.9rem;
    line-height: 1.5;
    opacity: 0.85;
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
  .graph-head-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .graph-semantic-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.78rem;
    opacity: 0.8;
    cursor: pointer;
    user-select: none;
  }
  .hint {
    margin-top: 24px;
    font-size: 0.8rem;
    opacity: 0.5;
  }
  .plugin-tb {
    font-size: 0.85rem;
  }
  .dv-sidebar {
    margin-top: 20px;
    padding: 12px 14px;
    border-radius: 10px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
  }
  .side-tabs {
    display: flex;
    gap: 6px;
    margin-bottom: 12px;
  }
  .side-tab {
    flex: 1;
    min-height: 48px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: inherit;
    font-size: 0.8rem;
    font-weight: 500;
    cursor: pointer;
    opacity: 0.65;
    touch-action: manipulation;
  }
  .side-tab.active {
    opacity: 1;
    border-color: rgba(120, 160, 255, 0.35);
    background: rgba(80, 120, 255, 0.1);
  }
  .side-panel {
    min-height: 120px;
  }
  .plugin-sidebar {
    margin-top: 20px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .plugin-card {
    padding: 12px 14px;
    border-radius: 10px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
  }
  .plugin-card-title {
    margin: 0 0 6px;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.65;
  }
  .plugin-card-body {
    margin: 0;
    font-size: 0.88rem;
    line-height: 1.45;
    white-space: pre-wrap;
  }
</style>
