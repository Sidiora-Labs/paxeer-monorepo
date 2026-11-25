import { spawn, type ChildProcess } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { connect, createServer } from 'node:net';
import { dirname, join } from 'node:path';
import React, { act } from 'react';
import { afterAll, afterEach, expect, test as registerTest } from 'vitest';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000/explore', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { SearchParamsContext } = await import('next/dist/shared/lib/hooks-client-context.shared-runtime');
const { Operations, SdkClient } = await import('@kindle/sdk');
const { paxeer } = await import('@/core/wagmi-config');
const { MarketClientContext } = await import('@/core/market/client');
const { default: ExploreModule } = await import('@/page-modules/explore');
const { ExploreLoading } = await import('./explore-loading');
const { presetFor, presetHref, presets } = await import('./preset-tabs');

const wallet = createConfig({ chains: [paxeer], connectors: [], storage: null, transports: { [paxeer.id]: http('http://127.0.0.1:1') }, multiInjectedProviderDiscovery: false });
const cleanup: Array<() => Promise<void>> = [];

function test(testFunction: () => void | Promise<void>) {
  registerTest(testFunction, { timeout: 30000 }, testFunction);
}

async function mount(node: React.ReactNode) {
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const host = document.createElement('div');
  document.body.appendChild(host);
  const root = createRoot(host);
  cleanup.push(async () => {
    await act(async () => root.unmount());
    await queries.cancelQueries();
    queries.clear();
    host.remove();
  });
  await act(async () => root.render(
    <WagmiProvider config={wallet}>
      <QueryClientProvider client={queries}>{node}</QueryClientProvider>
    </WagmiProvider>,
  ));
  return host;
}

afterEach(async () => {
  for (const close of cleanup.splice(0).reverse()) await close();
  localStorage.clear();
});
afterAll(() => dom.window.close());

function missingAndUnknownPresetsOpenTrending() {
  for (const value of [undefined, null, '', 'unknown']) {
    expect(presetFor(value).id).toBe('trending');
  }
  for (const preset of presets) expect(presetFor(preset.id)).toBe(preset);
}

function trendingHasTheDefaultAddressAndFirstTab() {
  expect(presets.map(preset => preset.id)).toEqual(['trending', 'new', 'movers', 'volume', 'breakout', 'unusual', 'mcap']);
  for (const preset of presets) {
    expect(presetHref(preset)).toBe(preset.id === 'trending' ? '/explore' : `/explore?preset=${preset.id}`);
  }
}

async function loadingMarksTrendingAsTheActivePreset() {
  const host = await mount(<ExploreLoading />);
  const active = host.querySelectorAll('nav[aria-label="Explore presets"] a[aria-current="page"]');
  expect(active).toHaveLength(1);
  expect(active[0].textContent).toBe('Trending');
  expect(active[0].getAttribute('href')).toBe('/explore');
  expect(host.querySelector('caption')?.textContent).toBe('Trending coins');
  expect(host.querySelector('[aria-busy="true"]')).not.toBeNull();
}

async function availablePort(): Promise<number> {
  const server = createServer();
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('A local service address is required');
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  return address.port;
}

async function stop(child: ChildProcess) {
  if (child.exitCode !== null || child.signalCode !== null) return;
  await new Promise<void>(resolve => {
    child.once('exit', () => resolve());
    child.kill('SIGTERM');
  });
}

async function waitFor(check: () => boolean | Promise<boolean>) {
  const deadline = Date.now() + 10000;
  while (!await check()) {
    if (Date.now() >= deadline) throw new Error('The coin list did not become ready');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

async function emptyPresetsExplainTheirListsAndOfferTheNextAction() {
  const configurationPath = process.env.JOINED_CONFIG_FILE;
  if (!configurationPath) throw new Error('Discovery services are required');
  const configuration = JSON.parse(readFileSync(configurationPath, 'utf8')) as {
    processes: { role: string; env?: Record<string, string> }[];
  };
  const api = configuration.processes.find(service => service.role === 'api');
  if (!api?.env) throw new Error('The coin API configuration is required');
  const redisPort = await availablePort();
  const apiPort = await availablePort();
  const redis = spawn('redis-server', ['--bind', '127.0.0.1', '--port', String(redisPort), '--save', '', '--appendonly', 'no'], { stdio: 'ignore' });
  const redisReady = new Promise<void>((resolve, reject) => {
    redis.once('spawn', resolve);
    redis.once('error', reject);
  });
  try {
    await redisReady;
    await waitFor(() => new Promise<boolean>(resolve => {
      const socket = connect(redisPort, '127.0.0.1');
      socket.once('connect', () => { socket.destroy(); resolve(true); });
      socket.once('error', () => { socket.destroy(); resolve(false); });
    }));
    const redisUrl = `redis://127.0.0.1:${redisPort}/0`;
    const origin = `http://127.0.0.1:${apiPort}`;
    const service = spawn(join(dirname(configurationPath), 'api'), [], {
      env: { ...api.env, NODE_ENV: 'test', PORT: String(apiPort), REDIS_URL: redisUrl, REDIS_BULL_URL: redisUrl },
      stdio: 'ignore',
    });
    try {
      await new Promise<void>((resolve, reject) => {
        service.once('spawn', resolve);
        service.once('error', reject);
      });
      await waitFor(async () => {
        if (service.exitCode !== null) throw new Error('The coin API stopped');
        try { return (await fetch(`${origin}/health/ready`)).ok; } catch { return false; }
      });
      const stream = origin.replace(/^http/, 'ws');
      const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
      for (const view of ['table', 'grid']) {
        localStorage.setItem('sidiora.explore.settings.v1', JSON.stringify({ view }));
        for (const [id, body] of [
          ['movers', 'Movers lists coins with strong price moves outside Trending.'],
          ['new', 'New lists the newest coins.'],
          ['trending', 'Trending lists the highest-ranked coins right now.'],
          ['volume', 'Volume lists the most traded coins.'],
          ['breakout', 'Breakout lists coins gaining momentum.'],
          ['unusual', 'Unusual lists coins with unusual trading activity.'],
          ['mcap', 'Market cap lists Trending coins by market cap.'],
        ]) {
          const ranking = await new Operations(client).call('IF_RANKINGS', { path: { category: presetFor(id).category } });
          expect(ranking.data.snapshot.items).toEqual([]);
          const host = await mount(
            <MarketClientContext.Provider value={client}>
              <SearchParamsContext.Provider value={new URLSearchParams({ preset: id })}>
                <ExploreModule />
              </SearchParamsContext.Provider>
            </MarketClientContext.Provider>,
          );
          await waitFor(() => host.querySelector('[role="status"] h3') !== null);
          if (view === 'table') expect(host.querySelector('table')).not.toBeNull();
          else {
            expect(host.querySelector('table')).toBeNull();
            expect(host.querySelector('button[aria-label="Grid"]')?.getAttribute('aria-checked')).toBe('true');
          }
          const empty = host.querySelector('[role="status"] h3')!.parentElement!.parentElement!;
          expect(empty.querySelector('h3')?.textContent).toBe(`No coins in ${presetFor(id).label} yet`);
          expect(empty.querySelector('p')?.textContent).toBe(body);
          expect(empty.querySelector('a[href="/create"]')?.textContent).toBe('Create coin');
          const browse = empty.querySelector('a[href="/explore?preset=new"]');
          if (id === 'new') expect(browse).toBeNull();
          else expect(browse?.textContent).toBe('Browse new coins');
        }
      }
    } finally {
      await stop(service);
    }
  } finally {
    await stop(redis);
  }
}

test(missingAndUnknownPresetsOpenTrending);
test(trendingHasTheDefaultAddressAndFirstTab);
test(loadingMarksTrendingAsTheActivePreset);
test(emptyPresetsExplainTheirListsAndOfferTheNextAction);
