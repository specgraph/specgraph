<script lang="ts">
  import type { AnalyticalFinding } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
  import { PassType } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
  import { FindingSeverity } from '$lib/api/gen/specgraph/v1/authoring_pb';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { severityBadgeClass } from './badge-variants';

  interface Props {
    findings: AnalyticalFinding[];
  }
  let { findings }: Props = $props();

  function passTypeLabel(pt: PassType): string {
    const labels: Record<number, string> = {
      [PassType.CONSTITUTION_CHECK]: 'Constitution Check',
      [PassType.RED_TEAM]: 'Red Team',
      [PassType.PERIPHERAL_VISION]: 'Peripheral Vision',
      [PassType.CONSISTENCY]: 'Consistency',
      [PassType.SIMPLICITY]: 'Simplicity',
    };
    return labels[pt] ?? `Pass ${pt}`;
  }

  // Maps a finding severity to a categorical badge-palette key (D-10). These
  // are CATEGORICAL data encodings pulled from the shared severity map, NOT the
  // theme accent — see badge-variants.ts.
  function severityKey(s: FindingSeverity): 'info' | 'warning' | 'error' {
    switch (s) {
      case FindingSeverity.NOTE: return 'info';
      case FindingSeverity.WARNING: return 'warning';
      case FindingSeverity.CRITICAL: return 'error';
      default: return 'info';
    }
  }

  function severityLabel(s: FindingSeverity): string {
    switch (s) {
      case FindingSeverity.NOTE: return 'Note';
      case FindingSeverity.WARNING: return 'Warning';
      case FindingSeverity.CRITICAL: return 'Critical';
      default: return 'Note';
    }
  }

  interface GroupedPass {
    passType: PassType;
    label: string;
    findings: AnalyticalFinding[];
  }

  const allPassTypes = [
    PassType.CONSTITUTION_CHECK,
    PassType.RED_TEAM,
    PassType.PERIPHERAL_VISION,
    PassType.CONSISTENCY,
    PassType.SIMPLICITY,
  ];

  let grouped = $derived(
    Object.values(
      findings.reduce((acc, f) => {
        const key = f.passType;
        if (!acc[key]) acc[key] = { passType: key, label: passTypeLabel(key), findings: [] };
        acc[key].findings.push(f);
        return acc;
      }, Object.fromEntries(
        allPassTypes.map(pt => [pt, { passType: pt, label: passTypeLabel(pt), findings: [] }])
      ) as Record<number, GroupedPass>)
    )
  );
</script>

{#each grouped as group}
  <Card.Root
    size="sm"
    class="mb-2 rounded-md border-l-4 {group.findings.length > 0
      ? 'border-l-amber-500'
      : 'border-l-green-500'}"
  >
    <Card.Header class="flex-row items-center justify-between space-y-0">
      <Card.Title class="text-sm font-semibold">{group.label}</Card.Title>
      {#if group.findings.length > 0}
        <Badge class={severityBadgeClass('warning')}>
          {group.findings.length} finding{group.findings.length > 1 ? 's' : ''}
        </Badge>
      {:else}
        <Badge class="bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300">
          passed
        </Badge>
      {/if}
    </Card.Header>
    {#if group.findings.length > 0}
      <Card.Content class="space-y-1.5">
        {#each group.findings as finding}
          <div class="flex flex-wrap items-baseline gap-2 text-[0.8rem]">
            <Badge class={severityBadgeClass(severityKey(finding.severity))}>
              {severityLabel(finding.severity)}
            </Badge>
            <span class="text-foreground">{finding.summary}</span>
            {#if finding.resolution}
              <span class="text-muted-foreground text-[0.75rem]">→ {finding.resolution}</span>
            {/if}
          </div>
        {/each}
      </Card.Content>
    {/if}
  </Card.Root>
{/each}
