import { test, expect } from '@playwright/test';

test('debug: project switch + refresh', async ({ page }) => {
  const logs: string[] = [];
  page.on('console', (msg) => logs.push(`[${msg.type()}] ${msg.text()}`));
  page.on('pageerror', (err) => logs.push(`PAGE_ERROR: ${err.message}`));
  page.on('requestfailed', (req) => logs.push(`FAIL: ${req.url()} ${req.failure()?.errorText}`));
  page.on('response', (resp) => {
    if (resp.url().includes('specgraph.v1') || resp.url().includes('/api/'))
      logs.push(`RESP: ${resp.status()} ${resp.url().split('/').pop()}`);
  });

  // Step 1: Load with default
  await page.goto('http://localhost:7890/');
  await page.waitForTimeout(3000);
  logs.push('=== AFTER INITIAL LOAD ===');
  
  // Step 2: Check localStorage
  const stored = await page.evaluate(() => localStorage.getItem('specgraph-project'));
  logs.push(`localStorage before: ${stored}`);
  
  // Step 3: Set localStorage to non-default and reload
  await page.evaluate(() => localStorage.setItem('specgraph-project', 'specgraph-specgraph'));
  logs.push('=== SET localStorage to specgraph-specgraph, RELOADING ===');
  await page.reload();
  await page.waitForTimeout(5000);
  
  const bodyText = await page.evaluate(() => document.body.innerText.substring(0, 300));
  logs.push(`BODY: ${bodyText}`);
  
  console.log(logs.join('\n'));
  expect(bodyText).not.toContain('Loading...');
});
