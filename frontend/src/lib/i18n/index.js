import { writable, derived } from 'svelte/store'
import en from './en.json'
import zhCN from './zh-CN.json'

const catalogs = { en, 'zh-CN': zhCN }

/** @type {import('svelte/store').Writable<'en' | 'zh-CN'>} */
export const locale = writable('en')

export const messages = derived(locale, ($l) => catalogs[$l] || catalogs.en)

/** @param {string} tag */
export function normalizeLocaleTag(tag) {
  const t = String(tag || '')
    .trim()
    .toLowerCase()
    .replace(/_/g, '-')
  if (!t) return 'en'
  if (t.startsWith('zh')) return 'zh-CN'
  return 'en'
}

export function detectBrowserLocale() {
  if (typeof navigator === 'undefined') return 'en'
  const list = navigator.languages?.length ? navigator.languages : [navigator.language]
  for (const raw of list) {
    const n = normalizeLocaleTag(raw)
    if (n === 'zh-CN') return 'zh-CN'
  }
  return 'en'
}

/** @param {unknown} obj @param {string} path */
export function pick(obj, path) {
  /** @type {unknown} */
  let o = obj
  for (const p of path.split('.')) {
    if (o == null || typeof o !== 'object') return undefined
    o = /** @type {Record<string, unknown>} */ (o)[p]
  }
  return o
}

/** @param {string} template @param {Record<string, string | number> | undefined} vars */
export function format(template, vars) {
  if (!template || typeof template !== 'string') return ''
  if (!vars) return template
  return template.replace(/\{(\w+)\}/g, (_, k) =>
    vars[k] !== undefined && vars[k] !== null ? String(vars[k]) : ''
  )
}

/**
 * @param {unknown} dict from $messages
 * @param {string} path
 * @param {Record<string, string | number> | undefined} [vars]
 */
export function tr(dict, path, vars) {
  const raw = pick(dict, path)
  return typeof raw === 'string' ? format(raw, vars) : path
}
