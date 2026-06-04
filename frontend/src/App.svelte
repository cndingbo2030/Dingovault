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
    MoveBlockUnder,
    InsertChildBlock,
    GetAppVersion,
    GetLocale,
    SetLocale,
    IsAIReachable,
    HealthResetLocalSearchIndex,
    ListVaultPages,
    ListVaultFiles,
    OpenVaultFile
  } from '../wailsjs/go/bridge/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { locale, messages, tr, detectBrowserLocale, normalizeLocaleTag } from './lib/i18n/index.js'
  import OutlineNode from './OutlineNode.svelte'
  import PageGraph from './PageGraph.svelte'
  import MindMap from './MindMap.svelte'
  import Backlinks from './Backlinks.svelte'
  import SemanticRelated from './SemanticRelated.svelte'
  import AIChatPanel from './AIChatPanel.svelte'
  import WorkspaceConsole from './WorkspaceConsole.svelte'
  import SettingsDialog from './SettingsDialog.svelte'
  import CommandPalette from './CommandPalette.svelte'
  import ToastStack from './ToastStack.svelte'
  import { readRecentPages, touchRecentPage } from './recentPages.js'
  import { pushToast } from './toastStore.js'
  import { toolbarEntries, sidebarEntries } from './pluginRegistry.js'
  import { hapticLight } from './lib/haptic.js'

  let notesRoot = ''
  let pagePath = 'README.md'
  /** @type {any[]} */
  let roots = []
  let paletteOpen = false
  let err = ''
  let staleBlockRecovering = false
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
  let mindMapOpen = false
  let settingsOpen = false
  let newPageDialogOpen = false
  let newPagePath = ''
  /** @type {HTMLInputElement | undefined} */
  let newPageInput
  let consoleOpen = false
  /** @type {any} */
  let consolePane
  let pagesOpen = true
  let inspectorOpen = true
  /** @type {'backlinks' | 'related' | 'ai'} */
  let sideTab = 'backlinks'
  /** @type {'outline' | 'pages' | 'side'} */
  let mobilePanel = 'outline'
  let sideSheetOpen = false
  /** @type {string[]} */
  let navStack = []
  /** @type {'default' | 'phone-portrait' | 'phone-land' | 'tablet-master' | 'small-tablet'} */
  let chromeMode = 'default'

  $: showMobileChrome =
    chromeMode === 'phone-portrait' ||
    chromeMode === 'phone-land' ||
    chromeMode === 'small-tablet'

  /** @type {boolean} */
  let aiReachable = true
  /** @type {string[]} */
  let vaultPages = []
  /** @type {{ path: string, name: string, ext: string, kind: string, size?: number, modifiedUnix?: number }[]} */
  let vaultFiles = []
  /** @type {string[]} */
  let recentPaths = []
  let pageFilter = ''
  let pagesLoading = false

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
    if (w >= 900) {
      chromeMode = 'tablet-master'
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
  function errorText(e) {
    if (e instanceof Error) return e.message || String(e)
    return String(e)
  }

  /** @param {unknown} e */
  function notifyErr(e) {
    const m = errorText(e)
    err = m
    pushToast(m, 'error')
  }

  /** @param {unknown} e */
  function isStaleBlockError(e) {
    const m = errorText(e).toLowerCase()
    return m.includes('lookup block:') && m.includes('block not found:')
  }

  /** @param {unknown} e */
  async function recoverStaleBlockError(e) {
    if (!isStaleBlockError(e)) return false
    err = ''
    if (staleBlockRecovering) return true
    staleBlockRecovering = true
    try {
      await loadPage(pagePath, { skipHistory: true, softNav: true, keepGraph: graphOpen, keepMindMap: mindMapOpen })
      pushToast(T('app.blockRecovered'), 'info', 2400)
    } finally {
      staleBlockRecovering = false
    }
    return true
  }

  /** @param {unknown} e */
  async function handleMutationError(e) {
    if (await recoverStaleBlockError(e)) return
    notifyErr(e)
  }

  /** @param {string} root */
  function vaultBasename(root) {
    if (!root) return T('app.vault')
    const p = root.replace(/[/\\]+$/, '')
    const parts = p.split(/[/\\]/).filter(Boolean)
    return parts.length ? parts[parts.length - 1] : T('app.vault')
  }

  /** @param {string} p */
  function pageTitle(p) {
    const seg = (p || '').split(/[/\\]/).pop() || p
    return seg.replace(/\.(md|markdown)$/i, '')
  }

  /** @param {string} p */
  function pageFolder(p) {
    const parts = (p || '').split(/[/\\]/).filter(Boolean)
    if (parts.length <= 1) return ''
    return parts.slice(0, -1).join('/')
  }

  /** @param {string} p */
  function isMarkdownPath(p) {
    return /\.(md|markdown)$/i.test(p || '')
  }

  /** @param {{ path?: string, name?: string, kind?: string }} file */
  function isMarkdownFile(file) {
    return file?.kind === 'markdown' || isMarkdownPath(file?.path || file?.name || '')
  }

  /** @param {{ path?: string, name?: string }} file */
  function fileTitle(file) {
    return pageTitle(file?.name || file?.path || '')
  }

  /** @param {{ ext?: string, kind?: string }} file */
  function fileKindLabel(file) {
    if (!file || isMarkdownFile(file)) return ''
    const ext = (file.ext || '').toUpperCase()
    return ext || (file.kind || '').toUpperCase()
  }

  /** @param {string} rel */
  function vaultFileFromPage(rel) {
    const name = (rel || '').split(/[/\\]/).pop() || rel
    return { path: rel, name, ext: 'md', kind: 'markdown' }
  }

  function refreshRecentPaths() {
    recentPaths = readRecentPages()
  }

  async function loadVaultPages() {
    pagesLoading = true
    try {
      const [pages, files] = await Promise.all([
        ListVaultPages().catch(() => []),
        ListVaultFiles().catch(() => [])
      ])
      vaultPages = Array.isArray(pages) ? pages : []
      vaultFiles = Array.isArray(files) ? files : []
      if (!vaultFiles.length) vaultFiles = vaultPages.map(vaultFileFromPage)
    } catch {
      vaultPages = []
      vaultFiles = []
    } finally {
      pagesLoading = false
    }
  }

  $: vaultFilePathSet = new Set((vaultFiles.length ? vaultFiles : (vaultPages || []).map(vaultFileFromPage)).map((f) => f.path))
  $: visibleRecentPaths = recentPaths
    .filter((rel) => rel !== pagePath)
    .filter((rel) => !vaultFilePathSet.size || vaultFilePathSet.has(rel))
    .slice(0, 6)

  $: filteredVaultFiles = (() => {
    const q = pageFilter.trim().toLowerCase()
    const files = vaultFiles.length ? vaultFiles : (vaultPages || []).map(vaultFileFromPage)
    if (!q) return files.slice(0, 360)
    return files
      .filter((f) => {
        const p = f.path || ''
        return p.toLowerCase().includes(q) || fileTitle(f).toLowerCase().includes(q) || (f.kind || '').includes(q)
      })
      .slice(0, 240)
  })()

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

  /** @param {string} id */
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

  /** @param {any[]} nodes */
  function countOutlineBlocks(nodes) {
    let total = 0
    for (const node of nodes || []) total += 1 + countOutlineBlocks(node.children || [])
    return total
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
      mindMapOpen = false
      inspectorOpen = false
      consoleOpen = false
      graphOpen = true
    } catch (e) {
      notifyErr(e)
    }
  }

  function openMindMap() {
    err = ''
    graphOpen = false
    mindMapOpen = true
    inspectorOpen = false
    consoleOpen = false
    mobilePanel = 'outline'
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

  async function openSettings() {
    err = ''
    try {
      appVersion = await GetAppVersion()
    } catch {
      appVersion = ''
    }
    settingsOpen = true
  }

  /** @param {string} movingId @param {string} beforeId */
  async function handleReorderBefore(movingId, beforeId) {
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await ReorderBlockBefore(movingId, beforeId)
      await loadPage(pagePath)
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} id @param {string} text */
  async function handleMindMapUpdate(id, text) {
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await UpdateBlock(id, text)
      await loadPage(pagePath, { skipHistory: true, softNav: true, keepMindMap: true })
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} movingId @param {string} parentId */
  async function handleMindMapMoveUnder(movingId, parentId) {
    if (!movingId || !parentId || movingId === parentId) return
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await MoveBlockUnder(movingId, parentId)
      await loadPage(pagePath, { skipHistory: true, softNav: true, keepMindMap: true })
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} parentId */
  async function handleMindMapInsertChild(parentId) {
    if (!parentId) return
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await InsertChildBlock(parentId, '')
      await loadPage(pagePath, { skipHistory: true, softNav: true, keepMindMap: true })
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} command */
  function isPlainReadOnlyCommand(command) {
    const cmd = command.trim()
    return /^(pwd|ls\b|find\b|rg\b|grep\b|cat\b|head\b|tail\b|less\b|git\s+(status|diff|log|show|branch|rev-parse)\b)/.test(cmd)
  }

  /** @param {string} text */
  function commandFromBlockText(text) {
    return String(text || '')
      .replace(/^`+|`+$/g, '')
      .trim()
  }

  /** @param {string} text */
  function cwdFromBlockText(text) {
    const raw = String(text || '').trim()
    const cleaned = raw
      .replace(/^\s*[-*+]\s+/, '')
      .replace(/^`+|`+$/g, '')
      .replace(/^\[\[|\]\]$/g, '')
      .trim()
    if (/^(\.\/|\.\.\/|\/|[A-Za-z]:[\\/])/.test(cleaned)) return cleaned
    if (/[\\/]/.test(cleaned)) return cleaned
    return pageFolder(pagePath)
  }

  /** @param {string} id @param {string} text */
  async function handleRunBlockCommand(id, text) {
    const cmd = commandFromBlockText(text)
    if (!cmd) return
    if (!isPlainReadOnlyCommand(cmd)) {
      const ok = window.confirm(T('terminal.confirmRun', { command: cmd }))
      if (!ok) return
    }
    err = ''
    consoleOpen = true
    await tick()
    try {
      await syncAllBlocksFromDOM()
      const result = await consolePane?.runBlockCommand?.(id, cmd, pageFolder(pagePath))
      await loadPage(pagePath, { skipHistory: true, softNav: true, keepMindMap: mindMapOpen })
      if (result) pushToast(T('terminal.resultAppended', { exitCode: result.exitCode }), result.exitCode === 0 ? 'info' : 'error')
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} _id @param {string} text */
  async function handleOpenTerminalContext(_id, text) {
    err = ''
    consoleOpen = true
    await tick()
    try {
      await consolePane?.startSessionForCwd?.(cwdFromBlockText(text))
    } catch (e) {
      notifyErr(e)
    }
  }

  /** @param {string} id */
  async function handleSwipeTodo(id) {
    hapticLight()
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await CycleBlockTodo(id)
      await loadPage(pagePath, { focusBlockId: id })
    } catch (e) {
      await handleMutationError(e)
    }
  }

  /** @param {string} id */
  async function handleSwipeClear(id) {
    if (typeof window !== 'undefined' && !window.confirm(T('app.confirmClearBlock'))) return
    hapticLight()
    err = ''
    try {
      await syncAllBlocksFromDOM()
      await UpdateBlock(id, '')
      await loadPage(pagePath, { focusBlockId: id })
    } catch (e) {
      await handleMutationError(e)
    }
  }

  let healthBusy = false
  async function runHealthReset() {
    if (!confirm(T('app.healthResetConfirm'))) return
    healthBusy = true
    err = ''
    try {
      await HealthResetLocalSearchIndex()
      indexEpoch++
      await loadPage(pagePath, { skipHistory: true, softNav: true })
      pushToast(T('app.healthResetDone'), 'info')
    } catch (e) {
      notifyErr(e)
    } finally {
      healthBusy = false
    }
  }

  function goBackAndroid() {
    if (paletteOpen) {
      paletteOpen = false
      return true
    }
    if (settingsOpen) {
      settingsOpen = false
      return true
    }
    if (graphOpen) {
      graphOpen = false
      return true
    }
    if (mindMapOpen) {
      mindMapOpen = false
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

  function goBackPage() {
    if (graphOpen) {
      graphOpen = false
      return
    }
    if (mindMapOpen) {
      mindMapOpen = false
      return
    }
    if (navStack.length <= 1) return
    const prev = navStack[navStack.length - 2]
    navStack = navStack.slice(0, -1)
    void loadPage(prev, { skipHistory: true })
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
        refreshRecentPaths()
        void refreshAIReach()
        void loadVaultPages()
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
      void loadVaultPages()
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
          t.scrollIntoView({ block: 'center', behavior: 'smooth', inline: 'nearest' })
          requestAnimationFrame(() => {
            const vv = window.visualViewport
            if (!vv) return
            const r = t.getBoundingClientRect()
            const navPad = 88
            const bottomLimit = vv.height + vv.offsetTop - navPad
            if (r.bottom > bottomLimit) {
              const delta = r.bottom - bottomLimit + 12
              window.scrollBy({ top: delta, behavior: 'smooth' })
            }
          })
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
   * @param {{ focusBlockId?: string, caretOffset?: number, skipHistory?: boolean, replaceTop?: boolean, softNav?: boolean, keepGraph?: boolean, keepMindMap?: boolean }} [opts]
   */
  async function loadPage(rel, opts) {
    const focusId = opts?.focusBlockId
    const caret = opts?.caretOffset
    const skipHist = opts?.skipHistory
    const replaceTop = opts?.replaceTop
    if (!opts?.keepGraph) graphOpen = false
    if (!opts?.keepMindMap) mindMapOpen = false
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
      refreshRecentPaths()
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

  /** @param {{ path: string, name?: string, kind?: string }} file */
  async function openVaultEntry(file) {
    if (!file?.path) return
    if (isMarkdownFile(file)) {
      mobilePanel = 'outline'
      await loadPage(file.path)
      return
    }
    err = ''
    try {
      await OpenVaultFile(file.path)
      pushToast(T('app.openedExternal', { path: file.path }), 'info')
    } catch (e) {
      notifyErr(e)
    }
  }

  async function openNewPageDialog() {
    newPagePath = T('app.newPageDefault')
    newPageDialogOpen = true
    await tick()
    newPageInput?.focus()
    newPageInput?.select()
  }

  async function submitNewPage() {
    err = ''
    let rel = newPagePath.trim()
    if (!rel) return
    if (!isMarkdownPath(rel)) rel += '.md'
    try {
      await EnsurePage(rel)
      newPageDialogOpen = false
      pageFilter = ''
      void loadVaultPages()
      await loadPage(rel, { replaceTop: true })
      pushToast(T('app.pageCreated', { path: rel }), 'info')
    } catch (e) {
      notifyErr(e)
    }
  }

  async function openOrCreate() {
    err = ''
    try {
      await EnsurePage(pagePath)
      void loadVaultPages()
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
        await handleMutationError(e)
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
      await handleMutationError(e)
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
      await handleMutationError(e)
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
      await handleMutationError(e)
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
      await handleMutationError(e)
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
      await handleMutationError(e)
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
      await handleMutationError(e)
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
  class="layout zen ide-shell"
  class:side-sheet-open={sideSheetOpen && chromeMode === 'small-tablet'}
  class:console-open={consoleOpen}
  class:pages-collapsed={!pagesOpen}
  class:inspector-collapsed={!inspectorOpen}
  data-chrome-mode={chromeMode}
>
  <aside class="activity-rail" aria-label={T('activity.aria')}>
    <button
      type="button"
      class="rail-btn"
      class:active={pagesOpen}
      aria-label={T('activity.files')}
      title={T('activity.files')}
      on:click={() => (pagesOpen = !pagesOpen)}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 5.5A2.5 2.5 0 0 1 6.5 3H10l2 2h5.5A2.5 2.5 0 0 1 20 7.5v11A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5zM6.5 5A.5.5 0 0 0 6 5.5V8h12v-.5a.5.5 0 0 0-.5-.5h-6.33l-2-2zM6 10v8.5a.5.5 0 0 0 .5.5h11a.5.5 0 0 0 .5-.5V10z" /></svg>
    </button>
    <button
      type="button"
      class="rail-btn"
      class:active={graphOpen}
      aria-label={T('activity.graph')}
      title={T('activity.graph')}
      on:click={() => openGraph()}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="6" cy="6" r="2.4" fill="currentColor" /><circle cx="18" cy="8" r="2.4" fill="currentColor" /><circle cx="9" cy="18" r="2.4" fill="currentColor" /><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M8 7.2l8 1.6M10.2 16.4l5.8-7M6.7 8.3l2.2 8" /></svg>
    </button>
    <button
      type="button"
      class="rail-btn"
      class:active={mindMapOpen}
      aria-label={T('activity.mindMap')}
      title={T('activity.mindMap')}
      on:click={openMindMap}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5.5" cy="12" r="2.3" fill="currentColor" /><circle cx="13" cy="6" r="2.2" fill="currentColor" /><circle cx="13" cy="18" r="2.2" fill="currentColor" /><circle cx="20" cy="12" r="2.1" fill="currentColor" /><path fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" d="M7.5 10.7 11.1 7.4M7.5 13.3l3.6 3.3M15.1 7.4 18.2 11M15.1 16.6l3.1-3.6" /></svg>
    </button>
    <button
      type="button"
      class="rail-btn"
      class:active={consoleOpen}
      aria-label={T('activity.console')}
      title={T('activity.console')}
      on:click={() => (consoleOpen = !consoleOpen)}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 5h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2m0 2v10h16V7zm3.4 2.2 3.1 2.8-3.1 2.8-1.2-1.3L7.9 12 6.2 10.5zm4.5 4.6h6.2v1.6h-6.2z" /></svg>
    </button>
    <button
      type="button"
      class="rail-btn"
      class:active={sideTab === 'ai' && inspectorOpen}
      aria-label={T('activity.ai')}
      title={T('activity.ai')}
      on:click={() => {
        inspectorOpen = true
        sideTab = 'ai'
      }}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M12 2.5 14.6 8l5.9.8-4.3 4.2 1 5.9L12 16.1 6.8 18.9l1-5.9-4.3-4.2L9.4 8zM12 7l-1.2 2.6-2.8.4 2 1.9-.5 2.8L12 13.3l2.5 1.4-.5-2.8 2-1.9-2.8-.4z" /></svg>
    </button>
    <span class="rail-spacer"></span>
    <button
      type="button"
      class="rail-btn rail-pro"
      class:active={settingsOpen}
      aria-label={T('activity.settings')}
      title={T('activity.settings')}
      on:click={() => openSettings()}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M19.4 13.5a7.8 7.8 0 0 0 .1-1.5 7.8 7.8 0 0 0-.1-1.5l2-1.5-2-3.4-2.4 1a7 7 0 0 0-2.6-1.5L14 2.5h-4l-.4 2.6A7 7 0 0 0 7 6.6l-2.4-1-2 3.4 2 1.5a7.8 7.8 0 0 0-.1 1.5 7.8 7.8 0 0 0 .1 1.5l-2 1.5 2 3.4 2.4-1a7 7 0 0 0 2.6 1.5l.4 2.6h4l.4-2.6a7 7 0 0 0 2.6-1.5l2.4 1 2-3.4zM12 15.2A3.2 3.2 0 1 1 12 8.8a3.2 3.2 0 0 1 0 6.4z" /></svg>
    </button>
  </aside>

  <section class="workspace-stage">
  <header class="top app-titlebar">
    <div class="titlebar-left">
      <button
        type="button"
        class="nav-icon"
        class:active={pagesOpen}
        aria-label={T('activity.files')}
        title={T('activity.files')}
        on:click={() => (pagesOpen = !pagesOpen)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 5.5A2.5 2.5 0 0 1 6.5 3H10l2 2h5.5A2.5 2.5 0 0 1 20 7.5v11A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5zM6.5 5A.5.5 0 0 0 6 5.5V8h12v-.5a.5.5 0 0 0-.5-.5h-6.33l-2-2zM6 10v8.5a.5.5 0 0 0 .5.5h11a.5.5 0 0 0 .5-.5V10z" /></svg>
      </button>
      {#if graphOpen || mindMapOpen || navStack.length > 1}
        <button
          type="button"
          class="nav-icon"
          aria-label={T('app.back')}
          title={T('app.back')}
          on:click={goBackPage}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M14.8 5.4 8.9 11.4l5.9 6-1.6 1.5L5.8 11.4l7.4-7.4z" /></svg>
        </button>
      {/if}
      <button
        type="button"
        class="nav-icon"
        aria-label={T('app.search')}
        title={T('app.search')}
        on:click={() => (paletteOpen = true)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" d="m20 20-4.2-4.2M18 10.8a7.2 7.2 0 1 1-14.4 0 7.2 7.2 0 0 1 14.4 0Z" /></svg>
      </button>
      <div class="tab-strip" aria-label={T('app.openTabs')}>
        <button type="button" class="doc-tab active" title={pagePath}>
          <span class="doc-tab-dot" aria-hidden="true"></span>
          <span>{graphOpen ? T('app.pageGraph') : mindMapOpen ? T('app.pageMindMap') : pageTitle(pagePath)}</span>
        </button>
      </div>
    </div>

    <div class="toolbar top-commandbar" class:tool-ribbon={chromeMode === 'tablet-master'}>
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
    <nav class="breadcrumbs" class:index-pulse={indexPulse} aria-label={T('app.breadcrumb')}>
      {#if graphOpen}
        <span class="crumb current">{T('app.pageGraph')}</span>
      {:else if mindMapOpen}
        <span class="crumb vault">{vaultBasename(notesRoot)}</span>
        <span class="sep" aria-hidden="true">›</span>
        <span class="crumb current">{T('app.pageMindMap')}</span>
      {:else}
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
      {/if}
    </nav>
    <input class="path-input" bind:value={pagePath} placeholder={T('app.pathPlaceholder')} />
    <button
      type="button"
      class="nav-icon command-icon primary"
      aria-label={T('app.open')}
      title={T('app.open')}
      on:click={() => loadPage(pagePath, { replaceTop: true })}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M5 5h9l5 5v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2m8 1.8V11h4.2zM7 14v2h7.6l-2.2 2.2 1.4 1.4 4.6-4.6-4.6-4.6-1.4 1.4 2.2 2.2z" /></svg>
    </button>
    {#if !showMobileChrome}
      <button
        type="button"
        class="nav-icon command-icon"
        aria-label={T('app.ensurePage')}
        title={T('app.ensurePage')}
        on:click={openOrCreate}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M6 3h9l5 5v13H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2m8 2v4h4zM9 13v2h3v3h2v-3h3v-2h-3v-3h-2v3z" /></svg>
      </button>
    {/if}
    {#each $toolbarEntries as p (p.id)}
      <button
        type="button"
        class="btn secondary plugin-tb"
        on:click={() => p.run?.()}
      >{p.label}</button>
    {/each}
    </div>

    <div class="titlebar-status">
      <button
        type="button"
        class="nav-icon"
        class:active={graphOpen}
        aria-label={T('activity.graph')}
        title={T('activity.graph')}
        on:click={openGraph}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="6" cy="6" r="2.2" fill="currentColor" /><circle cx="18" cy="8" r="2.2" fill="currentColor" /><circle cx="9" cy="18" r="2.2" fill="currentColor" /><path fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" d="M8 7.1 16 8.9M10.3 16.4 15.8 9.6M6.7 8.3 8.6 16" /></svg>
      </button>
      <button
        type="button"
        class="nav-icon"
        class:active={mindMapOpen}
        aria-label={T('activity.mindMap')}
        title={T('activity.mindMap')}
        on:click={openMindMap}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5.5" cy="12" r="2.1" fill="currentColor" /><circle cx="13" cy="6" r="2" fill="currentColor" /><circle cx="13" cy="18" r="2" fill="currentColor" /><circle cx="20" cy="12" r="1.9" fill="currentColor" /><path fill="none" stroke="currentColor" stroke-width="1.55" stroke-linecap="round" d="M7.3 10.8 11 7.4M7.3 13.2l3.7 3.4M15 7.4 18.3 11M15 16.6l3.3-3.6" /></svg>
      </button>
      <button
        type="button"
        class="nav-icon"
        class:active={consoleOpen}
        aria-label={T('activity.console')}
        title={T('activity.console')}
        on:click={() => (consoleOpen = !consoleOpen)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 5h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2m0 2v10h16V7zm3 3 3 2.5L7 15l-1.2-1.3L7.3 12 5.8 10.3zm5.2 3.4H18V15h-5.8z" /></svg>
      </button>
      <button
        type="button"
        class="nav-icon"
        class:active={inspectorOpen}
        aria-label={T('app.toggleInspector')}
        title={T('app.toggleInspector')}
        on:click={() => (inspectorOpen = !inspectorOpen)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 4h16v16H4zm2 2v12h9V6zm11 0v12h2V6z" /></svg>
      </button>
      <button
        type="button"
        class="nav-icon"
        class:active={settingsOpen}
        aria-label={T('activity.settings')}
        title={T('activity.settings')}
        on:click={openSettings}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M19.4 13.5a7.8 7.8 0 0 0 .1-1.5 7.8 7.8 0 0 0-.1-1.5l2-1.5-2-3.4-2.4 1a7 7 0 0 0-2.6-1.5L14 2.5h-4l-.4 2.6A7 7 0 0 0 7 6.6l-2.4-1-2 3.4 2 1.5a7.8 7.8 0 0 0-.1 1.5 7.8 7.8 0 0 0 .1 1.5l-2 1.5 2 3.4 2.4-1a7 7 0 0 0 2.6 1.5l.4 2.6h4l.4-2.6a7 7 0 0 0 2.6-1.5l2.4 1 2-3.4zM12 15.2A3.2 3.2 0 1 1 12 8.8a3.2 3.2 0 0 1 0 6.4z" /></svg>
      </button>
      <span class="vault-chip" title={notesRoot}>{vaultBasename(notesRoot)}</span>
      {#if lastFileEvent}
        <span class="event" title={lastFileEvent}>{T('app.indexed')}</span>
      {/if}
      {#if !aiReachable}
        <span class="ai-offline-pill" role="status">{T('app.aiOffline')}</span>
      {/if}
    </div>
  </header>

  {#if selectedIds.length > 0}
    <div class="bulk-bar" role="toolbar" aria-label={T('app.multiSelect')}>
      <span class="bulk-count">{T('app.selectedCount', { count: selectedIds.length })}</span>
      <button type="button" class="btn secondary sm" on:click={copySelectedMarkdown}>{T('app.copyText')}</button>
      <button type="button" class="btn secondary sm" on:click={clearSelection}>{T('app.clear')}</button>
    </div>
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

  <div
    class="layout-grid"
    class:pages-hidden={!pagesOpen}
    class:inspector-hidden={!inspectorOpen}
    data-mobile-panel={mobilePanel}
  >
  <aside class="vault-browser col-pages" aria-label={T('app.vaultBrowser')}>
    <div class="vault-browser-head">
      <div>
        <h2>{T('activity.files')}</h2>
      </div>
      <div class="vault-actions" role="toolbar" aria-label={T('activity.files')}>
        <button type="button" class="mini-btn" on:click={() => (paletteOpen = true)} title={T('app.search')} aria-label={T('app.search')}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" d="m20 20-4.2-4.2M18 10.8a7.2 7.2 0 1 1-14.4 0 7.2 7.2 0 0 1 14.4 0Z" /></svg>
        </button>
        <button type="button" class="mini-btn" on:click={openNewPageDialog} title={T('app.newNote')} aria-label={T('app.newNote')}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M6 3h9l5 5v13H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2m8 2v4h4zM9 13v2h3v3h2v-3h3v-2h-3v-3h-2v3z" /></svg>
        </button>
        <button type="button" class="mini-btn" on:click={() => loadVaultPages()} title={T('app.refreshPages')} aria-label={T('app.refreshPages')}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M17.7 6.3A8 8 0 1 0 20 12h-2a6 6 0 1 1-1.76-4.24L13 11h8V3z" /></svg>
        </button>
        <button type="button" class="mini-btn" on:click={() => (pagesOpen = false)} title={T('app.collapsePanel')} aria-label={T('app.collapsePanel')}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M7 5h10v2H7zm3.8 4.2 1.4 1.4L10.8 12l1.4 1.4-1.4 1.4L8 12zm4.2 0L17.8 12 15 14.8l-1.4-1.4L15.2 12l-1.6-1.4zM7 17h10v2H7z" /></svg>
        </button>
      </div>
    </div>
    <input
      class="page-filter"
      bind:value={pageFilter}
      placeholder={T('app.filterFiles')}
      autocomplete="off"
      spellcheck="false"
    />
    {#if visibleRecentPaths.length}
      <section class="page-section" aria-label={T('app.recentPages')}>
        <h3>{T('app.recentPages')}</h3>
        <div class="page-list compact">
          {#each visibleRecentPaths as rel (rel)}
            <button
              type="button"
              class="page-row"
              class:current={rel === pagePath}
              on:click={() => {
                mobilePanel = 'outline'
                void loadPage(rel)
              }}
            >
              <span class="page-dot" aria-hidden="true"></span>
              <span class="page-name">{pageTitle(rel)}</span>
            </button>
          {/each}
        </div>
      </section>
    {/if}
    <section class="page-section page-section-fill" aria-label={T('app.allFiles')}>
      <h3>{T('app.allFiles')}</h3>
      {#if pagesLoading}
        <p class="nav-muted">{T('app.filesLoading')}</p>
      {:else if !filteredVaultFiles.length}
        <p class="nav-muted">{T('app.noFiles')}</p>
      {:else}
        <div class="page-list">
          {#each filteredVaultFiles as file (file.path)}
            <button
              type="button"
              class="page-row"
              class:active={isMarkdownFile(file) && file.path === pagePath}
              class:external={!isMarkdownFile(file)}
              on:click={() => openVaultEntry(file)}
              on:dblclick={() => openVaultEntry(file)}
            >
              <span class="page-icon file-kind kind-{file.kind || 'other'}" aria-hidden="true">
                <svg viewBox="0 0 24 24"><path fill="currentColor" d="M6 3h9l5 5v13H6zM14 4.5V9h4.5" opacity="0.85" /></svg>
              </span>
              <span class="page-copy">
                <span class="page-name">{fileTitle(file)}</span>
                {#if pageFolder(file.path)}
                  <span class="page-folder">{pageFolder(file.path)}</span>
                {/if}
              </span>
              {#if fileKindLabel(file)}
                <span class="file-ext">{fileKindLabel(file)}</span>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </section>
    <div class="vault-footer">
      <button type="button" class="footer-icon" on:click={() => (pagesOpen = false)} aria-label={T('app.collapsePanel')} title={T('app.collapsePanel')}>
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="m15.4 6.4 1.4 1.4-4.2 4.2 4.2 4.2-1.4 1.4L9 12z" /></svg>
      </button>
      <span class="vault-footer-name" title={notesRoot}>{vaultBasename(notesRoot)}</span>
      <span class="vault-footer-count">{vaultFiles.length || vaultPages.length}</span>
    </div>
  </aside>

  {#if graphOpen}
  <section class="col-main graph-workspace">
    <div class="graph-view-head">
      <div>
        <h2>{T('app.pageGraph')}</h2>
        <p>{graphData.nodes.length} {T('app.pages')} · {graphData.edges.length} {T('graph.links')}</p>
      </div>
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
  {:else if mindMapOpen}
  <section class="col-main graph-workspace mindmap-workspace">
    <div class="graph-view-head">
      <div>
        <h2>{T('app.pageMindMap')}</h2>
        <p>{pageTitle(pagePath)} · {countOutlineBlocks(roots)} {T('mindmap.blocks')}</p>
      </div>
      <div class="graph-head-actions">
        <button type="button" class="btn secondary sm" on:click={() => (mindMapOpen = false)}>{T('app.close')}</button>
      </div>
    </div>
    <MindMap
      blocks={roots}
      pageTitle={pageTitle(pagePath)}
      collapsedMap={collapsedState}
      {selectedIds}
      onToggleCollapse={toggleCollapse}
      onToggleSelect={toggleSelect}
      onUpdateNode={handleMindMapUpdate}
      onMoveUnder={handleMindMapMoveUnder}
      onInsertChild={handleMindMapInsertChild}
      onRunCommand={handleRunBlockCommand}
      onOpenTerminalContext={handleOpenTerminalContext}
    />
  </section>
  {:else}
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
          onRunCommand={handleRunBlockCommand}
          onOpenTerminalContext={handleOpenTerminalContext}
        />
      {/each}
    {/if}
    </section>
  {/if}

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
        class:active={sideTab === 'related'}
        aria-selected={sideTab === 'related'}
        id="tab-related"
        on:click={() => (sideTab = 'related')}
      >
        {T('sidebar.tabRelated')}
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
      aria-labelledby={sideTab === 'backlinks' ? 'tab-backlinks' : sideTab === 'related' ? 'tab-related' : 'tab-ai'}
    >
      {#if sideTab === 'backlinks'}
        <Backlinks {notesRoot} {pagePath} indexEpoch={indexEpoch} onOpenPage={(rel) => loadPage(rel)} />
      {:else if sideTab === 'related'}
        <SemanticRelated {pagePath} indexEpoch={indexEpoch} onOpenPage={(rel) => loadPage(rel)} />
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

  <WorkspaceConsole bind:this={consolePane} {notesRoot} open={consoleOpen} onClose={() => (consoleOpen = false)} />
  </section>
</main>

{#if showMobileChrome}
  <div class="mobile-fab-stack" aria-label={T('app.fabAria')}>
    <button
      type="button"
      class="fab-btn"
      on:click={toggleTheme}
      aria-label={theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
      title={theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
    >
      {#if theme === 'dark'}
        <svg class="fab-ico" viewBox="0 0 24 24" aria-hidden="true"
          ><path
            fill="currentColor"
            d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10zm0-5h2v3h-2V2zm0 19h2v3h-2v-3zM2 11h3v2H2v-2zm19 0h3v2h-3v-2zM4.22 4.22l2.12 2.12-1.41 1.41L2.81 5.64 4.22 4.22zm12.73 12.73 2.12 2.12-1.41 1.41-2.12-2.12 1.41-1.41zM19.78 4.22l-2.12 2.12-1.41-1.41 2.12-2.12 1.41 1.41zM7.05 18.95l-2.12 2.12-1.41-1.41 2.12-2.12 1.41 1.41z"
          /></svg
        >
      {:else}
        <svg class="fab-ico" viewBox="0 0 24 24" aria-hidden="true"
          ><path fill="currentColor" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" /></svg
        >
      {/if}
    </button>
    <button
      type="button"
      class="fab-btn fab-primary"
      on:click={openNewPageDialog}
      aria-label={T('app.newNote')}
      title={T('app.newNote')}
    >
      <svg class="fab-ico" viewBox="0 0 24 24" aria-hidden="true"
        ><path
          fill="currentColor"
          d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm4 18H6V4h7v5h5v11zM12 11H8v2h4v4h2v-4h4v-2h-4V7h-2v4z"
        /></svg
      >
    </button>
  </div>
{/if}

<nav class="mobile-tabbar" aria-label={T('app.mobileNavAria')}>
  <button
    type="button"
    class="mobile-tab-btn"
    class:active={mobilePanel === 'outline' && !graphOpen && !mindMapOpen && !settingsOpen}
    on:click={() => {
      graphOpen = false
      mindMapOpen = false
      settingsOpen = false
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
    class:active={mobilePanel === 'pages' && !graphOpen && !mindMapOpen && !settingsOpen}
    on:click={() => {
      graphOpen = false
      mindMapOpen = false
      settingsOpen = false
      mobilePanel = 'pages'
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><path
          fill="currentColor"
          d="M4 4h16v3H4V4zm0 5h16v3H4V9zm0 5h11v3H4v-3zm0 5h8v2H4v-2z"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavPages')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn"
    class:active={mobilePanel === 'side' && !graphOpen && !mindMapOpen && !settingsOpen}
    on:click={() => {
      graphOpen = false
      mindMapOpen = false
      settingsOpen = false
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
    class:active={mindMapOpen}
    aria-label={T('app.mobileNavMindMap')}
    title={T('app.mobileNavMindMap')}
    on:click={() => {
      settingsOpen = false
      openMindMap()
    }}
  >
    <span class="mobile-tab-ico" aria-hidden="true">
      <svg viewBox="0 0 24 24"
        ><circle cx="5.5" cy="12" r="2.3" fill="currentColor" /><circle cx="13" cy="6" r="2.2" fill="currentColor" /><circle
          cx="13"
          cy="18"
          r="2.2"
          fill="currentColor"
        /><circle cx="20" cy="12" r="2.1" fill="currentColor" /><path
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          d="M7.5 10.7 11.1 7.4M7.5 13.3l3.6 3.3M15.1 7.4 18.2 11M15.1 16.6l3.1-3.6"
        /></svg
      >
    </span>
    <span class="mobile-tab-lbl">{T('app.mobileNavMindMap')}</span>
  </button>
  <button
    type="button"
    class="mobile-tab-btn mobile-tab-iconish"
    class:active={graphOpen}
    aria-label={T('app.mobileNavGraph')}
    title={T('app.mobileNavGraph')}
    on:click={() => {
      settingsOpen = false
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
    class:active={settingsOpen}
    aria-label={T('app.mobileNavSettings')}
    title={T('app.mobileNavSettings')}
    on:click={() => {
      graphOpen = false
      mindMapOpen = false
      void openSettings()
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
    <span class="mobile-tab-lbl">{T('app.mobileNavSettings')}</span>
  </button>
</nav>

<CommandPalette
  open={paletteOpen}
  {notesRoot}
  onSelectPage={(rel) => loadPage(rel)}
  onSelectBlockHit={openBlockHit}
  onClose={() => (paletteOpen = false)}
/>

{#if newPageDialogOpen}
  <div
    class="dialog-backdrop"
    role="presentation"
    on:click|self={() => (newPageDialogOpen = false)}
  >
    <form
      class="new-note-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="new-note-title"
      on:submit|preventDefault={submitNewPage}
    >
      <div class="dialog-head">
        <h2 id="new-note-title">{T('app.newNote')}</h2>
        <button type="button" class="dialog-close" aria-label={T('app.close')} on:click={() => (newPageDialogOpen = false)}>
          ×
        </button>
      </div>
      <label class="new-note-field">
        <span>{T('app.newPagePrompt')}</span>
        <input
          bind:this={newPageInput}
          bind:value={newPagePath}
          placeholder={T('app.newPageDefault')}
          autocomplete="off"
          spellcheck="false"
          on:keydown={(e) => {
            if (e.key === 'Escape') newPageDialogOpen = false
          }}
        />
      </label>
      <div class="dialog-actions">
        <button type="button" class="dialog-btn" on:click={() => (newPageDialogOpen = false)}>{T('app.cancel')}</button>
        <button type="submit" class="dialog-btn primary" disabled={!newPagePath.trim()}>{T('app.create')}</button>
      </div>
    </form>
  </div>
{/if}

<SettingsDialog
  open={settingsOpen}
  {appVersion}
  {theme}
  localeCode={$locale}
  {aiReachable}
  {notesRoot}
  {healthBusy}
  onClose={() => (settingsOpen = false)}
  onSetLanguage={(code) => setLanguage(code)}
  onToggleTheme={() => toggleTheme()}
  onHealthReset={() => runHealthReset()}
/>

<ToastStack />

<style>
  :global(html) {
    background: var(--dv-app-bg);
    --dv-fg: #d9dce4;
    --dv-muted: rgba(217, 220, 228, 0.58);
    --dv-border: rgba(255, 255, 255, 0.1);
    --dv-input: #151821;
    --dv-panel: #1a1d26;
    --dv-app-bg: #101218;
    --dv-rail-bg: #0b0d12;
    --dv-titlebar: #151821;
    --dv-surface: #161a22;
    --dv-surface-2: #1b202a;
    --dv-editor: #171b24;
    --dv-tool-window: #10131a;
    --dv-accent: #7aa2f7;
    --dv-accent-2: #65c89b;
    --dv-danger: #f7768e;
    --dv-hit-hover: rgba(255, 255, 255, 0.06);
    --dv-toast-bg: rgba(30, 30, 36, 0.96);
    --dv-toast-border: rgba(255, 255, 255, 0.12);
  }

  :global(html[data-theme='light']) {
    --dv-fg: #1f2328;
    --dv-muted: rgba(31, 35, 40, 0.56);
    --dv-border: rgba(31, 35, 40, 0.14);
    --dv-input: #fbfbfc;
    --dv-panel: #ffffff;
    --dv-app-bg: #eef1f5;
    --dv-rail-bg: #e9edf3;
    --dv-titlebar: #f7f8fa;
    --dv-surface: #f5f6f8;
    --dv-surface-2: #ffffff;
    --dv-editor: #ffffff;
    --dv-tool-window: #f5f6f8;
    --dv-accent: #6f4fd8;
    --dv-accent-2: #16865a;
    --dv-danger: #c2415b;
    --dv-hit-hover: rgba(0, 0, 0, 0.05);
    --dv-toast-bg: rgba(255, 252, 248, 0.98);
    --dv-toast-border: rgba(0, 0, 0, 0.1);
  }

  :global(body) {
    margin: 0;
    min-height: 100vh;
    min-height: 100dvh;
    background: var(--dv-app-bg);
    color: var(--dv-fg);
    font-family: var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', 'Segoe UI', sans-serif);
    font-size: 13px;
    line-height: 1.45;
    -webkit-font-smoothing: antialiased;
    overflow: hidden;
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
  main[data-chrome-mode='tablet-master'].layout {
    max-width: min(100%, 1440px);
  }
  .top {
    padding-left: max(0px, env(safe-area-inset-left, 0px));
    padding-right: max(0px, env(safe-area-inset-right, 0px));
    padding-top: max(0px, env(safe-area-inset-top, 0px));
    box-sizing: border-box;
  }
  main[data-chrome-mode='tablet-master'] .top {
    min-height: 4.75rem;
    padding-bottom: 10px;
  }
  main[data-chrome-mode='tablet-master'] .toolbar {
    margin-top: 14px;
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
  .dialog-backdrop {
    position: fixed;
    inset: 0;
    z-index: 1000;
    display: grid;
    place-items: center;
    padding: 24px;
    background: color-mix(in srgb, #000 28%, transparent);
    -webkit-backdrop-filter: blur(2px);
    backdrop-filter: blur(2px);
  }
  .new-note-dialog {
    width: min(420px, calc(100vw - 40px));
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: var(--dv-panel);
    color: var(--dv-fg);
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.22);
    padding: 14px;
  }
  .dialog-head {
    min-height: 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
  }
  .dialog-head h2 {
    margin: 0;
    font-size: 0.9rem;
    font-weight: 650;
    letter-spacing: 0;
  }
  .dialog-close {
    width: 26px;
    height: 26px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-muted);
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }
  .dialog-close:hover {
    background: color-mix(in srgb, var(--dv-fg) 7%, transparent);
    color: var(--dv-fg);
  }
  .new-note-field {
    display: grid;
    gap: 6px;
    color: var(--dv-muted);
    font-size: 0.76rem;
  }
  .new-note-field input {
    width: 100%;
    min-height: 34px;
    box-sizing: border-box;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: var(--dv-fg);
    padding: 7px 9px;
    font: inherit;
    font-size: 0.84rem;
  }
  .new-note-field input:focus {
    outline: none;
    border-color: color-mix(in srgb, var(--dv-accent) 38%, var(--dv-border));
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--dv-accent) 13%, transparent);
  }
  .dialog-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 14px;
  }
  .dialog-btn {
    min-height: 30px;
    padding: 0 12px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: inherit;
    font-size: 0.78rem;
    cursor: pointer;
  }
  .dialog-btn.primary {
    border-color: color-mix(in srgb, var(--dv-accent) 35%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-accent) 18%, transparent);
  }
  .dialog-btn:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
  .hamburger-btn {
    flex-shrink: 0;
  }

  @media (min-width: 900px) {
    .layout {
      max-width: min(100%, 1680px);
    }
    main[data-chrome-mode='tablet-master'] .layout-grid {
      display: grid;
      grid-template-columns: minmax(220px, 260px) minmax(360px, 1fr) minmax(280px, 340px);
      gap: 14px;
      align-items: start;
    }
    main[data-chrome-mode='tablet-master'] .layout-grid .dv-sidebar {
      margin-top: 0;
    }
    main[data-chrome-mode='tablet-master'] .layout-grid .vault-browser {
      margin-top: 0;
    }
    main[data-chrome-mode='tablet-master'] .layout-grid .col-main.outliner-panel {
      margin-top: 0;
    }
  }
  .mobile-tabbar {
    display: none;
  }
  @media (max-width: 899px) {
    main.layout:not([data-chrome-mode='tablet-master']) {
      max-width: 100%;
      padding: max(12px, env(safe-area-inset-top, 0px)) max(12px, env(safe-area-inset-left, 0px))
        calc(12px + 56px + max(env(safe-area-inset-bottom, 0px), 12px)) max(12px, env(safe-area-inset-right, 0px));
    }
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='outline'] .col-pages,
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='outline'] .col-side {
      display: none;
    }
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='pages'] .col-main,
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='pages'] .col-side {
      display: none;
    }
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='side'] .col-main,
    main.layout:not([data-chrome-mode='tablet-master']) .layout-grid[data-mobile-panel='side'] .col-pages {
      display: none;
    }
    .mobile-tabbar {
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
    .mobile-tab-btn {
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
    .mobile-tab-btn.active {
      border-color: rgba(120, 160, 255, 0.45);
      background: rgba(80, 120, 255, 0.14);
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
  .event {
    margin: 0;
    font-size: 0.85rem;
    opacity: 0.75;
  }
  .toolbar {
    display: flex;
    gap: 12px;
    margin-top: 16px;
    flex-wrap: wrap;
    align-items: center;
  }
  .toolbar.tool-ribbon {
    flex-wrap: nowrap;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: thin;
    align-items: center;
    padding-bottom: 2px;
    gap: 10px;
  }
  main[data-chrome-mode='tablet-master'] .toolbar.tool-ribbon .path-input {
    flex: 1 1 200px;
    min-width: 140px;
    max-width: min(360px, 42vw);
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
  .vault-browser {
    margin-top: 20px;
    padding: 12px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
    min-height: 240px;
    max-height: calc(100vh - 180px);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .vault-browser-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }
  .vault-browser h2,
  .page-section h3 {
    margin: 0;
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.58;
    font-weight: 650;
  }
  .mini-btn {
    width: 34px;
    height: 34px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: inherit;
  }
  .mini-btn svg {
    width: 17px;
    height: 17px;
  }
  .page-filter {
    width: 100%;
    min-height: 40px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font: inherit;
    font-size: 0.88rem;
  }
  .page-section {
    min-width: 0;
  }
  .page-section-fill {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .page-section h3 {
    margin-bottom: 8px;
  }
  .page-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-height: 0;
    overflow-y: auto;
    padding-right: 2px;
  }
  .page-list.compact {
    max-height: 180px;
  }
  .page-row {
    width: 100%;
    min-height: 34px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border: 1px solid transparent;
    border-radius: 6px;
    background: transparent;
    color: inherit;
    text-align: left;
    min-width: 0;
  }
  .page-row:hover {
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
  }
  .page-row:focus {
    outline: none;
  }
  .page-row:focus-visible {
    background: color-mix(in srgb, var(--dv-fg) 7%, transparent);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--dv-fg) 13%, transparent);
  }
  .page-row.current,
  .page-row.active {
    border-color: transparent;
    background: color-mix(in srgb, var(--dv-fg) 9%, transparent);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--dv-fg) 7%, transparent);
    color: color-mix(in srgb, var(--dv-fg) 96%, var(--dv-muted));
  }
  .page-row.current:focus-visible,
  .page-row.active:focus-visible {
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--dv-fg) 12%, transparent);
  }
  .page-row.external {
    color: color-mix(in srgb, var(--dv-fg) 84%, var(--dv-muted));
  }
  .page-icon {
    width: 17px;
    height: 17px;
    flex: 0 0 17px;
    color: color-mix(in srgb, var(--dv-fg) 62%, transparent);
  }
  .page-icon svg {
    width: 17px;
    height: 17px;
    display: block;
  }
  .file-kind.kind-office {
    color: color-mix(in srgb, #3a7bd5 55%, var(--dv-muted));
  }
  .file-kind.kind-pdf {
    color: color-mix(in srgb, #cf4f45 58%, var(--dv-muted));
  }
  .file-kind.kind-image {
    color: color-mix(in srgb, #4f8d62 58%, var(--dv-muted));
  }
  .file-kind.kind-cad {
    color: color-mix(in srgb, #9a7a2d 58%, var(--dv-muted));
  }
  .page-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex: 0 0 6px;
    background: color-mix(in srgb, var(--dv-accent) 72%, transparent);
  }
  .page-copy {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }
  .page-name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.86rem;
    line-height: 1.25;
  }
  .page-folder {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.68rem;
    opacity: 0.45;
    line-height: 1.25;
  }
  .file-ext {
    margin-left: auto;
    flex: 0 0 auto;
    max-width: 56px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 1px 5px;
    border-radius: 4px;
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: var(--dv-muted);
    font-size: 0.62rem;
    line-height: 1.35;
  }
  .nav-muted {
    margin: 0;
    padding: 8px 2px;
    font-size: 0.82rem;
    opacity: 0.52;
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
    main.layout:not([data-chrome-mode='tablet-master']) .btn.sm {
      min-height: 48px;
    }
  }
  .mobile-fab-stack {
    position: fixed;
    right: max(16px, env(safe-area-inset-right, 0px));
    bottom: calc(64px + max(12px, env(safe-area-inset-bottom, 0px)));
    z-index: 58;
    display: flex;
    flex-direction: column;
    gap: 12px;
    align-items: center;
  }
  .fab-btn {
    width: 52px;
    height: 52px;
    border-radius: 50%;
    border: 1px solid var(--dv-border);
    display: grid;
    place-items: center;
    background: color-mix(in srgb, var(--dv-panel) 92%, transparent);
    color: var(--dv-fg);
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.22);
    touch-action: manipulation;
    padding: 0;
    cursor: pointer;
  }
  .fab-btn.fab-primary {
    background: rgba(80, 120, 255, 0.35);
    border-color: rgba(120, 160, 255, 0.45);
  }
  .fab-ico {
    width: 24px;
    height: 24px;
    display: block;
  }
  main[data-chrome-mode='tablet-master'] .outliner-panel :global(.row) {
    margin-bottom: 4px;
  }
  main[data-chrome-mode='tablet-master'] .outliner-panel :global(.row textarea) {
    padding: 6px 8px;
  }
  .outliner-panel :global(textarea[data-block-id]) {
    scroll-margin-bottom: max(96px, calc(56px + env(safe-area-inset-bottom, 0px)));
  }
  .graph-view-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 40px;
    padding: 0 10px;
    border-bottom: 1px solid var(--dv-border);
  }
  .graph-view-head h2 {
    margin: 0;
    font-size: 0.86rem;
    font-weight: 520;
  }
  .graph-view-head p {
    margin: 2px 0 0;
    color: var(--dv-muted);
    font-size: 0.7rem;
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
  .plugin-tb {
    font-size: 0.85rem;
  }
  .dv-sidebar {
    margin-top: 20px;
    padding: 12px 14px;
    border-radius: 10px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 4%, transparent);
    max-height: calc(100vh - 180px);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .side-tabs {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 6px;
    margin-bottom: 12px;
    flex-shrink: 0;
  }
  .side-tab {
    min-width: 0;
    min-height: 40px;
    padding: 7px 8px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: inherit;
    font-size: 0.74rem;
    font-weight: 500;
    cursor: pointer;
    opacity: 0.65;
    touch-action: manipulation;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .side-tab.active {
    opacity: 1;
    border-color: rgba(120, 160, 255, 0.35);
    background: rgba(80, 120, 255, 0.1);
  }
  .side-panel {
    flex: 1;
    min-height: 120px;
    overflow-y: auto;
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

  .layout.ide-shell {
    max-width: none;
    width: 100vw;
    height: 100vh;
    height: 100dvh;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: 40px minmax(0, 1fr);
    grid-template-rows: 38px minmax(0, 1fr);
    background:
      linear-gradient(180deg, color-mix(in srgb, var(--dv-accent) 4%, transparent), transparent 220px),
      var(--dv-app-bg);
    overflow: hidden;
  }
  .layout.ide-shell::before {
    content: '';
    grid-column: 1;
    grid-row: 1;
    min-width: 0;
    min-height: 0;
    background: color-mix(in srgb, var(--dv-titlebar) 94%, transparent);
    border-bottom: 1px solid var(--dv-border);
    pointer-events: none;
  }
  .workspace-stage {
    grid-column: 2;
    grid-row: 1 / span 2;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .activity-rail {
    grid-column: 1;
    grid-row: 2;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    padding: max(7px, env(safe-area-inset-top, 0px)) 0 max(7px, env(safe-area-inset-bottom, 0px));
    background: var(--dv-rail-bg);
  }
  .rail-btn {
    position: relative;
    width: 40px;
    height: 37px;
    display: grid;
    place-items: center;
    border: 0;
    border-radius: 0;
    background: transparent;
    color: var(--dv-muted);
    cursor: pointer;
  }
  .rail-btn::before {
    content: '';
    position: absolute;
    left: 0;
    top: 50%;
    width: 2px;
    height: 18px;
    border-radius: 0 2px 2px 0;
    transform: translateY(-50%) scaleY(0.35);
    opacity: 0;
    background: linear-gradient(180deg, transparent, var(--dv-accent), transparent);
    box-shadow: 0 0 12px color-mix(in srgb, var(--dv-accent) 42%, transparent);
    transition:
      opacity 0.16s ease,
      transform 0.2s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .rail-btn:hover {
    color: var(--dv-fg);
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
  }
  .rail-btn.active {
    color: var(--dv-accent);
  }
  .rail-btn.active::before {
    opacity: 1;
    transform: translateY(-50%) scaleY(1);
  }
  .rail-btn svg {
    width: 16px;
    height: 16px;
  }
  .rail-spacer {
    flex: 1;
  }
  .rail-pro.active,
  .rail-pro:hover {
    color: var(--dv-accent-2);
  }
  .app-titlebar {
    height: 38px;
    min-height: 38px;
    padding: 0 7px 0 72px;
    display: grid;
    grid-template-columns: minmax(210px, 330px) minmax(160px, 1fr) auto;
    align-items: center;
    gap: 7px;
    border-bottom: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-titlebar) 94%, transparent);
    -webkit-backdrop-filter: blur(18px);
    backdrop-filter: blur(18px);
    --wails-draggable: drag;
  }
  main[data-chrome-mode='tablet-master'] .app-titlebar {
    min-height: 38px;
    padding-bottom: 0;
  }
  .titlebar-left {
    min-width: 0;
    height: 100%;
    display: flex;
    align-items: center;
    gap: 3px;
  }
  .nav-icon {
    --wails-draggable: no-drag;
    width: 28px;
    height: 28px;
    flex: 0 0 28px;
    padding: 0;
    display: grid;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-muted);
    cursor: pointer;
  }
  .nav-icon:hover {
    border-color: color-mix(in srgb, var(--dv-fg) 10%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    color: var(--dv-fg);
  }
  .nav-icon.active {
    color: var(--dv-accent);
    background: color-mix(in srgb, var(--dv-accent) 11%, transparent);
  }
  .nav-icon.primary {
    color: color-mix(in srgb, var(--dv-accent) 88%, var(--dv-fg));
    background: color-mix(in srgb, var(--dv-accent) 12%, transparent);
  }
  .nav-icon:disabled {
    opacity: 0.32;
    cursor: default;
  }
  .nav-icon:disabled:hover {
    border-color: transparent;
    background: transparent;
    color: var(--dv-muted);
  }
  .nav-icon svg {
    width: 15px;
    height: 15px;
    display: block;
  }
  .tab-strip {
    min-width: 0;
    display: flex;
    align-items: end;
    height: 100%;
    padding-top: 4px;
  }
  .doc-tab {
    --wails-draggable: no-drag;
    min-width: 0;
    max-width: 170px;
    height: 33px;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
    border: 1px solid var(--dv-border);
    border-bottom-color: color-mix(in srgb, var(--dv-editor) 92%, var(--dv-border));
    border-radius: 6px 6px 0 0;
    background: var(--dv-editor);
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.83rem;
  }
  .doc-tab span:last-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .doc-tab-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--dv-accent-2);
    flex: 0 0 7px;
  }
  .titlebar-status {
    display: flex;
    align-items: center;
    gap: 3px;
    min-width: 0;
  }
  .titlebar-status .event {
    margin: 0;
    color: var(--dv-muted);
    font-size: 0.72rem;
    opacity: 0.88;
  }
  .titlebar-status .ai-offline-pill {
    margin: 0;
  }
  .vault-chip {
    max-width: 150px;
    margin-left: 5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--dv-fg);
    font-size: 0.78rem;
    opacity: 0.9;
  }
  .top-commandbar.toolbar {
    min-height: 0;
    height: 100%;
    margin: 0;
    padding: 0;
    border: 0;
    background: transparent;
    align-items: center;
    gap: 5px;
    flex-wrap: nowrap;
    overflow: hidden;
  }
  main[data-chrome-mode='tablet-master'] .top-commandbar.toolbar,
  .top-commandbar.toolbar.tool-ribbon {
    margin-top: 0;
    padding-bottom: 0;
    gap: 5px;
  }
  .top-commandbar .breadcrumbs {
    flex: 1 1 auto;
    min-width: 0;
    max-width: none;
    min-height: 26px;
    margin: 0;
    padding: 0 7px;
    display: flex;
    flex-wrap: nowrap;
    justify-content: center;
    overflow: hidden;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-muted);
    text-align: center;
    opacity: 1;
    font-size: 0.75rem;
    letter-spacing: 0;
  }
  .top-commandbar .breadcrumbs .crumb {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .top-commandbar .breadcrumbs .vault {
    flex: 0 1 auto;
  }
  .path-input {
    --wails-draggable: no-drag;
    min-height: 26px;
    height: 26px;
    min-width: 120px;
    max-width: 260px;
    padding: 2px 7px;
    border-radius: 5px;
    font-size: 0.78rem;
    font-family: var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif);
  }
  main[data-chrome-mode='tablet-master'] .toolbar.tool-ribbon .path-input {
    flex: 1 1 160px;
    min-width: 120px;
    max-width: min(260px, 30vw);
  }
  .command-icon {
    width: 26px;
    height: 26px;
    flex-basis: 26px;
  }
  .top-commandbar .path-input,
  .top-commandbar .command-icon {
    display: none;
  }
  .btn {
    --wails-draggable: no-drag;
    min-height: 30px;
    padding: 4px 10px;
    border-radius: 5px;
    background: color-mix(in srgb, var(--dv-accent) 14%, transparent);
    font-size: 0.84rem;
    cursor: pointer;
  }
  .btn.secondary {
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .plugin-tb {
    --wails-draggable: no-drag;
    min-height: 26px;
    padding: 3px 8px;
    border-radius: 5px;
    font-size: 0.74rem;
    white-space: nowrap;
  }
  .btn:hover,
  .mini-btn:hover,
  .side-tab:hover,
  .page-row:hover {
    border-color: color-mix(in srgb, var(--dv-accent) 28%, var(--dv-border));
  }
  .bulk-bar,
  .err,
  .plugin-sidebar {
    margin: 8px 8px 0;
    flex-shrink: 0;
  }
  .layout-grid {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
  main[data-chrome-mode='tablet-master'] .layout-grid {
    display: grid;
    grid-template-columns: minmax(248px, 304px) minmax(420px, 1fr) minmax(286px, 340px);
    gap: 0;
    align-items: stretch;
    padding: 0;
  }
  main[data-chrome-mode='tablet-master'] .layout-grid.pages-hidden {
    grid-template-columns: minmax(420px, 1fr) minmax(300px, 360px);
  }
  main[data-chrome-mode='tablet-master'] .layout-grid.inspector-hidden {
    grid-template-columns: minmax(230px, 280px) minmax(420px, 1fr);
  }
  main[data-chrome-mode='tablet-master'] .layout-grid.pages-hidden.inspector-hidden {
    grid-template-columns: minmax(420px, 1fr);
  }
  .layout-grid.pages-hidden .col-pages,
  .layout-grid.inspector-hidden .col-side {
    display: none;
  }
  .vault-browser,
  .outliner-panel,
  .graph-workspace,
  .dv-sidebar {
    margin-top: 0;
    border-radius: 0;
    border: 0;
    background: var(--dv-surface-2);
    box-shadow: none;
  }
  .vault-browser,
  .dv-sidebar {
    padding: 0;
    max-height: none;
    min-height: 0;
    height: 100%;
  }
  .vault-browser {
    background: var(--dv-surface);
    border-right: 1px solid var(--dv-border);
    gap: 0;
  }
  .vault-browser-head {
    min-height: 34px;
    padding: 0 6px 0 10px;
    border-bottom: 1px solid var(--dv-border);
  }
  .vault-actions {
    display: flex;
    align-items: center;
    gap: 1px;
  }
  .vault-browser h2,
  .page-section h3,
  .outliner-panel h2,
  .graph-view-head h2,
  .plugin-card-title {
    letter-spacing: 0.04em;
    font-size: 0.68rem;
  }
  .vault-browser h2 {
    text-transform: none;
    letter-spacing: 0;
    font-size: 0.84rem;
    opacity: 0.82;
  }
  .mini-btn {
    width: 26px;
    height: 26px;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-muted);
  }
  .mini-btn:hover {
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    color: var(--dv-fg);
  }
  .mini-btn svg {
    width: 15px;
    height: 15px;
  }
  .page-filter {
    width: calc(100% - 16px);
    min-height: 28px;
    margin: 7px 8px 6px;
    padding: 4px 8px;
    border-radius: 5px;
    font-size: 0.8rem;
  }
  .page-section {
    padding: 0 6px 7px;
  }
  .page-section h3 {
    margin: 0;
    padding: 6px 7px 4px;
  }
  .page-list {
    gap: 0;
    padding-right: 0;
  }
  .page-list.compact {
    max-height: 130px;
  }
  .page-row {
    min-height: 26px;
    padding: 2px 7px;
    border-radius: 4px;
  }
  .page-name {
    font-size: 0.82rem;
  }
  .page-folder {
    font-size: 0.63rem;
  }
  .vault-footer {
    min-height: 34px;
    margin-top: auto;
    padding: 0 5px;
    display: flex;
    align-items: center;
    gap: 5px;
    border-top: 1px solid var(--dv-border);
    color: var(--dv-muted);
    font-size: 0.74rem;
  }
  .footer-icon {
    width: 25px;
    height: 25px;
    padding: 0;
    display: grid;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: inherit;
    cursor: pointer;
  }
  .footer-icon:hover {
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    color: var(--dv-fg);
  }
  .footer-icon svg {
    width: 14px;
    height: 14px;
  }
  .vault-footer-name {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .vault-footer-count {
    padding: 1px 5px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    font-size: 0.68rem;
  }
  .outliner-panel {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: 12px 16px;
    background: var(--dv-editor);
    border-right: 1px solid var(--dv-border);
  }
  .graph-workspace {
    height: 100%;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: var(--dv-editor);
    border-right: 1px solid var(--dv-border);
  }
  .graph-workspace :global(.graph-wrap) {
    flex: 1;
    min-height: 0;
    margin: 0;
    border: 0;
    border-radius: 0;
  }
  .outliner-panel :global(.row) {
    border-radius: 4px;
  }
  .outliner-panel :global(.ta) {
    border-radius: 5px;
    border-color: transparent;
    background: transparent;
    color: color-mix(in srgb, var(--dv-fg) 92%, var(--dv-muted));
    font-family: var(--dv-font-editor, var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', sans-serif));
    font-size: 0.9rem;
    line-height: 1.5;
  }
  .outliner-panel :global(.ta:focus) {
    border-color: color-mix(in srgb, var(--dv-fg) 18%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-fg) 3.5%, transparent);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--dv-accent) 16%, transparent);
  }
  .dv-sidebar {
    background: var(--dv-surface);
  }
  .side-tabs {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 3px;
    padding: 2px;
    border-radius: 5px;
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .side-tab {
    min-height: 28px;
    padding: 4px 6px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    font-size: 0.7rem;
  }
  .side-tab.active {
    background: var(--dv-panel);
    border-color: transparent;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  }
  .ai-offline-pill {
    min-height: 22px;
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 0.68rem;
    color: var(--dv-danger);
    background: color-mix(in srgb, var(--dv-danger) 10%, transparent);
    border-color: color-mix(in srgb, var(--dv-danger) 24%, transparent);
  }
  @media (max-width: 899px) {
    :global(body) {
      overflow: auto;
    }
    .layout.ide-shell {
      display: block;
      width: 100%;
      min-height: 100dvh;
      height: auto;
      padding: max(12px, env(safe-area-inset-top, 0px)) max(12px, env(safe-area-inset-left, 0px))
        calc(12px + 56px + max(env(safe-area-inset-bottom, 0px), 12px)) max(12px, env(safe-area-inset-right, 0px));
      overflow: visible;
    }
    .workspace-stage {
      overflow: visible;
      border-left: 0;
    }
    .activity-rail {
      display: none;
    }
    .app-titlebar {
      height: auto;
      min-height: 0;
      padding: 0;
      display: block;
      border: 0;
      background: transparent;
      -webkit-backdrop-filter: none;
      backdrop-filter: none;
    }
    .titlebar-left,
    .titlebar-status {
      display: none;
    }
    .top-commandbar.toolbar {
      height: auto;
      min-height: 0;
      padding: 8px;
      border-radius: 8px;
      border: 1px solid var(--dv-border);
      background: color-mix(in srgb, var(--dv-surface) 94%, var(--dv-panel));
      margin-bottom: 8px;
      flex-wrap: wrap;
      overflow: visible;
    }
    .top-commandbar .breadcrumbs {
      flex: 1 0 100%;
      max-width: none;
      margin-bottom: 0;
      border: 0;
      background: transparent;
    }
    .top-commandbar .path-input {
      display: block;
      flex: 1 1 180px;
      max-width: none;
      min-height: 38px;
      height: 38px;
      font-size: 16px;
    }
    .top-commandbar .command-icon {
      display: grid;
    }
  }
</style>
