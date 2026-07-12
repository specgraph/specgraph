<script lang="ts">
  import * as Accordion from '$lib/components/ui/accordion/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';

  interface Props {
    title: string;
    expanded?: boolean;
    badge?: string;
    children: import('svelte').Snippet;
  }
  let { title, expanded = false, badge = '', children }: Props = $props();
  let toggled = $state<boolean | null>(null);
  let open = $derived(toggled !== null ? toggled : expanded);
</script>

<Accordion.Root
  type="single"
  value={open ? 'section' : ''}
  onValueChange={(v) => (toggled = v === 'section')}
>
  <Accordion.Item value="section">
    <Accordion.Trigger class="text-foreground hover:text-primary hover:no-underline text-[0.95rem] font-semibold">
      <span class="flex items-center gap-2">
        <span>{title}</span>
        {#if badge}<Badge variant="secondary">{badge}</Badge>{/if}
      </span>
    </Accordion.Trigger>
    <Accordion.Content class="text-muted-foreground text-[0.9rem] leading-relaxed pl-5">
      {@render children()}
    </Accordion.Content>
  </Accordion.Item>
</Accordion.Root>
