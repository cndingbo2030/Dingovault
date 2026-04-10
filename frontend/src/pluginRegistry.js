import { writable } from 'svelte/store'

/**
 * @typedef {{ id?: string, label: string, run?: () => void | Promise<void> }} ToolbarSpec
 * @typedef {{ id?: string, title: string, body: string }} SidebarSpec
 */

/** @type {import('svelte/store').Writable<ToolbarSpec[]>} */
export const toolbarEntries = writable([])

/** @type {import('svelte/store').Writable<SidebarSpec[]>} */
export const sidebarEntries = writable([])

/** Register a toolbar button (e.g. from a plugin script). `run` is called on tap. */
export function registerToolbarButton(spec) {
  const id = spec.id || `tb-${Date.now()}`
  toolbarEntries.update((a) => [...a, { ...spec, id }])
}

/** Register a sidebar section (plain-text body; avoid HTML from untrusted plugins). */
export function registerSidebarSection(spec) {
  const id = spec.id || `sb-${Date.now()}`
  sidebarEntries.update((a) => [...a, { ...spec, id }])
}

const PLACEHOLDER_IMG =
  'data:image/svg+xml,' +
  encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="72" height="54" viewBox="0 0 72 54">
  <rect fill="#1e1e24" width="72" height="54" rx="8" stroke="rgba(255,255,255,0.12)" stroke-width="1"/>
  <path d="M20 36 L28 28 L36 34 L52 18 L56 22 L36 40 L28 32 L20 36 Z" fill="rgba(120,140,200,0.35)"/>
  <text x="36" y="48" text-anchor="middle" fill="rgba(255,255,255,0.35)" font-size="8" font-family="system-ui,sans-serif">?</text>
</svg>`
  )

/** Delegated capture: broken images get a neutral placeholder (404, file missing, etc.). */
export function initImageFallback() {
  if (typeof document === 'undefined') return
  document.addEventListener(
    'error',
    (e) => {
      const t = /** @type {EventTarget | null} */ (e.target)
      if (!t || !(t instanceof HTMLImageElement)) return
      if (t.dataset.dvImgFallback === '1') return
      t.dataset.dvImgFallback = '1'
      t.src = PLACEHOLDER_IMG
      t.alt = t.alt || 'Image unavailable'
    },
    true
  )
}

/** Expose a stable global for third-party scripts loaded after the app shell. */
export function exposePluginAPI() {
  if (typeof window === 'undefined') return
  window.__DINGOVAULT__ = {
    version: '1.3.0',
    registerToolbarButton,
    registerSidebarSection,
  }
}
