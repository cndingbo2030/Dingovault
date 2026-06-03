<script>
  import { onDestroy } from 'svelte'
  import { GetAISettings, SetAISettings, AIChat } from '../wailsjs/go/bridge/App.js'
  import { messages, tr } from './lib/i18n/index.js'
  import { pushToast } from './toastStore.js'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  /** @type {string} */
  export let pagePath = ''

  /** @type {{ role: 'user' | 'assistant', text: string }[]} */
  let thread = []
  let input = ''
  let busy = false
  /** Visible assistant text (typewriter) */
  let displayAssistant = ''
  let typing = false
  /** @type {ReturnType<typeof setInterval> | undefined} */
  let typeTimer

  let provider = 'ollama'
  let model = ''
  let endpoint = ''
  let apiKey = ''
  let temperature = 0.7
  let embeddingsModel = ''
  let disableEmbeddings = false
  /** @type {string} */
  let systemPrompt = ''
  let semanticTopK = 8
  let settingsLoaded = false

  async function loadSettings() {
    try {
      const s = await GetAISettings()
      provider = (s.provider || 'ollama').toLowerCase()
      model = s.model || ''
      endpoint = s.endpoint || ''
      apiKey = s.apiKey || ''
      temperature = typeof s.temperature === 'number' ? s.temperature : 0.7
      embeddingsModel = s.embeddingsModel || ''
      disableEmbeddings = !!s.disableEmbeddings
      systemPrompt = s.systemPrompt || ''
      semanticTopK = typeof s.semanticTopK === 'number' && s.semanticTopK > 0 ? s.semanticTopK : 8
      settingsLoaded = true
    } catch (e) {
      pushToast(String(e), 'error')
    }
  }

  async function saveSettings() {
    try {
      await SetAISettings({
        provider,
        model,
        endpoint,
        apiKey,
        temperature,
        embeddingsModel,
        disableEmbeddings,
        systemPrompt,
        semanticTopK
      })
      pushToast(T('aiChat.settingsSaved'), 'info')
    } catch (e) {
      pushToast(String(e), 'error')
    }
  }

  $: if (pagePath) {
    /* keep panel tied to page context */
  }

  onDestroy(() => {
    if (typeTimer) clearInterval(typeTimer)
  })

  /** @param {string} full */
  function startTypewriter(full) {
    if (typeTimer) clearInterval(typeTimer)
    typing = true
    displayAssistant = ''
    let i = 0
    const chunk = 2
    const fullStr = full || ''
    typeTimer = setInterval(() => {
      if (i >= fullStr.length) {
        if (typeTimer) clearInterval(typeTimer)
        typeTimer = undefined
        typing = false
        displayAssistant = fullStr
        return
      }
      i = Math.min(fullStr.length, i + chunk)
      displayAssistant = fullStr.slice(0, i)
    }, 16)
  }

  async function send() {
    const q = input.trim()
    if (!q || busy) return
    thread = [...thread, { role: 'user', text: q }]
    input = ''
    busy = true
    displayAssistant = ''
    try {
      const reply = await AIChat(pagePath, q)
      thread = [...thread, { role: 'assistant', text: reply }]
      startTypewriter(reply)
    } catch (e) {
      pushToast(String(e), 'error')
      thread = [...thread, { role: 'assistant', text: T('aiChat.errorPrefix') + String(e) }]
      displayAssistant = ''
    } finally {
      busy = false
    }
  }

  /** @param {KeyboardEvent} e */
  function onKeyDown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void send()
    }
  }

  void loadSettings()
</script>

<section class="ai-chat" aria-label={T('aiChat.aria')}>
  <div class="ai-head">
    <div class="ai-head-copy">
      <h2 class="title">{T('aiChat.title')}</h2>
      <p class="ctx">{T('aiChat.ragHint', { path: pagePath || '—' })}</p>
    </div>
    <span class="ai-mode">{provider || 'local'}</span>
  </div>

  <div class="thread" role="log" aria-live="polite">
    {#if !thread.length && !busy}
      <div class="ai-empty">
        <p class="empty-title">{T('aiChat.emptyTitle')}</p>
        <p>{T('aiChat.emptyHint')}</p>
      </div>
    {/if}
    {#each thread as m, i (i)}
      <div class="msg" class:user={m.role === 'user'} class:assistant={m.role === 'assistant'}>
        <span class="role">{m.role === 'user' ? T('aiChat.you') : T('aiChat.assistant')}</span>
        <div class="body">
          {#if m.role === 'assistant' && i === thread.length - 1 && typing}
            {displayAssistant}
          {:else}
            {m.text}
          {/if}
        </div>
      </div>
    {/each}
    {#if busy && thread.length && thread[thread.length - 1].role === 'user'}
      <div class="msg assistant">
        <span class="role">{T('aiChat.assistant')}</span>
        <div class="body"><span class="typing">{T('aiChat.thinking')}</span></div>
      </div>
    {/if}
  </div>

  <form class="composer" on:submit|preventDefault={() => send()}>
    <textarea
      rows="2"
      bind:value={input}
      placeholder={T('aiChat.placeholder')}
      disabled={busy}
      on:keydown={onKeyDown}
    />
    <button type="submit" class="send" disabled={busy || !input.trim()}>
      {T('aiChat.send')}
    </button>
  </form>

  <details class="ai-settings">
    <summary>{T('aiChat.settings')}</summary>
    {#if settingsLoaded}
      <label>
        <span>{T('aiChat.provider')}</span>
        <select bind:value={provider}>
          <option value="ollama">Ollama</option>
          <option value="openai">OpenAI</option>
        </select>
      </label>
      <label>
        <span>{T('aiChat.model')}</span>
        <input type="text" bind:value={model} placeholder="llama3.2 / gpt-4o-mini" />
      </label>
      <label>
        <span>{T('aiChat.endpoint')}</span>
        <input type="text" bind:value={endpoint} placeholder="http://127.0.0.1:11434" />
      </label>
      {#if provider === 'openai'}
        <label>
          <span>{T('aiChat.apiKey')}</span>
          <input type="password" bind:value={apiKey} autocomplete="off" />
        </label>
      {/if}
      <label>
        <span>{T('aiChat.temperature')}</span>
        <input type="number" step="0.1" min="0" max="2" bind:value={temperature} />
      </label>
      <label>
        <span>{T('aiChat.embeddingsModel')}</span>
        <input type="text" bind:value={embeddingsModel} placeholder="nomic-embed-text" />
      </label>
      <label class="check">
        <input type="checkbox" bind:checked={disableEmbeddings} />
        <span>{T('aiChat.disableEmbeddings')}</span>
      </label>
      <label>
        <span>{T('aiChat.semanticTopK')}</span>
        <input type="number" min="1" max="24" bind:value={semanticTopK} />
      </label>
      <label class="block">
        <span>{T('aiChat.systemPrompt')}</span>
        <textarea
          class="sys-prompt"
          rows="4"
          bind:value={systemPrompt}
          placeholder={T('aiChat.systemPromptPlaceholder')}
        />
      </label>
      <button type="button" class="save-cfg" on:click={() => saveSettings()}>{T('aiChat.saveSettings')}</button>
    {:else}
      <p class="muted">{T('aiChat.loadingSettings')}</p>
    {/if}
  </details>
</section>

<style>
  .ai-chat {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 0;
    overflow: hidden;
  }
  .ai-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
    padding: 1px 0 9px;
    border-bottom: 1px solid var(--dv-border);
    flex-shrink: 0;
  }
  .ai-head-copy {
    min-width: 0;
    flex: 1;
  }
  .title {
    margin: 0;
    font-size: 0.82rem;
    font-weight: 650;
    letter-spacing: 0;
    text-transform: none;
    opacity: 0.82;
  }
  .ctx {
    margin: 3px 0 0;
    color: var(--dv-muted);
    font-size: 0.72rem;
    line-height: 1.35;
    opacity: 0.88;
    overflow: hidden;
    display: -webkit-box;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    word-break: normal;
  }
  .ai-mode {
    flex: 0 0 auto;
    max-width: 76px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 2px 6px;
    border-radius: 5px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
    color: var(--dv-muted);
    font-size: 0.68rem;
    line-height: 1.25;
  }
  .thread {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
    padding: 0 1px 2px 0;
  }
  .ai-empty {
    min-height: 132px;
    display: grid;
    align-content: center;
    gap: 4px;
    padding: 14px 12px;
    border-radius: 6px;
    border: 1px solid color-mix(in srgb, var(--dv-border) 75%, transparent);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
    color: var(--dv-muted);
    font-size: 0.78rem;
    line-height: 1.42;
  }
  .ai-empty p {
    margin: 0;
  }
  .empty-title {
    color: color-mix(in srgb, var(--dv-fg) 82%, var(--dv-muted));
    font-size: 0.82rem;
    font-weight: 600;
  }
  .msg {
    border-radius: 6px;
    padding: 7px 9px;
    font-size: 0.8rem;
    line-height: 1.45;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
  }
  .msg.user {
    border-color: color-mix(in srgb, var(--dv-accent) 22%, var(--dv-border));
    background: color-mix(in srgb, var(--dv-accent) 8%, transparent);
  }
  .role {
    display: block;
    font-size: 0.68rem;
    color: var(--dv-muted);
    letter-spacing: 0;
    opacity: 0.9;
    margin-bottom: 4px;
  }
  .body {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .typing {
    opacity: 0.55;
    animation: blink 1s ease-in-out infinite;
  }
  @keyframes blink {
    50% {
      opacity: 0.2;
    }
  }
  .composer {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px;
    padding-top: 8px;
    border-top: 1px solid var(--dv-border);
    flex-shrink: 0;
  }
  .composer textarea {
    width: 100%;
    box-sizing: border-box;
    resize: none;
    min-height: 62px;
    max-height: 120px;
    padding: 8px 10px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font-family: inherit;
    font-size: 0.8rem;
    line-height: 1.4;
  }
  .send {
    align-self: end;
    min-width: 54px;
    min-height: 32px;
    padding: 0 12px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-accent) 22%, transparent);
    color: var(--dv-fg);
    cursor: pointer;
    font-size: 0.78rem;
  }
  .send:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .ai-settings {
    margin: 0;
    padding-top: 8px;
    border-top: 1px solid var(--dv-border);
    font-size: 0.78rem;
    flex-shrink: 0;
    max-height: 42%;
    overflow-y: auto;
  }
  .ai-settings summary {
    cursor: pointer;
    color: var(--dv-muted);
    margin-bottom: 8px;
    list-style: none;
    user-select: none;
  }
  .ai-settings summary::-webkit-details-marker {
    display: none;
  }
  .ai-settings label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 8px;
  }
  .ai-settings label.check {
    flex-direction: row;
    align-items: center;
    gap: 8px;
  }
  .ai-settings label.block {
    flex-direction: column;
  }
  .sys-prompt {
    width: 100%;
    box-sizing: border-box;
    min-height: 78px;
    padding: 8px 10px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font-family: inherit;
    font-size: 0.78rem;
    line-height: 1.45;
    resize: vertical;
  }
  .ai-settings input,
  .ai-settings select {
    padding: 6px 8px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font-size: 0.85rem;
  }
  .save-cfg {
    margin-top: 6px;
    padding: 6px 12px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 8%, transparent);
    color: inherit;
    cursor: pointer;
  }
  .muted {
    opacity: 0.55;
    margin: 0;
  }
</style>
