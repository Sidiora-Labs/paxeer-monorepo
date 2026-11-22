import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

import { observePage } from '../../browser.mjs';
import { readSurface } from '../../check.mjs';
import { LIMITS } from '../../config.mjs';
import { geometryFindings, measureParts, readGeometry } from '../../geometry.mjs';

export async function measureStatisticsGeometry(browser) {
  const surface = await readSurface(fileURLToPath(new URL('./measure.json', import.meta.url)));
  const part = JSON.parse(await readFile(new URL('./parts/statistics.json', import.meta.url), 'utf8'));
  const paths = new Set(part.views.map((view) => view.path));
  const stored = await readGeometry(surface.geometry.file);
  const renders = [];
  const findings = [];

  for (const view of surface.views.filter((view) => paths.has(view.path))) {
    for (const width of surface.geometry.widths) {
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        colorScheme: 'dark',
        serviceWorkers: 'block',
        hasTouch: width <= LIMITS.touchWidth,
      });
      try {
        await context.grantPermissions(['clipboard-write', 'local-network-access'], { origin: surface.address });
        await context.addInitScript(observePage);
        await context.route('**/*', async (route) => {
          const request = route.request();
          const url = new URL(request.url());
          if (!['http:', 'https:'].includes(url.protocol)) return route.continue();
          const answers = [...view.answers, ...surface.answers];
          let method = request.method();
          let address = request.url();
          if (url.origin === surface.address) {
            const response = await route.fetch({ maxRedirects: 0, timeout: LIMITS.navigationMs });
            const location = response.headers()['location'];
            if (response.status() >= 300 && response.status() < 400 && location && new URL(location, url).origin !== surface.address) {
              address = new URL(location, url).href;
              method = [307, 308].includes(response.status()) ? method : 'GET';
            } else {
              return route.fulfill({ response });
            }
          }
          const answer = answers.find((entry) => entry.method === method && entry.pattern.test(address));
          if (!answer) throw new Error(`No answer for ${method} ${address}`);
          return route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body });
        });
        const page = await context.newPage();
        page.setDefaultTimeout(LIMITS.navigationMs);
        await page.goto(new URL(view.path, surface.address).href, { waitUntil: 'load', timeout: LIMITS.navigationMs });
        await page.waitForFunction(() => document.fonts.status === 'loaded');
        let bottom = false;
        for (let index = 0; index < 200; index++) {
          bottom = await page.evaluate(() => {
            if (scrollY >= Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) - innerHeight) return true;
            scrollBy(0, innerHeight);
            return false;
          });
          if (bottom) break;
          await page.waitForTimeout(30);
        }
        if (!bottom) throw new Error('Document does not reach its end');
        await page.evaluate(() => scrollTo(0, 0));
        await page.waitForTimeout(LIMITS.settleMs);
        const measured = await measureParts(page, view.parts);
        const previous = stored.entries.filter((entry) => entry.view === view.name && entry.width === width);
        renders.push({ view: view.name, width, measured });
        findings.push(...geometryFindings(measured, previous).map((finding) => ({ ...finding, view: view.name, width })));
      } finally {
        await context.close();
      }
    }
  }
  return { renders, findings };
}
