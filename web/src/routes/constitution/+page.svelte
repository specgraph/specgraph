<script lang="ts">
  import { constitutionClient } from '$lib/api/client';
  import type { Constitution } from '$lib/api/gen/specgraph/v1/constitution_pb';
  import { ConstitutionLayer, ReferenceType } from '$lib/api/gen/specgraph/v1/constitution_pb';
  import AccordionSection from '$lib/components/AccordionSection.svelte';

  let constitution = $state<Constitution | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  async function load() {
    loading = true;
    error = null;
    try {
      const resp = await constitutionClient.getConstitution({});
      constitution = resp.constitution ?? null;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load constitution';
    } finally {
      loading = false;
    }
  }

  $effect(() => { load(); });

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

  function mapEntries(m: { [key: string]: string }): [string, string][] {
    return Object.entries(m).sort(([a], [b]) => a.localeCompare(b));
  }
</script>

<nav class="breadcrumb">
  <a href="/">Dashboard</a> / <span>Constitution</span>
</nav>

{#if loading}
  <p class="status">Loading...</p>
{:else if error}
  <p class="status error">{error}</p>
{:else if constitution}
  <h1>{constitution.name || 'Constitution'}</h1>

  <table class="meta">
    <tbody>
      <tr><td class="label">Layer</td><td><span class="badge">{layerLabel(constitution.layer)}</span></td></tr>
      <tr><td class="label">Version</td><td>{constitution.version}</td></tr>
    </tbody>
  </table>

  <div class="sections">
    {#if constitution.tech}
      <AccordionSection title="Tech Stack" expanded={true}>
        {#if constitution.tech.languages}
          <h3>Languages</h3>
          {#if constitution.tech.languages.primary}
            <p><strong>Primary:</strong> {constitution.tech.languages.primary}</p>
          {/if}
          {#if constitution.tech.languages.allowed.length > 0}
            <p><strong>Allowed:</strong> {constitution.tech.languages.allowed.join(', ')}</p>
          {/if}
          {#if constitution.tech.languages.forbidden.length > 0}
            <p><strong>Forbidden:</strong></p>
            <ul>
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
          <h3>Frameworks</h3>
          <table class="detail-table">
            <thead><tr><th>Area</th><th>Choice</th></tr></thead>
            <tbody>
              {#each mapEntries(constitution.tech.frameworks) as [area, choice]}
                <tr><td>{area}</td><td>{choice}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
        {#if mapEntries(constitution.tech.infrastructure).length > 0}
          <h3>Infrastructure</h3>
          <table class="detail-table">
            <thead><tr><th>Area</th><th>Choice</th></tr></thead>
            <tbody>
              {#each mapEntries(constitution.tech.infrastructure) as [area, choice]}
                <tr><td>{area}</td><td>{choice}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
        {#if mapEntries(constitution.tech.apiStandards).length > 0}
          <h3>API Standards</h3>
          <table class="detail-table">
            <thead><tr><th>Area</th><th>Standard</th></tr></thead>
            <tbody>
              {#each mapEntries(constitution.tech.apiStandards) as [area, standard]}
                <tr><td>{area}</td><td>{standard}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
        {#if mapEntries(constitution.tech.data).length > 0}
          <h3>Data</h3>
          <table class="detail-table">
            <thead><tr><th>Area</th><th>Store</th></tr></thead>
            <tbody>
              {#each mapEntries(constitution.tech.data) as [area, store]}
                <tr><td>{area}</td><td>{store}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </AccordionSection>
    {/if}

    {#if constitution.principles.length > 0}
      <AccordionSection title="Principles" badge={String(constitution.principles.length)} expanded={true}>
        {#each constitution.principles as p}
          <div class="principle-card">
            <strong>{p.id}: {p.statement}</strong>
            {#if p.rationale}<p class="detail">{p.rationale}</p>{/if}
            {#if p.exceptions}<p class="detail"><em>Exceptions:</em> {p.exceptions}</p>{/if}
          </div>
        {/each}
      </AccordionSection>
    {/if}

    {#if constitution.constraints.length > 0}
      <AccordionSection title="Constraints" badge={String(constitution.constraints.length)}>
        <ul>
          {#each constitution.constraints as c}
            <li>{c}</li>
          {/each}
        </ul>
      </AccordionSection>
    {/if}

    {#if constitution.antipatterns.length > 0}
      <AccordionSection title="Antipatterns" badge={String(constitution.antipatterns.length)}>
        {#each constitution.antipatterns as ap}
          <div class="antipattern-card">
            <strong>{ap.pattern}</strong>
            {#if ap.why}<p class="detail"><em>Why:</em> {ap.why}</p>{/if}
            {#if ap.instead}<p class="detail"><em>Instead:</em> {ap.instead}</p>{/if}
          </div>
        {/each}
      </AccordionSection>
    {/if}

    {#if constitution.process}
      <AccordionSection title="Process">
        {#if constitution.process.specReview}
          <p><strong>Spec Review:</strong> {constitution.process.specReview}</p>
        {/if}
        {#if constitution.process.securityReview?.when}
          <p><strong>Security Review:</strong> {constitution.process.securityReview.when}</p>
        {/if}
        {#if constitution.process.deployment}
          <h3>Deployment</h3>
          {#if constitution.process.deployment.strategy}
            <p><strong>Strategy:</strong> {constitution.process.deployment.strategy}</p>
          {/if}
          {#if constitution.process.deployment.rollback}
            <p><strong>Rollback:</strong> {constitution.process.deployment.rollback}</p>
          {/if}
        {/if}
        {#if constitution.process.documentation}
          <h3>Documentation</h3>
          {#if constitution.process.documentation.apiDocs}
            <p><strong>API Docs:</strong> {constitution.process.documentation.apiDocs}</p>
          {/if}
          {#if constitution.process.documentation.runbook}
            <p><strong>Runbook:</strong> {constitution.process.documentation.runbook}</p>
          {/if}
        {/if}
      </AccordionSection>
    {/if}

    {#if constitution.references.length > 0}
      <AccordionSection title="References" badge={String(constitution.references.length)}>
        <ul>
          {#each constitution.references as ref}
            <li>
              <span class="ref-type">{refTypeLabel(ref.referenceType)}</span>
              {ref.path}
            </li>
          {/each}
        </ul>
      </AccordionSection>
    {/if}
  </div>
{:else}
  <p class="status">No constitution found for this project.</p>
{/if}

<style>
  .breadcrumb {
    font-size: 0.85rem;
    color: #64748b;
    margin-bottom: 1.25rem;
  }

  .breadcrumb a {
    color: #2563eb;
    text-decoration: none;
  }

  .breadcrumb a:hover {
    text-decoration: underline;
  }

  .breadcrumb span {
    color: #1a1a2e;
    font-weight: 500;
  }

  h1 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0 0 1rem;
    color: #1a1a2e;
  }

  .status {
    color: #64748b;
    font-size: 0.95rem;
  }

  .status.error {
    color: #dc2626;
  }

  .meta {
    border-collapse: collapse;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .meta td {
    padding: 0.4rem 1rem 0.4rem 0;
    vertical-align: top;
  }

  .meta .label {
    color: #64748b;
    font-weight: 500;
    white-space: nowrap;
    min-width: 8rem;
  }

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
    background: #f1f5f9;
    color: #475569;
  }

  .sections {
    margin-top: 1rem;
  }

  h3 {
    font-size: 0.9rem;
    font-weight: 600;
    color: #374151;
    margin: 0.75rem 0 0.25rem;
  }

  ul {
    margin: 0.25rem 0 0.5rem;
    padding-left: 1.25rem;
  }

  li {
    font-size: 0.9rem;
    margin-bottom: 0.15rem;
  }

  .detail-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    margin: 0.25rem 0 0.5rem;
  }

  .detail-table th {
    text-align: left;
    padding: 0.3rem 0.5rem;
    background: #f1f5f9;
    color: #475569;
    font-weight: 600;
  }

  .detail-table td {
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid #f1f5f9;
  }

  .principle-card, .antipattern-card {
    padding: 0.5rem;
    margin: 0.25rem 0;
    background: #f8fafc;
    border-radius: 4px;
    border-left: 3px solid #2563eb;
  }

  .antipattern-card {
    border-left-color: #dc2626;
  }

  .detail {
    font-size: 0.85rem;
    color: #64748b;
    margin: 0.25rem 0 0;
  }

  .ref-type {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 600;
    background: #e2e8f0;
    color: #475569;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    margin-right: 0.4rem;
  }
</style>
