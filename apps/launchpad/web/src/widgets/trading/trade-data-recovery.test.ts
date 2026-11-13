import { createServer, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import React, { act, createElement, useState } from 'react';
import { afterAll, expect, test } from 'vitest';
import { createPublicClient, decodeFunctionData, encodeFunctionResult, erc20Abi, http, type Address } from 'viem';
import { decodeV2Contract } from '@kindle/sdk';
import { createReplayUdfDatafeed, CHART_WINDOW_CANDLES, CHART_METADATA_DEADLINE_MS, type ChartAccessibleView, type ChartFreshness } from './udf-datafeed';
import { ChainTradeProvider, productionTradeDeployment } from './transaction-controls/providers';
import { parseAmount } from './transaction-controls/domain';
import { useLiveBalances, type LiveBalances } from './use-live-balances';

interface BrowserDom { window: Window & typeof globalThis & { close(): void } }
const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
globals.window = dom.window;
globals.document = dom.window.document;
globals.Event = dom.window.Event;
globals.CustomEvent = dom.window.CustomEvent;
globals.IS_REACT_ACT_ENVIRONMENT = true;
const { createRoot } = await import('react-dom/client');
const { AmountInput, ActionButton, convertByPrice } = await import('./trade-panel-parts');
const { useAlternateTradeAmount } = await import('./trade-panel');
afterAll(() => dom.window.close());

const core = JSON.parse(readFileSync(new URL('../../core/test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8'));
const charts = JSON.parse(readFileSync(new URL('../../core/test-fixtures/market-production/charts-wire.json', import.meta.url), 'utf8'));
const recordedPage = decodeV2Contract('CandlesResponse', core.candles.body);
const pool = recordedPage.data.items[0].poolAddress;
const token = recordedPage.data.items[0].open.baseAsset.address! as Address;
const usdl = recordedPage.data.items[0].open.quoteAsset.address! as Address;
const accountA = `0x${'1'.repeat(40)}` as Address;
const accountB = `0x${'2'.repeat(40)}` as Address;
const pause = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms));
async function until(check: () => boolean, timeout = 5000) {
  const deadline = Date.now() + timeout;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error('Observable production result did not arrive');
    await act(async () => { await pause(10); });
  }
}
async function listen(server: Server) {
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}
async function close(server: Server) {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

async function alternateSellInputPreservesExactUnitsAndInvalidText() {
  let raw: bigint | undefined;
  let submitted = 0;
  function Surface({ decimals }: { decimals: number }) {
    const [amount, setAmount] = useState('');
    const alternate = useAlternateTradeAmount(text => convertByPrice(text, '3', 'usdl', decimals), setAmount);
    const parsed = parseAmount(amount, decimals);
    raw = parsed.parsedRaw;
    const valid = !alternate.invalid && !parsed.errorCode && raw !== undefined && raw > 0n;
    return createElement('form', { onSubmit: event => event.preventDefault() },
      createElement(AmountInput, { amount: alternate.altText, onChange: alternate.change, inputDecimals: decimals, unitLabel: 'USDL', errorCode: alternate.invalid ? 'invalid_decimal' : parsed.errorCode }),
      createElement(ActionButton, { side: 'sell', state: valid ? { kind: 'ready', side: 'sell', tokenSymbol: 'Coin' } : { kind: 'enter-amount' }, onClick: () => { if (valid) submitted++; } }));
  }
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container);
  const edit = async (text: string) => {
    const input = container.querySelector('input')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(input, text);
      input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    });
    return input;
  };
  try {
    for (const decimals of [37, 60, 77]) {
      await act(async () => root.render(createElement(Surface, { decimals, key: decimals })));
      const input = await edit('1');
      expect(input.value).toBe('1');
      expect(raw).toBe(10n ** BigInt(decimals) / 3n);
      expect(input.getAttribute('aria-invalid')).toBeNull();
      expect(container.querySelector('button')!.disabled).toBe(false);
      await act(async () => container.querySelector('button')!.click());
      for (const malformed of ['1e2', '1..2', ' 1', '1.', '-1']) {
        expect((await edit(malformed)).value).toBe(malformed);
        expect(raw).toBeUndefined();
        expect(input.getAttribute('aria-invalid')).toBe('true');
        expect(container.querySelector('button')!.disabled).toBe(true);
        await act(async () => container.querySelector('button')!.click());
      }
    }
    expect(submitted).toBe(3);
    // No intermediate truncation before multiplication, including scales above 36.
    expect(convertByPrice(`0.${'0'.repeat(59)}1`, '1000000000000000000000000000000000000000000000000000000000000', 'token', 6)).toBe('1');
    expect(convertByPrice('12', '3', 'token', 0)).toBe('36');
    await act(async () => root.render(createElement(Surface, { decimals: 78, key: 78 })));
    expect((await edit('1')).getAttribute('aria-invalid')).toBe('true');
    expect(raw).toBeUndefined();
    expect(container.querySelector('button')!.disabled).toBe(true);
  } finally { await act(async () => root.unmount()); container.remove(); }
}
test(alternateSellInputPreservesExactUnitsAndInvalidText.name, alternateSellInputPreservesExactUnitsAndInvalidText);

/** A bounded in-memory candle store. Canonical records use the committed producer transcript;
 * the store owns pagination, bucket keys and correction revisions across feed restarts. */
async function candleWindowPaginatesPansCorrectsAndRecovers() {
  const start = Date.parse(recordedPage.data.items[0].start);
  const store = new Map<number, typeof recordedPage.data.items[number]>();
  for (let index = 0; index < 7500; index++) store.set(index, decodeV2Contract('Candle', { ...recordedPage.data.items[0], start: new Date(start + index * 60000).toISOString() }));
  let revision = '2';
  let stale = false;
  let pages = 0;
  const server = createServer((request, response) => {
    const url = new URL(request.url!, 'http://localhost');
    response.setHeader('Content-Type', 'application/json'); response.setHeader('X-Schema-Version', '2');
    if (!url.pathname.endsWith('/candles')) { response.writeHead(503); response.end('{}'); return; }
    pages++;
    const from = Date.parse(url.searchParams.get('from')!), to = Date.parse(url.searchParams.get('to')!);
    const matching = [...store.values()].filter(candle => Date.parse(candle.start) >= from && Date.parse(candle.start) < to);
    const offset = Number(url.searchParams.get('cursor') ?? 0), limit = Number(url.searchParams.get('limit'));
    const items = matching.slice(offset, offset + limit).map(candle => stale ? { ...candle, revision: '2', volumeUsdlRaw: recordedPage.data.items[0].volumeUsdlRaw } : candle);
    const page = decodeV2Contract('CandlesResponse', { ...recordedPage, revision: stale ? '2' : revision, data: { ...recordedPage.data, items }, nextCursor: offset + limit < matching.length ? String(offset + limit) : null });
    response.end(JSON.stringify(page));
  });
  const origin = await listen(server);
  let view: ChartAccessibleView | undefined;
  const open = () => createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin }, value => { view = value; });
  let feed = open();
  const read = (from: number, count: number) => new Promise<Array<{ time: number; volume: number }>>((resolve, reject) => feed.getBars({}, '1', { from: start / 1000 + from * 60, to: start / 1000 + (from + count) * 60, countBack: count }, resolve, error => reject(new Error(error))));
  try {
    const first = await read(0, 5000);
    expect(first).toHaveLength(CHART_WINDOW_CANDLES);
    expect(pages).toBe(100);
    expect(view!.rows).toHaveLength(CHART_WINDOW_CANDLES);
    const correctedIndex = 4999;
    store.set(correctedIndex, { ...store.get(correctedIndex)!, revision: '3', volumeUsdlRaw: '2000000' }); revision = '3';
    expect((await read(correctedIndex, 1))[0].volume).toBe(2);
    const correction = store.get(correctedIndex)!;
    store.set(correctedIndex, { ...correction, revision: '2', volumeUsdlRaw: recordedPage.data.items[0].volumeUsdlRaw });
    expect((await read(correctedIndex, 1))[0].volume).toBe(2);
    store.set(correctedIndex, { ...correction, volumeUsdlRaw: '3000000' });
    await expect(read(correctedIndex, 1)).rejects.toThrow();
    store.set(correctedIndex, correction);
    await read(5000, 2500);
    expect(view!.rows).toHaveLength(CHART_WINDOW_CANDLES);
    expect(view!.rows.find(row => row.start === store.get(correctedIndex)!.start)?.revision).toBe('3');
    // A stale projection cannot reintroduce evicted provenance or overwrite a correction.
    stale = true;
    await expect(read(0, 2500)).rejects.toThrow();
    expect(view!.rows.find(row => row.start === store.get(correctedIndex)!.start)?.revision).toBe('3');
    stale = false;
    for (const from of [0, 5000, 0]) {
      expect(await read(from, 2500)).toHaveLength(2500);
      expect(view!.rows.length).toBeLessThanOrEqual(CHART_WINDOW_CANDLES);
    }
    feed.retry();
    expect((await read(correctedIndex, 1))[0].volume).toBe(2);
    feed.dispose(); expect(feed.resourceCounts().requests).toBe(0);
    feed = open();
    const recovered = await read(0, 5000);
    expect(recovered).toHaveLength(CHART_WINDOW_CANDLES);
    expect(recovered[correctedIndex].volume).toBe(2);
    expect(view!.rows.find(row => row.start === store.get(correctedIndex)!.start)?.revision).toBe('3');
  } finally { feed.dispose(); await close(server); }
}
test(candleWindowPaginatesPansCorrectsAndRecovers.name, candleWindowPaginatesPansCorrectsAndRecovers, 60000);

async function metadataRejectsStallsCancelsAndRetries() {
  let mode: 'reject' | 'invalid' | 'stall' | 'success' = 'reject';
  let aborted = 0;
  const held: ServerResponse[] = [];
  const server = createServer((request, response) => {
    const path = request.url!;
    response.setHeader('Content-Type', 'application/json');
    if (mode === 'stall') { held.push(response); response.on('close', () => { if (!response.writableEnded) aborted++; }); return; }
    if (mode === 'reject') { response.writeHead(503); response.end('{}'); return; }
    const recorded = path === '/v2/udf/config' ? charts.config : charts.time;
    response.end(mode === 'invalid' ? (path === '/v2/udf/config' ? '{"supports_time":false}' : '"invalid"') : recorded.bodyBytes);
  });
  const origin = await listen(server);
  const failures: Array<string | null> = [], freshness: ChartFreshness[] = [];
  const feed = createReplayUdfDatafeed(pool, 'price', value => freshness.push(value), undefined, { apiOrigin: origin, valuationOrigin: origin }, () => {}, () => {}, reason => failures.push(reason));
  const configs: Record<string, unknown>[] = [], times: number[] = [];
  try {
    feed.onReady(value => configs.push(value)); feed.getServerTime(value => times.push(value));
    await until(() => failures.filter(Boolean).length === 2);
    expect(configs).toEqual([]); expect(times).toEqual([]);
    expect(freshness.at(-1)?.connectionStatus).toBe('offline');
    mode = 'invalid'; feed.retry();
    await until(() => failures.filter(Boolean).length === 4);
    expect(configs).toEqual([]); expect(times).toEqual([]);
    mode = 'stall'; feed.retry();
    await until(() => held.length === 2);
    await until(() => failures.filter(Boolean).length === 6, CHART_METADATA_DEADLINE_MS + 5000);
    await until(() => aborted === 2);
    expect(feed.resourceCounts().requests).toBe(0);
    expect(configs).toEqual([]); expect(times).toEqual([]);
    mode = 'success'; feed.retry();
    await until(() => configs.length === 1 && times.length === 1);
    expect(configs[0]).toMatchObject({ supports_time: true, supports_marks: true });
    expect(times).toEqual([Number(charts.time.bodyBytes)]);
    expect(failures.at(-1)).toBeNull();
    expect(freshness.at(-1)?.connectionStatus).toBe('connecting');
    mode = 'stall'; feed.onReady(value => configs.push(value)); feed.getServerTime(value => times.push(value));
    await until(() => held.length === 4);
    feed.dispose();
    await until(() => aborted === 4);
    expect(configs).toHaveLength(1); expect(times).toHaveLength(1);
    expect(feed.resourceCounts().requests).toBe(0);
  } finally { feed.dispose(); await close(server); }
}
test(metadataRejectsStallsCancelsAndRetries.name, metadataRejectsStallsCancelsAndRetries, 30000);

/** The RPC peer is an in-memory ERC-20 balance store using the real ABI on both sides. */
async function slowBalanceRpcSettlesAcrossRefreshIdentityAndVisibility() {
  let delay = 5100, stall = false, active = 0, maximum = 0, calls = 0, aborted = 0;
  const balances = new Map([[accountA.toLowerCase(), 123n], [accountB.toLowerCase(), 456n]]);
  const server = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', (chunk: Buffer) => chunks.push(chunk));
    request.on('end', () => {
      const body = JSON.parse(Buffer.concat(chunks).toString());
      calls++; active++; maximum = Math.max(maximum, active);
      let timer: ReturnType<typeof setTimeout> | undefined;
      response.on('close', () => { active--; clearTimeout(timer); if (!response.writableEnded) aborted++; });
      const decoded = decodeFunctionData({ abi: erc20Abi, data: body.params[0].data });
      if (body.method !== 'eth_call' || decoded.functionName !== 'balanceOf') { response.writeHead(400); response.end(); return; }
      const balance = balances.get(decoded.args[0].toLowerCase());
      if (balance === undefined) { response.writeHead(400); response.end(); return; }
      if (stall) return;
      timer = setTimeout(() => {
        response.setHeader('Content-Type', 'application/json');
        response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: encodeFunctionResult({ abi: erc20Abi, functionName: 'balanceOf', result: balance }) }));
      }, delay);
    });
  });
  const origin = await listen(server);
  const client = createPublicClient({ transport: http(origin, { retryCount: 0, timeout: 20000 }) });
  const provider = new ChainTradeProvider(client, undefined, productionTradeDeployment);
  let latest: LiveBalances | undefined;
  function Surface({ account, scope = 'pool-a' }: { account: Address; scope?: string }) {
    latest = useLiveBalances({ reader: provider, usdl, token, account, enabled: true, scope, intervalMs: 4000 });
    return createElement('output', null, latest.usdl?.toString() ?? 'waiting');
  }
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container);
  const visibility = (state: 'hidden' | 'visible') => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: state });
    document.dispatchEvent(new dom.window.Event('visibilitychange'));
  };
  try {
    await act(async () => root.render(createElement(Surface, { account: accountA })));
    await until(() => calls === 2);
    await act(async () => { latest!.refresh(); latest!.refresh(); await pause(4200); });
    expect(calls).toBe(2); expect(latest!.usdl).toBeNull();
    await until(() => latest!.usdl === 123n);
    expect(container.textContent).toBe('123'); expect(maximum).toBe(2);
    // An identity change cancels the actual HTTP requests, including a hung transport.
    stall = true;
    await act(async () => latest!.refresh()); await until(() => active === 2);
    delay = 20; stall = false;
    await act(async () => root.render(createElement(Surface, { account: accountB })));
    expect(latest!.usdl).toBeNull(); expect(container.textContent).toBe('waiting');
    await until(() => latest!.usdl === 456n && aborted === 2);
    expect(latest!.token).toBe(456n);
    visibility('hidden'); const beforeHidden = calls;
    await act(async () => { latest!.refresh(); await pause(4100); });
    expect(calls).toBe(beforeHidden);
    visibility('visible'); await until(() => calls === beforeHidden + 2 && active === 0);
    // A settled timeout exposes an error and permits a later visible refresh.
    stall = true;
    await act(async () => latest!.refresh()); await until(() => active === 2);
    await until(() => latest!.error !== undefined, 20000);
    await until(() => active === 0);
    expect(aborted).toBe(4);
    stall = false; visibility('hidden'); visibility('visible');
    await until(() => latest!.error === undefined);
    expect(latest!.usdl).toBe(456n);
    // Session/pool changes invalidate even when the wallet address is unchanged.
    stall = true; await act(async () => latest!.refresh()); await until(() => active === 2);
    stall = false;
    await act(async () => root.render(createElement(Surface, { account: accountB, scope: 'pool-b' })));
    expect(latest!.usdl).toBeNull();
    await until(() => latest!.usdl === 456n && aborted === 6);
  } finally { await act(async () => root.unmount()); container.remove(); visibility('visible'); await close(server); }
}
test(slowBalanceRpcSettlesAcrossRefreshIdentityAndVisibility.name, slowBalanceRpcSettlesAcrossRefreshIdentityAndVisibility, 45000);
