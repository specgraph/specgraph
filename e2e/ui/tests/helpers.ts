import { type APIRequestContext, expect } from '@playwright/test';

// Must match the project in web/src/lib/api/client.ts interceptor
const PROJECT = 'default';
const BASE_URL = process.env.SPECGRAPH_BASE_URL ?? 'http://specgraph:9090';
const E2E_API_KEY = 'spgr_sk_e2e00000000000000000000000000000';
const BASE_HEADERS = { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', 'X-Specgraph-Project': PROJECT, 'Authorization': `Bearer ${E2E_API_KEY}` };

// Retry wrapper for transient Memgraph transaction conflicts (500).
async function postWithRetry(request: APIRequestContext, url: string, data: object, label: string): Promise<void> {
  for (let attempt = 0; attempt < 3; attempt++) {
    const resp = await request.post(url, { headers: BASE_HEADERS, data });
    if (resp.ok() || resp.status() === 409) return; // 409 = already_exists, fine for idempotent seeding
    if (resp.status() === 500 && attempt < 2) {
      await new Promise((r) => setTimeout(r, 500)); // retry after brief pause
      continue;
    }
    expect.soft(false, `${label} failed: ${resp.status()} ${await resp.text()}`).toBeTruthy();
    return;
  }
}

export async function seedSpec(request: APIRequestContext, slug: string, intent: string, priority = 'p2'): Promise<void> {
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.SpecService/CreateSpec`, { slug, intent, priority }, `seedSpec(${slug})`);
}

export async function seedEdge(request: APIRequestContext, fromSlug: string, toSlug: string): Promise<void> {
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.GraphService/AddEdge`, { from_slug: fromSlug, to_slug: toSlug, edge_type: 'EDGE_TYPE_DEPENDS_ON' }, `seedEdge(${fromSlug}->${toSlug})`);
}

export async function seedDecision(request: APIRequestContext, slug: string, title: string): Promise<void> {
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.DecisionService/CreateDecision`, { slug, title, decision: 'Test decision text', rationale: 'Test rationale' }, `seedDecision(${slug})`);
}

export async function seedSparkOutput(request: APIRequestContext, slug: string): Promise<void> {
  // Spark RPC creates the spec AND stores spark output atomically.
  // Idempotent: if spec already exists (409), that's fine — spark output was stored on first call.
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.AuthoringService/Spark`, {
    slug,
    output: {
      seed: 'E2E test seed idea',
      signal: 'Strong test signal',
      scopeSniff: 'SCOPE_SNIFF_SMALL',
      killTest: 'No blockers found',
      questions: ['How should we test this?'],
    },
  }, `seedSparkOutput(${slug})`);
}

// advanceToDone transitions a spec (already created via CreateSpec/Spark) through
// the full authoring funnel: shape → specify → decompose → approved → done.
export async function advanceToDone(request: APIRequestContext, slug: string): Promise<void> {
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.AuthoringService/Shape`, {
    slug,
    output: {
      scopeIn: ['in-scope'],
      scopeOut: ['out-scope'],
      approaches: [{ name: 'default', description: 'test approach' }],
      chosenApproach: 'default',
    },
  }, `advanceToDone shape(${slug})`);

  await postWithRetry(request, `${BASE_URL}/specgraph.v1.AuthoringService/Specify`, {
    slug,
    output: {
      interfaces: [{ name: 'API', body: 'test' }],
      verifyCriteria: [{ description: 'passes' }],
    },
  }, `advanceToDone specify(${slug})`);

  await postWithRetry(request, `${BASE_URL}/specgraph.v1.AuthoringService/Decompose`, {
    slug,
    output: {
      strategy: 'DECOMPOSITION_STRATEGY_SINGLE_UNIT',
      slices: [{ id: 'main', intent: 'test' }],
    },
  }, `advanceToDone decompose(${slug})`);

  await postWithRetry(request, `${BASE_URL}/specgraph.v1.AuthoringService/Approve`, {
    slug,
  }, `advanceToDone approve(${slug})`);

  // Claim + complete to reach "done".
  const agent = 'e2e-ui-agent';
  await postWithRetry(request, `${BASE_URL}/specgraph.v1.ClaimService/ClaimSpec`, {
    specSlug: slug,
    agent,
  }, `advanceToDone claim(${slug})`);

  await postWithRetry(request, `${BASE_URL}/specgraph.v1.ExecutionService/ReportCompletion`, {
    slug,
    agent,
  }, `advanceToDone complete(${slug})`);
}

// amendSpec calls LifecycleService/TransitionAmend. reEntryStage is optional;
// omitting it transitions the spec directly to the "amended" terminal stage.
export async function amendSpec(
  request: APIRequestContext,
  slug: string,
  reason: string,
  reEntryStage?: string,
): Promise<void> {
  const body: Record<string, string> = { slug, reason };
  if (reEntryStage) {
    body['reEntryStage'] = reEntryStage;
  }
  await postWithRetry(
    request,
    `${BASE_URL}/specgraph.v1.LifecycleService/TransitionAmend`,
    body,
    `amendSpec(${slug})`,
  );
}

// supersedeSpec calls LifecycleService/TransitionSupersede, replacing oldSlug
// with newSlug. The new spec must already exist.
export async function supersedeSpec(
  request: APIRequestContext,
  oldSlug: string,
  newSlug: string,
): Promise<void> {
  await postWithRetry(
    request,
    `${BASE_URL}/specgraph.v1.LifecycleService/TransitionSupersede`,
    { slug: oldSlug, newSlug },
    `supersedeSpec(${oldSlug}->${newSlug})`,
  );
}

// updateSpecIntent calls SpecService/UpdateSpec to change the intent field,
// which creates a new changelog entry and bumps the version.
export async function updateSpecIntent(
  request: APIRequestContext,
  slug: string,
  intent: string,
): Promise<void> {
  await postWithRetry(
    request,
    `${BASE_URL}/specgraph.v1.SpecService/UpdateSpec`,
    { slug, intent },
    `updateSpecIntent(${slug})`,
  );
}
