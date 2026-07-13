<script lang="ts">
  import { Input } from '$lib/components/ui/input/index.js';

  interface Props {
    value: string;
    placeholder?: string;
    onchange: (value: string) => void;
  }

  let { value, placeholder = 'Filter specs\u2026', onchange }: Props = $props();
</script>

<div class="flex">
  <Input
    type="text"
    {placeholder}
    {value}
    class="w-full"
    oninput={(e) => onchange(e.currentTarget.value)}
  />
</div>
