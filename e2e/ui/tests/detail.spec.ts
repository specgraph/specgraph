import { test, expect, request as playwrightRequest } from './fixtures';
import { seedSpec, seedDecision, seedEdge, seedSparkOutput } from './helpers';

test.describe('Detail Pages', () => {
  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, 'detail-spec', 'Detail test spec', 'p1');
    await seedDecision(request, 'detail-dec', 'Detail test decision');
    await request.dispose();
  });

  test('spec detail page shows metadata', async ({ page }) => {
    await page.goto('/spec/detail-spec');
    await expect(page.locator('h1')).toContainText('detail-spec');
    await expect(page.locator('.meta')).toContainText('Detail test spec');
    await expect(page.locator('.meta')).toContainText('p1');
    await expect(page.locator('.breadcrumb')).toContainText('Dashboard');
    await expect(page.locator('.breadcrumb')).toContainText('Graph');
  });

  test('decision detail page shows metadata', async ({ page }) => {
    await page.goto('/decision/detail-dec');
    await expect(page.locator('h1')).toContainText('Detail test decision');
    await expect(page.locator('.meta')).toContainText('detail-dec');
  });

  test('spec detail 404 shows error', async ({ page }) => {
    await page.goto('/spec/nonexistent-slug-xyz');
    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
  });
});

test.describe('Stage Data Accordions', () => {
  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    // Spark RPC creates the spec AND stores spark output in one call.
    // Do NOT call seedSpec first — it would cause AlreadyExists on Spark.
    await seedSparkOutput(request, 'detail-stage-e2e');
    await request.dispose();
  });

  test('spec detail page shows accordion sections for stage data', async ({ page }) => {
    await page.goto('/spec/detail-stage-e2e');
    await page.waitForLoadState('networkidle');

    const sparkHeader = page.locator('.accordion-header', { hasText: 'Spark' });
    await expect(sparkHeader).toBeVisible();
    await expect(page.locator('.sections')).toContainText('E2E test seed idea');
  });

  test('accordion expand/collapse works', async ({ page }) => {
    await page.goto('/spec/detail-stage-e2e');
    await page.waitForLoadState('networkidle');

    const sparkHeader = page.locator('.accordion-header', { hasText: 'Spark' });
    await expect(sparkHeader).toBeVisible();

    // The accordion body contains the seed text when expanded
    const sparkBody = sparkHeader.locator('..').locator('.accordion-body');

    // If the accordion is expanded (body visible), click to collapse
    if (await sparkBody.isVisible()) {
      await sparkHeader.click();
      await expect(sparkBody).not.toBeVisible();
      // Re-expand
      await sparkHeader.click();
      await expect(sparkBody).toBeVisible();
    } else {
      // Accordion starts collapsed — click to expand
      await sparkHeader.click();
      await expect(sparkBody).toBeVisible();
      // Collapse again
      await sparkHeader.click();
      await expect(sparkBody).not.toBeVisible();
    }
  });
});

test.describe('Edges Section', () => {
  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, 'edge-source-e2e', 'Edge source spec');
    await seedSpec(request, 'edge-target-e2e', 'Edge target spec');
    await seedEdge(request, 'edge-source-e2e', 'edge-target-e2e');
    await request.dispose();
  });

  test('edges section shows when edges exist', async ({ page }) => {
    await page.goto('/spec/edge-source-e2e');
    await page.waitForLoadState('networkidle');

    const edgesHeader = page.locator('.accordion-header', { hasText: 'Edges' });
    await expect(edgesHeader).toBeVisible();
    // Edges accordion is collapsed by default — click to expand
    await edgesHeader.click();
    const edgesBody = edgesHeader.locator('..').locator('.accordion-body');
    await expect(edgesBody).toBeVisible();
    await expect(edgesBody).toContainText('edge-target-e2e');
  });
});
