import { type Page, expect } from '@playwright/test';

const PROJECT = 'e2e-ui-test';
const BASE_HEADERS = { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', 'X-Specgraph-Project': PROJECT };

// Seed helpers use raw HTTP POST with proto JSON field names (snake_case).
export async function seedSpec(page: Page, slug: string, intent: string, priority = 'p2'): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.SpecService/CreateSpec', {
    headers: BASE_HEADERS,
    data: { slug, intent, priority },
  });
  expect(resp.ok()).toBeTruthy();
}

export async function seedEdge(page: Page, fromSlug: string, toSlug: string): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.GraphService/AddEdge', {
    headers: BASE_HEADERS,
    data: { from_slug: fromSlug, to_slug: toSlug, edge_type: 'EDGE_TYPE_DEPENDS_ON' },
  });
  expect(resp.ok()).toBeTruthy();
}

export async function seedDecision(page: Page, slug: string, title: string): Promise<void> {
  const resp = await page.request.post('/specgraph.v1.DecisionService/CreateDecision', {
    headers: BASE_HEADERS,
    data: { slug, title, decision: 'Test decision text', rationale: 'Test rationale' },
  });
  expect(resp.ok()).toBeTruthy();
}
