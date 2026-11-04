// @vitest-environment jsdom
import { createServer, type ServerResponse, type Server } from 'node:http';
import { once } from 'node:events';
import { act, createElement, type ReactElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ethers } from 'ethers';
import { MotionConfig } from 'framer-motion';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import type { SwapToken } from '@/lib/swap';
import type { PaxscanToken } from '@/lib/api';
import type { UseSwapQuotesOptions, UseSwapQuotesResult } from './useSwapQuotes';
import type { SwapBalances } from './useSwapBalances';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const owner = '0x0000000000000000000000000000000000000001';
const zeroAddress = '0x0000000000000000000000000000000000000042';
const unknownAddress = '0x0000000000000000000000000000000000000043';
// Committed explorer transcript, served through the production HTTP client/facade.
const explorerTokens = [
  {
    address_hash: zeroAddress,
    name: 'Whole Units',
    symbol: 'WHOLE',
    decimals: '0',
    type: 'ERC-20',
    holders_count: '1',
    exchange_rate: null,
    volume_24h: null,
    total_supply: '42',
    icon_url: null,
    circulating_market_cap: null,
  },
  {
    address_hash: unknownAddress,
    name: 'Unknown Precision',
    symbol: 'UNKNOWN',
    decimals: null,
    type: 'ERC-20',
    holders_count: '1',
    exchange_rate: null,
    volume_24h: null,
    total_supply: '42',
    icon_url: null,
    circulating_market_cap: null,
  },
];

let server: Server;
let baseUrl: string;
let originalRpc: string | undefined;
let sdk: typeof import('@/lib/swap/sdk');
let swap: typeof import('@/lib/swap');
let api: typeof import('@/lib/api');
let util: typeof import('./util');
let heldModule: typeof import('./useHeldTokens');
let balancesModule: typeof import('./useSwapBalances');
let outputModule: typeof import('./useOutputTokens');
let quotesModule: typeof import('./useSwapQuotes');
let selectionModule: typeof import('./useSwapQuoteSelection');
let modalModule: typeof import('./PaxSwapModal');
let confirmModule: typeof import('./SwapConfirmDialog');
let walletModule: typeof import('@/wallet/WalletProvider');
let cache: typeof import('@/lib/metaCache');
let repositories: typeof import('@/platform/storage/repositories');
let keys: typeof import('@/lib/queries/keys');
let codec: ethers.Interface;
let client: QueryClient;
let root: Root | null;
let host: HTMLDivElement;
let params: UseSwapQuotesOptions;
let latest: UseSwapQuotesResult;
let latestSelection: ReturnType<typeof selectionModule.useSwapQuoteSelection>;
let held: SwapToken[];
let balances: SwapBalances;
let output: SwapToken[];
let holdRpc = false;
let rpcCalls = 0;
let rpcReplies = 0;
let inputAmounts: bigint[] = [];
let pending: Array<() => void> = [];
let serverErrors: unknown[] = [];

function rpcResult(method: string, values: unknown[]): unknown {
  if (method === 'eth_chainId') return '0x7d';
  if (method !== 'eth_call') throw new Error(`Unexpected RPC method: ${method}`);
  const call = values[0] as { to: string; data: string };
  expect(call.to.toLowerCase()).toBe(sdk.CONTRACTS.PECORQuoter.toLowerCase());
  const decoded = codec.parseTransaction(call)!;
  expect(decoded.name).toBe('quoteExactIn');
  const amountIn = BigInt(decoded.args[2]);
  inputAmounts.push(amountIn);
  ++rpcCalls;
  // ABI-encoded quote transcript; the real SDK and aggregator decode and format it.
  const result = {
    amountIn,
    amountOut: (amountIn / 10n ** 12n) * BigInt(rpcCalls + 1),
    grossAmountOut: (amountIn / 10n ** 12n) * BigInt(rpcCalls + 1),
    executionPrice: ethers.parseEther('1'),
    spotPriceIn: ethers.parseEther('1'),
    spotPriceOut: ethers.parseEther('1'),
    feeAmount: 0n,
    feeBps: 0n,
    sufficientLiquidity: true,
    availableLiquidity: ethers.parseEther('100'),
    priceTimestampIn: 1700000000n,
    priceTimestampOut: 1700000000n,
    priceStaleIn: false,
    priceStaleOut: false,
    utilizationCapBps: 10000n,
    tradeUtilizationBps: 100n,
    cappedByUtilization: false,
    twapOut: ethers.parseEther('1'),
    twapDeviationBps: 0n,
    effectiveFeeBps: 0n,
    tier: 0n,
    priceImpactBps: 0n,
    priceImpactDeduction: 0n,
  };
  return codec.encodeFunctionResult('quoteExactIn', [result]);
}

function sendJson(response: ServerResponse, data: unknown) {
  response.writeHead(200, { 'content-type': 'application/json' });
  response.end(JSON.stringify(data));
}

beforeAll(async () => {
  server = createServer(async (request, response) => {
    try {
      const path = new URL(request.url!, 'http://localhost').pathname;
      if (request.method === 'POST') {
        let body = '';
        for await (const chunk of request) body += chunk;
        const payload = JSON.parse(body);
        const requests = Array.isArray(payload) ? payload : [payload];
        const replies = requests.map(
          (entry: { id: number; method: string; params?: unknown[] }) => ({
            jsonrpc: '2.0',
            id: entry.id,
            result: rpcResult(entry.method, entry.params ?? []),
          })
        );
        const reply = () => {
          sendJson(response, Array.isArray(payload) ? replies : replies[0]);
          rpcReplies += requests.filter(
            (entry: { method: string }) => entry.method === 'eth_call'
          ).length;
        };
        if (holdRpc && requests.some((entry: { method: string }) => entry.method === 'eth_call'))
          pending.push(reply);
        else reply();
        return;
      }
      if (path === '/api/v2/tokens')
        return sendJson(response, { items: explorerTokens, next_page_params: null });
      if (path.endsWith('/token-balances'))
        return sendJson(response, [
          { value: '42', token: explorerTokens[0], token_id: null, token_instance: null },
        ]);
      if (path.endsWith('/counters'))
        return sendJson(response, { transactions_count: '0', token_transfers_count: '0' });
      if (path === `/api/v2/addresses/${owner}`)
        return sendJson(response, {
          coin_balance: ethers.parseEther('100').toString(),
          exchange_rate: null,
        });
      throw new Error(`Unexpected HTTP request: ${path}`);
    } catch (error) {
      serverErrors.push(error);
      response.writeHead(500);
      response.end(String(error));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback address missing');
  baseUrl = `http://127.0.0.1:${address.port}`;
  // Configure the production provider before importing it; no external configuration is needed.
  originalRpc = process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URL = baseUrl;
  sdk = await import('@/lib/swap/sdk');
  codec = new ethers.Interface(sdk.PECOR_QUOTER_ABI);
  swap = await import('@/lib/swap');
  api = await import('@/lib/api');
  const data = await import('@/lib/data/client');
  data.initBlockscout({ baseUrl });
  util = await import('./util');
  heldModule = await import('./useHeldTokens');
  balancesModule = await import('./useSwapBalances');
  outputModule = await import('./useOutputTokens');
  quotesModule = await import('./useSwapQuotes');
  selectionModule = await import('./useSwapQuoteSelection');
  modalModule = await import('./PaxSwapModal');
  confirmModule = await import('./SwapConfirmDialog');
  walletModule = await import('@/wallet/WalletProvider');
  cache = await import('@/lib/metaCache');
  repositories = await import('@/platform/storage/repositories');
  keys = await import('@/lib/queries/keys');
});

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } },
  });
  host = document.createElement('div');
  document.body.appendChild(host);
  root = createRoot(host);
  holdRpc = false;
  rpcCalls = 0;
  rpcReplies = 0;
  inputAmounts = [];
  serverErrors = [];
  params = {
    fromToken: { ...swap.SWAP_TOKENS[0], iconUrl: '/wallet/default_icon.webp' },
    toToken: { ...swap.SWAP_TOKENS[2], iconUrl: '/wallet/default_icon.webp' },
    fromAmount: '1',
    slippageBps: 150,
    recipient: owner,
    debounceMs: 10,
  };
});

afterEach(async () => {
  holdRpc = false;
  for (const reply of pending.splice(0)) reply();
  await act(async () => root?.unmount());
  root = null;
  host.remove();
  client.clear();
  // Let the real provider's short RPC deduplication window expire between scenarios.
  await new Promise((resolve) => setTimeout(resolve, 260));
  cache.cacheDel('sidiora-token-list');
  expect(serverErrors).toEqual([]);
});

afterAll(async () => {
  if (originalRpc === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  else process.env.NEXT_PUBLIC_PAXEER_RPC_URL = originalRpc;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
});

async function until(assertion: () => void) {
  const deadline = Date.now() + 2000;
  while (true) {
    try {
      assertion();
      return;
    } catch (error) {
      if (Date.now() >= deadline) throw error;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 10));
      });
    }
  }
}

async function render(element: ReactElement) {
  await act(async () => root!.render(createElement(QueryClientProvider, { client }, element)));
}

function QuotesProbe() {
  latest = quotesModule.useSwapQuotes(params);
  return createElement('output', null, latest.bestQuote?.amountOutDisplay ?? latest.quoteError);
}

function SelectionProbe() {
  latestSelection = selectionModule.useSwapQuoteSelection(params);
  return createElement('output', null, latestSelection.quotes.bestQuote?.amountOutDisplay ?? '');
}

function HeldProbe() {
  held = heldModule.useHeldTokens(owner);
  return createElement('output', null, held.map((token) => token.symbol).join(','));
}

function BalanceProbe() {
  balances = balancesModule.useSwapBalances(owner, params.fromToken, params.toToken);
  return createElement('output', null, balances.fromBalance);
}

function OutputProbe() {
  output = outputModule.useOutputTokens();
  return createElement('output', null, output.map((token) => token.symbol).join(','));
}

async function held_zero_decimals_preserves_producer_units() {
  const portfolio = await api.fetchPortfolio(owner);
  expect(portfolio.token_holdings[0].decimals).toBe(0);
  client.setQueryData(keys.queryKeys.portfolio(owner), portfolio);
  await render(createElement(HeldProbe));
  const token = held.find((entry) => entry.address === zeroAddress)!;
  expect(token.decimals).toBe(0);
  expect(util.formatBalanceFull(42n, token.decimals)).toBe('42');
  expect(util.parseAmountToWei('42', token.decimals)).toBe('42');
  expect(util.parseAmountToWei('1.5', token.decimals)).toBeNull();
  for (const decimals of [null, undefined, -1, 1.5, 81, NaN]) {
    client.setQueryData(keys.queryKeys.portfolio(owner), {
      ...portfolio,
      token_holdings: [{ ...portfolio.token_holdings[0], decimals }],
    });
    await render(createElement(HeldProbe));
    expect(held.some((entry) => entry.address === zeroAddress)).toBe(false);
  }
}

async function swap_balance_zero_decimals_displays_actual_units_and_blocks_unknown_spending() {
  const portfolio = await api.fetchPortfolio(owner);
  params = {
    ...params,
    fromToken: { symbol: 'WHOLE', name: 'Whole Units', address: zeroAddress, decimals: 18 },
  };
  client.setQueryData(keys.queryKeys.portfolio(owner), portfolio);
  await render(createElement(BalanceProbe));
  expect(balances.fromBalance).toBe('42');
  expect(balances.fromBalanceRaw).toBeNull(); // stale selected precision must not fund a swap
  params = { ...params, fromToken: { ...params.fromToken, decimals: 0 } };
  await render(createElement(BalanceProbe));
  expect(balances.fromBalanceRaw).toBe(42n);
  expect(util.parseAmountToWei(balances.fromBalance!, params.fromToken.decimals)).toBe('42');
  for (const decimals of [null, undefined, -1, 1.5, 81, NaN]) {
    client.setQueryData(keys.queryKeys.portfolio(owner), {
      ...portfolio,
      token_holdings: [{ ...portfolio.token_holdings[0], decimals }],
    });
    await render(createElement(BalanceProbe));
    expect(balances.fromBalance).toBeNull();
    expect(balances.fromBalanceRaw).toBeNull();
  }
}

async function catalogue_zero_decimals_survives_http_codec_and_conversion() {
  const records = await api.fetchTopTokens();
  const token = util.paxscanToSwapToken(
    records.find((entry) => entry.address_hash === zeroAddress)!
  )!;
  expect(token.decimals).toBe(0);
  expect(util.parseAmountToWei('42', token.decimals)).toBe('42');
  expect(util.formatBalanceFull(42n, token.decimals)).toBe('42');
  const response = await fetch(`${baseUrl}/api/v2/tokens`);
  const transcript: { items: PaxscanToken[] } = await response.json();
  expect(
    util.paxscanToSwapToken(
      transcript.items.find((entry) => entry.address_hash === unknownAddress)!
    )
  ).toBeNull();
  for (const decimals of ['', ' ', '-1', '1.5', '81', 'NaN', 'Infinity']) {
    expect(util.paxscanToSwapToken({ ...records[0], decimals })).toBeNull();
    expect(util.parseAmountToWei('1', Number(decimals) === 0 ? NaN : Number(decimals))).toBeNull();
  }
}

async function swap_output_cache_shape_reopens_merged_list_with_original_freshness() {
  const records = await api.fetchTopTokens();
  const token = util.paxscanToSwapToken(records[0])!;
  const persisted: SwapToken[] = JSON.parse(JSON.stringify([{ ...token, isLaunchpad: true }]));
  cache.cacheSet('sidiora-token-list', persisted);
  const writtenAt = Date.now() - 60_000;
  repositories.metadataCacheRepository.update((entries) => ({
    ...entries,
    'sidiora-token-list': { ...entries['sidiora-token-list'], ts: writtenAt },
  }));
  for (let mount = 0; mount < 2; ++mount) {
    await render(createElement(OutputProbe));
    expect(output[0].isNative).toBe(true);
    for (const standard of swap.SWAP_TOKENS) {
      expect(output.some((entry) => entry.address === standard.address)).toBe(true);
    }
    expect(output.find((entry) => entry.address === zeroAddress)?.decimals).toBe(0);
    expect(new Set(output.map((entry) => (entry.isNative ? 'native' : entry.address))).size).toBe(
      output.length
    );
    expect(client.getQueryState(['swapOutputTokens'])?.dataUpdatedAt).toBe(writtenAt);
    await act(async () => root!.unmount());
    root = createRoot(host);
    client.clear();
  }
}

async function swap_stale_quote_after_clear_rejects_inflight_and_debounced_results() {
  holdRpc = true;
  await render(createElement(QuotesProbe));
  await until(() => expect(pending.length).toBe(1));
  await act(async () => latest.clear());
  for (const reply of pending.splice(0)) reply();
  await until(() => expect(rpcReplies).toBe(1));
  // Drain the real HTTP response and React update before asserting no resurrection.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 60));
  });
  expect(latest.bestQuote).toBeNull();
  expect(latest.quotes).toEqual([]);
  expect(latest.quoting).toBe(false);
  expect(latest.quoteSecondsLeft).toBe(0);
  params = { ...params, fromAmount: '2', debounceMs: 80 };
  await render(createElement(QuotesProbe));
  await act(async () => latest.clear());
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 100));
  });
  expect(rpcCalls).toBe(1);
  expect(latest.bestQuote).toBeNull();
}

async function swap_stale_quote_after_invalid_input_and_identity_change_never_resurrects() {
  holdRpc = true;
  await render(createElement(QuotesProbe));
  await until(() => expect(pending.length).toBe(1));
  params = { ...params, fromAmount: '' };
  await render(createElement(QuotesProbe));
  for (const reply of pending.splice(0)) reply();
  await until(() => expect(rpcReplies).toBe(1));
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 60));
  });
  expect(latest.bestQuote).toBeNull();
  expect(latest.quoting).toBe(false);
  params = { ...params, fromAmount: '2' };
  await render(createElement(QuotesProbe));
  await until(() => expect(pending.length).toBe(1));
  const oldReply = pending.shift()!;
  params = {
    ...params,
    slippageBps: 200,
    recipient: '0x0000000000000000000000000000000000000002',
    toToken: { ...params.toToken, decimals: 0 },
    fromAmount: '3',
  };
  await render(createElement(QuotesProbe));
  await until(() => expect(pending.length).toBe(1));
  await act(async () => oldReply());
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 60));
  });
  expect(latest.bestQuote).toBeNull();
  for (const reply of pending.splice(0)) reply();
  await until(() => expect(latest.bestQuote?.amountIn).toBe(ethers.parseEther('3').toString()));
  expect(latest.bestQuote?.tokenOut.decimals).toBe(0);
  expect(inputAmounts).toEqual([
    ethers.parseEther('1'),
    ethers.parseEther('2'),
    ethers.parseEther('3'),
  ]);
}

// Render the real modal and confirmation on the server: browser layout APIs are absent in SSR.
function renderQuoteViews(): string {
  const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window')!;
  Object.defineProperty(globalThis, 'window', { configurable: true, value: undefined });
  try {
    const approval = latestSelection.approval!;
    return renderToStaticMarkup(
      createElement(
        QueryClientProvider,
        { client },
        createElement(walletModule.WalletProvider, {
          config: null,
          children: createElement(
            MotionConfig,
            { isStatic: true },
            createElement(modalModule.PaxSwapModal, {
              ...params,
              quotes: latestSelection.quotes,
              onFromTokenChange: (token) => {
                params = { ...params, fromToken: token };
              },
              onToTokenChange: (token) => {
                params = { ...params, toToken: token };
              },
              onFromAmountChange: (amount) => {
                params = { ...params, fromAmount: amount };
              },
              onFlip: () => {
                params = { ...params, fromToken: params.toToken, toToken: params.fromToken };
              },
            }),
            createElement(confirmModule.SwapConfirmDialog, {
              open: true,
              loading: false,
              execError: '',
              bestQuote: approval.quote,
              fromToken: approval.quote.tokenIn,
              toToken: approval.quote.tokenOut,
              fromAmount: approval.fromAmount,
              slippageBps: approval.slippageBps,
              onClose: latestSelection.closeApproval,
              onConfirm: latestSelection.closeApproval,
            })
          ),
        })
      )
    );
  } finally {
    Object.defineProperty(globalThis, 'window', windowDescriptor);
  }
}

async function swap_two_quote_authorities_share_and_freeze_the_displayed_approval() {
  await render(createElement(SelectionProbe));
  await until(() => expect(latestSelection.quotes.bestQuote).not.toBeNull());
  const displayed = latestSelection.quotes.bestQuote!;
  expect(rpcCalls).toBe(1);
  await act(async () => latestSelection.openApproval());
  const approval = latestSelection.approval!;
  expect(approval.quote).toEqual(displayed);
  expect(latestSelection.quotes.bestQuote).toBe(approval.quote);
  expect(Object.isFrozen(approval.quote)).toBe(true);
  const html = renderQuoteViews();
  expect(html).toContain('Confirm Swap');
  expect(html).toContain(displayed.amountOutDisplay);
  expect(html).toContain('number-flow-react');
  expect(rpcCalls).toBe(1);
  await act(async () => latestSelection.quotes.refresh());
  expect(latestSelection.approval?.quote).toBe(approval.quote);
  expect(latestSelection.quotes.bestQuote).toBe(approval.quote);
  expect(rpcCalls).toBe(1);
  params = { ...params, recipient: '0x0000000000000000000000000000000000000002' };
  await render(createElement(SelectionProbe));
  expect(latestSelection.approval).toBeNull();
  expect(latestSelection.quotes.bestQuote).toBeNull();
  await until(() => expect(latestSelection.quotes.bestQuote).not.toBeNull());
  expect(rpcCalls).toBe(2);
  expect(latestSelection.quotes.bestQuote!.amountOut).not.toBe(displayed.amountOut);
}

test(held_zero_decimals_preserves_producer_units, held_zero_decimals_preserves_producer_units);
test(
  swap_balance_zero_decimals_displays_actual_units_and_blocks_unknown_spending,
  swap_balance_zero_decimals_displays_actual_units_and_blocks_unknown_spending
);
test(
  catalogue_zero_decimals_survives_http_codec_and_conversion,
  catalogue_zero_decimals_survives_http_codec_and_conversion
);
test(
  swap_output_cache_shape_reopens_merged_list_with_original_freshness,
  swap_output_cache_shape_reopens_merged_list_with_original_freshness
);
test(
  swap_stale_quote_after_clear_rejects_inflight_and_debounced_results,
  swap_stale_quote_after_clear_rejects_inflight_and_debounced_results
);
test(
  swap_stale_quote_after_invalid_input_and_identity_change_never_resurrects,
  swap_stale_quote_after_invalid_input_and_identity_change_never_resurrects
);
test(
  swap_two_quote_authorities_share_and_freeze_the_displayed_approval,
  swap_two_quote_authorities_share_and_freeze_the_displayed_approval
);
