import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

import { readSurface } from '../../check.mjs';
import { LIMITS } from '../../config.mjs';

const surface = await readSurface(fileURLToPath(new URL('./parts/tokens.json', import.meta.url)));

export function tokenServerAnswer(method, pathname, search) {
  const address = 'https://explorer.example.test' + pathname + search;
  return surface.answers.find((answer) => answer.method === method && answer.pattern.test(address));
}

export async function assertTokenPages() {
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      colorScheme: 'light',
      serviceWorkers: 'block',
    });
    let answers = surface.answers;
    const unanswered = [];
    try {
      await context.grantPermissions(['clipboard-write', 'local-network-access'], { origin: surface.address });
      await context.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (!['http:', 'https:'].includes(url.protocol)) return route.continue();
        let address = url.href;
        let method = request.method();
        if (url.origin === surface.address) {
          const response = await route.fetch({ maxRedirects: 0, timeout: LIMITS.navigationMs });
          const location = response.headers().location;
          const redirected = location && response.status() >= 300 && response.status() < 400 ? new URL(location, url) : null;
          if (!redirected || redirected.origin === surface.address) return route.fulfill({ response });
          address = redirected.href;
          method = [307, 308].includes(response.status()) ? method : 'GET';
        }
        const answer = answers.find((entry) => entry.method === method && entry.pattern.test(address));
        if (!answer) {
          unanswered.push(`${method} ${address}`);
          return route.abort('blockedbyclient');
        }
        await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body });
      });
      const page = await context.newPage();
      page.setDefaultTimeout(LIMITS.navigationMs);
      for (const view of surface.views) {
        const document = await fetch(new URL(view.path, surface.address), { signal: AbortSignal.timeout(LIMITS.navigationMs) });
        await document.arrayBuffer();
        assert.equal(document.status, 200, `${view.name}: document response`);
        answers = [...view.answers, ...surface.answers];
        const response = await page.goto(new URL(view.path, surface.address).href, {
          timeout: LIMITS.navigationMs,
          waitUntil: 'networkidle',
        });
        assert.equal(response.status(), 200, `${view.name}: page response`);
        await page.getByRole('main').getByRole('heading', { level: 1 }).waitFor({ state: 'visible' });
        assert.deepEqual(unanswered, [], `${view.name}: fixed answers`);
      }
    } finally {
      await context.close();
    }
  } finally {
    await browser.close();
  }
}
