<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import type { Constitution, ProvenanceEntry } from '$lib/api/gen/specgraph/v1/constitution_pb';
  import { ConstitutionLayer, ReferenceType } from '$lib/api/gen/specgraph/v1/constitution_pb';
  import AccordionSection from '$lib/components/AccordionSection.svelte';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { layerBadgeClass } from '$lib/components/badge-variants';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // The load streams constitution/provenance/loadError as separate promises off
  // one RPC. Combine them in a $derived so {#await} re-suspends to the Skeleton on
  // every invalidateAll() (a stable derived promise avoids re-suspending on
  // unrelated re-renders). Badges re-derive from the reloaded data.provenance —
  // there is NO local $state provenance, so a switch can never show stale
  // prior-project badges/sections (D-10 / Pitfall 4 / T-05-05).
  let view = $derived(
    Promise.all([data.constitution, data.provenance, data.loadError]).then(
      ([constitution, provenance, loadError]) => ({ constitution, provenance, loadError }),
    ),
  );

  function layerLabel(layer: ConstitutionLayer): string {
    const labels: Record<number, string> = {
      [ConstitutionLayer.USER]: 'User',
      [ConstitutionLayer.ORG]: 'Organization',
      [ConstitutionLayer.PROJECT]: 'Project',
      [ConstitutionLayer.DOMAIN]: 'Domain',
    };
    return labels[layer] ?? 'Unknown';
  }

  function refTypeLabel(rt: ReferenceType): string {
    const labels: Record<number, string> = {
      [ReferenceType.ADR]: 'ADR',
      [ReferenceType.SPEC]: 'Spec',
      [ReferenceType.DOC]: 'Doc',
      [ReferenceType.URL]: 'URL',
    };
    return labels[rt] ?? 'Ref';
  }

  function mapEntries(m: { [key: string]: string } | undefined): [string, string][] {
    if (!m) return [];
    return Object.entries(m).sort(([a], [b]) => a.localeCompare(b));
  }

  // Categorical layer key (lowercase) derived from the reloaded provenance array.
  function layerOf(path: string, provenance: ProvenanceEntry[]): string {
    const entry = provenance.find((p) => p.path === path);
    if (!entry) return '';
    const labels: Record<number, string> = { 1: 'user', 2: 'org', 3: 'project', 4: 'domain' };
    return labels[entry.layer] ?? '';
  }
</script>

{#snippet layerBadge(path: string, provenance: ProvenanceEntry[])}
  {@const l = layerOf(path, provenance)}
  {#if l}
    <Badge class={layerBadgeClass(l) + ' ml-1.5 text-[0.65rem] uppercase tracking-wide'}>{l}</Badge>
  {/if}
{/snippet}

{#await view}
  <!-- Loading: Skeleton meta + section rows (State Matrix). The streamed promise
       re-suspends here on invalidateAll() so a switch returns to skeletons with no
       stale previous-project badges/sections (Pitfall 3, T-05-05). -->
  <Skeleton class="mb-4 h-6 w-48" />
  <Skeleton class="mb-2 h-4 w-40" />
  <Skeleton class="mb-6 h-4 w-24" />
  <div class="space-y-2">
    <Skeleton class="h-10 w-full" />
    <Skeleton class="h-10 w-full" />
    <Skeleton class="h-10 w-full" />
  </div>
{:then d}
  {#if d.loadError}
    <!-- Error: inline Retry card (do not reach +error.svelte, T-05-15). -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>Couldn't load constitution.</Card.Title>
        <Card.Description>Check your connection and try again.</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Button variant="outline" onclick={() => invalidateAll()}>Retry</Button>
      </Card.Footer>
    </Card.Root>
  {:else if d.constitution}
    {@const constitution = d.constitution}
    <h1 class="mb-4 text-xl font-semibold text-foreground">{constitution.name || 'Constitution'}</h1>

    <table class="mb-5 border-collapse text-sm">
      <tbody>
        {#if d.provenance.length > 0}
          <tr>
            <td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">View</td>
            <td class="py-1.5 pr-4 align-top"><Badge variant="secondary">Merged</Badge></td>
          </tr>
        {:else}
          <tr>
            <td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Layer</td>
            <td class="py-1.5 pr-4 align-top"><Badge variant="secondary">{layerLabel(constitution.layer)}</Badge></td>
          </tr>
          <tr>
            <td class="min-w-32 whitespace-nowrap py-1.5 pr-4 align-top font-medium text-muted-foreground">Version</td>
            <td class="py-1.5 pr-4 align-top">{constitution.version}</td>
          </tr>
        {/if}
      </tbody>
    </table>

    <div class="mt-4">
      {#if constitution.tech}
        <AccordionSection title="Tech Stack" expanded={true}>
          {#if constitution.tech.languages}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Languages</h3>
            {#if constitution.tech.languages.primary}
              <p><strong>Primary:</strong> {constitution.tech.languages.primary}</p>
            {/if}
            {#if constitution.tech.languages.allowed.length > 0}
              <p><strong>Allowed:</strong> {constitution.tech.languages.allowed.join(', ')}</p>
            {/if}
            {#if constitution.tech.languages.forbidden.length > 0}
              <p><strong>Forbidden:</strong></p>
              <ul class="my-1 ml-5 list-disc space-y-0.5">
                {#each constitution.tech.languages.forbidden as lang}
                  <li>
                    {lang}
                    {#if constitution.tech?.languages?.forbiddenReasons[lang]}
                      — {constitution.tech.languages.forbiddenReasons[lang]}
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}
          {/if}
          {#if mapEntries(constitution.tech.frameworks).length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Frameworks</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Area</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Choice</th></tr></thead>
              <tbody>
                {#each mapEntries(constitution.tech.frameworks) as [area, choice]}
                  <tr><td class="border-b border-border px-2 py-1.5">{area}</td><td class="border-b border-border px-2 py-1.5">{choice}{@render layerBadge("tech_config.frameworks[" + area + "]", d.provenance)}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
          {#if mapEntries(constitution.tech.infrastructure).length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Infrastructure</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Area</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Choice</th></tr></thead>
              <tbody>
                {#each mapEntries(constitution.tech.infrastructure) as [area, choice]}
                  <tr><td class="border-b border-border px-2 py-1.5">{area}</td><td class="border-b border-border px-2 py-1.5">{choice}{@render layerBadge("tech_config.infrastructure[" + area + "]", d.provenance)}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
          {#if mapEntries(constitution.tech.apiStandards).length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">API Standards</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Area</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Standard</th></tr></thead>
              <tbody>
                {#each mapEntries(constitution.tech.apiStandards) as [area, standard]}
                  <tr><td class="border-b border-border px-2 py-1.5">{area} {@render layerBadge("tech_config.api_standards[" + area + "]", d.provenance)}</td><td class="border-b border-border px-2 py-1.5">{standard}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
          {#if mapEntries(constitution.tech.data).length > 0}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Data</h3>
            <table class="my-1 w-full border-collapse text-sm">
              <thead><tr><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Area</th><th class="bg-muted px-2 py-1.5 text-left font-semibold text-muted-foreground">Store</th></tr></thead>
              <tbody>
                {#each mapEntries(constitution.tech.data) as [area, store]}
                  <tr><td class="border-b border-border px-2 py-1.5">{area} {@render layerBadge("tech_config.data[" + area + "]", d.provenance)}</td><td class="border-b border-border px-2 py-1.5">{store}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </AccordionSection>
      {/if}

      {#if constitution.principles.length > 0}
        <AccordionSection title="Principles" badge={String(constitution.principles.length)} expanded={true}>
          {#each constitution.principles as p}
            <div class="my-1 rounded border-l-2 border-primary bg-muted/50 p-2">
              <strong>{p.id}: {p.statement}</strong>{@render layerBadge("principles[" + p.id + "]", d.provenance)}
              {#if p.rationale}<p class="mt-1 text-sm text-muted-foreground">{p.rationale}</p>{/if}
              {#if p.exceptions}<p class="mt-1 text-sm text-muted-foreground"><em>Exceptions:</em> {p.exceptions}</p>{/if}
            </div>
          {/each}
        </AccordionSection>
      {/if}

      {#if constitution.constraints.length > 0}
        <AccordionSection title="Constraints" badge={String(constitution.constraints.length)}>
          <ul class="my-1 ml-5 list-disc space-y-0.5">
            {#each constitution.constraints as c}
              <li>{c}{@render layerBadge("constraints[" + c + "]", d.provenance)}</li>
            {/each}
          </ul>
        </AccordionSection>
      {/if}

      {#if constitution.antipatterns.length > 0}
        <AccordionSection title="Antipatterns" badge={String(constitution.antipatterns.length)}>
          {#each constitution.antipatterns as ap}
            <div class="my-1 rounded border-l-2 border-destructive bg-muted/50 p-2">
              <strong>{ap.pattern}</strong>{@render layerBadge("antipatterns[" + ap.pattern + "]", d.provenance)}
              {#if ap.why}<p class="mt-1 text-sm text-muted-foreground"><em>Why:</em> {ap.why}</p>{/if}
              {#if ap.instead}<p class="mt-1 text-sm text-muted-foreground"><em>Instead:</em> {ap.instead}</p>{/if}
            </div>
          {/each}
        </AccordionSection>
      {/if}

      {#if constitution.process}
        <AccordionSection title="Process">
          {#if constitution.process.specReview}
            <p><strong>Spec Review:</strong> {constitution.process.specReview}{@render layerBadge("process.spec_review", d.provenance)}</p>
          {/if}
          {#if constitution.process.securityReview?.when}
            <p><strong>Security Review:</strong> {constitution.process.securityReview.when}{@render layerBadge("process.security_review.when", d.provenance)}</p>
          {/if}
          {#if constitution.process.deployment}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Deployment</h3>
            {#if constitution.process.deployment.strategy}
              <p><strong>Strategy:</strong> {constitution.process.deployment.strategy}{@render layerBadge("process.deployment.strategy", d.provenance)}</p>
            {/if}
            {#if constitution.process.deployment.rollback}
              <p><strong>Rollback:</strong> {constitution.process.deployment.rollback}{@render layerBadge("process.deployment.rollback", d.provenance)}</p>
            {/if}
          {/if}
          {#if constitution.process.documentation}
            <h3 class="mb-1 mt-3 text-sm font-semibold text-foreground">Documentation</h3>
            {#if constitution.process.documentation.apiDocs}
              <p><strong>API Docs:</strong> {constitution.process.documentation.apiDocs}{@render layerBadge("process.documentation.api_docs", d.provenance)}</p>
            {/if}
            {#if constitution.process.documentation.runbook}
              <p><strong>Runbook:</strong> {constitution.process.documentation.runbook}{@render layerBadge("process.documentation.runbook", d.provenance)}</p>
            {/if}
          {/if}
        </AccordionSection>
      {/if}

      {#if constitution.references.length > 0}
        <AccordionSection title="References" badge={String(constitution.references.length)}>
          <ul class="my-1 ml-5 list-disc space-y-0.5">
            {#each constitution.references as ref}
              <li>
                <Badge variant="outline" class="mr-1.5">{refTypeLabel(ref.referenceType)}</Badge>
                {ref.path}{@render layerBadge("references[" + ref.path + "]", d.provenance)}
              </li>
            {/each}
          </ul>
        </AccordionSection>
      {/if}
    </div>
  {:else}
    <!-- Empty: this project has no constitution (UI-SPEC D-10 copy). No stale
         badges/sections linger because everything is derived from d.provenance. -->
    <Card.Root class="max-w-md">
      <Card.Header>
        <Card.Title>No constitution for this project</Card.Title>
        <Card.Description>
          This project doesn't have a constitution yet. Define one via the CLI, then switch back to view it here.
        </Card.Description>
      </Card.Header>
    </Card.Root>
  {/if}
{/await}
