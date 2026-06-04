<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { WriteTerminalInput, ResizeTerminal } from '../wailsjs/go/bridge/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'

  /** @type {{ id: string, cwd: string, kind?: string, command?: string, exitCode?: number | null }} */
  export let session
  export let active = true

  /** @type {HTMLDivElement | undefined} */
  let mountEl
  /** @type {any} */
  let term
  /** @type {any} */
  let fitAddon
  /** @type {ResizeObserver | undefined} */
  let resizeObserver
  /** @type {(() => void)[]} */
  let offEvents = []
  let disposed = false

  const theme = {
    background: '#15161a',
    foreground: '#d8dbe3',
    cursor: '#c9b7ff',
    selectionBackground: '#6f4fd855',
    black: '#1c1d22',
    red: '#ff6b72',
    green: '#56d68a',
    yellow: '#d6ba62',
    blue: '#8aa7ff',
    magenta: '#ba9cff',
    cyan: '#6fd7e5',
    white: '#e6e8ef',
    brightBlack: '#686d78',
    brightRed: '#ff858b',
    brightGreen: '#77e6a1',
    brightYellow: '#e0ca82',
    brightBlue: '#a3b8ff',
    brightMagenta: '#c9b7ff',
    brightCyan: '#87e2ee',
    brightWhite: '#ffffff'
  }

  async function fitAndResize() {
    if (!term || !fitAddon || !mountEl || disposed) return
    await tick()
    try {
      fitAddon.fit()
      if (session?.kind !== 'command') {
        await ResizeTerminal(session.id, term.rows, term.cols)
      }
    } catch {
      /* resize can race with hidden tabs */
    }
  }

  export function focus() {
    term?.focus()
  }

  $: if (active) {
    void fitAndResize()
  }

  async function initTerminal() {
    const [{ Terminal: XTerm }, { FitAddon }] = await Promise.all([
      import('@xterm/xterm'),
      import('@xterm/addon-fit'),
      import('@xterm/xterm/css/xterm.css')
    ])
    if (disposed || !mountEl) return

    term = new XTerm({
      convertEol: true,
      cursorBlink: session?.kind !== 'command',
      disableStdin: session?.kind === 'command',
      allowProposedApi: false,
      fontFamily: "var(--dv-font-mono, 'JetBrains Mono', ui-monospace, monospace)",
      fontSize: 12,
      lineHeight: 1.22,
      scrollback: 6000,
      theme
    })
    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(mountEl)
    term.onData((/** @type {string} */ data) => {
      if (session?.kind === 'command') return
      void WriteTerminalInput(session.id, data)
    })

    offEvents = [
      EventsOn('terminal-output', (/** @type {any} */ payload) => {
        if (!payload || payload.sessionId !== session.id) return
        term.write(String(payload.data || ''))
      }),
      EventsOn('terminal-error', (/** @type {any} */ payload) => {
        if (!payload || payload.sessionId !== session.id) return
        term.write(`\r\n\x1b[31m${String(payload.message || 'terminal error')}\x1b[0m\r\n`)
      }),
      EventsOn('terminal-exit', (/** @type {any} */ payload) => {
        if (!payload || payload.sessionId !== session.id) return
        term.write(`\r\n\x1b[2m[exit ${payload.exitCode ?? 0}]\x1b[0m\r\n`)
      })
    ].filter(Boolean)

    resizeObserver = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(() => void fitAndResize()) : undefined
    if (mountEl && resizeObserver) resizeObserver.observe(mountEl)
    void fitAndResize()
  }

  onMount(() => {
    void initTerminal()
  })

  onDestroy(() => {
    disposed = true
    for (const off of offEvents) {
      try {
        off?.()
      } catch {
        /* ignore runtime unsubscribe races */
      }
    }
    resizeObserver?.disconnect()
    term?.dispose()
  })
</script>

<div class="terminal-frame" class:active bind:this={mountEl}></div>

<style>
  .terminal-frame {
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: #15161a;
  }
  .terminal-frame :global(.xterm) {
    height: 100%;
    padding: 8px 10px;
  }
  .terminal-frame :global(.xterm-viewport) {
    background: transparent !important;
  }
  .terminal-frame :global(.xterm-screen) {
    letter-spacing: 0;
  }
</style>
