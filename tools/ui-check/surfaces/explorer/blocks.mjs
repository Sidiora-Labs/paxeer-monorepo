import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { readFile, writeFile } from 'node:fs/promises';
import { gzipSync } from 'node:zlib';
import { chromium } from 'playwright';

import { readSurface } from '../../check.mjs';
import { LIMITS } from '../../config.mjs';

const preparedPagesFile = '/tmp/explorer-block-pages.json';

export async function prepareBlockResponses() {
  const surface = await readSurface(fileURLToPath(new URL('./parts/blocks.json', import.meta.url)));
  const buildId = await readFile(new URL('../../../../apps/explorer/.next/BUILD_ID', import.meta.url), 'utf8');
  const pages = [];
  for (const view of surface.views) {
    const response = await fetch(new URL(view.path, surface.address), {
      headers: { 'accept-encoding': 'identity' },
      signal: AbortSignal.timeout(LIMITS.navigationMs),
    });
    assert.equal(response.status, 200, `${view.name}: prepared page status`);
    assert.match(response.headers.get('content-type'), /^text\/html\b/, `${view.name}: prepared page type`);
    const body = await response.text();
    assert.ok(body.includes('id="__NEXT_DATA__"'), `${view.name}: application page data`);
    pages.push({ path: view.path, body, type: response.headers.get('content-type') });
  }
  await writeFile(preparedPagesFile, JSON.stringify({ buildId, pages }));
}

export async function readBlockResponses() {
  const buildId = await readFile(new URL('../../../../apps/explorer/.next/BUILD_ID', import.meta.url), 'utf8');
  const prepared = JSON.parse(await readFile(preparedPagesFile, 'utf8'));
  assert.equal(prepared.buildId, buildId, 'Prepared block pages match the application build');
  const surface = await readSurface(fileURLToPath(new URL('./parts/blocks.json', import.meta.url)));
  assert.deepEqual(prepared.pages.map((page) => page.path), surface.views.map((view) => view.path));
  return new Map(prepared.pages.map((page) => {
    const body = Buffer.from(page.body);
    return [page.path, { body, compressed: gzipSync(body), type: page.type }];
  }));
}

async function assertBlockState(surface, browser, view, state, width) {
  const context = await browser.newContext({
    viewport: { width, height: 900 },
    colorScheme: 'light',
    serviceWorkers: 'block',
  });
  let active = true;
  const answers = [...(state.answers ?? []), ...view.answers, ...surface.answers];
  const unanswered = [];
  const responses = [];
  context.on('close', () => { active = false; });
  context.on('response', (response) => {
    responses.push({ method: response.request().method(), address: response.url(), status: response.status() });
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
    const response = await page.goto(new URL(view.path, surface.address).href, {
      timeout: LIMITS.navigationMs,
      waitUntil: state.name === 'loading' ? 'load' : 'networkidle',
    });
    assert.equal(response.status(), 200, `${view.name}: page status`);
    const title = view.name === '/blocks' ? 'Blocks' :
      view.name === '/block/countdown' ? 'Block countdown' :
        `Block #${view.path.split('/').at(-1)}`;
    await page.getByRole('heading', { name: title, level: 1, exact: true }).waitFor({ state: 'visible' });
    if (view.name === '/block/[height_or_hash]' || (view.name === '/blocks' && ['default', 'loading'].includes(state.name))) {
      await page.locator('[data-list-page]').waitFor({ state: 'visible' });
    }
    for (const step of state.steps ?? []) {
      if (step.action === 'wait') await page.getByText(step.text, { exact: false }).first().waitFor({ state: 'visible' });
      else if (step.action === 'press') await page.locator(step.selector).press(step.key);
      else if (step.action === 'click') await page.locator(step.selector).click();
      else if (step.action === 'fill') await page.locator(step.selector).fill(step.value);
      else assert.fail(`Unsupported block state action: ${step.action}`);
    }
    for (const expected of state.expects ?? view.expects ?? []) {
      assert.ok(responses.some((response) => response.method === expected.method &&
        expected.matcher.test(response.address) && response.status === expected.status), `${view.name}: ${state.name} response`);
    }
    assert.deepEqual(unanswered, [], `${view.name}: ${state.name} fixed answers`);
  } catch (error) {
    error.message = `${view.name}: ${state.name} at ${width}px: ${error.message}`;
    throw error;
  } finally {
    await context.close();
  }
}

export async function assertBlockPages() {
  const surface = await readSurface(fileURLToPath(new URL('./parts/blocks.json', import.meta.url)));
  const browser = await chromium.launch({ headless: true });
  try {
    for (const width of [390, 1440]) {
      for (const view of surface.views) {
        for (const state of [{ name: 'default' }, ...(view.states ?? [])]) {
          await assertBlockState(surface, browser, view, state, width);
        }
      }
    }
  } finally {
    await browser.close();
  }
}
