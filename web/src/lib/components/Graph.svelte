<script lang="ts">
  import dagre from '@dagrejs/dagre';
  import type { GraphNode } from '$lib/api/gen/specgraph/v1/graph_pb';
  import type { Edge } from '$lib/api/gen/specgraph/v1/graph_pb';
  import { EdgeType } from '$lib/api/gen/specgraph/v1/graph_pb';
  import { stageColors } from '$lib/colors';

  interface Props {
    nodes: GraphNode[];
    edges: Edge[];
    compact?: boolean;
    filterText?: string;
  }

  let { nodes, edges, compact = false, filterText = '' }: Props = $props();

  const allColors: Record<string, string> = {
    ...stageColors,
    in_progress: '#ea580c',
    proposed: '#8b5cf6',
    accepted: '#0d9488',
    deprecated: '#6b7280',
    superseded: '#9ca3af',
  };

  const NODE_W = 180;
  const NODE_H = 60;
  const DECISION_SIZE = 50;

  function edgeStyle(et: EdgeType): { dash: string; color: string } {
    switch (et) {
      case EdgeType.DEPENDS_ON: return { dash: '', color: '#475569' };
      case EdgeType.BLOCKS: return { dash: '8,4', color: '#dc2626' };
      case EdgeType.COMPOSES: return { dash: '2,4', color: '#64748b' };
      case EdgeType.RELATES_TO: return { dash: '4,4', color: '#9ca3af' };
      case EdgeType.INFORMS: return { dash: '6,3', color: '#7c3aed' };
      case EdgeType.DECIDED_IN: return { dash: '6,3', color: '#0d9488' };
      case EdgeType.SUPERSEDES: return { dash: '10,4', color: '#d97706' };
      default: return { dash: '2,2', color: '#cbd5e1' };
    }
  }

  interface LayoutNode {
    slug: string;
    label: string;
    stage: string;
    intent: string;
    priority: string;
    x: number;
    y: number;
    isDecision: boolean;
  }

  interface LayoutEdge {
    fromId: string;
    toId: string;
    edgeType: EdgeType;
    points: Array<{ x: number; y: number }>;
  }

  let layout = $derived.by(() => {
    const g = new dagre.graphlib.Graph();
    g.setGraph({ rankdir: 'TB', nodesep: 40, ranksep: 60, marginx: 20, marginy: 20 });
    g.setDefaultEdgeLabel(() => ({}));

    for (const n of nodes) {
      const isDecision = n.label === 'Decision';
      g.setNode(n.slug, {
        width: isDecision ? DECISION_SIZE * 1.5 : NODE_W,
        height: isDecision ? DECISION_SIZE * 1.5 : NODE_H,
      });
    }

    for (const e of edges) {
      if (g.hasNode(e.fromId) && g.hasNode(e.toId)) {
        g.setEdge(e.fromId, e.toId);
      }
    }

    dagre.layout(g);

    const graphMeta = g.graph();
    const graphWidth = (graphMeta?.width ?? 400) + 40;
    const graphHeight = (graphMeta?.height ?? 300) + 40;

    const nodeMap = new Map<string, { x: number; y: number }>();
    for (const id of g.nodes()) {
      const pos = g.node(id);
      if (pos) nodeMap.set(id, { x: pos.x, y: pos.y });
    }

    const layoutNodes: LayoutNode[] = nodes.map((n) => {
      const pos = nodeMap.get(n.slug) ?? { x: 0, y: 0 };
      return {
        slug: n.slug,
        label: n.label,
        stage: n.stage,
        intent: n.intent,
        priority: n.priority,
        x: pos.x,
        y: pos.y,
        isDecision: n.label === 'Decision',
      };
    });

    const layoutEdges: LayoutEdge[] = edges
      .filter((e) => g.hasNode(e.fromId) && g.hasNode(e.toId))
      .map((e) => {
        const dagreEdge = g.edge(e.fromId, e.toId);
        const points = dagreEdge?.points ?? [
          nodeMap.get(e.fromId) ?? { x: 0, y: 0 },
          nodeMap.get(e.toId) ?? { x: 0, y: 0 },
        ];
        return {
          fromId: e.fromId,
          toId: e.toId,
          edgeType: e.edgeType,
          points: points as Array<{ x: number; y: number }>,
        };
      });

    return { layoutNodes, layoutEdges, graphWidth, graphHeight };
  });

  let panX = $state(0);
  let panY = $state(0);
  let zoom = $state(1);
  let dragging = $state(false);
  let dragStartX = $state(0);
  let dragStartY = $state(0);
  let panStartX = $state(0);
  let panStartY = $state(0);
  let hoveredSlug = $state<string | null>(null);

  function matchesFilter(n: LayoutNode): boolean {
    if (!filterText) return true;
    const lower = filterText.toLowerCase();
    return (
      n.slug.toLowerCase().includes(lower) ||
      n.intent.toLowerCase().includes(lower) ||
      n.stage.toLowerCase().includes(lower)
    );
  }

  function onWheel(e: WheelEvent) {
    if (compact) return;
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    zoom = Math.max(0.1, Math.min(4, zoom * factor));
  }

  function onPointerDown(e: PointerEvent) {
    if (compact) return;
    const target = e.target as SVGElement;
    if (target.closest('.graph-node')) return;
    dragging = true;
    dragStartX = e.clientX;
    dragStartY = e.clientY;
    panStartX = panX;
    panStartY = panY;
  }

  function onPointerMove(e: PointerEvent) {
    if (!dragging) return;
    panX = panStartX + (e.clientX - dragStartX);
    panY = panStartY + (e.clientY - dragStartY);
  }

  function onPointerUp() {
    dragging = false;
  }

  function pointsToPath(points: Array<{ x: number; y: number }>): string {
    if (points.length === 0) return '';
    let d = `M ${points[0].x} ${points[0].y}`;
    for (let i = 1; i < points.length; i++) {
      d += ` L ${points[i].x} ${points[i].y}`;
    }
    return d;
  }

  function stageColor(stage: string): string {
    return allColors[stage] ?? '#6b7280';
  }

  function truncate(text: string, max: number): string {
    return text.length > max ? text.slice(0, max - 1) + '\u2026' : text;
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<svg
  class="graph-svg"
  class:compact
  viewBox="0 0 {layout.graphWidth} {layout.graphHeight}"
  onwheel={onWheel}
  onpointerdown={onPointerDown}
  onpointermove={onPointerMove}
  onpointerup={onPointerUp}
>
  <defs>
    <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="#475569" />
    </marker>
  </defs>

  <g transform="translate({compact ? 0 : panX},{compact ? 0 : panY}) scale({compact ? 1 : zoom})">
    {#each layout.layoutEdges as edge, i (edge.fromId + '-' + edge.toId + '-' + i)}
      {@const style = edgeStyle(edge.edgeType)}
      <path
        d={pointsToPath(edge.points)}
        fill="none"
        stroke={style.color}
        stroke-width="1.5"
        stroke-dasharray={style.dash}
        marker-end="url(#arrowhead)"
      />
    {/each}

    {#each layout.layoutNodes as node (node.slug)}
      {@const faded = filterText && !matchesFilter(node)}
      {@const color = stageColor(node.stage)}
      <g
        class="graph-node"
        opacity={faded ? 0.2 : 1}
        onpointerenter={() => { if (!compact) hoveredSlug = node.slug; }}
        onpointerleave={() => { hoveredSlug = null; }}
      >
        <a href="{node.isDecision ? '/decision' : '/spec'}/{node.slug}">
          {#if node.isDecision}
            <g transform="translate({node.x},{node.y})">
              <rect
                x={-DECISION_SIZE / 2}
                y={-DECISION_SIZE / 2}
                width={DECISION_SIZE}
                height={DECISION_SIZE}
                rx="4"
                fill="white"
                stroke={color}
                stroke-width="2"
                transform="rotate(45)"
              />
              <text text-anchor="middle" dy="0.35em" font-size="10" fill={color}>
                {truncate(node.slug, 12)}
              </text>
            </g>
          {:else}
            <rect
              x={node.x - NODE_W / 2}
              y={node.y - NODE_H / 2}
              width={NODE_W}
              height={NODE_H}
              rx="8"
              fill="white"
              stroke={color}
              stroke-width="2"
            />
            <text
              x={node.x}
              y={node.y - 6}
              text-anchor="middle"
              font-size="12"
              font-weight="600"
              fill="#1a1a2e"
            >
              {truncate(node.slug, 22)}
            </text>
            <text
              x={node.x}
              y={node.y + 12}
              text-anchor="middle"
              font-size="10"
              fill={color}
            >
              {node.stage}{node.priority ? ` / ${node.priority}` : ''}
            </text>
          {/if}
        </a>

        {#if !compact && hoveredSlug === node.slug}
          <g transform="translate({node.x + NODE_W / 2 + 8},{node.y - 40})">
            <rect
              x="0" y="0" width="220" height="80" rx="6"
              fill="white" stroke="#e2e8f0" stroke-width="1"
              filter="drop-shadow(0 2px 4px rgba(0,0,0,0.1))"
            />
            <text x="10" y="18" font-size="11" font-weight="600" fill="#1a1a2e">
              {node.slug}
            </text>
            <text x="10" y="34" font-size="10" fill="#64748b">
              {truncate(node.intent, 30)}
            </text>
            <text x="10" y="50" font-size="10" fill={color}>
              Stage: {node.stage}
            </text>
            <text x="10" y="66" font-size="10" fill="#64748b">
              Priority: {node.priority || 'n/a'}
            </text>
          </g>
        {/if}
      </g>
    {/each}
  </g>
</svg>

<style>
  .graph-svg {
    width: 100%;
    height: 500px;
    border: 1px solid #e2e8f0;
    border-radius: 8px;
    background: #fafbfc;
    cursor: grab;
    user-select: none;
  }

  .graph-svg:active {
    cursor: grabbing;
  }

  .graph-svg.compact {
    height: 250px;
    cursor: default;
    pointer-events: none;
  }

  .graph-node a {
    cursor: pointer;
  }

  .graph-node a:hover rect {
    filter: brightness(0.97);
  }
</style>
