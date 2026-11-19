import { createServer, request as forwardRequest } from 'node:http';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import React, { act, createElement, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterAll, afterEach, expect, test } from 'vitest';
import { Operations, SdkClient, decodeV2Contract } from '@kindle/sdk';
import { createConfig, WagmiProvider } from 'wagmi';
import { createPublicClient, custom, erc20Abi, formatUnits, http, type Address } from 'viem';
import { MarketClientContext } from '@/core/market/client';
import { resolvePoolSnapshot, type PoolSnapshot } from '@/core/market/pool-resolver';
import { statsMarketCap, statsSpotPrice } from '@/core/market/pool-stats-display';
import { queryKeys } from '@/core/query-keys';
import { serveRecorded, v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import TokenDescription from '@/widgets/trading/token-description';
import AboutTab from '@/widgets/coin/about-tab';
import SwapPanel, { loadSwapTokens, readSwapQuote, TokenInputBox } from '@/widgets/swap/swap-panel';
import { parseAmount, type TradeContext } from '@/widgets/trading/transaction-controls/domain';
import { creatorFixture } from '@/widgets/my-token/creator.fixture';
import { CreatorPoolInfo } from '@/widgets/my-token/my-token-view';
import { currentTradeQuote } from '@/widgets/trading/use-trade-panel';
import { tradePreviewState } from '@/widgets/trading/trade-panel';
import { ChainTradeProvider } from '@/widgets/trading/transaction-controls/providers';
import { closeAuthModal, useAuthModal } from '@/shell/auth-modal-store';
import { applyBrowserSurface } from '@/core/app-foundation/browser-surface.test-support';
import { paxeerWalletChain } from '@/core/wallet-sdk/client';
import { homeBalanceReader, marketWalletChain, type HomeChain } from '@/widgets/trade/home-chain';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => { window: Window & typeof globalThis & { close(): void } };
};
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: process.env.NEXT_PUBLIC_DATA_API_URL!, pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
const installed: string[] = [];
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in browserGlobals || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
  installed.push(key);
}
const originalEvents = { Event: globalThis.Event, CustomEvent: globalThis.CustomEvent };
Object.assign(globalThis, { Event: dom.window.Event, CustomEvent: dom.window.CustomEvent, IS_REACT_ACT_ENVIRONMENT: true });
afterAll(() => {
  dom.window.close();
  for (const key of installed) delete browserGlobals[key];
  Object.assign(globalThis, originalEvents);
  delete browserGlobals.IS_REACT_ACT_ENVIRONMENT;
});
const initialEnvironment = { ...process.env };
afterEach(() => {
  process.env = { ...initialEnvironment };
  closeAuthModal();
});
const pool = v2Reads.pools.real;
const recorded = v2Reads.responses;

function sdk(origin: string, mediaOrigin = origin) {
  const streamOrigin = origin.replace(/^http/, 'ws');
  return new SdkClient({ apiOrigin: origin, mediaOrigin, identityOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
}

/** Forward service bytes unchanged; a disconnected request has no HTTP answer. */
async function readRelay(origin: string) {
  const requests: string[] = [];
  let disconnectedPath: string | null = null;
  let heldPath: string | null = null;
  const server = createServer((request, response) => {
    const path = request.url ?? '/';
    requests.push(path);
    if (disconnectedPath && path.startsWith(disconnectedPath)) { request.socket.destroy(); return; }
    if (heldPath && path.startsWith(heldPath)) return;
    const upstream = forwardRequest(new URL(path, origin), { method: request.method, headers: { ...request.headers, host: new URL(origin).host } }, reply => {
      response.writeHead(reply.statusCode!, reply.headers);
      reply.pipe(response);
    });
    upstream.on('error', () => response.destroy());
    response.on('close', () => upstream.destroy());
    request.pipe(upstream);
  });
  await new Promise<void>(resolve => server.listen(0, new URL(origin).hostname, resolve));
  const address = new URL(origin);
  address.port = String((server.address() as AddressInfo).port);
  return {
    origin: address.origin,
    requests,
    disconnect(path: string | null) { disconnectedPath = path; },
    hold(path: string | null) { heldPath = path; },
    async close() { server.closeAllConnections(); await new Promise<void>(resolve => server.close(() => resolve())); },
  };
}

async function until(predicate: () => boolean, timeout = 15000) {
  const deadline = Date.now() + timeout;
  while (!predicate()) {
    if (Date.now() > deadline) throw new Error('Token trade view did not settle');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

async function mount(node: ReactNode, client: SdkClient, query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })) {
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  try {
    await act(async () => root.render(createElement(MarketClientContext.Provider, { value: client }, createElement(QueryClientProvider, { client: query }, node))));
  } catch (error) {
    await act(async () => root.unmount());
    query.clear();
    host.remove();
    throw error;
  }
  return {
    host, query,
    async close() { await act(async () => root.unmount()); query.clear(); host.remove(); },
  };
}

function seedSnapshot(snapshot: PoolSnapshot) {
  const query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  query.setQueryData(queryKeys.poolSnapshot(pool), snapshot);
  return query;
}

async function tokenMetadataFailureKeepsMarketFactsAndOffersRetry() {
  const replay = await serveRecorded([recorded.poolDetailReal]);
  const relay = await readRelay(replay.origin);
  relay.disconnect('/v2/tokens/');
  const client = sdk(replay.origin, relay.origin);
  let view: Awaited<ReturnType<typeof mount>> | undefined;
  try {
    const snapshot = await resolvePoolSnapshot(pool, { operations: new Operations(client) });
    expect(snapshot.stats).toEqual(decodeV2Contract('PoolDetailResponse', recorded.poolDetailReal.body).data.stats);
    expect(snapshot.token?.address).toBe(v2Reads.pools.realToken);
    expect(snapshot.metadata).toBeNull();
    expect(snapshot.metadataError).toBeTruthy();
    view = await mount(createElement(TokenDescription, { poolAddress: pool }), client, seedSnapshot(snapshot));
    expect(view.host.textContent).toContain(snapshot.metadataError);
    expect(view.host.textContent).not.toContain("The creator hasn't added one");
    const before = relay.requests.length;
    relay.disconnect(null);
    await act(async () => view!.host.querySelector('button')!.click());
    await until(() => relay.requests.length > before
      && view!.query.getQueryState(queryKeys.poolSnapshot(pool))?.fetchStatus === 'idle'
      && view!.host.textContent?.includes(snapshot.metadataError!) === false);
    expect(view.query.getQueryData<PoolSnapshot>(queryKeys.poolSnapshot(pool))?.metadataError).toBeUndefined();
    expect(view.host.textContent).not.toContain(snapshot.metadataError);
  } finally {
    await view?.close();
    await relay.close();
    await replay.close();
  }
}

async function descriptionLinksUseThePublishedSocialDestinations() {
  const replay = await serveRecorded([recorded.poolDetailReal]);
  let view: Awaited<ReturnType<typeof mount>> | undefined;
  try {
    const client = sdk(replay.origin);
    const snapshot = await resolvePoolSnapshot(pool, { operations: new Operations(client) });
    expect(snapshot.token).not.toBeNull();
    const metadata = decodeV2Contract('MetadataGetData', {
      token: snapshot.token, displayName: null, displaySymbol: null,
      description: 'Coin details', tags: [], revision: '1',
      socialLinks: [{ label: 'X', url: 'https://x.com/sidiora' }, { label: 'Telegram', url: 'https://t.me/Sidiorafunsupport' }],
    });
    view = await mount(createElement(TokenDescription, { poolAddress: pool }), client, seedSnapshot({ ...snapshot, metadata }));
    expect([...view.host.querySelectorAll('a')].map(link => link.href)).toEqual(metadata.socialLinks.map(link => link.url));
    for (const link of view.host.querySelectorAll('a')) expect(link.rel).toContain('noopener');
  } finally { await view?.close(); await replay.close(); }
}

async function failedPoolReadEndsChatLoadingWithRecovery() {
  const replay = await serveRecorded([recorded.poolDetailReal]);
  const relay = await readRelay(replay.origin);
  relay.disconnect('/v2/pools/');
  const client = sdk(relay.origin);
  const query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  let view: Awaited<ReturnType<typeof mount>> | undefined;
  try {
    await expect(query.fetchQuery({ queryKey: queryKeys.poolSnapshot(pool), queryFn: ({ signal }) => resolvePoolSnapshot(pool, { operations: new Operations(client) }, signal) })).rejects.toThrow();
    view = await mount(createElement(AboutTab, { poolAddress: pool }), client, query);
    expect(view.host.querySelector('[data-tab-unavailable="Chat"]')).not.toBeNull();
    expect(view.host.textContent).not.toContain('Chat opens once');
    const before = relay.requests.length;
    relay.disconnect(null);
    relay.hold('/v2/pools/');
    await act(async () => view!.host.querySelector<HTMLButtonElement>('[data-tab-unavailable="Chat"] button')!.click());
    await until(() => relay.requests.length > before);
    expect(query.getQueryState(queryKeys.poolSnapshot(pool))?.fetchStatus).toBe('fetching');
  } finally { await view?.close(); query.clear(); await relay.close(); await replay.close(); }
}

async function swapTokenRowsUseServedPricesAndPropagateStatisticsOutages() {
  const replay = await serveRecorded([recorded.poolListTrending, recorded.statsBatch]);
  const relay = await readRelay(replay.origin);
  process.env.NEXT_PUBLIC_METADATA_API_URL = relay.origin;
  const operations = new Operations(sdk(relay.origin));
  try {
    const tokens = await loadSwapTokens(operations, new AbortController().signal);
    const rows = decodeV2Contract('StatsBatchResponse', recorded.statsBatch.body).data.items;
    expect(tokens).toHaveLength(rows.length);
    for (const token of tokens) {
      const row = rows.find(row => row.poolAddress === token.poolAddress)!;
      expect(token.price).toBe(statsSpotPrice(row).value === null ? null : Number(statsSpotPrice(row).value));
      expect(token.marketCap).toBe(statsMarketCap(row).value === null ? null : Number(formatUnits(BigInt(statsMarketCap(row).value!), 6)));
    }
    expect(relay.requests).toContain('/v2/pools?sort=trending&limit=100');
    expect(relay.requests).toContain('/v2/pools/stats:batch');
    relay.disconnect('/v2/pools/stats:batch');
    await expect(loadSwapTokens(operations, new AbortController().signal)).rejects.toThrow();
  } finally { await relay.close(); await replay.close(); }
}

async function swapTokenMetadataCancellationEndsTheRead() {
  const replay = await serveRecorded([recorded.poolListTrending, recorded.statsBatch]);
  const relay = await readRelay(replay.origin);
  relay.hold('/metadata/');
  process.env.NEXT_PUBLIC_METADATA_API_URL = relay.origin;
  try {
    const controller = new AbortController();
    const outcome = loadSwapTokens(new Operations(sdk(relay.origin)), controller.signal).then(value => ({ value, error: null }), (error: unknown) => ({ value: null, error }));
    await until(() => relay.requests.some(path => path.startsWith('/metadata/')));
    controller.abort();
    const result = await outcome;
    expect(result.value).toBeNull();
    expect(result.error).toMatchObject({ name: 'AbortError' });
  } finally { await relay.close(); await replay.close(); }
}

async function swapAmountUsesChainDecimalsAndRejectsUnrepresentableInput() {
  const fixture = JSON.parse(readFileSync(process.env.L26_FIXTURE_JSON!, 'utf8'));
  const rpc = createPublicClient({ chain: paxeerWalletChain, transport: http(process.env.L26_RPC_URL!, { retryCount: 0 }) });
  const decimals = await rpc.readContract({ address: fixture.token as Address, abi: erc20Abi, functionName: 'decimals' });
  const balance = await rpc.readContract({ address: fixture.token as Address, abi: erc20Abi, functionName: 'balanceOf', args: [fixture.account as Address] });
  expect(decimals).toBe(6);
  const replay = await serveRecorded([]);
  let view: Awaited<ReturnType<typeof mount>> | undefined;
  try {
    view = await mount(createElement(TokenInputBox, { label: 'You pay', token: null, amount: '', balance, decimals, onSelectClick() {} }), sdk(replay.origin));
    expect(view.host.querySelector('input')?.type).toBe('text');
    expect(view.host.querySelector('input')?.inputMode).toBe('decimal');
    expect(parseAmount(formatUnits(balance, decimals), decimals, balance).parsedRaw).toBe(balance);
    expect(parseAmount('1.0000001', decimals).errorCode).toBe('excess_precision');
    expect(parseAmount('1e3', decimals).errorCode).toBe('invalid_decimal');
    expect(parseAmount('1', undefined).errorCode).toBe('awaiting_asset_metadata');
    expect(parseAmount(formatUnits(balance + 1n, decimals), decimals, balance).errorCode).toBe('insufficient_balance');
    expect(readSwapQuote({})).toBeNull();
    expect(readSwapQuote([0n, 1n, 0n, 0n, 0n, 0n, 0n, pool, pool])).toBeNull();
  } finally { await view?.close(); await replay.close(); }
}

async function swapConnectWalletControlOpensAuthentication() {
  const closeSurface = applyBrowserSurface();
  const replay = await serveRecorded([recorded.poolListTrending, recorded.statsBatch]);
  process.env.NEXT_PUBLIC_METADATA_API_URL = replay.origin;
  const config = createConfig({ chains: [paxeerWalletChain], connectors: [], transports: { 125: http(process.env.L26_RPC_URL!, { retryCount: 0 }) }, multiInjectedProviderDiscovery: false, storage: null, batch: { multicall: false } });
  let authentication: ReturnType<typeof useAuthModal> | undefined;
  function AuthState() { authentication = useAuthModal(); return null; }
  const view = await mount(createElement(WagmiProvider, { config, reconnectOnMount: false }, createElement(React.Fragment, null, createElement(AuthState), createElement(SwapPanel))), sdk(replay.origin));
  try {
    const connect = [...view.host.querySelectorAll('button')].find(button => button.textContent === 'Connect wallet')!;
    expect(connect).toBeDefined();
    expect(connect.disabled).toBe(false);
    await act(async () => connect.click());
    expect(authentication?.open).toBe(true);
    expect(authentication?.reason).toBe('Sign in to trade');
    await until(() => !view.host.textContent?.includes('Loading tokens...'));
  } finally { await view.close(); await replay.close(); await closeSurface(); }
}

async function marketBalancesUseTheirRpcAndRequireTheWalletHomeChain() {
  const fixture = JSON.parse(readFileSync(process.env.L26_FIXTURE_JSON!, 'utf8'));
  const home: HomeChain = { registryNumber: 0, chainId: 125, rpcUrl: process.env.L26_RPC_URL!, walletNetwork: null };
  const rpc = createPublicClient({ chain: paxeerWalletChain, transport: http(home.rpcUrl, { retryCount: 0 }) });
  const height = await rpc.getBlockNumber({ cacheTime: 0 });
  expect(await homeBalanceReader(home).readBalance(fixture.token, fixture.account, height)).toBe(await rpc.readContract({ address: fixture.token, abi: erc20Abi, functionName: 'balanceOf', args: [fixture.account], blockNumber: height }));
  expect(marketWalletChain(home, 125)).toBe(true);
  expect(marketWalletChain({ ...home, chainId: 125000007, registryNumber: 7 }, 125)).toBe(false);
  expect(marketWalletChain({ ...home, chainId: 125000007, registryNumber: 7 }, 125000007)).toBe(true);
  expect(marketWalletChain(null, 125)).toBe(false);
}

async function creatorCoinInformationCanRetryAnInterruptedMetadataRead() {
  const fixture = await creatorFixture();
  const relay = await readRelay(fixture.origin);
  relay.disconnect('/metadata/');
  process.env.NEXT_PUBLIC_METADATA_API_URL = relay.origin;
  const config = createConfig({ chains: [paxeerWalletChain], connectors: [], transports: { 125: custom(fixture.provider, { retryCount: 0 }) }, multiInjectedProviderDiscovery: false, storage: null, batch: { multicall: false } });
  const coin = fixture.coins[0];
  const replay = await serveRecorded([]);
  const view = await mount(createElement(WagmiProvider, { config, reconnectOnMount: false }, createElement(CreatorPoolInfo, { creator: coin.Creator as Address, poolAddress: coin.Pool })), sdk(replay.origin));
  try {
    await until(() => view.host.textContent?.includes('Coin info unavailable') === true);
    expect(view.host.querySelector('[role="alert"]')?.textContent).toBe('Coin info unavailable');
    expect([...view.host.querySelectorAll('span')].filter(span => span.textContent === '—')).toHaveLength(3);
    expect(view.host.textContent).not.toContain('...');
    relay.disconnect(null);
    await act(async () => view.host.querySelector('button')!.click());
    await until(() => view.host.textContent?.includes(coin.Name) === true);
    expect(view.host.textContent).toContain(coin.Token);
    expect(view.host.textContent).not.toContain('Coin info unavailable');
    expect(fixture.errors).toEqual([]);
    expect(fixture.provider.errors).toEqual([]);
  } finally { await view.close(); await relay.close(); await fixture.close(); await replay.close(); }
}

async function expiredCoinQuoteShowsRefreshAndReadFailureEndsThePreview() {
  const bytes = readFileSync(process.env.L26_FIXTURE_JSON!);
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.L26_FIXTURE_SHA256);
  const fixture = JSON.parse(bytes.toString());
  const rpc = createPublicClient({ chain: paxeerWalletChain, transport: http(process.env.L26_RPC_URL!, { retryCount: 0 }) });
  const provider = new ChainTradeProvider(rpc, undefined, fixture.deployment.tradeDeployment);
  const info = await provider.inspectPool(fixture.pool);
  const context: TradeContext = { chainId: 125, manifestRevision: provider.deployment.revision, pool: fixture.pool, token: provider.deployment.usdl, router: provider.deployment.router, sender: fixture.account, recipient: fixture.account, inputRaw: 1000000n, isBuy: true, opticalRevision: info.opticalRevision, opticalData: '0x' };
  const quote = await provider.readQuote(context);
  expect(currentTradeQuote({ quote }, context, quote.quotedAt).quote).toBe(quote);
  const expired = currentTradeQuote({ quote }, context, quote.expiresAt);
  expect(expired.quote).toBeUndefined();
  expect(expired.staleQuote).toBe(quote);
  expect(tradePreviewState({ ...expired, validAmount: true, tradeReady: true, approvalError: undefined, signedIn: true, deploymentUnavailable: false }).loading).toBe(false);
  const replay = await serveRecorded([recorded.poolDetailNotFound]);
  try {
    const failure = await new Operations(sdk(replay.origin)).call('IF_POOL_DETAIL', { path: { poolAddress: v2Reads.pools.unknown } }).catch((error: unknown) => error);
    expect(failure).toBeInstanceOf(Error);
    expect(tradePreviewState({ validAmount: true, tradeReady: true, quote: undefined, staleQuote: undefined, approvalError: (failure as Error).message, signedIn: true, deploymentUnavailable: true })).toEqual({ loading: false, showError: true });
  } finally { await replay.close(); }
}

async function joinedTokenReadsResolveTheIndexedCoinAndSwapInventory() {
  const bytes = readFileSync(process.env.JOINED_CONFIG_FILE!);
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  const config = JSON.parse(bytes.toString());
  const chain = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8'));
  expect(chain.realm).toBe('private-anvil125');
  expect(chain.productionActivation).toBe(false);
  const gateway = config.processes.find((item: { role: string }) => item.role === 'gateway');
  const client = sdk(config.api_origin, new URL(gateway.ready_url).origin);
  process.env.NEXT_PUBLIC_METADATA_API_URL = new URL(gateway.ready_url).origin;
  const operations = new Operations(client);
  const snapshot = await resolvePoolSnapshot(chain.pool, { operations });
  expect(snapshot.pool.tokenAddress).toBe(chain.token.toLowerCase());
  expect(snapshot.pool.poolAddress).toBe(chain.pool.toLowerCase());
  expect(snapshot.stats?.poolAddress).toBe(chain.pool.toLowerCase());
  await expect.poll(async () => {
    const tokens = await loadSwapTokens(operations, new AbortController().signal);
    return tokens.find(token => token.poolAddress === chain.pool.toLowerCase())?.tokenAddress;
  }, { timeout: 30_000, interval: 1_000 }).toBe(chain.token.toLowerCase());
  const page = await operations.call('IF_TRADES', { path: { poolAddress: chain.pool }, query: { limit: 100 } });
  expect(page.data.items.length).toBeGreaterThan(0);
  expect(page.data.items.every(item => item.poolAddress === chain.pool.toLowerCase())).toBe(true);
}

for (const scenario of [
  tokenMetadataFailureKeepsMarketFactsAndOffersRetry,
  descriptionLinksUseThePublishedSocialDestinations,
  failedPoolReadEndsChatLoadingWithRecovery,
  swapTokenRowsUseServedPricesAndPropagateStatisticsOutages,
  swapTokenMetadataCancellationEndsTheRead,
  swapAmountUsesChainDecimalsAndRejectsUnrepresentableInput,
  swapConnectWalletControlOpensAuthentication,
  marketBalancesUseTheirRpcAndRequireTheWalletHomeChain,
  creatorCoinInformationCanRetryAnInterruptedMetadataRead,
  expiredCoinQuoteShowsRefreshAndReadFailureEndsThePreview,
  joinedTokenReadsResolveTheIndexedCoinAndSwapInventory,
]) test(scenario, { timeout: 60000 }, scenario);
