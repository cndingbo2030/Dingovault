/**
 * Subsequence fuzzy match with scoring; prefers path segments and consecutive hits.
 * @param {string} query
 * @param {string} relPath vault-relative path with forward slashes
 */
export function fuzzyPageScore(query, relPath) {
  const q = query.trim().toLowerCase()
  const t = relPath.toLowerCase().replace(/\\/g, '/')
  if (!q) return { ok: true, score: 0 }
  const file = t.split('/').pop() || t
  const stem = file.replace(/\.md$/i, '')
  if (stem.startsWith(q)) {
    return { ok: true, score: 2000 - stem.length }
  }
  let qi = 0
  let score = 0
  let last = -2
  for (let i = 0; i < t.length && qi < q.length; i++) {
    if (t[i] === q[qi]) {
      const consecutive = last === i - 1 ? 8 : 0
      const edge = i === 0 || t[i - 1] === '/' ? 10 : 0
      score += 12 + consecutive + edge
      last = i
      qi++
    }
  }
  if (qi < q.length) return { ok: false, score: 0 }
  if (stem.includes(q)) score += 80
  return { ok: true, score }
}

/**
 * @param {string} query
 * @param {string[]} paths
 * @param {number} limit
 */
export function rankPagePaths(query, paths, limit) {
  const q = query.trim()
  if (!q) return []
  const scored = []
  for (const p of paths) {
    const { ok, score } = fuzzyPageScore(q, p)
    if (ok) scored.push({ path: p, score })
  }
  scored.sort((a, b) => b.score - a.score)
  if (scored.length > limit) return scored.slice(0, limit)
  return scored
}
