<script lang="ts">
  import type { AnalyticalFinding } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
  import { PassType } from '$lib/api/gen/specgraph/v1/analytical_pass_pb';
  import { FindingSeverity } from '$lib/api/gen/specgraph/v1/authoring_pb';

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

  function severityClass(s: FindingSeverity): string {
    switch (s) {
      case FindingSeverity.NOTE: return 'info';
      case FindingSeverity.WARNING: return 'warning';
      case FindingSeverity.CRITICAL: return 'error';
      default: return 'info';
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
  <div class="pass-card {group.findings.length > 0 ? 'has-findings' : 'passed'}">
    <div class="pass-header">
      <strong>{group.label}</strong>
      <span class="count-badge {group.findings.length > 0 ? 'amber' : 'green'}">
        {group.findings.length > 0 ? `${group.findings.length} finding${group.findings.length > 1 ? 's' : ''}` : 'passed'}
      </span>
    </div>
    {#if group.findings.length > 0}
      <div class="findings-list">
        {#each group.findings as finding}
          <div class="finding {severityClass(finding.severity)}">
            <span class="finding-summary">{finding.summary}</span>
            {#if finding.resolution}
              <span class="finding-resolution">→ {finding.resolution}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/each}

<style>
  .pass-card {
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.5rem;
    border-radius: 0 4px 4px 0;
  }

  .pass-card.has-findings {
    border-left: 3px solid #f59e0b;
    background: #fffbeb;
  }

  .pass-card.passed {
    border-left: 3px solid #22c55e;
    background: #f0fdf4;
  }

  .pass-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .count-badge {
    font-size: 0.7rem;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    font-weight: 600;
  }

  .count-badge.amber {
    background: #fef3c7;
    color: #b45309;
  }

  .count-badge.green {
    background: #dcfce7;
    color: #16a34a;
  }

  .findings-list {
    margin-top: 0.35rem;
  }

  .finding {
    font-size: 0.8rem;
    padding: 0.15rem 0;
    color: #92400e;
  }

  .finding.error {
    color: #dc2626;
  }

  .finding.info {
    color: #475569;
  }

  .finding-summary::before {
    content: '⚠ ';
  }

  .finding.error .finding-summary::before {
    content: '✘ ';
  }

  .finding.info .finding-summary::before {
    content: 'ℹ ';
  }

  .finding-resolution {
    font-size: 0.75rem;
    color: #64748b;
    margin-left: 0.25rem;
  }
</style>
