import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { request } from 'node:http';
import { createServer } from 'node:https';
import type { AddressInfo } from 'node:net';
import { setTimeout as wait } from 'node:timers/promises';
import { chromium, type Locator, type Page, type Request as BrowserRequest } from 'playwright';
import { createPublicClient, encodeFunctionData, getAddress, http, parseAbi, parseEventLogs, type Address, type Hex } from 'viem';
import { decodeV2Contract } from '@kindle/sdk';
import { expect, test } from 'vitest';
import type { IChartingLibraryWidget } from '../../../public/charting_library/charting_library';
import { createReplayUdfDatafeed, type ChartAccessibleView } from './udf-datafeed';

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('Expected chart configuration');
  return Object.fromEntries(Object.entries(value));
}

function text(value: unknown): string {
  if (typeof value !== 'string' || !value) throw new Error('Expected chart configuration value');
  return value;
}

function loopback(value: unknown): string {
  const url = new URL(text(value));
  if (!['http:', 'https:'].includes(url.protocol) || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password) throw new Error('Private chart authority required');
  return url.origin;
}

function services() {
  const bytes = readFileSync(text(process.env.JOINED_CONFIG_FILE));
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  expect(process.env.JOINED_ALLOW_CHAIN_MUTATION).toBe('1');
  const config = object(JSON.parse(bytes.toString()));
  const chain = object(JSON.parse(readFileSync(text(process.env.JOINED_CHAIN_FILE), 'utf8')));
  expect(chain.realm).toBe('private-anvil125');
  expect(chain.productionActivation).toBe(false);
  const roles = object(object(chain.manifest).roles);
  return {
    api: loopback(config.api_origin),
    browser: loopback(config.browser_origin),
    application: loopback(config.stream_application_origin),
    rpc: loopback(config.rpc_url),
    pool: getAddress(text(chain.pool)),
    account: getAddress(text(chain.account)),
    router: getAddress(text(object(roles.Router).address)),
  };
}

async function chartBrowser(actual: ReturnType<typeof services>) {
  const origin = new URL(actual.application);
  if (origin.protocol !== 'https:') throw new Error('Secure chart application origin required');
  const upstream = new URL(actual.browser);
  const connections = new Set<() => void>();
  const server = createServer({ key: readFileSync(text(process.env.LAUNCHPAD_TLS_KEY)), cert: readFileSync(text(process.env.LAUNCHPAD_TLS_CERT)) }, (incoming, outgoing) => {
    const forwarded = request(new URL(incoming.url ?? '/', upstream), {
      method: incoming.method,
      headers: { ...incoming.headers, host: upstream.host, 'x-forwarded-host': origin.host, 'x-forwarded-proto': 'https' },
    }, response => {
      outgoing.writeHead(response.statusCode!, response.headers);
      response.pipe(outgoing);
    });
    forwarded.on('error', error => outgoing.destroy(error));
    incoming.pipe(forwarded);
  });
  server.on('connection', socket => {
    const close = () => { socket.destroy(); };
    connections.add(close);
    socket.on('close', () => connections.delete(close));
  });
  server.on('upgrade', (incoming, socket, head) => {
    const forwarded = request(new URL(incoming.url ?? '/', upstream), { headers: { ...incoming.headers, host: upstream.host } });
    forwarded.on('upgrade', (response, peer, peerHead) => {
      socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n${response.rawHeaders.reduce((headers, value, index) => headers + (index % 2 ? `${value}\r\n` : `${value}: `), '')}\r\n`);
      if (head.length) peer.write(head);
      if (peerHead.length) socket.write(peerHead);
      socket.pipe(peer); peer.pipe(socket);
      socket.on('close', () => peer.destroy());
      peer.on('error', error => socket.destroy(error));
    });
    forwarded.on('error', error => socket.destroy(error));
    socket.on('error', () => forwarded.destroy());
    forwarded.end();
  });
  await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(0, origin.hostname, resolve); });
  return {
    origin: origin.origin,
    resolve: `MAP ${origin.hostname}:${origin.port || 443} ${origin.hostname}:${(server.address() as AddressInfo).port}`,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      for (const close of connections) close();
    }),
  };
}

async function eventually(check: () => boolean | Promise<boolean>) {
  const deadline = performance.now() + 45_000;
  while (!await check()) {
    if (performance.now() >= deadline) throw new Error('Chart state did not converge');
    await wait(25);
  }
}

async function rpc(actual: ReturnType<typeof services>, method: string, params: unknown[]) {
  const response = await fetch(actual.rpc, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  expect(response.ok).toBe(true);
  const result = object(await response.json());
  expect(result.error).toBeUndefined();
  return result.result;
}

async function send(actual: ReturnType<typeof services>, to: Address, data: Hex) {
  const call = { from: actual.account, to, data };
  const gas = BigInt(text(await rpc(actual, 'eth_estimateGas', [call])));
  const hash = text(await rpc(actual, 'eth_sendTransaction', [{ ...call, gas: '0x' + ((gas * 5n + 3n) / 4n).toString(16) }]));
  if (!/^0x[0-9a-f]{64}$/i.test(hash)) throw new Error('Expected transaction hash');
  const receipt = await createPublicClient({ transport: http(actual.rpc) }).waitForTransactionReceipt({ hash: hash as Hex });
  expect(receipt.status).toBe('success');
  return receipt;
}

async function buy(actual: ReturnType<typeof services>, pool: Address) {
  const block = object(await rpc(actual, 'eth_getBlockByNumber', ['latest', false]));
  return send(actual, actual.router, encodeFunctionData({
    abi: parseAbi(['function buy(address pool,uint256 usdlAmountIn,uint256 minTokensOut,uint256 deadline) returns (uint256)']),
    functionName: 'buy', args: [pool, 1_000_000n, 1n, BigInt(text(block.timestamp)) + 600n],
  }));
}

type ObservedWindow = Window & typeof globalThis & {
  __chart?: {
    widget: IChartingLibraryWidget;
    feed: ReturnType<typeof createReplayUdfDatafeed>;
    ready: boolean;
    presentations: number;
  };
};

async function markCount(page: Page) {
  return page.evaluate(() => new Promise<number>(resolve => {
    const chart = (window as ObservedWindow).__chart;
    if (!chart) throw new Error('Ready chart required');
    chart.feed.getMarks({}, 0, 4_000_000_000, marks => resolve(marks.length));
  }));
}

async function activate(page: Page, control: Locator, touch: boolean) {
  if (touch) await control.tap();
  else { await control.focus(); await page.keyboard.press('Enter'); }
}

async function completeFirstVisit(page: Page, touch: boolean) {
  const rules = page.getByRole('dialog', { name: 'Platform rules', exact: true });
  await rules.waitFor();
  await activate(page, rules.getByRole('button', { name: 'I understand and agree', exact: true }), touch);
  await rules.waitFor({ state: 'hidden' });
  const decline = page.getByRole('button', { name: 'Decline', exact: true });
  await decline.waitFor();
  await activate(page, decline, touch);
  await decline.waitFor({ state: 'hidden' });
}

async function chartReadsTradesOnceWhileNothingChanges() {
  const actual = services();
  const created = await send(actual, actual.router, encodeFunctionData({
    abi: parseAbi(['function createMarket(string name,string symbol,uint8 feeStrategy,address optical) returns (address,address,uint256)']),
    functionName: 'createMarket', args: ['Chart Trades', 'CHART', 0, '0x0000000000000000000000000000000000000000'],
  }));
  const events = parseEventLogs({
    abi: parseAbi(['event MarketCreated(address indexed token,address indexed pool,address indexed creator,uint256 nftId)']),
    logs: created.logs.filter(log => log.address.toLowerCase() === actual.router.toLowerCase()),
  });
  expect(events).toHaveLength(1);
  const pool = events[0].args.pool;
  await buy(actual, pool);
  await send(actual, pool, encodeFunctionData({ abi: parseAbi(['function syncReserves() returns (uint256,uint256)']), functionName: 'syncReserves' }));
  await eventually(async () => {
    const response = await fetch(new URL(`/v2/pools/${pool.toLowerCase()}/trades?limit=50`, actual.api), { headers: { 'X-Schema-Version': '2' } });
    if (!response.ok) return false;
    const page = decodeV2Contract('TradesResponse', await response.json());
    return page.data.items.length === 1;
  });
  const now = Math.floor(Date.now() / 1000);
  await eventually(async () => {
    const url = new URL(`/v2/pools/${pool.toLowerCase()}/candles`, actual.api);
    for (const [key, value] of Object.entries({ interval: '1m', from: new Date((now - 3000) * 1000).toISOString(), to: new Date((now + 60) * 1000).toISOString(), limit: '50' })) url.searchParams.set(key, value);
    const response = await fetch(url, { headers: { 'X-Schema-Version': '2' } });
    if (!response.ok) return false;
    return decodeV2Contract('CandlesResponse', await response.json()).data.items.length === 1;
  });
  await eventually(async () => {
    const topic = `market.candles.${pool.toLowerCase()}.1m`;
    const url = new URL('/v2/snapshots', actual.api);
    url.searchParams.set('topics', topic);
    const response = await fetch(url, { headers: { 'X-Schema-Version': '2', Origin: actual.application } });
    if (!response.ok) return false;
    const snapshot = decodeV2Contract('TopicSnapshotReadResponse', await response.json()).data.snapshot;
    return snapshot.topics.length === 1 && snapshot.topics[0].topic === topic;
  });

  const application = await chartBrowser(actual);
  const browser = await chromium.launch({ args: [`--host-resolver-rules=${application.resolve}`] });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const page = await context.newPage();
    await page.addInitScript(() => {
      localStorage.setItem('sidiora_chart_settings_v1', JSON.stringify({ height: 400, interval: '1', mode: 'price', markers: true }));
      let api: Window['TradingView'];
      Object.defineProperty(window, 'TradingView', {
        configurable: true, get: () => api,
        set: value => {
          api = value;
          if (typeof value?.widget !== 'function') return;
          const constructor = value.widget;
          value.widget = new Proxy(constructor, {
            construct(target, args, newTarget) {
              const widget = Reflect.construct(target, args, newTarget) as IChartingLibraryWidget;
              const feed = args[0].datafeed as ReturnType<typeof createReplayUdfDatafeed>;
              const observation = { widget, feed, ready: false, presentations: 0 };
              (window as ObservedWindow).__chart = observation;
              const present = feed.setMarkPresentation;
              feed.setMarkPresentation = presentation => { observation.presentations++; present(presentation); };
              widget.onChartReady(() => { observation.ready = true; });
              return widget;
            },
          });
        },
      });
    });
    let tradeReads = 0;
    const pendingTradeReads = new Set<BrowserRequest>();
    page.on('request', request => {
      const url = new URL(request.url());
      if (url.pathname.toLowerCase() === `/v2/pools/${pool.toLowerCase()}/trades` && url.searchParams.get('limit') === '50') { tradeReads++; pendingTradeReads.add(request); }
    });
    page.on('requestfinished', request => pendingTradeReads.delete(request));
    page.on('requestfailed', request => pendingTradeReads.delete(request));
    await page.goto(new URL(`/coin/${pool}`, application.origin).href, { waitUntil: 'load' });
    await completeFirstVisit(page, false);
    await eventually(() => page.evaluate(() => {
      const chart = (window as ObservedWindow).__chart;
      return chart?.ready === true && chart.widget.activeChart().dataReady();
    }));
    await eventually(async () => await markCount(page) === 1);
    await eventually(() => page.evaluate(() => {
      const chart = (window as ObservedWindow).__chart;
      return chart?.feed.replayStatus().some(sub => sub.replayComplete) === true && chart.feed.resourceCounts().requests === 0;
    }));
    await eventually(() => pendingTradeReads.size === 0);
    expect(tradeReads).toBeGreaterThan(0);
    await page.clock.install();
    await page.clock.pauseAt(new Date(Date.now() + 100));
    const before = tradeReads;
    const presentations = await page.evaluate(() => (window as ObservedWindow).__chart!.presentations);
    for (let second = 0; second < 10; second++) {
      await page.evaluate(() => (window as ObservedWindow).__chart!.widget.activeChart().refreshMarks());
      await page.clock.runFor(1000);
      expect(await markCount(page)).toBe(1);
      expect(tradeReads).toBe(before);
    }
    await page.clock.resume();
    const block = object(await rpc(actual, 'eth_getBlockByNumber', ['latest', false]));
    const nextMinute = (BigInt(text(block.timestamp)) / 60n + 1n) * 60n;
    await rpc(actual, 'evm_setNextBlockTimestamp', [Number(nextMinute)]);
    await buy(actual, pool);
    await eventually(async () => await markCount(page) === 2);
    expect(tradeReads).toBe(before + 1);
    expect(await page.evaluate(() => (window as ObservedWindow).__chart!.presentations)).toBe(presentations);
    const options = page.getByRole('button', { name: 'Chart options', exact: true });
    await options.click();
    const hideMarkers = page.getByRole('menuitemradio', { name: 'Hide trade markers', exact: true });
    expect(await hideMarkers.isChecked()).toBe(true);
    await hideMarkers.click();
    await options.click();
    const showMarkers = page.getByRole('menuitemradio', { name: 'Show trade markers', exact: true });
    expect(await showMarkers.isChecked()).toBe(false);
    expect(await markCount(page)).toBe(0);
    expect(tradeReads).toBe(before + 1);
    await showMarkers.click();
    await options.click();
    expect(await hideMarkers.isChecked()).toBe(true);
    await page.keyboard.press('Escape');
    await eventually(async () => await markCount(page) === 2);
    expect(tradeReads).toBe(before + 2);
    await page.clock.runFor(1000);
    expect(tradeReads).toBe(before + 2);
  } finally { await browser.close(); await application.close(); }
}
test(chartReadsTradesOnceWhileNothingChanges, chartReadsTradesOnceWhileNothingChanges, 180_000);

async function chartPublishesOnlyChangedViews() {
  const actual = services();
  const views: ChartAccessibleView[] = [];
  const feed = createReplayUdfDatafeed(actual.pool, 'price', () => {}, undefined, { apiOrigin: actual.api, valuationOrigin: actual.browser }, view => views.push(view));
  const now = Math.floor(Date.now() / 1000);
  const period = { from: now - 3 * 86400, to: now + 60 };
  const history = () => new Promise<void>((resolve, reject) => feed.getBars({}, '1D', period, () => resolve(), reject));
  const marks = () => new Promise(resolve => feed.getMarks({}, period.from, period.to, resolve));
  try {
    await history();
    await marks();
    expect(views.length).toBeGreaterThan(0);
    expect(views.at(-1)?.markers.length).toBeGreaterThan(0);
    const count = views.length;
    await history();
    await marks();
    expect(views).toHaveLength(count);
    feed.setMarkPresentation({ enabled: false });
    feed.setMarkPresentation({ enabled: true });
    await marks();
    expect(views).toHaveLength(count);
  } finally { feed.dispose(); }
}
test(chartPublishesOnlyChangedViews, chartPublishesOnlyChangedViews, 60_000);
