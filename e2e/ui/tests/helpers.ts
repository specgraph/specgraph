import { type Page, type APIRequestContext, expect } from '@playwright/test';

const PROJECT = 'e2e-ui-test';
const BASE_URL = process.env.SPECGRAPH_BASE_URL ?? 'http://specgraph:9090';
const BASE_HEADERS = { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', 'X-Specgraph-Project': PROJECT };

// Seed helpers use raw HTTP POST with proto JSON field names (snake_case).
// Uses the full base URL since beforeAll pages don't inherit baseURL from config.
export async function seedSpec(request: APIRequestContext, slug: string, intent: string, priority = 'p2'): Promise<void> {
  const resp = await request.post(`${BASE_URL}/specgraph.v1.SpecService/CreateSpec`, {
    headers: BASE_HEADERS,
    data: { slug, intent, priority },
  });
  expect(resp.ok(), `seedSpec(${slug}) failed: ${resp.status()} ${await resp.text()}`).toBeTruthy();
}

export async function seedEdge(request: APIRequestContext, fromSlug: string, toSlug: string): Promise<void> {
  const resp = await request.post(`${BASE_URL}/specgraph.v1.GraphService/AddEdge`, {
    headers: BASE_HEADERS,
    data: { from_slug: fromSlug, to_slug: toSlug, edge_type: 'EDGE_TYPE_DEPENDS_ON' },
  });
  expect(resp.ok(), `seedEdge(${fromSlug}->${toSlug}) failed: ${resp.status()} ${await resp.text()}`).toBeTruthy();
}

export async function seedDecision(request: APIRequestContext, slug: string, title: string): Promise<void> {
  const resp = await request.post(`${BASE_URL}/specgraph.v1.DecisionService/CreateDecision`, {
    headers: BASE_HEADERS,
    data: { slug, title, decision: 'Test decision text', rationale: 'Test rationale' },
  });
  expect(resp.ok(), `seedDecision(${slug}) failed: ${resp.status()} ${await resp.text()}`).toBeTruthy();
}
