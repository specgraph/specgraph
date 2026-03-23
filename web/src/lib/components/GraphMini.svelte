<script lang="ts">
  import { goto } from '$app/navigation';
  import Graph from './Graph.svelte';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';

  interface Props {
    nodes: GraphNode[];
    edges: Edge[];
  }

  let { nodes, edges }: Props = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="mini-wrapper"
  onclick={() => goto('/graph')}
  role="link"
  tabindex="0"
  onkeydown={(e) => e.key === 'Enter' && goto('/graph')}
>
  <h3>Graph Preview</h3>
  <Graph {nodes} {edges} compact />
</div>

<style>
  .mini-wrapper {
    background: white;
    border-radius: 8px;
    padding: 1.25rem;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
    cursor: pointer;
    transition: box-shadow 0.15s;
  }

  .mini-wrapper:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
  }

  h3 {
    margin: 0 0 0.75rem;
    font-size: 0.95rem;
    color: #1a1a2e;
  }
</style>
