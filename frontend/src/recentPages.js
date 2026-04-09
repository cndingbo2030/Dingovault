const KEY = 'dingovault-recent-pages'
const MAX = 20

/** @returns {string[]} */
export function readRecentPages() {
  try {
    const raw = localStorage.getItem(KEY)
    const a = raw ? JSON.parse(raw) : []
    return Array.isArray(a) ? a.filter((x) => typeof x === 'string') : []
  } catch {
    return []
  }
}

/** @param {string} relPath */
export function touchRecentPage(relPath) {
  const p = (relPath || '').replace(/\\/g, '/').replace(/^\/+/, '')
  if (!p) return
  let list = readRecentPages().filter((x) => x !== p)
  list.unshift(p)
  if (list.length > MAX) list = list.slice(0, MAX)
  localStorage.setItem(KEY, JSON.stringify(list))
}
