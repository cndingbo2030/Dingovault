<script>
  import { tick } from 'svelte'
  import { hierarchy, tree, cluster } from 'd3-hierarchy'
  import { linkHorizontal } from 'd3-shape'
  import { messages, tr } from './lib/i18n/index.js'
  import { pushToast } from './toastStore.js'

  /** @type {any[]} */
  export let blocks = []
  export let pageTitle = 'Untitled'
  /** @type {Record<string, boolean>} */
  export let collapsedMap = {}
  /** @type {string[]} */
  export let selectedIds = []
  /** @type {'tree' | 'cluster'} */
  export let layout = 'tree'
  /** @type {(id: string) => void} */
  export let onToggleCollapse = () => {}
  /** @type {(id: string, on: boolean) => void} */
  export let onToggleSelect = () => {}
  /** @type {(id: string, text: string) => Promise<void>} */
  export let onUpdateNode = async () => {}
  /** @type {(movingId: string, newParentId: string) => Promise<void>} */
  export let onMoveUnder = async () => {}
  /** @type {(parentId: string) => Promise<void>} */
  export let onInsertChild = async () => {}
  /** @type {(id: string, text: string) => Promise<void>} */
  export let onRunCommand = async () => {}
  /** @type {(id: string, text: string) => Promise<void>} */
  export let onOpenTerminalContext = async () => {}

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  const w = 1600
  const h = 900
  const levelGap = 250
  const branchHues = [262, 202, 155, 28, 335, 226, 96, 12, 292, 178, 48, 5]
  const pathFor = linkHorizontal()
    .x((/** @type {any} */ d) => d.x)
    .y((/** @type {any} */ d) => d.y)

  /** @type {SVGSVGElement | undefined} */
  let svgEl
  let panX = 0
  let panY = 0
  let zoom = 1
  let panning = false
  let panStartX = 0
  let panStartY = 0
  let panBaseX = 0
  let panBaseY = 0
  let hoveredId = ''
  let dropTargetId = ''
  let editingId = ''
  let editingText = ''
  let lastSourceKey = ''
  /** @type {HTMLInputElement | undefined} */
  let editInput
  /** @type {{ id: string, label: string, startX: number, startY: number, x: number, y: number, moved: boolean, descendants: Set<string> } | null} */
  let dragSession = null

  /** @param {number} v @param {number} min @param {number} max */
  function clamp(v, min, max) {
    return Math.max(min, Math.min(max, v))
  }

  /** @param {string} text */
  function cleanLabel(text) {
    return String(text || '')
      .replace(/^\s*[-*+]\s+/, '')
      .replace(/\s+/g, ' ')
      .trim()
  }

  /** @param {string} text @param {number} max */
  function shortLabel(text, max = 52) {
    const label = cleanLabel(text)
    return label.length > max ? label.slice(0, max - 1) + '...' : label
  }

  /** @param {any[]} nodes */
  function countBlocks(nodes) {
    let total = 0
    for (const node of nodes || []) {
      total += 1 + countBlocks(node.children || [])
    }
    return total
  }

  /** @param {number} branchIndex @param {number} depth */
  function branchColor(branchIndex, depth) {
    const hue = branchHues[((branchIndex % branchHues.length) + branchHues.length) % branchHues.length]
    const sat = depth <= 1 ? 70 : 58
    const light = depth <= 1 ? 57 : 49
    return `hsl(${hue}, ${sat}%, ${light}%)`
  }

  /** @param {any} block @param {number} branchIndex @param {number} depth */
  function decorateBlock(block, branchIndex, depth) {
    const id = String(block?.id || '')
    const children = Array.isArray(block?.children) ? block.children : []
    return {
      id,
      label: cleanLabel(block?.content || id),
      content: String(block?.content || ''),
      source: block,
      synthetic: false,
      collapsed: !!collapsedMap[id],
      childCount: children.length,
      branchIndex,
      branchColor: branchColor(branchIndex, depth),
      children: children.map((/** @type {any} */ child) => decorateBlock(child, branchIndex, depth + 1))
    }
  }

  /** @param {any[]} pageBlocks */
  function decorateRoot(pageBlocks) {
    const children = (pageBlocks || []).map((block, index) => decorateBlock(block, index, 1))
    return {
      id: '__page__',
      label: pageTitle || 'Untitled',
      content: pageTitle || 'Untitled',
      synthetic: true,
      collapsed: false,
      childCount: children.length,
      branchIndex: -1,
      branchColor: 'var(--dv-accent)',
      children
    }
  }

  /** @param {any[]} pageBlocks */
  function sourceKeyFor(pageBlocks) {
    /** @type {string[]} */
    const ids = []
    const walk = (/** @type {any[]} */ nodes) => {
      for (const node of nodes || []) {
        ids.push(`${node.id}:${node.content || ''}`)
        walk(node.children || [])
      }
    }
    walk(pageBlocks)
    return `${pageTitle}:${ids.join('|')}`
  }

  /** @param {any[]} pageBlocks */
  function buildTree(pageBlocks) {
    const total = countBlocks(pageBlocks)
    const rowGap = total > 420 ? 30 : total > 220 ? 38 : total > 90 ? 48 : 62
    const root = hierarchy(decorateRoot(pageBlocks), (/** @type {any} */ d) => (d.collapsed ? null : d.children))
    const maker = /** @type {any} */ (layout === 'cluster' ? cluster() : tree())
    maker.nodeSize([rowGap, levelGap])(root)
    const nodes = root.descendants()
    const links = root.links()
    let minX = Infinity
    let maxX = -Infinity
    let minY = Infinity
    let maxY = -Infinity
    for (const node of nodes) {
      const x = node.y + 92
      const y = node.x
      node.renderX = x
      node.renderY = y
      minX = Math.min(minX, x)
      maxX = Math.max(maxX, x)
      minY = Math.min(minY, y)
      maxY = Math.max(maxY, y)
    }
    if (!nodes.length || !Number.isFinite(minX)) {
      minX = 0
      maxX = w
      minY = 0
      maxY = h
    }
    const offsetY = h / 2 - (minY + maxY) / 2
    for (const node of nodes) node.renderY += offsetY
    minY += offsetY
    maxY += offsetY
    return {
      root,
      nodes,
      links,
      total,
      sourceKey: sourceKeyFor(pageBlocks),
      bounds: { minX, maxX, minY, maxY }
    }
  }

  $: selectedSet = new Set(selectedIds || [])
  $: mapTree = buildTree(blocks || [])
  $: if (mapTree.sourceKey !== lastSourceKey) {
    lastSourceKey = mapTree.sourceKey
    resetView()
  }
  $: zoomLabel = `${Math.round(zoom * 100)}%`

  /** @param {{ minX: number, maxX: number, minY: number, maxY: number }} bounds */
  function fitToBounds(bounds) {
    const spanX = Math.max(1, bounds.maxX - bounds.minX + 280)
    const spanY = Math.max(1, bounds.maxY - bounds.minY + 220)
    zoom = clamp(Math.min((w - 80) / spanX, (h - 80) / spanY), 0.32, 1.28)
    panX = w / 2 - ((bounds.minX + bounds.maxX) / 2) * zoom
    panY = h / 2 - ((bounds.minY + bounds.maxY) / 2) * zoom
  }

  function resetView() {
    fitToBounds(mapTree?.bounds || { minX: 0, maxX: w, minY: 0, maxY: h })
    hoveredId = ''
    dropTargetId = ''
  }

  /** @param {{ clientX: number, clientY: number }} e */
  function svgClientPoint(e) {
    if (!svgEl) return { x: w / 2, y: h / 2 }
    const pt = svgEl.createSVGPoint()
    pt.x = e.clientX
    pt.y = e.clientY
    const ctm = svgEl.getScreenCTM()
    if (!ctm) return { x: w / 2, y: h / 2 }
    return pt.matrixTransform(ctm.inverse())
  }

  /** @param {{ clientX: number, clientY: number }} e */
  function svgWorldPoint(e) {
    const p = svgClientPoint(e)
    return { x: (p.x - panX) / zoom, y: (p.y - panY) / zoom }
  }

  /** @param {number} sx @param {number} sy @param {number} nextZoom */
  function zoomAt(sx, sy, nextZoom) {
    const world = { x: (sx - panX) / zoom, y: (sy - panY) / zoom }
    zoom = clamp(nextZoom, 0.24, 4.8)
    panX = sx - world.x * zoom
    panY = sy - world.y * zoom
  }

  /** @param {number} factor */
  function zoomBy(factor) {
    zoomAt(w / 2, h / 2, zoom * factor)
  }

  /** @param {WheelEvent} e */
  function onWheel(e) {
    const p = svgClientPoint(e)
    const factor = Math.exp(-e.deltaY * 0.0012)
    zoomAt(p.x, p.y, zoom * factor)
  }

  /** @param {PointerEvent} e */
  function startPan(e) {
    if (e.button !== 0 || dragSession || editingId) return
    panning = true
    panStartX = e.clientX
    panStartY = e.clientY
    panBaseX = panX
    panBaseY = panY
    svgEl?.setPointerCapture(e.pointerId)
  }

  /** @param {PointerEvent} e @param {any} node */
  function startNodePointer(e, node) {
    if (node.data.synthetic || editingId) return
    e.stopPropagation()
    hoveredId = node.data.id
    dragSession = {
      id: node.data.id,
      label: node.data.label,
      startX: e.clientX,
      startY: e.clientY,
      x: node.renderX,
      y: node.renderY,
      moved: false,
      descendants: new Set(node.descendants().map((/** @type {any} */ d) => d.data.id))
    }
    svgEl?.setPointerCapture(e.pointerId)
  }

  /** @param {{ x: number, y: number }} point */
  function nearestDropTarget(point) {
    if (!dragSession?.moved) return ''
    let bestId = ''
    let best = Infinity
    for (const node of mapTree.nodes) {
      if (node.data.synthetic || node.data.id === dragSession.id || dragSession.descendants.has(node.data.id)) continue
      const dx = node.renderX - point.x
      const dy = node.renderY - point.y
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < best) {
        best = dist
        bestId = node.data.id
      }
    }
    return best <= 74 ? bestId : ''
  }

  /** @param {PointerEvent} e */
  function movePointer(e) {
    if (dragSession) {
      const moved = Math.abs(e.clientX - dragSession.startX) + Math.abs(e.clientY - dragSession.startY) > 7
      const p = svgWorldPoint(e)
      dragSession = { ...dragSession, moved: dragSession.moved || moved, x: p.x, y: p.y }
      dropTargetId = nearestDropTarget(p)
      return
    }
    if (panning) {
      panX = panBaseX + (e.clientX - panStartX)
      panY = panBaseY + (e.clientY - panStartY)
    }
  }

  /** @param {PointerEvent} e */
  function endPointer(e) {
    const session = dragSession
    const target = dropTargetId
    dragSession = null
    dropTargetId = ''
    panning = false
    try {
      svgEl?.releasePointerCapture(e.pointerId)
    } catch {
      /* pointer may already be released */
    }
    if (!session) return
    if (session.moved && target) {
      void onMoveUnder(session.id, target)
      return
    }
    onToggleSelect(session.id, !selectedSet.has(session.id))
  }

  /** @param {any} node */
  function nodeRadius(node) {
    if (node.data.synthetic) return 12
    return clamp(7 + Math.sqrt((node.data.childCount || 0) + 1) * 2.2, 8.5, 17)
  }

  /** @param {any} node */
  function labelVisible(node) {
    if (editingId === node.data.id) return false
    if (hoveredId === node.data.id || selectedSet.has(node.data.id)) return true
    if (node.data.synthetic || node.depth <= 2) return true
    if (mapTree.total <= 130) return true
    return zoom >= 0.82
  }

  /** @param {any} link */
  function linkPath(link) {
    return (
      pathFor({
        source: { x: link.source.renderX, y: link.source.renderY },
        target: { x: link.target.renderX, y: link.target.renderY }
      }) || ''
    )
  }

  /** @param {any} node */
  function beginEdit(node) {
    if (node.data.synthetic) return
    editingId = node.data.id
    editingText = node.data.content
    void tick().then(() => {
      editInput?.focus()
      editInput?.select()
    })
  }

  async function commitEdit() {
    const id = editingId
    const text = editingText
    editingId = ''
    editingText = ''
    if (!id) return
    await onUpdateNode(id, text)
  }

  function cancelEdit() {
    editingId = ''
    editingText = ''
  }

  /** @param {KeyboardEvent} e @param {() => void} fn */
  function activateKey(e, fn) {
    if (e.key !== 'Enter' && e.key !== ' ') return
    e.preventDefault()
    fn()
  }

  /** @param {any[]} pageBlocks @param {number} depth @returns {string[]} */
  function markdownOutline(pageBlocks, depth = 0) {
    /** @type {string[]} */
    const lines = []
    for (const block of pageBlocks || []) {
      lines.push(`${'  '.repeat(depth)}- ${cleanLabel(block.content || '')}`)
      lines.push(...markdownOutline(block.children || [], depth + 1))
    }
    return lines
  }

  async function copyMarkdownOutline() {
    try {
      await navigator.clipboard.writeText(markdownOutline(blocks || []).join('\n'))
      pushToast(T('mindmap.copied'), 'info')
    } catch {
      pushToast(T('app.clipboardFailed'), 'error')
    }
  }

  function filenameBase() {
    return (pageTitle || 'mind-map').replace(/[^\w.-]+/g, '-').replace(/^-+|-+$/g, '') || 'mind-map'
  }

  /** @param {string} name @param {Blob} blob */
  function downloadBlob(name, blob) {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  }

  function serializedSvg() {
    if (!svgEl) return ''
    const clone = /** @type {SVGSVGElement} */ (svgEl.cloneNode(true))
    clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
    clone.setAttribute('width', String(w))
    clone.setAttribute('height', String(h))
    for (const el of clone.querySelectorAll('foreignObject')) el.remove()
    const style = document.createElementNS('http://www.w3.org/2000/svg', 'style')
    style.textContent = `
      .mindmap-bg{fill:${getComputedStyle(svgEl).backgroundColor || '#fff'}}
      .mind-link{fill:none;stroke:var(--link-color,#8a8f98);stroke-width:1.35;stroke-linecap:round;opacity:.48}
      .mind-node .node-core{fill:var(--node-color,#8b6eea);stroke:${getComputedStyle(svgEl).backgroundColor || '#fff'};stroke-width:2}
      .mind-node .node-halo{fill:var(--node-color,#8b6eea);opacity:.14}
      .mind-node .node-label{font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;font-size:14px;fill:${getComputedStyle(document.documentElement).getPropertyValue('--dv-fg') || '#222'};paint-order:stroke;stroke:${getComputedStyle(svgEl).backgroundColor || '#fff'};stroke-width:5;stroke-linejoin:round}
    `
    clone.insertBefore(style, clone.firstChild)
    return new XMLSerializer().serializeToString(clone)
  }

  function exportSvg() {
    const text = serializedSvg()
    if (!text) return
    downloadBlob(`${filenameBase()}-mind-map.svg`, new Blob([text], { type: 'image/svg+xml;charset=utf-8' }))
    pushToast(T('mindmap.exported', { format: 'SVG' }), 'info')
  }

  async function exportPng() {
    const text = serializedSvg()
    if (!text) return
    const url = URL.createObjectURL(new Blob([text], { type: 'image/svg+xml;charset=utf-8' }))
    const img = new Image()
    img.decoding = 'async'
    await new Promise((resolve, reject) => {
      img.onload = resolve
      img.onerror = reject
      img.src = url
    })
    const canvas = document.createElement('canvas')
    canvas.width = w * 2
    canvas.height = h * 2
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      URL.revokeObjectURL(url)
      return
    }
    ctx.scale(2, 2)
    ctx.drawImage(img, 0, 0, w, h)
    URL.revokeObjectURL(url)
    canvas.toBlob((blob) => {
      if (!blob) return
      downloadBlob(`${filenameBase()}-mind-map.png`, blob)
      pushToast(T('mindmap.exported', { format: 'PNG' }), 'info')
    }, 'image/png')
  }
</script>

<div class="mindmap-wrap" aria-label={T('mindmap.aria')}>
  <div class="mindmap-toolbar" role="toolbar" aria-label={T('mindmap.toolbar')}>
    <button type="button" aria-label="Zoom out" on:click={() => zoomBy(0.84)}>−</button>
    <button type="button" class="reset" on:click={resetView}>{T('mindmap.reset')}</button>
    <button type="button" aria-label="Zoom in" on:click={() => zoomBy(1.18)}>+</button>
    <span class="zoom-pill">{zoomLabel}</span>
    <span class="toolbar-sep" aria-hidden="true"></span>
    <button type="button" class="export" on:click={exportSvg}>{T('mindmap.exportSvg')}</button>
    <button type="button" class="export" on:click={() => void exportPng()}>{T('mindmap.exportPng')}</button>
    <button type="button" class="export wide" on:click={() => void copyMarkdownOutline()}>{T('mindmap.copyMarkdown')}</button>
  </div>

  {#if blocks.length === 0}
    <div class="mindmap-empty">{T('mindmap.empty')}</div>
  {/if}

  <svg
    bind:this={svgEl}
    class:panning
    width={w}
    height={h}
    viewBox={`0 0 ${w} ${h}`}
    role="img"
    aria-label={T('mindmap.aria')}
    on:pointerdown={startPan}
    on:pointermove={movePointer}
    on:pointerup={endPointer}
    on:pointercancel={endPointer}
    on:pointerleave={() => {
      if (!dragSession) hoveredId = ''
    }}
    on:wheel|preventDefault={onWheel}
  >
    <defs>
      <filter id="mind-node-shadow" x="-80%" y="-80%" width="260%" height="260%">
        <feDropShadow dx="0" dy="1.4" stdDeviation="1.7" flood-color="rgba(0,0,0,0.22)" flood-opacity="0.24" />
      </filter>
    </defs>
    <rect class="mindmap-bg" x="0" y="0" width={w} height={h} />
    <g transform={`translate(${panX},${panY}) scale(${zoom})`}>
      <g class="links">
        {#each mapTree.links as link (`${link.source.data.id}->${link.target.data.id}`)}
          <path
            class="mind-link"
            d={linkPath(link)}
            style={`--link-color:${link.target.data.branchColor}`}
          />
        {/each}
      </g>
      <g class="nodes">
        {#each mapTree.nodes as node (node.data.id)}
          {@const r = nodeRadius(node)}
          {@const selected = selectedSet.has(node.data.id)}
          {@const hasChildren = node.data.childCount > 0}
          {@const isDropTarget = dropTargetId === node.data.id}
          <g
            class="mind-node"
            class:root={node.data.synthetic}
            class:selected
            class:collapsed={node.data.collapsed}
            class:drop-target={isDropTarget}
            class:dragging={dragSession?.id === node.data.id}
            style={`--node-color:${node.data.branchColor}`}
            transform={`translate(${node.renderX},${node.renderY})`}
            role="button"
            tabindex="0"
            aria-label={node.data.synthetic ? node.data.label : T('mindmap.editNode', { label: node.data.label })}
            on:pointerdown={(e) => startNodePointer(e, node)}
            on:pointerenter={() => (hoveredId = node.data.id)}
            on:pointerleave={() => {
              if (!dragSession) hoveredId = ''
            }}
            on:dblclick|stopPropagation={() => beginEdit(node)}
            on:keydown={(e) => {
              if (!node.data.synthetic) activateKey(e, () => beginEdit(node))
            }}
          >
            <circle class="node-halo" r={r + 11} />
            <circle class="node-core" r={r} />
            {#if hasChildren && !node.data.synthetic}
              <g
                class="collapse-chip"
                transform={`translate(${r + 12},${-13})`}
                role="button"
                tabindex="0"
                aria-label={T('mindmap.toggleCollapse', { label: node.data.label })}
                on:pointerdown|stopPropagation
                on:click|stopPropagation={() => onToggleCollapse(node.data.id)}
                on:keydown={(e) => activateKey(e, () => onToggleCollapse(node.data.id))}
              >
                <rect x="0" y="0" width="24" height="24" rx="6" />
                <path d={node.data.collapsed ? 'M8 12h8M12 8v8' : 'M8 12h8'} />
              </g>
            {/if}
            {#if !node.data.synthetic}
              <g
                class="add-chip"
                transform={`translate(${r + 40},${-13})`}
                role="button"
                tabindex="0"
                aria-label={T('mindmap.addChildTo', { label: node.data.label })}
                on:pointerdown|stopPropagation
                on:click|stopPropagation={() => void onInsertChild(node.data.id)}
                on:keydown={(e) => activateKey(e, () => void onInsertChild(node.data.id))}
              >
                <rect x="0" y="0" width="24" height="24" rx="6" />
                <path d="M8 12h8M12 8v8" />
              </g>
              <g
                class="run-chip"
                transform={`translate(${r + 68},${-13})`}
                role="button"
                tabindex="0"
                aria-label={T('mindmap.runCommand', { label: node.data.label })}
                on:pointerdown|stopPropagation
                on:click|stopPropagation={() => void onRunCommand(node.data.id, node.data.content)}
                on:keydown={(e) => activateKey(e, () => void onRunCommand(node.data.id, node.data.content))}
              >
                <rect x="0" y="0" width="24" height="24" rx="6" />
                <path d="M9 7.5 16 12 9 16.5Z" />
              </g>
              <g
                class="context-chip"
                transform={`translate(${r + 96},${-13})`}
                role="button"
                tabindex="0"
                aria-label={T('mindmap.openTerminalHere', { label: node.data.label })}
                on:pointerdown|stopPropagation
                on:click|stopPropagation={() => void onOpenTerminalContext(node.data.id, node.data.content)}
                on:keydown={(e) => activateKey(e, () => void onOpenTerminalContext(node.data.id, node.data.content))}
              >
                <rect x="0" y="0" width="24" height="24" rx="6" />
                <path d="M7 8.5 10.5 12 7 15.5M12 16h6" />
              </g>
            {/if}
            {#if labelVisible(node)}
              <text class="node-label" x={node.data.synthetic ? 0 : r + 14} y="5" text-anchor={node.data.synthetic ? 'middle' : 'start'}>
                {shortLabel(node.data.label, node.data.synthetic ? 38 : 58)}
              </text>
            {/if}
            {#if editingId === node.data.id}
              <foreignObject class="node-editor" x={r + 12} y="-18" width="330" height="40">
                <input
                  bind:this={editInput}
                  bind:value={editingText}
                  on:blur={() => void commitEdit()}
                  on:keydown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      void commitEdit()
                    } else if (e.key === 'Escape') {
                      e.preventDefault()
                      cancelEdit()
                    }
                  }}
                />
              </foreignObject>
            {/if}
          </g>
        {/each}
      </g>
      {#if dragSession?.moved}
        <g class="drag-ghost" transform={`translate(${dragSession.x},${dragSession.y})`}>
          <circle r="10" />
          <text x="18" y="5">{shortLabel(dragSession.label, 46)}</text>
        </g>
      {/if}
    </g>
  </svg>
  <p class="caption">
    {mapTree.total} {T('mindmap.blocks')}
    {#if mapTree.total > 130 && zoom < 0.82}
      · {T('mindmap.labelsHidden')}
    {/if}
  </p>
</div>

<style>
  .mindmap-wrap {
    --mind-bg: var(--dv-panel);
    --mind-label: var(--dv-fg);
    --mind-muted: var(--dv-muted);
    position: relative;
    height: 100%;
    min-height: 420px;
    overflow: hidden;
    background: var(--mind-bg);
  }
  svg {
    display: block;
    width: 100%;
    height: 100%;
    min-height: 420px;
    cursor: grab;
    background: var(--mind-bg);
    touch-action: none;
    user-select: none;
  }
  svg.panning,
  svg:active {
    cursor: grabbing;
  }
  .mindmap-bg {
    fill: var(--mind-bg);
  }
  .mind-link {
    fill: none;
    stroke: var(--link-color);
    stroke-width: 1.35;
    stroke-linecap: round;
    opacity: 0.46;
    vector-effect: non-scaling-stroke;
  }
  .mind-node {
    cursor: grab;
  }
  .node-halo {
    fill: var(--node-color);
    opacity: 0.13;
    transition: opacity 0.12s ease;
  }
  .node-core {
    fill: var(--node-color);
    stroke: var(--mind-bg);
    stroke-width: 2;
    filter: url(#mind-node-shadow);
    vector-effect: non-scaling-stroke;
  }
  .mind-node.root .node-core {
    fill: var(--dv-fg);
  }
  .mind-node.selected .node-core,
  .mind-node.drop-target .node-core,
  .mind-node:hover .node-core {
    stroke: var(--dv-accent);
    stroke-width: 2.4;
  }
  .mind-node.drop-target .node-halo {
    opacity: 0.34;
  }
  .mind-node.dragging {
    opacity: 0.42;
  }
  .node-label {
    font-family: var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', sans-serif);
    font-size: 14px;
    font-weight: 460;
    letter-spacing: 0;
    fill: var(--mind-label);
    pointer-events: none;
    paint-order: stroke;
    stroke: var(--mind-bg);
    stroke-width: 5px;
    stroke-linejoin: round;
  }
  .mind-node.root .node-label {
    font-size: 15px;
    font-weight: 600;
  }
  .collapse-chip,
  .add-chip,
  .run-chip,
  .context-chip {
    opacity: 0;
    cursor: pointer;
    transition: opacity 0.12s ease;
  }
  .mind-node:hover .collapse-chip,
  .mind-node:hover .add-chip,
  .mind-node:hover .run-chip,
  .mind-node:hover .context-chip,
  .mind-node.selected .collapse-chip,
  .mind-node.selected .add-chip,
  .mind-node.selected .run-chip,
  .mind-node.selected .context-chip {
    opacity: 1;
  }
  .collapse-chip rect,
  .add-chip rect,
  .run-chip rect,
  .context-chip rect {
    fill: color-mix(in srgb, var(--dv-panel) 92%, transparent);
    stroke: var(--dv-border);
    stroke-width: 1;
    filter: url(#mind-node-shadow);
  }
  .collapse-chip path,
  .add-chip path,
  .run-chip path,
  .context-chip path {
    fill: none;
    stroke: var(--dv-fg);
    stroke-width: 1.7;
    stroke-linecap: round;
  }
  .run-chip path {
    fill: var(--dv-fg);
    stroke: none;
  }
  .node-editor input {
    width: 318px;
    height: 31px;
    box-sizing: border-box;
    border: 1px solid var(--dv-accent);
    border-radius: 7px;
    padding: 4px 8px;
    background: var(--dv-panel);
    color: var(--dv-fg);
    font: inherit;
    font-size: 13px;
    outline: none;
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--dv-accent) 14%, transparent);
  }
  .drag-ghost {
    pointer-events: none;
    opacity: 0.78;
  }
  .drag-ghost circle {
    fill: var(--dv-accent);
    stroke: var(--mind-bg);
    stroke-width: 2;
  }
  .drag-ghost text {
    fill: var(--mind-label);
    font-size: 13px;
    paint-order: stroke;
    stroke: var(--mind-bg);
    stroke-width: 5px;
  }
  .mindmap-toolbar {
    position: absolute;
    z-index: 2;
    left: 14px;
    top: 14px;
    display: inline-flex;
    align-items: center;
    gap: 2px;
    max-width: calc(100% - 28px);
    padding: 3px;
    border: 1px solid var(--dv-border);
    border-radius: 7px;
    background: color-mix(in srgb, var(--dv-panel) 94%, transparent);
    box-shadow: 0 12px 34px rgba(0, 0, 0, 0.12);
    -webkit-backdrop-filter: blur(14px);
    backdrop-filter: blur(14px);
  }
  .mindmap-toolbar button {
    min-width: 29px;
    height: 27px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.78rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .mindmap-toolbar button.reset {
    min-width: 52px;
  }
  .mindmap-toolbar button.export {
    min-width: 76px;
    padding: 0 8px;
  }
  .mindmap-toolbar button.wide {
    min-width: 132px;
  }
  .mindmap-toolbar button:hover {
    background: color-mix(in srgb, var(--dv-fg) 7%, transparent);
  }
  .zoom-pill {
    min-width: 42px;
    height: 27px;
    display: grid;
    place-items: center;
    color: var(--dv-muted);
    font-size: 0.72rem;
  }
  .toolbar-sep {
    width: 1px;
    height: 18px;
    margin: 0 3px;
    background: var(--dv-border);
  }
  .caption {
    position: absolute;
    left: 14px;
    bottom: 10px;
    margin: 0;
    color: var(--dv-muted);
    font-size: 0.72rem;
    pointer-events: none;
  }
  .mindmap-empty {
    position: absolute;
    z-index: 1;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    color: var(--dv-muted);
    font-size: 0.86rem;
    pointer-events: none;
  }
  @media (max-width: 720px) {
    .mindmap-toolbar {
      right: 10px;
      left: 10px;
      overflow-x: auto;
    }
    .mindmap-toolbar button.export.wide {
      min-width: 112px;
    }
  }
</style>
