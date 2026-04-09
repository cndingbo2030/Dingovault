<script>
  import { toasts, dismissToast } from './toastStore.js'

  /** @param {'error'|'info'|'success'|'warning'} kind */
  function iconPath(kind) {
    if (kind === 'success') return 'M10 18 L15 23 L26 12'
    if (kind === 'warning') return 'M18 9 L28 27 H8 Z'
    if (kind === 'info') return 'M18 11 A7 7 0 1 1 17.99 11 M18 16 V22 M18 13.2 L18 13.2'
    return 'M10 10 L26 26 M26 10 L10 26'
  }
</script>

<div class="toast-stack" aria-live="polite">
  {#each $toasts as t (t.id)}
    <div
      class="toast"
      class:error={t.kind === 'error'}
      class:info={t.kind === 'info'}
      class:success={t.kind === 'success'}
      class:warning={t.kind === 'warning'}
      role="status"
    >
      <span class="icon" aria-hidden="true">
        <svg viewBox="0 0 36 36" width="16" height="16">
          {#if t.kind === 'warning'}
            <path d={iconPath(t.kind)} fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"></path>
            <path d="M18 15 V20 M18 24.5 L18 24.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"></path>
          {:else}
            <path d={iconPath(t.kind)} fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"></path>
          {/if}
        </svg>
      </span>
      <span class="msg">{t.message}</span>
      <button type="button" class="dismiss" aria-label="Dismiss" on:click={() => dismissToast(t.id)}>×</button>
    </div>
  {/each}
</div>

<style>
  .toast-stack {
    position: fixed;
    bottom: 20px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 200;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    max-width: min(420px, calc(100vw - 24px));
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 12px;
    border-radius: 10px;
    border: 1px solid var(--dv-toast-border, rgba(255, 255, 255, 0.12));
    background: var(--dv-toast-bg, rgba(30, 30, 36, 0.96));
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
    font-size: 0.88rem;
    line-height: 1.35;
  }
  .toast.error {
    border-color: rgba(248, 113, 113, 0.35);
    color: #fecaca;
  }
  .toast.info {
    border-color: rgba(120, 160, 255, 0.35);
    color: #c7d2fe;
  }
  .toast.success {
    border-color: rgba(34, 197, 94, 0.35);
    color: #bbf7d0;
  }
  .toast.warning {
    border-color: rgba(245, 158, 11, 0.35);
    color: #fde68a;
  }
  .icon {
    margin-top: 1px;
    flex-shrink: 0;
    opacity: 0.95;
  }
  .msg {
    flex: 1;
    word-break: break-word;
  }
  .dismiss {
    flex-shrink: 0;
    border: none;
    background: transparent;
    color: inherit;
    opacity: 0.55;
    cursor: pointer;
    font-size: 1.2rem;
    line-height: 1;
    padding: 0 2px;
  }
  .dismiss:hover {
    opacity: 1;
  }
</style>
