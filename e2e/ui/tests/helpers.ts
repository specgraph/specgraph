import { type APIRequestContext, expect } from '@playwright/test';

// Must match the project in web/src/lib/api/client.ts interceptor
const PROJECT = 'default';
const BASE_URL = process.env.SPECGRAPH_BASE_URL ?? 'http://specgraph:9090';
const BASE_HEADERS = { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', 'X-Specgraph-Project': PROJECT };

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
