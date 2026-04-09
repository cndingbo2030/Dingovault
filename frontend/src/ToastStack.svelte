<script>
  import { toasts, dismissToast } from './toastStore.js'
</script>

<div class="toast-stack" aria-live="polite">
  {#each $toasts as t (t.id)}
    <div class="toast" class:error={t.kind === 'error'} class:info={t.kind === 'info'} role="status">
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
  }
  .toast.info {
    border-color: rgba(120, 160, 255, 0.35);
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
