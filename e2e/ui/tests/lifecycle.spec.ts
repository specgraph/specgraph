// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

import { test, expect, request as playwrightRequest } from './fixtures';
import {
  seedSpec,
  advanceToDone,
  advanceToApproved,
  amendSpec,
  supersedeSpec,
  updateSpecIntent,
} from './helpers';

// Open the Changelog accordion and lazy-load its entries. Returns once at least
// one timeline entry is visible.
async function openChangelog(page: import('@playwright/test').Page): Promise<void> {
  const changelogHeader = page.getByTestId('accordion-header').filter({ hasText: 'Changelog' });
  await expect(changelogHeader).toBeVisible({ timeout: 10_000 });
  await changelogHeader.click();

  const loadBtn = page.getByRole('button', { name: 'Load changelog' });
  if (await loadBtn.isVisible({ timeout: 2_000 }).catch(() => false)) {
    await loadBtn.click();
  }

  await expect(page.getByTestId('timeline-entry').first()).toBeVisible({ timeout: 10_000 });
}

// ---------------------------------------------------------------------------
// Amendment tests
// ---------------------------------------------------------------------------

test.describe('Amendment', () => {
  const slug = 'lc-amend-e2e';

  test.beforeAll(async () => {
    const request = await playwrightRequest.newContext();
    await seedSpec(request, slug, 'Lifecycle amend E2E test spec', 'p2');
    await advanceToApproved(request, slug);
    await amendSpec(request, slug, 'Requirements changed after implementation', 'shape');
    await request.dispose();
  });

  test('stage badge updates after amend', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');
    // After amend with re-entry stage "shape", the spec lands at "spark" (one before shape).
    await expect(page.getByTestId('stage-badge')).toContainText('spark', { timeout: 10_000 });
  });

  test('changelog accordion shows entries', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');

    // Opening the changelog surfaces at least one timeline entry.
    await openChangelog(page);
    await expect(page.getByTestId('timeline-entry').first()).toBeVisible();
  });

  test('clicking a changelog entry expands diff view', async ({ page }) => {
    await page.goto(`/spec/${slug}`);
    await page.waitForLoadState('networkidle');

    await openChangelog(page);

    // Click the first timeline card to expand its inline diff.
    const firstCard = page.getByTestId('timeline-card').first();
    await expect(firstCard).toBeVisible({ timeout: 10_000 });
    await firstCard.click();

    // An inline diff panel should now be visible.
    await expect(page.getByTestId('diff-view').first()).toBeVisible({ timeout: 10_000 });
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

    const banner = page.getByTestId('superseded-banner');
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

    const banner = page.getByTestId('supersedes-banner');
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

    // Open changelog accordion (VersionCompare renders alongside the timeline).
    await openChangelog(page);

    // Pick a concrete "From" version via the shadcn Select (not a native <select>).
    await page.getByLabel('From version').click();
    // Option index 0 is "auto (previous)"; index 1 is the first real version.
    await page.getByRole('option').nth(1).click();

    await page.getByRole('button', { name: 'Compare' }).click();
    await expect(page.getByTestId('compare-result')).toBeVisible({ timeout: 10_000 });
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
    await expect(page.getByText('Amended').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Superseded').first()).toBeVisible({ timeout: 10_000 });
  });
});
