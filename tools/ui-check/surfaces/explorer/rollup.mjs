import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

import { readSurface } from '../../check.mjs';
import { LIMITS } from '../../config.mjs';

export async function assertRollupPages() {
  const surface = await readSurface(fileURLToPath(new URL('./parts/rollup.json', import.meta.url)));
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      colorScheme: 'light',
      serviceWorkers: 'block',
    });
    let active = true;
    let answers = surface.answers;
    const unanswered = [];
    const headResponses = [];
    context.on('close', () => { active = false; });
    context.on('response', (response) => {
      if (response.request().method() === 'HEAD') {
        headResponses.push({ address: response.url(), status: response.status() });
      }
    });
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
      for (const view of surface.views) {
        answers = [...view.answers, ...surface.answers];
        headResponses.length = 0;
        const response = await page.goto(new URL(view.path, surface.address).href, {
          timeout: LIMITS.navigationMs,
          waitUntil: 'networkidle',
        });
        assert.equal(response.status(), 404, `${view.name}: not-found status`);
        const main = page.getByRole('main', { name: 'Page not found', exact: true });
        await main.waitFor({ state: 'visible' });
        await main.getByRole('heading', { name: 'Page not found', level: 1, exact: true }).waitFor({ state: 'visible' });
        const home = main.getByRole('link', { name: 'Back to home', exact: true });
        await home.waitFor({ state: 'visible' });
        assert.equal(new URL(await home.getAttribute('href'), surface.address).pathname.replace(/\/$/, ''), '/explorer', `${view.name}: home destination`);
        assert.equal(await home.locator('a, button, input, [tabindex]').count(), 0, `${view.name}: one native link`);
        const probe = view.expects.find((entry) => entry.method === 'HEAD');
        assert.ok(probe, `${view.name}: not-found data probe declaration`);
        assert.ok(headResponses.some((response) => probe.matcher.test(response.address) && response.status === probe.status),
          `${view.name}: not-found data probe response`);
        assert.deepEqual(unanswered, [], `${view.name}: fixed answers`);
      }
    } finally {
      await context.close();
    }
  } finally {
    await browser.close();
  }
}
