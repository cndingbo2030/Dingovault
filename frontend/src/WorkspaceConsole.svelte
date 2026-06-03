<script>
  import { RunVaultCommand } from '../wailsjs/go/bridge/App.js'
  import { messages, tr } from './lib/i18n/index.js'
  import { pushToast } from './toastStore.js'

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

  const quick = [
    { key: 'status', command: 'git status --short' },
    { key: 'list', command: 'ls -la' },
    { key: 'test', command: 'go test ./...' },
    { key: 'wails', command: 'wails build -clean' },
    { key: 'wave', command: 'open -a Wave .' }
  ]

  /** @param {string} p */
  function shortPath(p) {
    if (!p) return '~'
    const home = typeof window !== 'undefined' ? '' : ''
    const parts = p.replace(home, '~').split(/[/\\]/).filter(Boolean)
    if (parts.length <= 2) return p
    return `…/${parts.slice(-2).join('/')}`
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
    history = [pending, ...history].slice(0, 24)
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
</script>

<section class="workspace-console" class:open aria-label={T('console.aria')} hidden={!open}>
  <div class="console-grip" aria-hidden="true"></div>
  <header class="console-tabs">
    <div class="console-title">
      <span class="console-mark" aria-hidden="true">⌁</span>
      <span>{T('console.title')}</span>
      <span class="console-chip">{T('console.local')}</span>
    </div>
    <div class="console-quick" role="toolbar" aria-label={T('console.quickAria')}>
      {#each quick as q (q.key)}
        <button type="button" class="console-tab-btn" disabled={busy} on:click={() => run(q.command)}>
          {T(`console.quick.${q.key}`)}
        </button>
      {/each}
      <button type="button" class="console-close" aria-label={T('app.close')} title={T('app.close')} on:click={onClose}>×</button>
    </div>
  </header>
  <div class="console-body">
    <div class="console-history" aria-live="polite">
      {#if history.length === 0}
        <div class="console-empty">
          <code>$ git status --short</code>
          <code>$ open -a Wave .</code>
        </div>
      {:else}
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
      {/if}
    </div>
    <form
      class="console-command"
      on:submit|preventDefault={() => run()}
    >
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
</section>

<style>
  .workspace-console {
    flex: 0 0 220px;
    min-height: 148px;
    max-height: 34vh;
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
    min-height: 34px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border-bottom: 1px solid var(--dv-border);
  }
  .console-title,
  .console-quick {
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
  }
  .console-tab-btn {
    padding: 3px 8px;
  }
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
    color: var(--dv-muted);
    opacity: 0.65;
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
    color: var(--dv-fg);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }
  .run-block pre {
    margin: 0;
    padding: 8px;
    max-height: 180px;
    overflow: auto;
    color: color-mix(in srgb, var(--dv-fg) 86%, var(--dv-muted));
    font-size: 0.76rem;
    line-height: 1.45;
    white-space: pre-wrap;
  }
  .console-command {
    display: grid;
    grid-template-columns: minmax(90px, 160px) minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
    padding: 8px 10px 10px;
    border-top: 1px solid var(--dv-border);
  }
  .prompt {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--dv-muted);
    font-size: 0.72rem;
  }
  .console-command input {
    min-width: 0;
    min-height: 32px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: var(--dv-fg);
    font: inherit;
    font-family: var(--dv-font-mono, 'JetBrains Mono', ui-monospace, monospace);
    font-size: 0.78rem;
    padding: 5px 8px;
  }
  .console-command input:focus {
    outline: none;
    border-color: color-mix(in srgb, var(--dv-accent) 46%, var(--dv-border));
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--dv-accent) 14%, transparent);
  }
  .console-command button {
    padding: 4px 10px;
    border-color: color-mix(in srgb, var(--dv-accent) 36%, transparent);
    color: color-mix(in srgb, var(--dv-accent) 88%, var(--dv-fg));
    background: color-mix(in srgb, var(--dv-accent) 10%, transparent);
  }
  button:disabled,
  input:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
  @media (max-width: 899px) {
    .workspace-console {
      display: none;
    }
  }
</style>
