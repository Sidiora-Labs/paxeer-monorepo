import assert from 'node:assert/strict';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

import { readSurface } from '../../check.mjs';
import { LIMITS } from '../../config.mjs';

export async function assertTransactionPages() {
  const surface = await readSurface(fileURLToPath(new URL('./parts/transactions.json', import.meta.url)));
  const browser = await chromium.launch({ headless: true });
  try {
    for (const view of surface.views) {
      const context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
        colorScheme: 'light',
        serviceWorkers: 'block',
      });
      let active = true;
      const answers = [...view.answers, ...surface.answers];
      const unanswered = [];
      context.on('close', () => { active = false; });
      try {
        await context.grantPermissions(['clipboard-write', 'local-network-access'], { origin: surface.address });
        await context.route('**/*', async (route) => {
          try {
            const request = route.request();
            const url = new URL(request.url());
            if (!['http:', 'https:'].includes(url.protocol)) {
              await route.continue();
              return;
            }
            let address = url.href;
            let method = request.method();
            if (url.origin === surface.address) {
              const response = await route.fetch({ maxRedirects: 0, timeout: LIMITS.navigationMs });
              const location = response.headers()['location'];
              const redirected = location && response.status() >= 300 && response.status() < 400 ? new URL(location, url) : null;
              if (!redirected || redirected.origin === surface.address) {
                await route.fulfill({ response });
                return;
              }
              address = redirected.href;
              method = [307, 308].includes(response.status()) ? method : 'GET';
            }
            const answer = answers.find((entry) => entry.method === method && entry.pattern.test(address));
            if (!answer) {
              unanswered.push(`${method} ${address}`);
              await route.abort('blockedbyclient');
              return;
            }
            await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body });
          } catch (error) {
            if (active) unanswered.push(error.message);
          }
        });
        const page = await context.newPage();
        page.setDefaultTimeout(LIMITS.navigationMs);
        const response = await page.goto(new URL(view.path, surface.address).href, {
          timeout: LIMITS.navigationMs,
          waitUntil: 'networkidle',
        });
        const documentStatus = view.expects.find((entry) => entry.method === 'GET' && entry.matcher.test(response.url()))?.status ?? 200;
        assert.equal(response.status(), documentStatus, `${view.name}: document status`);
        const heading = page.getByRole('heading', { level: 1 });
        await heading.waitFor({ state: 'visible' });
        assert.ok((await heading.innerText()).trim(), `${view.name}: page heading`);
        if (documentStatus === 404) {
          await page.getByRole('main', { name: 'Page not found', exact: true }).waitFor({ state: 'visible' });
          await page.getByRole('link', { name: 'Back to home', exact: true }).waitFor({ state: 'visible' });
        }
        assert.deepEqual(unanswered, [], `${view.name}: fixed answers`);
      } finally {
        await context.close();
      }
    }
  } finally {
    await browser.close();
  }
}

export async function prepareTransactionAnswers() {
  const surface = await readSurface(fileURLToPath(new URL('./parts/transactions.json', import.meta.url)));
  const cache = new URL('../../../../apps/explorer/.next/cache/', import.meta.url);
  const buildId = (await readFile(new URL('../BUILD_ID', cache), 'utf8')).trim();
  const answers = surface.answers.map((answer) => ({
    ...answer,
    pattern: { source: answer.pattern.source, flags: answer.pattern.flags },
    body: answer.body.toString('base64'),
  }));
  await mkdir(cache, { recursive: true });
  await writeFile(new URL('explorer-transaction-answers.json', cache), JSON.stringify({ buildId, answers }));
}
