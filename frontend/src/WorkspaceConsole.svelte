<script>
  import { onMount, tick } from 'svelte'
  import {
    RunVaultCommand,
    StartTerminalSession,
    CloseTerminalSession,
    RunBlockCommand,
    OpenInWave
  } from '../wailsjs/go/bridge/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { messages, tr } from './lib/i18n/index.js'
  import { pushToast } from './toastStore.js'
  import Terminal from './Terminal.svelte'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  export let notesRoot = ''
  export let open = false
  export let onClose = () => {}

  let command = 'git status --short'
  let busy = false
  /** @type {{ id: string, command: string, cwd: string, output: string, exitCode: number, durationMs: number, timedOut: boolean, pending?: boolean }[]} */
  let history = []
  /** @type {{ id: string, cwd: string, kind: 'interactive' | 'command', title: string, command?: string, exitCode?: number | null }[]} */
  let sessions = []
  let activeSessionId = ''
  /** @type {Record<string, any>} */
  let terminalRefs = {}

  const quick = [
    { key: 'status', command: 'git status --short' },
    { key: 'list', command: 'ls -la' },
    { key: 'test', command: 'go test ./...' },
    { key: 'wails', command: 'wails build -clean' }
  ]

  /** @param {string} p */
  function shortPath(p) {
    if (!p) return '~'
    const parts = p.split(/[/\\]/).filter(Boolean)
    if (parts.length <= 2) return p
    return `…/${parts.slice(-2).join('/')}`
  }

  /** @param {string} cmd */
  function shortCommand(cmd) {
    const s = String(cmd || '').replace(/\s+/g, ' ').trim()
    return s.length > 26 ? s.slice(0, 25) + '…' : s || T('console.terminal')
  }

  /** @param {{ id?: string, sessionId?: string, cwd?: string, kind?: string, command?: string }} payload */
  function upsertSession(payload) {
    const id = payload.id || payload.sessionId || ''
    if (!id) return
    const kind = /** @type {'interactive' | 'command'} */ (payload.kind === 'command' ? 'command' : 'interactive')
    const existing = sessions.find((s) => s.id === id)
    if (existing) {
      sessions = sessions.map((s) =>
        s.id === id ? { ...s, cwd: payload.cwd || s.cwd, kind, command: payload.command || s.command } : s
      )
    } else {
      sessions = [
        ...sessions,
        {
          id,
          cwd: payload.cwd || notesRoot,
          kind,
          command: payload.command,
          exitCode: null,
          title: kind === 'command' ? shortCommand(payload.command || '') : shortPath(payload.cwd || notesRoot)
        }
      ].slice(-8)
    }
    activeSessionId = id
  }

  /** @param {string} cwd */
  export async function startSessionForCwd(cwd = '') {
    const info = await StartTerminalSession(cwd)
    upsertSession({ id: info.id, cwd: info.cwd, kind: 'interactive' })
    open = true
    await tick()
    terminalRefs[info.id]?.focus?.()
    return info
  }

  /** @param {string} blockID @param {string} cmd @param {string} cwd @param {boolean} confirmed */
  export async function runBlockCommand(blockID, cmd, cwd = '', confirmed = false) {
    open = true
    const result = await RunBlockCommand(blockID, cmd, cwd, confirmed)
    upsertSession({ id: result.sessionId, cwd: result.cwd, kind: 'command', command: result.command })
    sessions = sessions.map((s) => (s.id === result.sessionId ? { ...s, exitCode: result.exitCode } : s))
    activeSessionId = result.sessionId
    return result
  }

  /** @param {string | undefined} preset */
  async function run(preset = '') {
    const cmd = (preset || command).trim()
    if (!cmd || busy) return
    command = cmd
    busy = true
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`
    const pending = {
      id,
      command: cmd,
      cwd: notesRoot,
      output: '',
      exitCode: 0,
      durationMs: 0,
      timedOut: false,
      pending: true
    }
    history = [pending, ...history].slice(0, 18)
    try {
      const result = await RunVaultCommand(cmd)
      history = history.map((h) => (h.id === id ? { id, ...result, pending: false } : h))
    } catch (e) {
      history = history.map((h) =>
        h.id === id
          ? {
              ...h,
              output: String(e),
              exitCode: -1,
              durationMs: 0,
              timedOut: false,
              pending: false
            }
          : h
      )
      pushToast(String(e), 'error')
    } finally {
      busy = false
    }
  }

  async function openWave() {
    try {
      const result = await OpenInWave('')
      pushToast(result.message || (result.opened ? T('console.waveOpened') : T('console.waveMissing')), result.opened ? 'info' : 'error')
    } catch (e) {
      pushToast(String(e), 'error')
    }
  }

  /** @param {string} id */
  async function closeSession(id) {
    const s = sessions.find((x) => x.id === id)
    if (!s) return
    if (s.kind === 'interactive') {
      try {
        await CloseTerminalSession(id)
      } catch {
        /* already closed */
      }
    }
    sessions = sessions.filter((x) => x.id !== id)
    if (activeSessionId === id) activeSessionId = sessions[sessions.length - 1]?.id || ''
  }

  onMount(() => {
    const offStarted = EventsOn('terminal-session-started', (/** @type {any} */ payload) => {
      upsertSession({
        id: payload?.sessionId,
        cwd: payload?.cwd,
        kind: payload?.kind,
        command: payload?.command
      })
    })
    const offExit = EventsOn('terminal-exit', (/** @type {any} */ payload) => {
      if (!payload?.sessionId) return
      sessions = sessions.map((s) =>
        s.id === payload.sessionId ? { ...s, exitCode: Number(payload.exitCode ?? 0) } : s
      )
    })
    return () => {
      offStarted?.()
      offExit?.()
    }
  })
</script>

<section class="workspace-console" class:open aria-label={T('console.aria')} hidden={!open}>
  <div class="console-grip" aria-hidden="true"></div>
  <header class="console-tabs">
    <div class="console-title">
      <span class="console-mark" aria-hidden="true">⌁</span>
      <span>{T('console.title')}</span>
      <span class="console-chip">{T('console.local')}</span>
    </div>
    <div class="terminal-tabs" role="tablist" aria-label={T('console.sessions')}>
      {#each sessions as s (s.id)}
        <button
          type="button"
          class="terminal-tab"
          class:active={activeSessionId === s.id}
          class:done={s.exitCode != null}
          on:click={() => (activeSessionId = s.id)}
          title={s.command || s.cwd}
        >
          <span>{s.kind === 'command' ? '▶' : '$'}</span>
          {s.title}
        </button>
      {/each}
    </div>
    <div class="console-quick" role="toolbar" aria-label={T('console.quickAria')}>
      <button type="button" class="console-tab-btn primary" on:click={() => startSessionForCwd('')}>
        {T('console.newTerminal')}
      </button>
      {#each quick as q (q.key)}
        <button type="button" class="console-tab-btn" disabled={busy} on:click={() => run(q.command)}>
          {T(`console.quick.${q.key}`)}
        </button>
      {/each}
      <button type="button" class="console-tab-btn" on:click={openWave}>{T('console.quick.wave')}</button>
      <button type="button" class="console-close" aria-label={T('app.close')} title={T('app.close')} on:click={onClose}>×</button>
    </div>
  </header>
  <div class="console-body">
    <div class="terminal-blocks" aria-label={T('console.sessions')}>
      {#if sessions.length === 0}
        <div class="console-empty">
          <code>$ {T('console.noTerminal')}</code>
          <p>{T('console.guardrail')}</p>
        </div>
      {:else}
        {#each sessions as s (s.id)}
          <article class="terminal-block" class:active={activeSessionId === s.id}>
            <div class="terminal-head">
              <span title={s.cwd}>{shortPath(s.cwd)}</span>
              {#if s.command}
                <code title={s.command}>{shortCommand(s.command)}</code>
              {/if}
              {#if s.exitCode != null}
                <span class:bad={s.exitCode !== 0}>{s.exitCode === 0 ? 'OK' : `exit ${s.exitCode}`}</span>
              {/if}
              <button type="button" aria-label={T('console.closeSession')} title={T('console.closeSession')} on:click={() => closeSession(s.id)}>×</button>
            </div>
            <div class="terminal-host">
              <Terminal bind:this={terminalRefs[s.id]} session={s} active={activeSessionId === s.id} />
            </div>
          </article>
        {/each}
      {/if}
    </div>
    <div class="quick-run-panel">
      <div class="console-history" aria-live="polite">
        {#each history as item (item.id)}
          <article class="run-block" class:failed={item.exitCode !== 0 && !item.pending} class:pending={item.pending}>
            <div class="run-meta">
              <code>$ {item.command}</code>
              <span>{shortPath(item.cwd)}</span>
              <span>{item.pending ? T('console.running') : item.timedOut ? T('console.timeout') : item.exitCode === 0 ? 'OK' : `exit ${item.exitCode}`}</span>
              {#if !item.pending}
                <span>{item.durationMs}ms</span>
              {/if}
            </div>
            <pre>{item.pending ? T('console.runningOutput') : item.output || T('console.noOutput')}</pre>
          </article>
        {/each}
      </div>
      <form class="console-command" on:submit|preventDefault={() => run()}>
        <span class="prompt">{shortPath(notesRoot)}</span>
        <input
          bind:value={command}
          spellcheck="false"
          autocomplete="off"
          placeholder={T('console.placeholder')}
          disabled={busy}
        />
        <button type="submit" disabled={busy || !command.trim()}>{busy ? T('console.running') : T('console.run')}</button>
      </form>
    </div>
  </div>
</section>

<style>
  .workspace-console {
    flex: 0 0 300px;
    min-height: 190px;
    max-height: 46vh;
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--dv-border);
    background: var(--dv-tool-window);
    box-shadow: 0 -10px 22px rgba(0, 0, 0, 0.12);
  }
  .workspace-console[hidden] {
    display: none;
  }
  .console-grip {
    height: 5px;
    background: linear-gradient(180deg, color-mix(in srgb, var(--dv-fg) 8%, transparent), transparent);
  }
  .console-tabs {
    min-height: 36px;
    padding: 0 10px;
    display: grid;
    grid-template-columns: auto minmax(120px, 1fr) auto;
    align-items: center;
    gap: 10px;
    border-bottom: 1px solid var(--dv-border);
  }
  .console-title,
  .console-quick,
  .terminal-tabs {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: 6px;
  }
  .console-title {
    font-size: 0.78rem;
    font-weight: 650;
  }
  .console-mark {
    color: var(--dv-accent-2);
    font-family: var(--dv-font-mono, ui-monospace, monospace);
  }
  .console-chip {
    padding: 1px 6px;
    border-radius: 999px;
    color: var(--dv-muted);
    background: color-mix(in srgb, var(--dv-fg) 6%, transparent);
    font-size: 0.68rem;
    font-weight: 500;
  }
  .terminal-tabs {
    overflow-x: auto;
  }
  .terminal-tab,
  .console-tab-btn,
  .console-close,
  .console-command button {
    min-height: 26px;
    border-radius: 5px;
    border: 1px solid transparent;
    background: transparent;
    color: var(--dv-muted);
    font: inherit;
    font-size: 0.74rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .terminal-tab {
    max-width: 170px;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .terminal-tab.active,
  .console-tab-btn.primary {
    color: var(--dv-fg);
    border-color: color-mix(in srgb, var(--dv-accent) 28%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-accent) 12%, transparent);
  }
  .terminal-tab.done:not(.active) {
    opacity: 0.72;
  }
  .console-tab-btn {
    padding: 3px 8px;
  }
  .terminal-tab:hover,
  .console-tab-btn:hover,
  .console-close:hover {
    border-color: var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 7%, transparent);
    color: var(--dv-fg);
  }
  .console-close {
    width: 28px;
    padding: 0;
    font-size: 1rem;
  }
  .console-body {
    min-height: 0;
    flex: 1;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(260px, 34%);
  }
  .terminal-blocks {
    min-height: 0;
    border-right: 1px solid var(--dv-border);
    overflow: hidden;
  }
  .terminal-block {
    display: none;
    height: 100%;
    min-height: 0;
    grid-template-rows: 28px minmax(0, 1fr);
  }
  .terminal-block.active {
    display: grid;
  }
  .terminal-head {
    min-height: 28px;
    padding: 0 8px;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--dv-muted);
    border-bottom: 1px solid color-mix(in srgb, var(--dv-fg) 8%, transparent);
    font-size: 0.72rem;
  }
  .terminal-head code {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--dv-font-mono, ui-monospace, monospace);
  }
  .terminal-head .bad {
    color: var(--dv-danger);
  }
  .terminal-head button {
    margin-left: auto;
    border: 0;
    background: transparent;
    color: var(--dv-muted);
    cursor: pointer;
  }
  .terminal-host {
    min-height: 0;
  }
  .quick-run-panel {
    min-height: 0;
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto;
  }
  .console-history {
    min-height: 0;
    overflow: auto;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .console-empty {
    min-height: 100%;
    display: grid;
    align-content: center;
    justify-content: center;
    gap: 8px;
    padding: 20px;
    color: var(--dv-muted);
    text-align: center;
  }
  .console-empty p {
    max-width: 460px;
    margin: 0;
    font-size: 0.78rem;
    line-height: 1.45;
  }
  .console-empty code,
  .run-meta code,
  .run-block pre,
  .prompt {
    font-family: var(--dv-font-mono, 'JetBrains Mono', ui-monospace, monospace);
  }
  .run-block {
    border-radius: 6px;
    border: 1px solid color-mix(in srgb, var(--dv-fg) 9%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
    overflow: hidden;
  }
  .run-block.failed {
    border-color: color-mix(in srgb, var(--dv-danger) 36%, transparent);
  }
  .run-block.pending {
    border-color: color-mix(in srgb, var(--dv-accent) 34%, transparent);
  }
  .run-meta {
    min-height: 30px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 8px;
    color: var(--dv-muted);
    border-bottom: 1px solid color-mix(in srgb, var(--dv-fg) 7%, transparent);
    font-size: 0.72rem;
  }
  .run-meta code {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--dv-fg);
  }
  .run-block pre {
    margin: 0;
    padding: 8px;
    max-height: 120px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
    font-size: 0.72rem;
    line-height: 1.45;
    color: color-mix(in srgb, var(--dv-fg) 84%, transparent);
  }
  .console-command {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 8px;
    padding: 8px 10px;
    border-top: 1px solid var(--dv-border);
  }
  .prompt {
    align-self: center;
    color: var(--dv-muted);
    font-size: 0.72rem;
    max-width: 130px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .console-command input {
    min-width: 0;
    border: 1px solid var(--dv-border);
    border-radius: 6px;
    background: var(--dv-input);
    color: var(--dv-fg);
    padding: 6px 8px;
    font-family: var(--dv-font-mono, 'JetBrains Mono', ui-monospace, monospace);
    font-size: 0.78rem;
  }
  .console-command button {
    padding: 0 12px;
    border-color: color-mix(in srgb, var(--dv-accent) 30%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-accent) 12%, transparent);
    color: var(--dv-fg);
  }
  @media (max-width: 899px) {
    .workspace-console {
      max-height: 52vh;
    }
    .console-tabs {
      grid-template-columns: 1fr;
      align-items: stretch;
      padding: 6px 10px;
    }
    .console-body {
      grid-template-columns: 1fr;
    }
    .quick-run-panel {
      display: none;
    }
  }
</style>
