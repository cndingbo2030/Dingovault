<script>
  import { onDestroy } from 'svelte'
  import { forceSimulation, forceLink, forceManyBody, forceCenter, forceCollide } from 'd3-force'

  /** @type {{ nodes: { id: string, label: string }[], edges: { source: string, target: string }[] }} */
  export let graph = { nodes: [], edges: [] }

  /** @type {{ source: string, target: string, score?: number }[]} */
  export let semanticEdges = []

  /** When true, overlay semantic similarity links (faint, undirected). */
  export let semanticOn = false

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
    const wikiEdges = graph.edges || []
    simNodes = nodes.map((n) => ({
      id: n.id,
      label: n.label || n.id,
      x: w / 2 + (Math.random() - 0.5) * 80,
      y: h / 2 + (Math.random() - 0.5) * 80
    }))
    const wikiLinks = wikiEdges.map((e) => ({
      source: e.source,
      target: e.target,
      semantic: false,
      score: 0
    }))
    const sem = semanticOn
      ? (semanticEdges || []).map((e) => ({
          source: e.source,
          target: e.target,
          semantic: true,
          score: typeof e.score === 'number' ? e.score : 0.65
        }))
      : []
    simLinks = [...wikiLinks, ...sem]

    sim = forceSimulation(simNodes)
      .force(
        'link',
        forceLink(simLinks)
          .id((/** @type {{ id: string }} */ d) => d.id)
          .distance((/** @type {{ semantic?: boolean, score?: number }} */ l) =>
            l.semantic ? 108 + (1 - (l.score || 0.5)) * 40 : 90
          )
          .strength((/** @type {{ semantic?: boolean, score?: number }} */ l) =>
            l.semantic ? 0.05 + (l.score || 0.5) * 0.08 : 0.35
          )
      )
      .force('charge', forceManyBody().strength(-220))
      .force('center', forceCenter(w / 2, h / 2))
      .force('collide', forceCollide().radius(36))

    sim.on('tick', () => {
      simNodes = simNodes
    })
    sim.alpha(0.9).restart()
  }

  $: graph, semanticEdges, semanticOn, restart()

  onDestroy(() => {
    if (sim) sim.stop()
  })

  $: wikiCount = (graph.edges || []).length
  $: semCount = semanticOn ? (semanticEdges || []).length : 0
</script>

<div class="graph-wrap" aria-label="Page link graph">
  <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`}>
    <defs>
      <marker id="arrow" markerWidth="8" markerHeight="8" refX="22" refY="4" orient="auto">
        <path d="M0,0 L8,4 L0,8 Z" fill="rgba(140,160,220,0.5)" />
      </marker>
    </defs>
    {#each simLinks as l, li (`${li}-${typeof l.source === 'object' ? l.source.id : l.source}-${typeof l.target === 'object' ? l.target.id : l.target}-${l.semantic ? 's' : 'w'}`)}
      {@const x1 = l.source.x ?? 0}
      {@const y1 = l.source.y ?? 0}
      {@const x2 = l.target.x ?? 0}
      {@const y2 = l.target.y ?? 0}
      {@const op = l.semantic ? 0.1 + 0.42 * (l.score || 0.5) : 0.35}
      <line
        class="edge"
        class:semantic={l.semantic}
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        stroke-opacity={op}
        marker-end={l.semantic ? 'none' : 'url(#arrow)'}
      />
    {/each}
    {#each simNodes as n (n.id)}
      <g class="node" transform="translate({n.x ?? 0},{n.y ?? 0})">
        <circle r="22" />
        <text y="4" text-anchor="middle">{n.label.length > 14 ? n.label.slice(0, 12) + '…' : n.label}</text>
      </g>
    {/each}
  </svg>
  <p class="caption">
    {simNodes.length} pages · {wikiCount} wiki links{#if semanticOn}
      · {semCount} semantic{/if}
  </p>
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
  .edge.semantic {
    stroke: rgba(180, 140, 220, 0.55);
    stroke-width: 1;
    stroke-dasharray: 4 5;
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
