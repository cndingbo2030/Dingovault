# Mind Map Performance Fixture

This note documents the large-page measurement used for the current `MindMap.svelte` density rules.

## Fixture

Generated outline tree:

- 8 root branches.
- 7 second-level children per root.
- 5 third-level children per second-level child.
- 2 fourth-level children per third-level child.

That produces 904 blocks.

```bash
node - <<'NODE'
const roots = Array.from({ length: 8 }, (_, i) => ({ id: `r${i}`, children: [] }))
let total = roots.length
for (const root of roots) {
  for (let a = 0; a < 7; a++) {
    const child = { id: `${root.id}-a${a}`, children: [] }
    root.children.push(child); total++
    for (let b = 0; b < 5; b++) {
      const grand = { id: `${child.id}-b${b}`, children: [] }
      child.children.push(grand); total++
      for (let c = 0; c < 2; c++) {
        grand.children.push({ id: `${grand.id}-c${c}`, children: [] }); total++
      }
    }
  }
}
function walk(nodes, depth = 1, out = []) {
  for (const n of nodes) { out.push({ ...n, depth }); walk(n.children, depth + 1, out) }
  return out
}
const nodes = walk(roots)
const collapseDepth = total > 520 ? 3 : 4
const collapsed = new Set(nodes.filter((n) => n.depth >= collapseDepth && n.children.length > 0).map((n) => n.id).slice(0, 240))
function visibleCount(nodes) {
  let n = 0
  for (const node of nodes) { n++; if (!collapsed.has(node.id)) n += visibleCount(node.children) }
  return n
}
console.log({ total, collapseDepth, collapsedByLimit: collapsed.size, initialVisibleNodes: visibleCount(roots) })
NODE
```

Observed output:

```text
{ total: 904, collapseDepth: 3, collapsedByLimit: 240, initialVisibleNodes: 424 }
```

## Current Density Rules

- Large-page threshold: 300 blocks.
- Very large page collapse depth: 3 for more than 520 blocks, otherwise 4.
- Auto-collapse cap: 240 branch nodes on first open for a page.
- Labels stay visible for root, major nodes, collapsed nodes, selected nodes, and hovered nodes.
- Minor deep labels on large maps appear only after zooming in.

This keeps the initial SVG label/node load bounded while preserving explicit expand affordances for collapsed branches.
