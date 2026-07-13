<script lang="ts">
  import { goto } from '$app/navigation';
  import Graph from './Graph.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import type { GraphNode, Edge } from '$lib/api/gen/specgraph/v1/graph_pb';

  interface Props {
    nodes: GraphNode[];
    edges: Edge[];
  }

  let { nodes, edges }: Props = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<Card.Root
  class="mini-wrapper cursor-pointer p-5 transition-shadow hover:shadow-md"
  onclick={() => goto('/graph')}
  role="link"
  tabindex={0}
  onkeydown={(e: KeyboardEvent) => e.key === 'Enter' && goto('/graph')}
>
  <h3 class="mini-title">Graph Preview</h3>
  <Graph {nodes} {edges} compact />
</Card.Root>

<style>
  .mini-title {
    margin: 0 0 0.75rem;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--foreground);
  }
</style>
