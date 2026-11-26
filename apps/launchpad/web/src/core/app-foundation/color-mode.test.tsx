// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { chromium, type Browser } from 'playwright';
import { launchpadCanvasColors } from '@/app/presentation-tokens';
import { ColorModeControl } from '@/ui/primitives/color-mode-control';
import { applyBrowserSurface } from './browser-surface.test-support';
import {
  applyInitialColorMode, COLOR_MODE_STORAGE_KEY, colorModeScript, deploymentColorMode,
  getColorMode, getColorScheme, subscribeColorMode,
} from './color-mode';

let browser: Browser;
let server: Server;
let origin: string;
let restoreSurface: () => Promise<void>;
let root: Root | undefined;
let container: HTMLDivElement | undefined;
const defaultSetting = process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT;
const css = readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8');

beforeAll(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  restoreSurface = applyBrowserSurface();
  browser = await chromium.launch({ headless: true });
  server = createServer((_request, response) => {
    const mode = deploymentColorMode();
    response.setHeader('Content-Type', 'text/html');
    response.end(`<html data-color-mode="${mode}" data-color-mode-default="${mode}"><head><meta name="theme-color" content="${launchpadCanvasColors[mode === 'light' ? 'light' : 'dark']}"><style>${css}</style><script>${colorModeScript(mode)}</script></head><body>Launchpad</body></html>`);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Colour mode page is unavailable');
  origin = `http://127.0.0.1:${address.port}`;
});

beforeEach(() => {
  delete process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT;
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-color-mode');
  document.documentElement.removeAttribute('data-color-mode-default');
});
afterEach(() => {
  if (root) act(() => root!.unmount());
  root = undefined;
  container?.remove();
  container = undefined;
  if (defaultSetting === undefined) delete process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT;
  else process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT = defaultSetting;
});
afterAll(async () => {
  await browser.close();
  await restoreSurface();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

async function firstPaintUsesDarkWithoutAChoice() {
  const page = await browser.newPage({ colorScheme: 'light' });
  try {
    await page.goto(origin);
    expect(await page.locator('html').getAttribute('data-color-mode')).toBe('dark');
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe('dark');
    expect(await page.locator('meta[name="theme-color"]').getAttribute('content')).toBe(launchpadCanvasColors.dark);
  } finally { await page.close(); }
}

async function deploymentDefaultAcceptsLightAndDeviceAndRejectsOtherValues() {
  const page = await browser.newPage({ colorScheme: 'light' });
  try {
    for (const [value, expected] of [['light', 'light'], ['device', 'device'], ['dark', 'dark'], ['LIGHT', 'dark'], ['', 'dark'], ['unknown', 'dark']]) {
      process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT = value;
      expect(deploymentColorMode()).toBe(expected);
      await page.goto(origin);
      expect(await page.locator('html').getAttribute('data-color-mode')).toBe(expected);
      expect(await page.locator('meta[name="theme-color"]').getAttribute('content')).toBe(
        launchpadCanvasColors[expected === 'dark' ? 'dark' : 'light']
      );
    }
  } finally { await page.close(); }
}

async function storedChoicePrevailsBeforeContentAndInvalidChoiceUsesDefault() {
  process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT = 'light';
  for (const stored of ['dark', 'light', 'device', 'invalid']) {
    const context = await browser.newContext({ colorScheme: 'dark' });
    await context.addInitScript(({ key, stored }) => localStorage.setItem(key, stored), { key: COLOR_MODE_STORAGE_KEY, stored });
    const page = await context.newPage();
    try {
      await page.goto(origin);
      const expected = stored === 'invalid' ? 'light' : stored;
      expect(await page.locator('html').getAttribute('data-color-mode')).toBe(expected);
      expect(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe(expected === 'device' ? 'dark' : expected);
      await page.reload();
      expect(await page.locator('html').getAttribute('data-color-mode')).toBe(expected);
    } finally { await context.close(); }
  }
}

async function browserThatRefusesStorageUsesDeploymentDefault() {
  const page = await browser.newPage({ colorScheme: 'dark' });
  try {
    await page.goto(origin);
    const markup = `<html data-color-mode="light"><head><script>${colorModeScript('light')}</script></head><body>Launchpad</body></html>`;
    const frameElement = await page.evaluateHandle(markup => {
      const frame = document.createElement('iframe');
      frame.sandbox.add('allow-scripts');
      frame.srcdoc = markup;
      document.body.append(frame);
      return frame;
    }, markup);
    const frame = await frameElement.asElement()!.contentFrame();
    if (!frame) throw new Error('Colour mode frame is unavailable');
    await frame.waitForSelector('body');
    expect(await frame.evaluate(() => {
      try { window.localStorage.getItem('choice'); return false; }
      catch (error) { return error instanceof DOMException && error.name === 'SecurityError'; }
    })).toBe(true);
    expect(await frame.locator('html').getAttribute('data-color-mode')).toBe('light');
    expect(await frame.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe('light');
  } finally { await page.close(); }
}

function controlAppliesAndStoresTheChoiceAndNotifiesReaders() {
  applyInitialColorMode(deploymentColorMode(), COLOR_MODE_STORAGE_KEY, launchpadCanvasColors);
  const changes: string[] = [];
  const stop = subscribeColorMode(() => changes.push(`${getColorMode()}:${getColorScheme()}`));
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  try {
    act(() => root!.render(<ColorModeControl />));
    const choices = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
    expect(choices.map(choice => choice.textContent)).toEqual(['Dark', 'Light', 'Device']);
    expect(choices[0].getAttribute('aria-checked')).toBe('true');
    act(() => choices[1].click());
    expect(document.documentElement.getAttribute('data-color-mode')).toBe('light');
    expect(window.localStorage.getItem(COLOR_MODE_STORAGE_KEY)).toBe('light');
    expect(choices[1].getAttribute('aria-checked')).toBe('true');
    expect(changes.at(-1)).toBe('light:light');
    expect(document.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe(launchpadCanvasColors.light);
    document.documentElement.setAttribute('data-color-mode', 'dark');
    expect(applyInitialColorMode('dark', COLOR_MODE_STORAGE_KEY, launchpadCanvasColors)).toBe('light');
    act(() => choices[2].click());
    expect(getColorMode()).toBe('device');
    expect(window.localStorage.getItem(COLOR_MODE_STORAGE_KEY)).toBe('device');
    expect(changes.at(-1)?.startsWith('device:')).toBe(true);
  } finally { stop(); }
}

async function deviceChoiceFollowsDeviceAfterTheFirstPaint() {
  process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT = 'device';
  const page = await browser.newPage({ colorScheme: 'light' });
  try {
    await page.goto(origin);
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe('light');
    await page.emulateMedia({ colorScheme: 'dark' });
    // The root retains the visitor's device choice while its CSS palette changes.
    expect(await page.locator('html').getAttribute('data-color-mode')).toBe('device');
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--sl-canvas').trim())).toBe(launchpadCanvasColors.dark);
  } finally { await page.close(); }
}

for (const check of [firstPaintUsesDarkWithoutAChoice, deploymentDefaultAcceptsLightAndDeviceAndRejectsOtherValues,
  storedChoicePrevailsBeforeContentAndInvalidChoiceUsesDefault, browserThatRefusesStorageUsesDeploymentDefault,
  controlAppliesAndStoresTheChoiceAndNotifiesReaders, deviceChoiceFollowsDeviceAfterTheFirstPaint]) test(check.name, check);
