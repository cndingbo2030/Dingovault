<script>
  import { onDestroy } from 'svelte'
  import { forceSimulation, forceLink, forceManyBody, forceCenter, forceCollide } from 'd3-force'

  /** @type {{ nodes: { id: string, label: string }[], edges: { source: string, target: string }[] }} */
  export let graph = { nodes: [], edges: [] }

  const w = 560
  const h = 380

  /** @type {any} */
  let sim = null
  /** @type {any[]} */
  let simNodes = []
  /** @type {any[]} */
  let simLinks = []

  function restart() {
    if (sim) {
      sim.stop()
      sim = null
    }
    const nodes = graph.nodes || []
    const edges = graph.edges || []
    simNodes = nodes.map((n) => ({
      id: n.id,
      label: n.label || n.id,
      x: w / 2 + (Math.random() - 0.5) * 80,
      y: h / 2 + (Math.random() - 0.5) * 80
    }))
    simLinks = edges.map((e) => ({ source: e.source, target: e.target }))

    sim = forceSimulation(simNodes)
      .force(
        'link',
        forceLink(simLinks)
          .id((d) => d.id)
          .distance(90)
          .strength(0.35)
      )
      .force('charge', forceManyBody().strength(-220))
      .force('center', forceCenter(w / 2, h / 2))
      .force('collide', forceCollide().radius(36))

    sim.on('tick', () => {
      simNodes = simNodes
    })
    sim.alpha(0.9).restart()
  }

  $: graph && restart()

  onDestroy(() => {
    if (sim) sim.stop()
  })
</script>

<div class="graph-wrap" aria-label="Page link graph">
  <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`}>
    <defs>
      <marker id="arrow" markerWidth="8" markerHeight="8" refX="22" refY="4" orient="auto">
        <path d="M0,0 L8,4 L0,8 Z" fill="rgba(140,160,220,0.5)" />
      </marker>
    </defs>
    {#each simLinks as l, li (`${li}-${typeof l.source === 'object' ? l.source.id : l.source}-${typeof l.target === 'object' ? l.target.id : l.target}`)}
      {@const x1 = l.source.x ?? 0}
      {@const y1 = l.source.y ?? 0}
      {@const x2 = l.target.x ?? 0}
      {@const y2 = l.target.y ?? 0}
      <line
        class="edge"
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        marker-end="url(#arrow)"
      />
    {/each}
    {#each simNodes as n (n.id)}
      <g class="node" transform="translate({n.x ?? 0},{n.y ?? 0})">
        <circle r="22" />
        <text y="4" text-anchor="middle">{n.label.length > 14 ? n.label.slice(0, 12) + '…' : n.label}</text>
      </g>
    {/each}
  </svg>
  <p class="caption">{simNodes.length} pages · {simLinks.length} links</p>
</div>

<style>
  .graph-wrap {
    margin-top: 12px;
    border-radius: 12px;
    border: 1px solid var(--dv-border, rgba(255, 255, 255, 0.12));
    background: color-mix(in srgb, var(--dv-fg) 3%, transparent);
    overflow: hidden;
  }
  svg {
    display: block;
    width: 100%;
    height: auto;
    max-width: 100%;
  }
  .edge {
    stroke: rgba(120, 140, 200, 0.35);
    stroke-width: 1.2;
  }
  .node circle {
    fill: rgba(80, 110, 200, 0.22);
    stroke: rgba(130, 160, 255, 0.45);
    stroke-width: 1.2;
  }
  .node text {
    font-size: 9px;
    fill: var(--dv-fg, #e8e8ec);
    pointer-events: none;
  }
  .caption {
    margin: 0;
    padding: 8px 12px 10px;
    font-size: 0.75rem;
    opacity: 0.5;
    border-top: 1px solid var(--dv-border, rgba(255, 255, 255, 0.08));
  }
</style>
