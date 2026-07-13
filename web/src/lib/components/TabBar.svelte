<script lang="ts">
  import * as Tabs from '$lib/components/ui/tabs/index.js';

  interface Props {
    tabs: string[];
    active: string;
    onchange: (tab: string) => void;
  }
  let { tabs, active, onchange }: Props = $props();
</script>

<Tabs.Root value={active} onValueChange={(v) => onchange(v)} class="mb-4">
  <Tabs.List variant="line" class="w-full justify-start gap-1 border-b border-border">
    {#each tabs as tab}
      <Tabs.Trigger
        value={tab}
        class="text-[0.85rem] data-active:text-primary data-active:font-semibold data-active:after:bg-primary"
      >
        {tab}
      </Tabs.Trigger>
    {/each}
  </Tabs.List>
</Tabs.Root>
