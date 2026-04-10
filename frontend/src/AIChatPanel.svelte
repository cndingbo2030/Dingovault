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
  <h2 class="title">{T('aiChat.title')}</h2>
  <p class="ctx">{T('aiChat.ragHint', { path: pagePath || '—' })}</p>

  <div class="thread" role="log" aria-live="polite">
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

  <div class="composer">
    <textarea
      rows="3"
      bind:value={input}
      placeholder={T('aiChat.placeholder')}
      disabled={busy}
      on:keydown={onKeyDown}
    />
    <button type="button" class="send" disabled={busy || !input.trim()} on:click={() => send()}>
      {T('aiChat.send')}
    </button>
  </div>

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
    margin-top: 0;
  }
  .title {
    margin: 0 0 6px;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
  }
  .ctx {
    margin: 0 0 12px;
    font-size: 0.78rem;
    opacity: 0.5;
    word-break: break-all;
  }
  .thread {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: min(42vh, 420px);
    overflow-y: auto;
    padding: 4px 0 12px;
    margin-bottom: 8px;
    border-bottom: 1px solid var(--dv-border);
  }
  .msg {
    border-radius: 10px;
    padding: 8px 10px;
    font-size: 0.88rem;
    line-height: 1.45;
    border: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .msg.user {
    border-color: rgba(120, 160, 255, 0.28);
    background: rgba(80, 120, 255, 0.08);
  }
  .role {
    display: block;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    opacity: 0.45;
    margin-bottom: 4px;
  }
  .body {
    white-space: pre-wrap;
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
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 12px;
  }
  .composer textarea {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    min-height: 72px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font-family: inherit;
    font-size: 0.9rem;
  }
  .send {
    align-self: flex-end;
    padding: 8px 16px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: rgba(80, 120, 255, 0.28);
    color: var(--dv-fg);
    cursor: pointer;
  }
  .send:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .ai-settings {
    margin-top: 4px;
    font-size: 0.82rem;
  }
  .ai-settings summary {
    cursor: pointer;
    opacity: 0.7;
    margin-bottom: 8px;
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
    min-height: 88px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--dv-border);
    background: var(--dv-input);
    color: inherit;
    font-family: inherit;
    font-size: 0.82rem;
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
    border-radius: 8px;
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
