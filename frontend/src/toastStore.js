import { writable } from 'svelte/store'

/** @type {import('svelte/store').Writable<{ id: number, message: string, kind: 'error'|'info'|'success'|'warning' }[]>} */
export const toasts = writable([])

let nextId = 0
const defaultMs = 5200

/**
 * @param {string} message
 * @param {'error' | 'info' | 'success' | 'warning'} [kind]
 * @param {number} [ms]
 */
export function pushToast(message, kind = 'error', ms = defaultMs) {
  const id = ++nextId
  toasts.update((a) => [...a, { id, message: String(message), kind }])
  window.setTimeout(() => dismissToast(id), ms)
}

/** @param {number} id */
export function dismissToast(id) {
  toasts.update((a) => a.filter((t) => t.id !== id))
}
