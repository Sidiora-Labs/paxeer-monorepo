import { spawn } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { request } from 'node:https';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { expect, test } from 'vitest';

async function coinFirstTradeAndPhoneComposition() {
  const repository = resolve(fileURLToPath(new URL('.', import.meta.url)), '../../../../../..');
  const server = spawn(process.execPath, ['tools/ui-check/surfaces/launchpad/start.mjs'], {
    cwd: repository,
    env: { ...process.env, NODE_EXTRA_CA_CERTS: resolve(tmpdir(), 'launchpad-ui-certificates/sidiora.test.pem') },
    detached: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  server.stdout.on('data', bytes => { output += bytes; });
  server.stderr.on('data', bytes => { output += bytes; });
  const origin = 'https://sidiora.test:4173';
  const browser = await chromium.launch();
  try {
    const deadline = Date.now() + 90_000;
    while (true) {
      try {
        const ca = await readFile(resolve(tmpdir(), 'launchpad-ui-certificates/sidiora.test.pem'));
        await new Promise<void>((accept, reject) => {
          const probe = request(`${origin}/`, { ca, timeout: 1000 }, response => {
            response.resume();
            response.on('end', () => response.statusCode === 200 ? accept() : reject(new Error(`Page returned ${response.statusCode}`)));
          });
          probe.on('timeout', () => probe.destroy(new Error('Page did not answer')));
          probe.on('error', reject);
          probe.end();
        });
        break;
      } catch (error) {
        if (server.exitCode !== null || Date.now() >= deadline) throw new Error(`${String(error)}\n${output}`);
        await new Promise(accept => setTimeout(accept, 100));
      }
    }
    for (const colorScheme of ['dark', 'light'] as const) {
      const context = await browser.newContext({ viewport: { width: 390, height: 844 }, colorScheme, serviceWorkers: 'block' });
      try {
        const page = await context.newPage();
        await page.goto(`${origin}/coin/0x1111111111111111111111111111111111111111?render=empty`);
        await page.locator('[data-shell-ready]').waitFor();
        await page.locator('[data-coin-header="ready"]').waitFor();
        await page.locator('[data-chart-empty]').waitFor();
        await page.locator('[data-freshness-chip="waiting"]').waitFor();
        await page.evaluate(() => document.fonts.ready);
        const composition = await page.evaluate(() => {
          const header = document.querySelector<HTMLElement>('[data-coin-header="ready"]')!;
          const toolbar = document.querySelector<HTMLElement>('[data-chart-toolbar]')!;
          const rows = [...header.children].map(element => element.getBoundingClientRect());
          const toolbarRows = [...toolbar.children].filter(element => getComputedStyle(element).display !== 'none').map(element => element.getBoundingClientRect());
          const canvas = document.querySelector<HTMLElement>('[data-chart-canvas]')!;
          const empty = document.querySelector<HTMLElement>('[data-chart-empty]')!;
          return {
            rows: rows.length,
            identityAboveValue: rows[0].bottom <= rows[1].top,
            headerRowsDoNotWrap: rows.every(row => row.width <= header.clientWidth),
            singleToolbarRow: toolbarRows.every(row => row.top < toolbarRows[0].bottom && row.bottom > toolbarRows[0].top),
            toolbarScrolls: toolbar.scrollWidth > toolbar.clientWidth && getComputedStyle(toolbar).overflowX === 'auto',
            libraryHidden: getComputedStyle(canvas).visibility === 'hidden' && [...canvas.querySelectorAll('iframe')].every(frame => getComputedStyle(frame).visibility === 'hidden'),
            canvasInert: canvas.hasAttribute('inert') && canvas.getAttribute('aria-hidden') === 'true',
            emptyText: empty.textContent,
            chartText: document.querySelector<HTMLElement>('section[aria-label="Price chart"]')!.innerText,
            retryCount: [...document.querySelectorAll('section[aria-label="Price chart"] button')].filter(button => button.textContent === 'Try again').length,
            retryHidden: [...document.querySelectorAll('section[aria-label="Price chart"] button')].filter(button => button.textContent === 'Try again').every(button => getComputedStyle(button).visibility === 'hidden' && button.hasAttribute('inert') && button.getAttribute('aria-hidden') === 'true'),
            chartWarning: document.querySelector('section[aria-label="Price chart"] [role="alert"], section[aria-label="Price chart"] .lucide-circle-alert') !== null,
            neutralChip: document.querySelector('[data-freshness-chip="waiting"]')!.textContent,
            directionChip: header.querySelector('[data-change]') !== null,
            sideways: document.documentElement.scrollWidth > document.documentElement.clientWidth,
          };
        });
        expect(composition.rows).toBe(2);
        expect(composition.identityAboveValue).toBe(true);
        expect(composition.headerRowsDoNotWrap).toBe(true);
        expect(composition.singleToolbarRow).toBe(true);
        expect(composition.toolbarScrolls).toBe(true);
        expect(composition.libraryHidden).toBe(true);
        expect(composition.canvasInert).toBe(true);
        expect(composition.emptyText).toContain('No trades yet');
        expect(composition.emptyText).toContain('The chart starts with the first trade.');
        expect(composition.emptyText).toContain('Opening price');
        expect(composition.chartText).not.toMatch(/Try again|No data|No Data|Data is unavailable/);
        expect(composition.chartWarning).toBe(false);
        expect(composition.retryCount).toBe(1);
        expect(composition.retryHidden).toBe(true);
        expect(composition.neutralChip).toBe('Waiting for trades');
        expect(composition.directionChip).toBe(false);
        expect(composition.sideways).toBe(false);
        await page.locator('[data-coin-header="ready"] button[aria-haspopup="menu"]').click();
        await page.getByRole('menuitem', { name: 'Share coin link', exact: true }).waitFor();
        await page.keyboard.press('Escape');
        await page.getByRole('button', { name: 'Chart options', exact: true }).click();
        const menu = page.getByRole('menu', { name: 'Chart options', exact: true });
        await menu.waitFor();
        expect(await menu.textContent()).toContain('Currency');
        expect(await menu.textContent()).toContain('Trade markers');
        await page.keyboard.press('Escape');
        expect(await menu.count()).toBe(0);
        const cta = page.locator('[data-mobile-trade-cta]');
        await cta.waitFor();
        const ctaBox = await cta.boundingBox();
        const navigationBox = await page.getByRole('navigation', { name: 'Mobile navigation', exact: true }).boundingBox();
        expect(ctaBox!.y + ctaBox!.height).toBeLessThanOrEqual(navigationBox!.y);
        await page.evaluate(() => {
          const root = document.documentElement;
          root.style.fontSize = `${parseFloat(getComputedStyle(root).fontSize) * 2}px`;
        });
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.goto(`${origin}/coin/0x1111111111111111111111111111111111111111`);
        await page.locator('[data-coin-header="ready"] [data-freshness-chip] .sr-only').waitFor({ state: 'attached' });
        await page.evaluate(() => {
          const sizes = [...document.querySelectorAll<HTMLElement>('*')].map(element => [element, parseFloat(getComputedStyle(element).fontSize)] as const);
          for (const [element, size] of sizes) element.style.setProperty('font-size', `${size * 2}px`, 'important');
        });
        const valueRow = page.locator('[data-coin-header-row="value"]');
        expect(await valueRow.evaluate(element => element.scrollWidth > element.clientWidth)).toBe(true);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.locator('[data-coin-header="ready"] button[aria-haspopup="menu"]').click();
        await page.getByRole('menuitem', { name: 'Share coin link', exact: true }).waitFor();
        await page.keyboard.press('Escape');
        for (const width of [834, 1440, 1920]) {
          await page.setViewportSize({ width, height: 900 });
          await page.goto(`${origin}/coin/0x1111111111111111111111111111111111111111?render=empty`);
          await page.locator('[data-coin-header="ready"] button[aria-label="Share coin link"]').waitFor();
        }
        await page.setViewportSize({ width: 390, height: 844 });
        await page.goto(`${origin}/coin/0x1111111111111111111111111111111111111111?render=error`);
        await page.locator('[data-coin-header="degraded"]').waitFor();
        await page.locator('[data-chart-toolbar]').waitFor();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
      } finally {
        await context.close();
      }
    }
  } finally {
    await browser.close();
    if (server.pid) process.kill(-server.pid, 'SIGTERM');
    await new Promise<void>(accept => { if (server.exitCode !== null || server.signalCode !== null) accept(); else server.once('exit', () => accept()); });
  }
}

test(coinFirstTradeAndPhoneComposition, { timeout: 150_000 }, coinFirstTradeAndPhoneComposition);
