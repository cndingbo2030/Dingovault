<script>
  import { onDestroy } from 'svelte'
  import { forceSimulation, forceLink, forceManyBody, forceCenter, forceCollide } from 'd3-force'
  import { messages, tr } from './lib/i18n/index.js'

  /** @type {{ nodes: { id: string, label: string }[], edges: { source: string, target: string }[] }} */
  export let graph = { nodes: [], edges: [] }

  /** @type {{ source: string, target: string, score?: number }[]} */
  export let semanticEdges = []

  /** When true, overlay semantic similarity links (faint, undirected). */
  export let semanticOn = false

  $: L = $messages
  /** @param {string} path @param {Record<string, string | number> | undefined} [vars] */
  function T(path, vars) {
    return tr(L, path, vars)
  }

  const w = 1600
  const h = 900
  const goldenAngle = Math.PI * (3 - Math.sqrt(5))

  /** @type {SVGSVGElement | undefined} */
  let svgEl
  /** @type {any} */
  let sim = null
  /** @type {any[]} */
  let simNodes = []
  /** @type {any[]} */
  let simLinks = []
  /** @type {Map<string, Set<string>>} */
  let neighborMap = new Map()
  let panX = 0
  let panY = 0
  let zoom = 1
  /** @type {any | null} */
  let dragNode = null
  let panning = false
  let panStartX = 0
  let panStartY = 0
  let panBaseX = 0
  let panBaseY = 0
  let hoveredId = ''

  /** @param {number} v @param {number} min @param {number} max */
  function clamp(v, min, max) {
    return Math.max(min, Math.min(max, v))
  }

  /** @param {string} label */
  function shortLabel(label) {
    const s = String(label || '')
    return s.length > 42 ? s.slice(0, 40) + '...' : s
  }

  /** @param {any} l */
  function sourceId(l) {
    return typeof l?.source === 'object' ? l.source.id : l?.source
  }

  /** @param {any} l */
  function targetId(l) {
    return typeof l?.target === 'object' ? l.target.id : l?.target
  }

  /** @param {string} a @param {string} b */
  function addNeighbor(a, b) {
    if (!a || !b) return
    if (!neighborMap.has(a)) neighborMap.set(a, new Set())
    neighborMap.get(a)?.add(b)
  }

  /** @param {any} l */
  function linkTouchesHover(l) {
    if (!hoveredId) return false
    return sourceId(l) === hoveredId || targetId(l) === hoveredId
  }

  /** @param {any} n */
  function nodeConnectedToHover(n) {
    if (!hoveredId) return true
    return n.id === hoveredId || !!neighborMap.get(hoveredId)?.has(n.id)
  }

  /** @param {any} n */
  function labelVisible(n) {
    if (hoveredId && nodeConnectedToHover(n)) return true
    if (simNodes.length <= 60) return true
    return zoom >= 0.86 || n.major
  }

  /** @param {any} n */
  function nodeSize(n) {
    return n.size || 5.5
  }

  /** @param {number} count */
  function chargeForCount(count) {
    if (count > 420) return -34
    if (count > 180) return -58
    if (count > 80) return -135
    return -360
  }

  /** @param {number} count */
  function defaultZoomForCount(count) {
    if (count <= 8) return 1.58
    if (count <= 40) return 1.22
    return 1
  }

  /** @param {number} count */
  function applyDefaultView(count) {
    zoom = defaultZoomForCount(count)
    panX = (w / 2) * (1 - zoom)
    panY = (h / 2) * (1 - zoom)
  }

  function restart() {
    if (sim) {
      sim.stop()
      sim = null
    }

    const nodes = graph.nodes || []
    const wikiEdges = graph.edges || []
    const count = nodes.length
    const degree = new Map()
    neighborMap = new Map()

    for (const e of wikiEdges) {
      degree.set(e.source, (degree.get(e.source) || 0) + 1)
      degree.set(e.target, (degree.get(e.target) || 0) + 1)
      addNeighbor(e.source, e.target)
      addNeighbor(e.target, e.source)
    }

    simNodes = nodes.map((n, i) => {
      const d = degree.get(n.id) || 0
      const major = d >= 3 || (count <= 80 && d >= 1)
      const angle = i * goldenAngle
      const radiusStep = count <= 8 ? 84 : count > 260 ? 21 : count > 90 ? 25 : 42
      const radius = Math.sqrt(i + 0.7) * radiusStep
      return {
        id: n.id,
        label: n.label || n.id,
        degree: d,
        major,
        size: clamp(4.6 + Math.sqrt(d + 1) * (count > 140 ? 1.45 : 2.1), 5, major ? 14 : 9),
        x: w / 2 + Math.cos(angle) * radius,
        y: h / 2 + Math.sin(angle) * radius * 0.74
      }
    })

    const wikiLinks = wikiEdges.map((e) => ({
      source: e.source,
      target: e.target,
      semantic: false,
      score: 0
    }))
    const sem = semanticOn
      ? (semanticEdges || []).map((e) => {
          addNeighbor(e.source, e.target)
          addNeighbor(e.target, e.source)
          return {
            source: e.source,
            target: e.target,
            semantic: true,
            score: typeof e.score === 'number' ? e.score : 0.65
          }
        })
      : []
    simLinks = [...wikiLinks, ...sem]

    const dense = count > 180
    const medium = count > 80
    const linkForce = /** @type {any} */ (forceLink(simLinks)).id((/** @type {{ id: string }} */ d) => d.id)
    linkForce
      .distance((/** @type {{ semantic?: boolean, score?: number }} */ l) => {
        if (l.semantic) return medium ? 108 + (1 - (l.score || 0.5)) * 42 : 148
        return dense ? 82 : medium ? 108 : 140
      })
      .strength((/** @type {{ semantic?: boolean, score?: number }} */ l) =>
        l.semantic ? 0.035 + (l.score || 0.5) * 0.045 : dense ? 0.14 : 0.25
      )

    sim = forceSimulation(simNodes)
      .force('link', linkForce)
      .force(
        'charge',
        forceManyBody().strength((/** @type {any} */ d) => chargeForCount(count) - Math.sqrt((d.degree || 0) + 1) * 13)
      )
      .force('center', forceCenter(w / 2, h / 2))
      .force('collide', forceCollide().radius((/** @type {any} */ d) => nodeSize(d) + (dense ? 5 : d.major ? 20 : 11)).strength(0.72))
      .velocityDecay(0.46)

    sim.on('tick', () => {
      simNodes = simNodes
    })
    applyDefaultView(count)
    sim.alpha(0.95).restart()
  }

  $: graph, semanticEdges, semanticOn, restart()

  onDestroy(() => {
    if (sim) sim.stop()
  })

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
    zoom = clamp(nextZoom, 0.22, 4.8)
    panX = sx - world.x * zoom
    panY = sy - world.y * zoom
  }

  /** @param {number} factor */
  function zoomBy(factor) {
    zoomAt(w / 2, h / 2, zoom * factor)
  }

  /** @param {PointerEvent} e @param {any} n */
  function startNodeDrag(e, n) {
    e.stopPropagation()
    dragNode = n
    hoveredId = n.id
    const p = svgWorldPoint(e)
    n.fx = p.x
    n.fy = p.y
    n.x = p.x
    n.y = p.y
    svgEl?.setPointerCapture(e.pointerId)
    sim?.alphaTarget(0.16).restart()
  }

  /** @param {PointerEvent} e */
  function startPan(e) {
    if (e.button !== 0 || dragNode) return
    panning = true
    panStartX = e.clientX
    panStartY = e.clientY
    panBaseX = panX
    panBaseY = panY
    svgEl?.setPointerCapture(e.pointerId)
  }

  /** @param {PointerEvent} e */
  function movePointer(e) {
    if (dragNode) {
      const p = svgWorldPoint(e)
      dragNode.fx = p.x
      dragNode.fy = p.y
      dragNode.x = p.x
      dragNode.y = p.y
      simNodes = simNodes
      return
    }
    if (panning) {
      panX = panBaseX + (e.clientX - panStartX)
      panY = panBaseY + (e.clientY - panStartY)
    }
  }

  /** @param {PointerEvent} e */
  function endPointer(e) {
    if (dragNode) {
      dragNode.fx = dragNode.x
      dragNode.fy = dragNode.y
      sim?.alphaTarget(0)
    }
    dragNode = null
    panning = false
    try {
      svgEl?.releasePointerCapture(e.pointerId)
    } catch {
      /* pointer may already be released */
    }
  }

  /** @param {WheelEvent} e */
  function onWheel(e) {
    const p = svgClientPoint(e)
    const factor = Math.exp(-e.deltaY * 0.0012)
    zoomAt(p.x, p.y, zoom * factor)
  }

  function resetView() {
    applyDefaultView(simNodes.length)
    hoveredId = ''
    for (const n of simNodes) {
      n.fx = null
      n.fy = null
    }
    sim?.alpha(0.75).restart()
  }

  $: wikiCount = (graph.edges || []).length
  $: semCount = semanticOn ? (semanticEdges || []).length : 0
  $: zoomLabel = `${Math.round(zoom * 100)}%`
</script>

<div class="graph-wrap" aria-label={T('graph.aria')}>
  <div class="graph-toolbar" role="toolbar" aria-label={T('graph.toolbar')}>
    <button type="button" aria-label="Zoom out" on:click={() => zoomBy(0.84)}>−</button>
    <button type="button" class="reset" on:click={resetView}>{T('graph.reset')}</button>
    <button type="button" aria-label="Zoom in" on:click={() => zoomBy(1.18)}>+</button>
    <span class="zoom-pill">{zoomLabel}</span>
  </div>
  <div class="graph-options" aria-label={T('graph.options')}>
    <button type="button"><span>›</span>{T('graph.filters')}</button>
    <button type="button"><span>›</span>{T('graph.groups')}</button>
    <button type="button"><span>›</span>{T('graph.display')}</button>
    <button type="button"><span>›</span>{T('graph.forces')}</button>
  </div>
  <svg
    bind:this={svgEl}
    class:panning
    width={w}
    height={h}
    viewBox={`0 0 ${w} ${h}`}
    role="img"
    aria-label={T('graph.aria')}
    on:pointerdown={startPan}
    on:pointermove={movePointer}
    on:pointerup={endPointer}
    on:pointercancel={endPointer}
    on:pointerleave={() => {
      if (!dragNode) hoveredId = ''
    }}
    on:dblclick={resetView}
    on:wheel|preventDefault={onWheel}
  >
    <defs>
      <filter id="node-shadow" x="-80%" y="-80%" width="260%" height="260%">
        <feDropShadow dx="0" dy="1.2" stdDeviation="1.2" flood-color="rgba(0,0,0,0.22)" flood-opacity="0.26" />
      </filter>
    </defs>
    <g transform={`translate(${panX},${panY}) scale(${zoom})`}>
      <g class="edges">
        {#each simLinks as l, li (`${li}-${sourceId(l)}-${targetId(l)}-${l.semantic ? 's' : 'w'}`)}
          {@const x1 = typeof l.source === 'object' ? l.source.x ?? 0 : 0}
          {@const y1 = typeof l.source === 'object' ? l.source.y ?? 0 : 0}
          {@const x2 = typeof l.target === 'object' ? l.target.x ?? 0 : 0}
          {@const y2 = typeof l.target === 'object' ? l.target.y ?? 0 : 0}
          {@const op = l.semantic ? 0.12 + 0.24 * (l.score || 0.5) : 0.42}
          <line
            class="edge"
            class:semantic={l.semantic}
            class:dim={hoveredId && !linkTouchesHover(l)}
            class:active={linkTouchesHover(l)}
            x1={x1}
            y1={y1}
            x2={x2}
            y2={y2}
            stroke-opacity={op}
          />
        {/each}
      </g>
      <g class="nodes">
        {#each simNodes as n (n.id)}
          {@const r = nodeSize(n)}
          {@const visibleLabel = labelVisible(n)}
          <g
            class="node"
            class:major={n.major}
            class:dim={hoveredId && !nodeConnectedToHover(n)}
            class:active={hoveredId === n.id}
            class:neighbor={hoveredId && hoveredId !== n.id && nodeConnectedToHover(n)}
            class:dragging={dragNode && dragNode.id === n.id}
            transform="translate({n.x ?? 0},{n.y ?? 0})"
            on:pointerdown={(e) => startNodeDrag(e, n)}
            on:pointerenter={() => (hoveredId = n.id)}
            on:pointerleave={() => {
              if (!dragNode) hoveredId = ''
            }}
          >
            <circle r={r} />
            {#if visibleLabel}
              <text y={r + 18} text-anchor="middle">{shortLabel(n.label)}</text>
            {/if}
          </g>
        {/each}
      </g>
    </g>
  </svg>
  <p class="caption">
    {simNodes.length} {T('app.pages')} · {wikiCount} {T('graph.links')}{#if semanticOn}
      · {semCount} {T('graph.semantic')}{/if}
  </p>
</div>

<style>
  .graph-wrap {
    --graph-node: #5b5d60;
    --graph-node-major: #3f4246;
    --graph-node-active: #26282c;
    --graph-node-neighbor: #4e5156;
    --graph-edge: #c7cbd1;
    --graph-edge-active: #8c9199;
    --graph-edge-semantic: #aab3c2;
    --graph-label: #1e2024;
    --graph-label-muted: rgba(30, 32, 36, 0.66);
    --graph-halo: rgba(255, 255, 255, 0.94);
    position: relative;
    height: 100%;
    min-height: 420px;
    border-radius: 0;
    background: var(--dv-panel);
    overflow: hidden;
  }
  :global(html:not([data-theme='light'])) .graph-wrap {
    --graph-node: #9da3ad;
    --graph-node-major: #c4c8d1;
    --graph-node-active: #f4f6fb;
    --graph-node-neighbor: #d7dbe4;
    --graph-edge: #4c5361;
    --graph-edge-active: #9199a9;
    --graph-edge-semantic: #6f7890;
    --graph-label: #f0f2f6;
    --graph-label-muted: rgba(240, 242, 246, 0.72);
    --graph-halo: rgba(18, 20, 26, 0.9);
  }
  svg {
    display: block;
    width: 100%;
    height: 100%;
    min-height: 420px;
    cursor: grab;
    background: var(--dv-panel);
    touch-action: none;
    user-select: none;
  }
  svg.panning,
  svg:active {
    cursor: grabbing;
  }
  .edge {
    stroke: var(--graph-edge);
    stroke-width: 1.05;
    vector-effect: non-scaling-stroke;
    transition:
      opacity 0.12s ease,
      stroke 0.12s ease,
      stroke-width 0.12s ease;
  }
  .edge.semantic {
    stroke: var(--graph-edge-semantic);
    stroke-width: 0.95;
    stroke-dasharray: 4 5;
  }
  .edge.active {
    stroke: var(--graph-edge-active);
    stroke-width: 1.45;
    opacity: 1;
  }
  .edge.dim {
    opacity: 0.09;
  }
  .node {
    cursor: grab;
    transition: opacity 0.12s ease;
  }
  .node circle {
    fill: var(--graph-node);
    stroke: var(--dv-panel);
    stroke-width: 1.4;
    filter: url(#node-shadow);
    vector-effect: non-scaling-stroke;
    transition:
      fill 0.12s ease,
      stroke 0.12s ease,
      r 0.12s ease;
  }
  .node.major circle {
    fill: var(--graph-node-major);
  }
  .node.neighbor circle {
    fill: var(--graph-node-neighbor);
  }
  .node.dragging circle,
  .node.active circle,
  .node:hover circle {
    fill: var(--graph-node-active);
    stroke: color-mix(in srgb, var(--dv-accent) 34%, var(--dv-panel));
    stroke-width: 2;
  }
  .node.dim {
    opacity: 0.18;
  }
  .node text {
    font-family: var(--dv-font, -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', sans-serif);
    font-size: 15px;
    font-weight: 440;
    letter-spacing: 0;
    fill: var(--graph-label);
    pointer-events: none;
    paint-order: stroke;
    stroke: var(--graph-halo);
    stroke-width: 5px;
    stroke-linejoin: round;
  }
  .node:not(.major):not(.active):not(.neighbor) text {
    fill: var(--graph-label-muted);
  }
  .graph-toolbar {
    position: absolute;
    z-index: 2;
    left: 14px;
    top: 14px;
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--dv-border);
    border-radius: 7px;
    background: color-mix(in srgb, var(--dv-panel) 93%, transparent);
    box-shadow: 0 12px 34px rgba(0, 0, 0, 0.12);
    -webkit-backdrop-filter: blur(14px);
    backdrop-filter: blur(14px);
  }
  .graph-toolbar button {
    min-width: 29px;
    height: 27px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .graph-toolbar button.reset {
    min-width: 52px;
  }
  .graph-toolbar button:hover {
    background: color-mix(in srgb, var(--dv-fg) 7%, transparent);
  }
  .zoom-pill {
    min-width: 42px;
    height: 27px;
    display: inline-grid;
    place-items: center;
    padding: 0 6px;
    color: var(--dv-muted);
    font-size: 0.7rem;
    border-left: 1px solid color-mix(in srgb, var(--dv-fg) 10%, transparent);
  }
  .graph-options {
    position: absolute;
    z-index: 2;
    top: 14px;
    right: 14px;
    width: 240px;
    overflow: hidden;
    border: 1px solid var(--dv-border);
    border-radius: 7px;
    background: color-mix(in srgb, var(--dv-panel) 94%, transparent);
    box-shadow: 0 18px 42px rgba(0, 0, 0, 0.13);
    -webkit-backdrop-filter: blur(14px);
    backdrop-filter: blur(14px);
  }
  .graph-options button {
    width: 100%;
    min-height: 39px;
    display: flex;
    align-items: center;
    gap: 8px;
    border: 0;
    border-bottom: 1px solid color-mix(in srgb, var(--dv-fg) 9%, transparent);
    background: transparent;
    color: var(--dv-fg);
    font: inherit;
    font-size: 0.84rem;
    text-align: left;
    padding: 0 12px;
    cursor: pointer;
  }
  .graph-options button:last-child {
    border-bottom: 0;
  }
  .graph-options button:hover {
    background: color-mix(in srgb, var(--dv-fg) 5%, transparent);
  }
  .graph-options span {
    color: var(--dv-muted);
    font-size: 1rem;
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
</style>
