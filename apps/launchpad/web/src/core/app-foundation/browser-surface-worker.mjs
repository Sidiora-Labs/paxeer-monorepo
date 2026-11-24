import { parentPort, workerData } from 'node:worker_threads';
import { createRequire } from 'node:module';

const { chromium } = createRequire(import.meta.url)('playwright');
const signal = new Int32Array(workerData.signal);
const bytes = new Uint8Array(workerData.bytes);
let browser;
let page;

async function evaluate(operation, args) {
  if (operation === 'open') {
    browser = await chromium.launch({ headless: true });
    page = await browser.newPage({ viewport: args.viewport });
    await page.exposeFunction('surfaceMediaChanged', (id, matches) => {
      parentPort.postMessage({ id, matches });
    });
    await page.addStyleTag({ content: args.css });
    return page.evaluate(() => {
      document.documentElement.style.fontSize = '16px';
      window.surfaceMedia = new Map();
      const root = getComputedStyle(document.documentElement);
      const measure = document.createElement('span');
      measure.style.cssText = 'position:fixed;padding:0;border:0';
      document.body.append(measure);
      const declarations = ['font-size:16px'];
      for (const name of root) {
        if (!name.startsWith('--sl-')) continue;
        let value = root.getPropertyValue(name).trim();
        if (/^(?:[\d.]|calc\(|min\(|max\(|clamp\()/.test(value) && CSS.supports('width', value)) {
          measure.style.width = value;
          value = getComputedStyle(measure).width;
        }
        declarations.push(`${name}:${value}`);
      }
      measure.remove();
      return `html { ${declarations.join(';')} }`;
    });
  }
  if (operation === 'media') {
    return page.evaluate(({ id, query }) => {
      const media = window.matchMedia(query);
      window.surfaceMedia.set(id, media);
      media.addEventListener('change', event => window.surfaceMediaChanged(id, event.matches));
      return { media: media.media, matches: media.matches };
    }, args);
  }
  if (operation === 'matches') return page.evaluate(id => window.surfaceMedia.get(id).matches, args);
  if (operation === 'viewport') return page.setViewportSize(args);
  if (operation === 'close') {
    await browser.close();
    return null;
  }
  throw new Error(`Unknown browser surface operation: ${operation}`);
}

parentPort.on('message', async ({ operation, args }) => {
  let response;
  try {
    response = { value: await evaluate(operation, args) };
  } catch (error) {
    response = { error: String(error) };
  }
  const encoded = new TextEncoder().encode(JSON.stringify(response));
  if (encoded.length > bytes.length) throw new Error('Browser surface response exceeds its buffer');
  bytes.set(encoded);
  Atomics.store(signal, 1, encoded.length);
  Atomics.store(signal, 0, 1);
  Atomics.notify(signal, 0);
});
