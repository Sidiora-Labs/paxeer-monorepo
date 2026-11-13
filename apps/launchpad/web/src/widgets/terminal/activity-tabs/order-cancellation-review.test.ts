import { createServer, type IncomingHttpHeaders } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { createHash } from 'node:crypto';
import React, { act } from 'react';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';
import { Operations, SdkClient, decodeV2Contract } from '@kindle/sdk';
import { createPublicClient, createWalletClient, encodeFunctionData, http } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import type { Order, OrderCancelResponse, OrderListResponse, TransactionRequest } from '../../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';
import { advancedOrdersAbi, createAdvancedProvider, type AdvancedManifest } from '@/widgets/meta-ag/advanced-provider';

// Keep Node's fetch and AbortController for the real SDK's loopback transport.
interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
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
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { accountEpoch, accountSession, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');
const { OrdersList, terminalOrdersKey } = await import('./orders-panel');
const { admitOrderCancellation, OrderCancellationUnavailableError } = await import('./order-cancellation-review');
const { paxeerWalletChain } = await import('@/core/wallet-sdk/client');

const wallet = privateKeyToAccount(`0x${'11'.repeat(32)}`);
const pool = `0x${'33'.repeat(20)}`;
const orderId = '7f1c2a3b-4d5e-4f60-8a1b-2c3d4e5f6a7b';
const requestId = '5b8f0c1e-2d3a-4b5c-8d6e-7f8091a2b3c4';
const activeOrder: Order = { chainOrderId: '7', inputRaw: '5000000', kind: 'limit', orderId, poolAddress: pool, status: 'active', triggerPrice: null, wallet: wallet.address };
const transactionRequest: TransactionRequest = {
  chainId: '125', from: wallet.address, to: `0x${'44'.repeat(20)}`, valueRaw: '0',
  data: encodeFunctionData({ abi: advancedOrdersAbi, functionName: 'cancelLimitOrder', args: [7n] }),
  gasLimitRaw: null, maxFeePerGasRaw: null, maxPriorityFeePerGasRaw: null,
};
const metadata = { generatedAt: '2026-10-13T10:00:00.000Z', nextCursor: null, quality: 'complete' as const, requestId, revision: null, watermark: null };

/** In-memory HTTP contract fixture; no admitted successful API producer exists yet. */
class OrderContractStore {
  mode: 'prepared' | 'confirmed' | 'unconfirmed' | 'unavailable' | 'mismatch' = 'prepared';
  cancelled = false;
  readonly seen: { method: string; url: string; headers: IncomingHttpHeaders }[] = [];
  readonly prepared = new Map<string, OrderCancelResponse>();
  read(): OrderListResponse { return decodeV2Contract('OrderListResponse', { ...metadata, data: { items: this.cancelled ? [] : [activeOrder] } }); }
  cancel(key: string): OrderCancelResponse {
    const prior = this.prepared.get(key);
    if (prior) return prior;
    this.cancelled = this.mode === 'confirmed';
    const result = decodeV2Contract('OrderCancelResponse', {
      ...metadata,
      data: {
        order: { ...activeOrder, ...(this.cancelled ? { status: 'cancelled' } : {}), ...(this.mode === 'mismatch' ? { chainOrderId: '8' } : {}) },
        transactionRequest: this.mode === 'prepared' ? transactionRequest : null,
      },
    });
    // Failed/unconfirmed fixture responses are not completed preparations.
    if (this.mode === 'prepared' || this.mode === 'confirmed') this.prepared.set(key, result);
    return result;
  }
}
let store = new OrderContractStore();
const server = createServer((request, response) => {
  const url = request.url ?? '';
  store.seen.push({ method: request.method ?? '', url, headers: request.headers });
  response.setHeader('Content-Type', 'application/json');
  response.setHeader('X-Schema-Version', '2');
  response.setHeader('X-Request-ID', requestId);
  if (request.method === 'GET' && url === '/v2/me/orders?status=active&limit=100') { response.end(JSON.stringify(store.read())); return; }
  if (request.method === 'POST' && url === `/v2/me/orders/${orderId}:cancel`) {
    if (store.mode === 'unavailable') {
      response.statusCode = 503;
      // WriteV2NotConfiguredError's explicit, nonretryable dependency boundary.
      response.end(JSON.stringify({ code: 'DEPENDENCY_UNAVAILABLE', message: 'Dependency unavailable', requestId, retryable: false, retryAfterMs: null, fieldErrors: [], details: null }));
    } else response.end(JSON.stringify(store.cancel(String(request.headers['idempotency-key']))));
    return;
  }
  if (request.method === 'POST' && url === '/rpc') {
    let body = '';
    request.on('data', chunk => { body += String(chunk); });
    request.on('end', () => {
      const rpc = JSON.parse(body) as { id: number; method: string };
      response.end(JSON.stringify(rpc.method === 'eth_chainId' ? { jsonrpc: '2.0', id: rpc.id, result: '0x7d' } : { jsonrpc: '2.0', id: rpc.id, error: { code: -32601, message: 'No deployed orders service' } }));
    });
    return;
  }
  response.statusCode = 404; response.end();
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
const operations = new Operations(new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: origin.replace('http:', 'ws:'), approvedStreamOrigins: [origin.replace('http:', 'ws:')], production: false, session: { mode: 'bearer', token: () => accountSession()?.accessToken } }));

let mounted: ReturnType<typeof mount> | undefined;
function mount(cancellationProvider?: Awaited<ReturnType<typeof createAdvancedProvider>>) {
  const scope = accountSession();
  if (!scope) throw new Error('Fixture account must be installed');
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(React.createElement(QueryClientProvider, { client }, React.createElement(OrdersList, { operations, scope, poolAddress: pool, cancellationProvider }))));
  return { root, container, client, scope };
}
function button(label: string): HTMLButtonElement {
  const match = Array.from(document.body.querySelectorAll('button')).find(button => button.textContent === label);
  if (!match) throw new Error(`Missing button ${label}: ${document.body.textContent}`);
  return match;
}
async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error(`Condition timed out: ${document.body.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}
const posts = () => store.seen.filter(entry => entry.method === 'POST' && entry.url.endsWith(':cancel'));
const reads = () => store.seen.filter(entry => entry.method === 'GET');
async function openCancellation() {
  await waitFor(() => !!mounted?.container.querySelector('[data-order-id]'));
  act(() => button('Cancel').click());
  expect(posts()).toHaveLength(0);
}
beforeEach(() => {
  store = new OrderContractStore();
  installAccountSession({ accessExpiresAt: new Date(Date.now() + 3600000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86400000).toISOString(), revokedAt: null, scopes: ['account'], sessionId: 'sess-order-repair', deviceLabel: 'Web browser', wallet: wallet.address }, 'order-repair-token', accountEpoch());
});
afterEach(() => {
  if (mounted) { act(() => mounted?.root.unmount()); mounted.client.clear(); mounted.container.remove(); mounted = undefined; }
  clearAccountSession(false);
});
afterAll(async () => { await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); dom.window.close(); });

async function orderCancelWalletStepRetainsPreparedRequestUntilConfirmed() {
  mounted = mount();
  await openCancellation();
  act(() => { const confirm = button('Cancel order'); confirm.click(); confirm.click(); });
  await waitFor(() => !!document.querySelector('[aria-label="Wallet cancellation review"]'));
  await waitFor(() => document.body.textContent?.includes('Wallet cancellation is not available yet') ?? false);
  expect(posts()).toHaveLength(1);
  expect(posts()[0].headers.authorization).toBe('Bearer order-repair-token');
  expect(posts()[0].headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
  const review = document.querySelector('[aria-label="Wallet cancellation review"]');
  expect(review?.textContent).toContain(transactionRequest.from);
  expect(review?.textContent).toContain(transactionRequest.to);
  expect(review?.textContent).toContain(orderId);
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  expect(mounted.container.querySelector('[data-order-id]')).not.toBeNull();
  expect(mounted.client.getQueryState(terminalOrdersKey(mounted.scope))?.isInvalidated).toBe(false);
  expect(reads()).toHaveLength(1);
  expect(document.body.textContent).not.toContain('Your order is almost cancelled');
  expect(document.body.textContent).not.toContain('Finish cancelling it from your wallet');
  act(() => button('Keep order').click());
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  expect(reads()).toHaveLength(1);
}
test(orderCancelWalletStepRetainsPreparedRequestUntilConfirmed);

async function orderCancelWalletStepKeepsUnavailableExplicitAndRetryIdempotent() {
  store.mode = 'unavailable';
  mounted = mount();
  await openCancellation();
  act(() => button('Cancel order').click());
  await waitFor(() => document.body.textContent?.includes('Order cancellation') ?? false);
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  expect(reads()).toHaveLength(1);
  expect(posts()).toHaveLength(1);
  const key = posts()[0].headers['idempotency-key'];
  store.mode = 'prepared';
  act(() => button('Cancel order').click());
  await waitFor(() => !!document.querySelector('[aria-label="Wallet cancellation review"]'));
  expect(posts()).toHaveLength(2);
  expect(posts()[1].headers['idempotency-key']).toBe(key);
  expect(reads()).toHaveLength(1);
}
test(orderCancelWalletStepKeepsUnavailableExplicitAndRetryIdempotent);

async function orderCancelWalletStepRefreshesOnlyConfirmedApiOutcome() {
  store.mode = 'unconfirmed';
  mounted = mount();
  await openCancellation();
  act(() => button('Cancel order').click());
  await waitFor(() => document.body.textContent?.includes('The order could not be cancelled right now.') ?? false);
  expect(reads()).toHaveLength(1);
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  const key = posts()[0].headers['idempotency-key'];
  store.mode = 'confirmed';
  act(() => button('Cancel order').click());
  await waitFor(() => !!mounted?.container.textContent?.includes('No open orders'));
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  expect(posts()).toHaveLength(2);
  expect(posts()[1].headers['idempotency-key']).toBe(key);
  expect(reads()).toHaveLength(2);
}
test(orderCancelWalletStepRefreshesOnlyConfirmedApiOutcome);

async function orderCancelWalletStepRejectsMismatchedResponse() {
  store.mode = 'mismatch';
  mounted = mount();
  await openCancellation();
  act(() => button('Cancel order').click());
  await waitFor(() => document.body.textContent?.includes('The order could not be cancelled right now.') ?? false);
  expect(document.querySelector('[aria-label="Wallet cancellation review"]')).toBeNull();
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  expect(reads()).toHaveLength(1);
  expect(mounted.container.querySelector('[data-order-id]')).not.toBeNull();
}
test(orderCancelWalletStepRejectsMismatchedResponse);

async function orderCancelWalletStepUsesRealUnavailableProvider() {
  const manifest: AdvancedManifest = { version: 2, chainId: 125, revision: 'orders-unavailable', deploymentBlock: '0', confirmations: 1, usdl: `0x${'55'.repeat(20)}`, priceDenomination: `0x${'55'.repeat(20)}`, orders: null, router: null, quoter: null, vault: null, adapters: [], priceScale: 18, supportedKinds: [], maximumOrderLifetimeSeconds: '3600', keeperHealthUrl: null };
  const manifestJson = JSON.stringify(manifest);
  const provider = await createAdvancedProvider({
    publicClient: createPublicClient({ chain: paxeerWalletChain, transport: http(`${origin}/rpc`, { retryCount: 0 }) }),
    walletClient: createWalletClient({ account: wallet, chain: paxeerWalletChain, transport: http(`${origin}/rpc`, { retryCount: 0 }) }),
    manifestJson, manifestSha256: createHash('sha256').update(manifestJson).digest('hex'), attempts: window.localStorage,
  });
  expect(provider.blockers.get('orders')).toContain('no verified active deployment');
  const scope = accountSession();
  if (!scope) throw new Error('Fixture session missing');
  await expect(admitOrderCancellation(transactionRequest, activeOrder, scope, provider)).rejects.toBeInstanceOf(OrderCancellationUnavailableError);
  mounted = mount(provider);
  await openCancellation();
  act(() => button('Cancel order').click());
  await waitFor(() => document.body.textContent?.includes('Wallet cancellation is not available yet') ?? false);
  expect(document.body.textContent).not.toContain('Continue in wallet');
  expect(store.seen.filter(entry => entry.url === '/rpc')).toHaveLength(2);
  expect(reads()).toHaveLength(1);
}
test(orderCancelWalletStepUsesRealUnavailableProvider);
