<script>
  import { fly, fade } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { messages, tr } from './lib/i18n/index.js'

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  export let open = false
  export let appVersion = ''
  export let theme = 'light'
  export let localeCode = 'en'
  export let aiReachable = false
  export let notesRoot = ''
  export let healthBusy = false
  /** @type {() => void} */
  export let onClose = () => {}
  /** @type {(code: string) => void | Promise<void>} */
  export let onSetLanguage = (_code) => {}
  /** @type {() => void | Promise<void>} */
  export let onToggleTheme = () => {}
  /** @type {() => void | Promise<void>} */
  export let onHealthReset = () => {}

  let section = 'general'

  const coreSections = [
    ['general', 'settings.general'],
    ['editor', 'settings.editor'],
    ['files', 'settings.files'],
    ['appearance', 'settings.appearance'],
    ['hotkeys', 'settings.hotkeys']
  ]
  const productSections = [
    ['ai', 'settings.ai'],
    ['sync', 'settings.sync'],
    ['commercial', 'settings.commercial'],
    ['advanced', 'settings.advanced']
  ]

  /** @type {Record<string, string>} */
  const iconPaths = {
    general: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-5 1.4 2.3 2.7.6.3 2.8 1.8 2.1-1.8 2.1-.3 2.8-2.7.6L12 21l-1.4-2.3-2.7-.6-.3-2.8-1.8-2.1 1.8-2.1.3-2.8 2.7-.6Z',
    editor: 'M5 19h4l10-10-4-4L5 15v4Zm10.5-13.5 2 2',
    files: 'M4 6h6l2 2h8v12H4z',
    appearance: 'M12 4a8 8 0 0 0 0 16c1.7 0 2.3-.9 1.4-1.9-.5-.6-.2-1.6.8-1.6H16a6 6 0 0 0 0-12zM7.8 11.4h.1M10 8.3h.1M14 8.3h.1M16.2 11.4h.1',
    hotkeys: 'M5 7h14v10H5zM8 10h.1M11 10h.1M14 10h.1M17 10h.1M8 14h8',
    ai: 'M12 3l2.2 5 5.3.6-4 3.6 1.1 5.2L12 14.8 7.4 17.4l1.1-5.2-4-3.6 5.3-.6z',
    sync: 'M7 7h9l-2-2m2 2-2 2M17 17H8l2 2m-2-2 2-2',
    commercial: 'M5 5h14v14H5zM8 9h8M8 13h5',
    advanced: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-5v3M12 18v3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M3 12h3M18 12h3M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1'
  }
</script>

{#if open}
  <div class="settings-backdrop" role="presentation" transition:fade={{ duration: 120 }} on:click|self={onClose}>
    <section
      class="settings-window"
      role="dialog"
      aria-modal="true"
      aria-labelledby="settings-title"
      transition:fly={{ y: 10, duration: 180, easing: cubicOut }}
    >
      <aside class="settings-nav" aria-label={T('settings.options')}>
        <h2 id="settings-title">{T('settings.options')}</h2>
        <nav>
          {#each coreSections as item (item[0])}
            <button type="button" class:active={section === item[0]} on:click={() => (section = item[0])}>
              <span class="nav-ico" aria-hidden="true"><svg viewBox="0 0 24 24"><path d={iconPaths[item[0]]} /></svg></span>
              <span>{T(item[1])}</span>
            </button>
          {/each}
        </nav>
        <h3>{T('settings.product')}</h3>
        <nav>
          {#each productSections as item (item[0])}
            <button type="button" class:active={section === item[0]} on:click={() => (section = item[0])}>
              <span class="nav-ico" aria-hidden="true"><svg viewBox="0 0 24 24"><path d={iconPaths[item[0]]} /></svg></span>
              <span>{T(item[1])}</span>
            </button>
          {/each}
        </nav>
      </aside>

      <main class="settings-content">
        <button type="button" class="settings-close" aria-label={T('app.close')} title={T('app.close')} on:click={onClose}>×</button>

        {#if section === 'general'}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('settings.version')}</h3>
                <p>{appVersion || 'v1.6.0'}</p>
              </div>
              <button type="button" class="setting-action">{T('settings.checkUpdates')}</button>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.language')}</h3>
                <p>{T('settings.languageHint')}</p>
              </div>
              <select value={localeCode} on:change={(e) => onSetLanguage(/** @type {HTMLSelectElement} */ (e.target).value)}>
                <option value="en">English</option>
                <option value="zh-CN">中文</option>
              </select>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.vaultRoot')}</h3>
                <p>{notesRoot || '...'}</p>
              </div>
            </div>
          </section>

          <h2 class="settings-section-title">{T('settings.account')}</h2>
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('settings.localAccount')}</h3>
                <p>{T('settings.localAccountHint')}</p>
              </div>
              <button type="button" class="setting-action muted">{T('settings.manage')}</button>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.commercialLicense')}</h3>
                <p>{T('settings.commercialHint')}</p>
              </div>
              <button type="button" class="setting-action primary">{T('settings.activate')}</button>
            </div>
          </section>
        {:else if section === 'appearance'}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('settings.theme')}</h3>
                <p>{theme === 'dark' ? T('app.themeModeDark') : T('app.themeModeLight')}</p>
              </div>
              <button type="button" class="setting-action" on:click={onToggleTheme}>
                {theme === 'dark' ? T('app.themeModeLight') : T('app.themeModeDark')}
              </button>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.fontDensity')}</h3>
                <p>{T('settings.fontDensityHint')}</p>
              </div>
              <span class="setting-pill">13 px</span>
            </div>
          </section>
        {:else if section === 'ai'}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('settings.aiStatus')}</h3>
                <p>{aiReachable ? T('settings.aiReachable') : T('settings.aiOffline')}</p>
              </div>
              <span class:ok={aiReachable} class="status-dot"></span>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('aiChat.settings')}</h3>
                <p>{T('settings.aiPanelHint')}</p>
              </div>
              <button type="button" class="setting-action" on:click={() => (section = 'advanced')}>{T('settings.open')}</button>
            </div>
          </section>
        {:else if section === 'commercial'}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('settings.openSource')}</h3>
                <p>{T('settings.openSourceHint')}</p>
              </div>
              <span class="setting-pill">AGPL</span>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.catalyst')}</h3>
                <p>{T('settings.catalystHint')}</p>
              </div>
              <button type="button" class="setting-action primary">{T('settings.purchase')}</button>
            </div>
            <div class="setting-row">
              <div>
                <h3>{T('settings.teamPlan')}</h3>
                <p>{T('settings.teamPlanHint')}</p>
              </div>
              <button type="button" class="setting-action">{T('settings.contact')}</button>
            </div>
          </section>
        {:else if section === 'advanced'}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T('app.healthReset')}</h3>
                <p>{T('settings.healthHint')}</p>
              </div>
              <button type="button" class="setting-action danger" disabled={healthBusy} on:click={onHealthReset}>
                {T('settings.reset')}
              </button>
            </div>
          </section>
        {:else}
          <section class="settings-card">
            <div class="setting-row">
              <div>
                <h3>{T(`settings.${section}`)}</h3>
                <p>{T('settings.comingSoon')}</p>
              </div>
            </div>
          </section>
        {/if}
      </main>
    </section>
  </div>
{/if}

<style>
  .settings-backdrop {
    position: fixed;
    inset: 0;
    z-index: 90;
    display: grid;
    place-items: center;
    padding: 18px;
    background: rgba(0, 0, 0, 0.18);
  }
  .settings-window {
    width: min(1088px, calc(100vw - 36px));
    height: min(760px, calc(100vh - 36px));
    display: grid;
    grid-template-columns: 250px minmax(0, 1fr);
    overflow: hidden;
    border: 1px solid var(--dv-border);
    border-radius: 8px;
    background: var(--dv-panel);
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.16);
  }
  .settings-nav {
    padding: 28px 10px 16px 16px;
    border-right: 1px solid var(--dv-border);
    background: color-mix(in srgb, var(--dv-panel) 94%, var(--dv-app-bg));
  }
  .settings-nav h2,
  .settings-nav h3 {
    margin: 0 0 8px 8px;
    color: var(--dv-muted);
    font-size: 0.74rem;
    font-weight: 700;
  }
  .settings-nav h3 {
    margin-top: 28px;
  }
  .settings-nav nav {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .settings-nav button {
    min-height: 28px;
    display: flex;
    align-items: center;
    gap: 9px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.88rem;
    text-align: left;
    cursor: pointer;
    padding: 4px 8px;
  }
  .settings-nav button.active {
    background: color-mix(in srgb, var(--dv-fg) 9%, transparent);
  }
  .nav-ico {
    width: 16px;
    height: 16px;
    flex: 0 0 13px;
    color: color-mix(in srgb, var(--dv-fg) 76%, transparent);
  }
  .nav-ico svg {
    width: 16px;
    height: 16px;
    display: block;
  }
  .nav-ico path {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .settings-content {
    position: relative;
    overflow: auto;
    padding: 34px 82px 44px 68px;
    background: var(--dv-panel);
  }
  .settings-close {
    position: absolute;
    top: 10px;
    right: 12px;
    width: 28px;
    height: 28px;
    border: 0;
    background: transparent;
    color: var(--dv-muted);
    font-size: 1.35rem;
    line-height: 1;
    cursor: pointer;
  }
  .settings-close:hover {
    color: var(--dv-fg);
  }
  .settings-card {
    border-radius: 10px;
    background: color-mix(in srgb, var(--dv-fg) 3.5%, transparent);
    overflow: hidden;
  }
  .setting-row {
    min-height: 72px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 24px;
    padding: 16px 20px;
    border-bottom: 1px solid color-mix(in srgb, var(--dv-fg) 10%, transparent);
  }
  .setting-row:last-child {
    border-bottom: 0;
  }
  .setting-row h3 {
    margin: 0 0 4px;
    font-size: 0.96rem;
    font-weight: 520;
  }
  .setting-row p {
    margin: 0;
    max-width: 74ch;
    color: var(--dv-muted);
    font-size: 0.78rem;
    line-height: 1.35;
  }
  .settings-section-title {
    margin: 28px 0 14px;
    font-size: 1rem;
    font-weight: 700;
  }
  .setting-action,
  .setting-row select,
  .setting-pill {
    min-height: 30px;
    border-radius: 6px;
    border: 1px solid var(--dv-border);
    background: var(--dv-panel);
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.82rem;
    padding: 5px 12px;
  }
  .setting-action {
    cursor: pointer;
  }
  .setting-action.primary {
    border-color: color-mix(in srgb, var(--dv-accent) 52%, transparent);
    background: var(--dv-accent);
    color: #fff;
  }
  .setting-action.danger {
    color: var(--dv-danger);
  }
  .setting-action.muted {
    color: var(--dv-muted);
  }
  .setting-pill {
    display: inline-flex;
    align-items: center;
    color: var(--dv-muted);
  }
  .status-dot {
    width: 38px;
    height: 22px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--dv-danger) 62%, #aaa);
    position: relative;
  }
  .status-dot::after {
    content: '';
    position: absolute;
    top: 3px;
    left: 3px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #fff;
  }
  .status-dot.ok {
    background: var(--dv-accent);
  }
  .status-dot.ok::after {
    left: 19px;
  }
  button:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
  @media (max-width: 760px) {
    .settings-window {
      grid-template-columns: 1fr;
    }
    .settings-nav {
      display: none;
    }
    .settings-content {
      padding: 42px 16px 22px;
    }
  }
</style>
