// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

import { test, expect, request as playwrightRequest } from './fixtures';
import {
  seedSpec,
  advanceToDone,
  amendSpec,
  supersedeSpec,
  updateSpecIntent,
} from './helpers';

// ---------------------------------------------------------------------------
// Amendment tests
// ---------------------------------------------------------------------------

test.describe('Amendment', () => {
  const slug = 'lc-amend-e2e';

  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, slug, 'Lifecycle amend E2E test spec', 'p2');
    await advanceToDone(request, slug);
    await amendSpec(request, slug, 'Requirements changed after implementation', 'shape');
    await request.dispose();
  });

  test('stage badge updates after amend', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');
    // After amend with re-entry stage "shape", the spec should be at "shape".
    await expect(page.locator('.badge.stage-shape')).toBeVisible({ timeout: 10_000 });
  });

  test('changelog accordion shows entries', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');

    // Open the changelog accordion via its "Load changelog" button.
    const changelogHeader = page.locator('.accordion-header', { hasText: 'Changelog' });
    await expect(changelogHeader).toBeVisible({ timeout: 10_000 });
    await changelogHeader.click();

    // The "Load changelog" button may appear inside the accordion body.
    const loadBtn = page.getByRole('button', { name: 'Load changelog' });
    if (await loadBtn.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await loadBtn.click();
    }

    // At least one timeline entry should appear.
    await expect(page.locator('.timeline-entry').first()).toBeVisible({ timeout: 10_000 });
  });

  test('clicking a changelog entry expands diff view', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');

    const changelogHeader = page.locator('.accordion-header', { hasText: 'Changelog' });
    await changelogHeader.click();

    const loadBtn = page.getByRole('button', { name: 'Load changelog' });
    if (await loadBtn.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await loadBtn.click();
    }

    await expect(page.locator('.timeline-entry').first()).toBeVisible({ timeout: 10_000 });

    // Click the first non-checkpoint entry (timeline-card) to expand a diff.
    const firstCard = page.locator('.timeline-card').first();
    await expect(firstCard).toBeVisible({ timeout: 10_000 });
    await firstCard.click();

    // A diff panel or inline diff should now be visible.
    await expect(
      page.locator('.compare-result, .diff-view, .version-diff').first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});

// ---------------------------------------------------------------------------
// Supersede tests
// ---------------------------------------------------------------------------

test.describe('Supersede', () => {
  const oldSlug = 'lc-supersede-old-e2e';
  const newSlug = 'lc-supersede-new-e2e';

  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    // Old spec: create, advance to done, then supersede.
    await seedSpec(request, oldSlug, 'Lifecycle supersede old spec', 'p2');
    await advanceToDone(request, oldSlug);
    // New spec: create (spark stage is fine for supersede target).
    await seedSpec(request, newSlug, 'Lifecycle supersede new spec', 'p2');
    await supersedeSpec(request, oldSlug, newSlug);
    await request.dispose();
  });

  test('old spec shows superseded-by banner with link', async ({ page }) => {
    await page.goto(`/spec/${oldSlug}`);
    await page.waitForLoadState('networkidle');

    const banner = page.locator('.superseded-banner');
    await expect(banner).toBeVisible({ timeout: 10_000 });
    await expect(banner).toContainText(newSlug);

    // The banner should contain a working link to the new spec.
    const link = banner.locator('a');
    await expect(link).toBeVisible();
    const href = await link.getAttribute('href');
    expect(href).toContain(newSlug);
  });

  test('new spec shows supersedes banner with link', async ({ page }) => {
    await page.goto(`/spec/${newSlug}`);
    await page.waitForLoadState('networkidle');

    const banner = page.locator('.supersedes-banner');
    await expect(banner).toBeVisible({ timeout: 10_000 });
    await expect(banner).toContainText(oldSlug);

    const link = banner.locator('a');
    await expect(link).toBeVisible();
    const href = await link.getAttribute('href');
    expect(href).toContain(oldSlug);
  });
});

// ---------------------------------------------------------------------------
// Version Compare tests
// ---------------------------------------------------------------------------

test.describe('Version Compare', () => {
  const slug = 'lc-version-cmp-e2e';

  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    // Create spec (version 1), then update intent to produce version 2+.
    await seedSpec(request, slug, 'Version compare initial intent', 'p2');
    await updateSpecIntent(request, slug, 'Version compare updated intent');
    await request.dispose();
  });

  test('version picker compare produces diff panel', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');

    // Open changelog accordion.
    const changelogHeader = page.locator('.accordion-header', { hasText: 'Changelog' });
    await expect(changelogHeader).toBeVisible({ timeout: 10_000 });
    await changelogHeader.click();

    const loadBtn = page.getByRole('button', { name: 'Load changelog' });
    if (await loadBtn.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await loadBtn.click();
    }

    await expect(page.locator('.timeline-entry').first()).toBeVisible({ timeout: 10_000 });

    // Select version 1 in the "From" dropdown to trigger a real comparison.
    const fromSelect = page.locator('.compare-label select').first();
    await expect(fromSelect).toBeVisible({ timeout: 5_000 });
    await fromSelect.selectOption({ index: 1 }); // first real version after "auto"

    const compareBtn = page.locator('.compare-btn').first();
    await expect(compareBtn).toBeVisible({ timeout: 3_000 });
    await compareBtn.click();
    await expect(page.locator('.compare-result')).toBeVisible({ timeout: 10_000 });
  });
});

// ---------------------------------------------------------------------------
// Dashboard stats tests
// ---------------------------------------------------------------------------

test.describe('Dashboard lifecycle stats', () => {
  test('dashboard shows Amended and Superseded stat labels', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The stats section should surface "Amended" and "Superseded" labels.
    await expect(page.locator('text=Amended').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('text=Superseded').first()).toBeVisible({ timeout: 10_000 });
  });
});
