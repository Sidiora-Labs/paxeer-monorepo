import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { randomBytes } from 'node:crypto';
import { renderToStaticMarkup } from 'react-dom/server';
import { chromium, type Browser } from 'playwright';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { AgentEnrollment, AgentKeyNotice, AgentLimitsForm, AgentTransferForm } from './agent-forms';
import { AgentsAccess } from './agents-page';
import { AgentConnectView, type AgentCopyState } from './agent-connect';
import { agentConnectionConfiguration } from '@/core/agent-interface';
import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';

const require = createRequire(import.meta.url);
const postcss = require.resolve('@tailwindcss/postcss');
const tailwind = createRequire(postcss).resolve('@tailwindcss/node');
const owner = `0x${'11'.repeat(20)}` as const;
const balances = { native: 10000000000000000000n, quote: 25000000n, quoteDecimals: 6 };
let browser: Browser;
let css = '';
let markup = '';
let connectMarkup: string[] = [];
const noop = () => {};

beforeAll(async () => {
  markup = renderToStaticMarkup(<AgentsAccess state="ready" onSignIn={noop} onRetry={noop}>
    <AgentKeyNotice value={{ did: `did:matrix:${'agent'.repeat(20)}:${'ab'.repeat(8)}`, publicKey: randomBytes(32).toString('hex'), privateKey: randomBytes(32).toString('hex') }} onDismiss={noop} />
    <AgentEnrollment ownerId="owner" busy={false} hasKey={true} onCreate={noop} onClaim={noop} />
    <div className="grid min-w-0 gap-3 lg:grid-cols-2"><AgentTransferForm kind="fund" owner={owner} balances={balances} busy={false} onSend={noop} /><AgentTransferForm kind="sweep" owner={owner} balances={balances} busy={false} onSend={noop} /></div>
    <AgentLimitsForm quoteDecimals={6} busy={false} onSave={noop} />
  </AgentsAccess>);
  const connection = agentConnectionConfiguration();
  const agent = { did: `did:matrix:${'agent'.repeat(20)}:${'ab'.repeat(8)}`, label: 'Launchpad agent' };
  connectMarkup = [
    renderToStaticMarkup(<AgentConnectView agent={null} connection={connection} copyState="idle" onCopy={noop} />),
    renderToStaticMarkup(<AgentConnectView agent={agent} connection={{ status: 'unavailable' }} copyState="idle" onCopy={noop} />),
    ...(['idle', 'copying', 'copied', 'manual'] as AgentCopyState[]).map(copyState => renderToStaticMarkup(<AgentConnectView agent={agent} connection={connection} copyState={copyState} onCopy={noop} />)),
  ];
  const { compile } = await import(pathToFileURL(tailwind).href) as { compile(source: string, options: { base: string; onDependency: (path: string) => void }): Promise<{ build(candidates: string[]): string }> };
  const base = resolve(process.cwd(), 'src/app');
  const compiler = await compile(readFileSync(resolve(base, 'globals.css'), 'utf8'), { base, onDependency: noop });
  const classes = Array.from([markup, ...connectMarkup].join('').matchAll(/class="([^"]+)"/g)).flatMap(match => match[1].split(/\s+/));
  css = compiler.build(classes);
  browser = await chromium.launch({ headless: true });
}, 30000);
afterAll(async () => { if (browser) await browser.close(); });

async function agentsPageFitsPhoneAndDesktopInBothColorModes() {
  const page = await browser.newPage();
  try {
    await page.setContent(`<style>${css}</style><main>${markup}</main>`);
    for (const width of [360, 768, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      for (const mode of ['dark', 'light']) {
        await page.evaluate(value => document.documentElement.setAttribute('data-color-mode', value), mode);
        const metrics = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth, scheme: getComputedStyle(document.documentElement).colorScheme }));
        expect(metrics.scheme).toBe(mode);
        expect(metrics.scroll, `${mode} at ${width}`).toBeLessThanOrEqual(metrics.width);
        const controls = await page.locator('button, input:not([type="checkbox"]), select, textarea').evaluateAll(elements => elements.map(element => ({ width: element.getBoundingClientRect().width, height: element.getBoundingClientRect().height })));
        for (const control of controls) {
          expect(control.width).toBeGreaterThan(0);
          if (width <= 834) expect(control.height).toBeGreaterThanOrEqual(44);
        }
      }
    }
  } finally { await page.close(); }
}

async function agentsPageUsesLabelsKeyboardFocusAndThemeControlEdges() {
  assertControlEdges(markup);
  const page = await browser.newPage();
  try {
    await page.setContent(`<style>${css}</style><main>${markup}</main>`);
    await expect.poll(() => page.getByRole('heading', { name: 'Agents', exact: true }).count()).toBe(1);
    for (const label of ['Agent name', 'Agent DID', 'Funding asset', 'Sweep asset', 'Funding amount', 'Sweep amount', 'Agent preset', 'PAX per transaction', 'PAX per day', 'USDL per approval']) {
      expect(await page.getByLabel(label, { exact: true }).count(), label).toBe(1);
    }
    await page.getByLabel('Agent name', { exact: true }).focus();
    await page.keyboard.press('Tab');
    expect(await page.getByLabel('Agent DID', { exact: true }).evaluate(element => element === document.activeElement)).toBe(true);
    await page.keyboard.press('Tab');
    expect(await page.getByRole('button', { name: 'Prepare claim', exact: true }).evaluate(element => element === document.activeElement)).toBe(true);
    const focus = await page.getByRole('button', { name: 'Prepare claim', exact: true }).evaluate(element => ({ width: getComputedStyle(element).outlineWidth, style: getComputedStyle(element).outlineStyle }));
    expect(focus.width).toBe('2px'); expect(focus.style).toBe('solid');
    for (const mode of ['dark', 'light']) {
      await page.evaluate(value => document.documentElement.setAttribute('data-color-mode', value), mode);
      const colors = await page.getByRole('button', { name: 'Prepare claim', exact: true }).evaluate(element => {
        const style = getComputedStyle(element);
        return { text: style.color, edge: style.borderColor, surface: style.backgroundColor };
      });
      expect(colors.text).not.toBe(colors.surface); expect(colors.edge).not.toBe(colors.surface);
    }
  } finally { await page.close(); }
}

async function agentConnectStatesFitBothColorModesAndPhoneWidths() {
  const expected = ['Choose an agent to connect', 'Connection details unavailable', 'Your key stays on the agent’s machine.', 'Copying configuration', 'Configuration copied.', 'Copy is unavailable.'];
  const page = await browser.newPage();
  try {
    for (const [index, markup] of connectMarkup.entries()) {
      assertControlEdges(markup);
      await page.setContent(`<style>${css}</style><main class="mx-auto max-w-5xl p-4">${markup}</main>`);
      expect(await page.locator('main').textContent()).toContain(expected[index]);
      for (const width of [360, 768, 1280]) {
        await page.setViewportSize({ width, height: 900 });
        for (const mode of ['dark', 'light']) {
          await page.evaluate(value => document.documentElement.setAttribute('data-color-mode', value), mode);
          const metrics = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth, scheme: getComputedStyle(document.documentElement).colorScheme }));
          expect(metrics.scheme).toBe(mode);
          expect(metrics.scroll, `${index}: ${mode} at ${width}`).toBeLessThanOrEqual(metrics.width);
          const controls = await page.locator('button, textarea, a').evaluateAll(elements => elements.map(element => ({ width: element.getBoundingClientRect().width, height: element.getBoundingClientRect().height })));
          for (const control of controls) {
            expect(control.width).toBeGreaterThan(0);
            if (width <= 834) expect(control.height).toBeGreaterThanOrEqual(44);
          }
        }
      }
    }
  } finally { await page.close(); }
}

for (const check of [agentsPageFitsPhoneAndDesktopInBothColorModes, agentsPageUsesLabelsKeyboardFocusAndThemeControlEdges, agentConnectStatesFitBothColorModesAndPhoneWidths]) test(check.name, check);
