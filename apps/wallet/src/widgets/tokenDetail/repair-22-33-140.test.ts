// @vitest-environment jsdom
import { parseWalletReadRequest, walletReadResponseHeaders } from '@/lib/wallet-read-contract';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ethers } from 'ethers';
import { afterEach, beforeEach, expect, test } from 'vitest';
import {
  initBlockscout,
  type Token,
  type TokenTransfer,
  type Address,
  type Transaction,
} from '@/lib/data';
import { TOKENS, CHAIN_ID } from '@/lib/swap/sdk/addresses';
import { IdentitySession } from '@/wallet/identity';
import {
  WalletProvider,
  WalletNetworkProvider,
  useWallet,
  type WalletContextValue,
} from '@/wallet/WalletProvider';
import {
  EMAIL,
  EMAIL_CODE,
  loadConstructions,
  startGateway,
  type TestGateway,
} from '@/wallet/test/gateway';
import { MarketStatsCard } from './MarketStatsCard';
import { TokenDetailWidget, resolveTokenDetailIdentity } from './TokenDetailWidget';
import {
  supportedTokenDecimals,
  type TokenDetailHistory,
  type MarketSeries,
} from './detailQueries';
import type { AppRoute } from '@/widgets/shell/useAppRoute';
import { queryKeys } from '@/lib/queries/keys';
import { scopedQueryKey } from '@/lib/queries/scope';
import { preferencesRepository } from '@/platform/storage/repositories';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const TOKEN_A = new ethers.Wallet(ethers.id('token-detail-zero-decimal-fixture')).address;
const TOKEN_B = new ethers.Wallet(ethers.id('token-detail-other-contract-fixture')).address;
const HASH_A = ethers.id('token-detail-transfer-a');
const HASH_B = ethers.id('token-detail-transfer-b');
const HASH_NATIVE = ethers.id('token-detail-native');
const INTERVAL = 15 * 60 * 1000;
const DAY = 24 * 60 * 60 * 1000;
const nativeFetch = globalThis.fetch;

let gateway: TestGateway;
let server: Server;
let origin: string;
let root: Root | null;
let container: HTMLDivElement;
let client: QueryClient;
let identity: IdentitySession;
let current: WalletContextValue | null;
let routeId: string;
let metadataPrecision: number | undefined;
let chainPrecision: string | null;
let holdingPrecision: string | null;
let includeHolding: boolean;
let requests: URL[];
let sent: string[];
let navigated: AppRoute[];
let selectedTransactions: string[];

function address(hash: string): Address {
  return {
    hash,
    is_contract: false,
    is_scam: false,
    is_verified: false,
    name: null,
    ens_domain_name: null,
    metadata: null,
    proxy_type: null,
    implementations: [],
    private_tags: [],
    public_tags: [],
    watchlist_names: [],
  };
}

function token(contract: string, decimals: string | null): Token {
  return {
    address_hash: contract,
    name: 'Whole Units',
    symbol: 'SAME',
    decimals,
    type: 'ERC-20',
    holders_count: '2',
    exchange_rate: '2',
    volume_24h: null,
    total_supply: '100',
    icon_url: null,
    circulating_market_cap: null,
  };
}

function transfer(contract: string, hash: string, index: number): TokenTransfer {
  return {
    transaction_hash: hash,
    from: address(loadConstructions().address),
    to: address(TOKEN_B),
    token: token(contract, '0'),
    total: { value: '42', decimals: '0' },
    type: 'token_transfer',
    timestamp: '2026-01-01T00:00:00Z',
    method: null,
    block_hash: ethers.id('token-detail-block'),
    block_number: 1,
    log_index: index,
  };
}

function nativeTransaction(): Transaction {
  return {
    hash: HASH_NATIVE,
    result: 'success',
    status: 'ok',
    block_number: 1,
    timestamp: '2026-01-01T00:00:00Z',
    from: address(loadConstructions().address),
    to: address(TOKEN_B),
    created_contract: null,
    confirmations: 1,
    confirmation_duration: [],
    value: '1000000000000000000',
    fee: { type: 'actual', value: '0' },
    gas_price: '0',
    type: 2,
    gas_used: '21000',
    gas_limit: '21000',
    max_fee_per_gas: null,
    max_priority_fee_per_gas: null,
    base_fee_per_gas: null,
    priority_fee: null,
    transaction_burnt_fee: null,
    nonce: 0,
    position: 0,
    revert_reason: null,
    raw_input: '0x',
    decoded_input: null,
    token_transfers: [],
    token_transfers_overflow: false,
    actions: [],
    exchange_rate: '2',
    historic_exchange_rate: null,
    method: null,
    transaction_types: ['coin_transfer'],
    transaction_tag: null,
    has_error_in_internal_transactions: false,
    authorization_list: null,
  };
}

async function startReadServer(): Promise<void> {
  server = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/v2/')) {
      const read = parseWalletReadRequest(url);
      for (const [key, value] of Object.entries(walletReadResponseHeaders(read)))
        res.setHeader(key, value);
    }
    requests.push(url);
    const path = url.pathname;
    let body: unknown;
    if (path === '/wallet/api/candle/pax/history') {
      const end = Math.floor((Number(url.searchParams.get('to')) * 1000) / INTERVAL) * INTERVAL;
      const fullDay = url.searchParams.get('countback') === '97';
      const timestamps = fullDay
        ? [
            end - DAY - INTERVAL,
            ...Array.from({ length: 96 }, (_, i) => end - DAY + i * INTERVAL),
            end + INTERVAL,
          ]
        : [end - INTERVAL];
      body = {
        s: 'ok',
        t: timestamps.map((t) => t / 1000),
        o: timestamps.map(() => 3),
        h: timestamps.map((t) => (t < end - DAY || t >= end || !fullDay ? 999 : 7)),
        l: timestamps.map((t) => (t < end - DAY || t >= end || !fullDay ? 0.01 : 2)),
        c: timestamps.map(() => 3),
        v: timestamps.map(() => 1),
      };
    } else if (path.endsWith('/token-balances')) {
      body = includeHolding
        ? [
            {
              value: '42',
              token: token(TOKEN_A, holdingPrecision),
              token_id: null,
              token_instance: null,
            },
          ]
        : [];
    } else if (path.endsWith('/counters')) {
      body = {
        transactions_count: '1',
        token_transfers_count: '2',
        gas_usage_count: '0',
        validations_count: '0',
      };
    } else if (path.endsWith('/token-transfers')) {
      body = {
        items: [transfer(TOKEN_A, HASH_A, 0), transfer(TOKEN_B, HASH_B, 1)],
        next_page_params: null,
      };
    } else if (path.endsWith('/transactions')) {
      body = { items: [nativeTransaction()], next_page_params: null };
    } else if (path.startsWith('/api/v2/addresses/')) {
      body = {
        ...address(loadConstructions().address),
        coin_balance: '1000000000000000000',
        exchange_rate: '2',
      };
    } else if (path.startsWith('/api/v2/tokens/')) {
      body = token(path.split('/').at(-1) ?? TOKEN_A, chainPrecision);
    } else if (path.startsWith('/wallet/api/sdk/metadata/metadata/')) {
      body = {
        token_address: path.split('/').at(-1)?.replace('.json', ''),
        decimals: metadataPrecision,
      };
    } else {
      res.writeHead(404, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: `Unrecorded read: ${path}` }));
      return;
    }
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(body));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  initBlockscout({ baseUrl: origin });
  // Node fetch lacks the browser's same-origin URL resolution. This bridge
  // resolves URLs only; every response still crosses a real loopback socket.
  globalThis.fetch = (input, init) => {
    const target = typeof input === 'string' ? new URL(input, origin).href : input;
    return nativeFetch(target, init);
  };
}

function Harness() {
  const wallet = useWallet();
  useLayoutEffect(() => {
    current = wallet;
  });
  return wallet.status === 'ready'
    ? createElement(TokenDetailWidget, {
        tokenId: routeId,
        onNavigate: (route) => navigated.push(route),
        onSendToken: (id) => sent.push(id),
        onTxDetail: (hash) => selectedTransactions.push(hash),
      })
    : null;
}

function renderHarness() {
  root!.render(
    createElement(QueryClientProvider, {
      client,
      children: createElement(WalletProvider, {
        config: gateway.config,
        identity,
        eventTarget: window,
        children: createElement(Harness),
      }),
    })
  );
}

async function until(check: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (check()) return;
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  }
  throw new Error(
    `Condition not reached. Wallet: ${current?.status}; DOM: ${container.textContent}`
  );
}

async function mount(id: string): Promise<void> {
  routeId = id;
  root = createRoot(container);
  await act(async () => renderHarness());
  await until(() => current?.status === 'signed-out');
  await act(async () => {
    await current!.verifyEmailCode(EMAIL, EMAIL_CODE);
  });
  await until(() => current?.status === 'ready' && client.isFetching() === 0);
}

async function openToken(id: string): Promise<void> {
  routeId = id;
  await act(async () => renderHarness());
  await until(() => client.isFetching() === 0);
}

function button(label: string): HTMLButtonElement {
  const found = Array.from(container.querySelectorAll('button')).find(
    (b) => b.textContent === label
  );
  if (!found) throw new Error(`Missing button ${label}`);
  return found;
}

function metric(label: string): string | null {
  const found = Array.from(container.querySelectorAll('p, span')).find(
    (p) => p.textContent === label
  );
  return found?.parentElement?.lastElementChild?.textContent ?? null;
}

beforeEach(async () => {
  localStorage.clear();
  requests = [];
  sent = [];
  navigated = [];
  selectedTransactions = [];
  metadataPrecision = 0;
  chainPrecision = '0';
  holdingPrecision = '0';
  includeHolding = true;
  current = null;
  root = null;
  gateway = await startGateway();
  await startReadServer();
  identity = IdentitySession.create(gateway.config, {
    persistSession: false,
    detectSessionInUrl: false,
  });
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } },
  });
  client.setQueryData(queryKeys.paxPrice('PAX'), { latest: 3 });
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(async () => {
  await act(async () => root?.unmount());
  client.clear();
  await identity.client.auth.stopAutoRefresh();
  await gateway.close();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
  globalThis.fetch = nativeFetch;
  container.remove();
  localStorage.clear();
});

async function market_stats_window_is_independent_of_chart_period() {
  await mount('pax');
  expect(metric('24h High')).toBe('$7.00');
  expect(metric('24h Low')).toBe('$2.00');
  const market = client.getQueryData<MarketSeries>([
    'tokenDetailMarketDay',
    CHAIN_ID,
    'pax',
    'PAX',
  ]);
  expect(market?.window.to! - market?.window.from!).toBe(DAY);
  expect(market?.candles).toHaveLength(98);
  for (const period of ['1H', '1W', 'ALL']) {
    await act(async () => button(period).click());
    await until(() => client.isFetching() === 0);
    expect(metric('24h High')).toBe('$7.00');
    expect(metric('24h Low')).toBe('$2.00');
  }
  expect(requests.filter((r) => r.searchParams.get('countback') === '97')).toHaveLength(1);
  expect(requests.some((r) => r.searchParams.get('resolution') === '1')).toBe(true);
  expect(requests.some((r) => r.searchParams.get('resolution') === '1440')).toBe(true);
}

async function market_stats_window_reports_missing_coverage() {
  const to = Math.floor(Date.now() / INTERVAL) * INTERVAL;
  const window = { from: to - DAY, to, intervalMs: INTERVAL };
  const candles = Array.from({ length: 96 }, (_, i) => ({
    timestamp: window.from + i * INTERVAL,
    open: 3,
    high: 7,
    low: 2,
    close: 3,
    volume: 1,
  }));
  root = createRoot(container);
  for (const incomplete of [
    [],
    candles.slice(1),
    candles.slice(0, -1),
    candles.filter((_, i) => i !== 40),
  ]) {
    await act(async () =>
      root!.render(createElement(MarketStatsCard, { paxPrice: 3, ohlcData: incomplete, window }))
    );
    expect(container.textContent).toContain('24h range unavailable: incomplete coverage');
    expect(metric('24h High')).toBeNull();
    expect(metric('24h Low')).toBeNull();
  }
}

async function wpax_address_detection_uses_active_chain_and_exact_alias() {
  await mount(ethers.getAddress(TOKENS.WPAX9.address));
  expect(metric('Symbol')).toBe('WPAX');
  expect(metric('Name')).toBe('Wrapped PAX');
  expect(metric('24h High')).toBe('$7.00');
  expect(container.textContent).toContain('$3.00');
  expect(requests.some((r) => r.searchParams.get('symbol') === 'WPAX')).toBe(true);
  expect(requests.some((r) => r.pathname.includes('/metadata/'))).toBe(false);
  await act(async () => button('Send').click());
  expect(sent).toEqual([TOKENS.WPAX9.address.toLowerCase()]);
  expect(resolveTokenDetailIdentity(' WPAX ', CHAIN_ID)).toEqual({
    tokenId: TOKENS.WPAX9.address.toLowerCase(),
    isPax: false,
    isWpax: true,
  });
  expect(resolveTokenDetailIdentity(TOKENS.WPAX9.address, 125000001).isWpax).toBe(false);
  expect(resolveTokenDetailIdentity('wpax', 125000001).isWpax).toBe(false);
  expect(resolveTokenDetailIdentity('not-wpax', CHAIN_ID).isWpax).toBe(false);
  expect(resolveTokenDetailIdentity(TOKENS.WPAX9.address, null).isWpax).toBe(false);
  const provider = walletContext().wallet?.provider;
  if (!(provider instanceof WalletNetworkProvider))
    throw new Error('Expected the real network provider');
  // Seed the persisted network through its production validator. Registering
  // a virtual network derives a root RPC from environment configuration;
  // switching an already saved network needs no environment or live RPC.
  preferencesRepository.update((preferences) => ({
    ...preferences,
    networks: [
      ...preferences.networks,
      {
        chainId: 125000001,
        name: 'Fixture Network',
        kind: 'virtual',
        rpcUrl: 'https://rpc.example.test/chain/1',
        explorerUrl: 'https://explorer.example.test/chains/1',
      },
    ],
  }));
  await act(async () => {
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: ethers.toQuantity(125000001) }],
    });
  });
  await until(
    () =>
      walletContext().chainId === 125000001 &&
      client.getQueryState(
        scopedQueryKey(['tokenMeta', 125000001, TOKENS.WPAX9.address.toLowerCase(), false])
      )?.status === 'success' &&
      client.isFetching() === 0 &&
      metric('Symbol') !== '???'
  );
  expect(metric('Symbol')).toBe('SAME');
  expect(metric('24h High')).toBeNull();
  expect(container.textContent).toContain('Activity unavailable for this network');
  expect(
    client.getQueryData([
      'tokenDetailHistory',
      125000001,
      loadConstructions().address.toLowerCase(),
    ])
  ).toBeUndefined();
}

async function token_detail_zero_decimals_preserves_units_through_display_and_actions() {
  await mount(TOKEN_A);
  await until(
    () =>
      client.getQueryState(queryKeys.portfolio(loadConstructions().address))?.status ===
        'success' && metric('Balance') !== null
  );
  expect(metric('Decimals')).toBe('0');
  expect(metric('Balance')).toBe('42 SAME');
  expect(metric('Value')).toBe('$84.00');
  expect(button('Send').disabled).toBe(false);
  await act(async () => {
    button('Send').click();
    button('Swap').click();
  });
  expect(sent).toEqual([TOKEN_A.toLowerCase()]);
  expect(navigated).toEqual(['swap']);
  expect(
    client.getQueryData<{ token_holdings: Array<{ balance_raw: string; decimals: number }> }>(
      queryKeys.portfolio(loadConstructions().address)
    )?.token_holdings[0]
  ).toMatchObject({ balance_raw: '42', decimals: 0 });
  expect(container.textContent).toContain('-42.0 SAME');
}

async function token_detail_precision_is_null_aware_and_validated() {
  // Each metadata source may be the one that explicitly reports zero.
  for (const [pub, holding, chain] of [
    [0, '6', '18'],
    [undefined, '0', '18'],
    [undefined, null, '0'],
  ] as const) {
    metadataPrecision = pub;
    holdingPrecision = holding;
    chainPrecision = chain;
    includeHolding = holding !== null;
    if (root) {
      await act(async () => root!.unmount());
      root = null;
    }
    client.clear();
    client.setQueryData(queryKeys.paxPrice('PAX'), { latest: 3 });
    current = null;
    // Reuse the real signed-in identity; the fresh provider restores it.
    routeId = TOKEN_A;
    root = createRoot(container);
    await act(async () => renderHarness());
    await until(() => current?.status === 'signed-out' || current?.status === 'ready');
    if (walletContext().status === 'signed-out')
      await act(async () => {
        await walletContext().verifyEmailCode(EMAIL, EMAIL_CODE);
      });
    await until(() => current?.status === 'ready' && client.isFetching() === 0);
    expect(metric('Decimals')).toBe('0');
    expect(button('Send').disabled).toBe(false);
  }
  expect(supportedTokenDecimals(80)).toBe(80);
  for (const invalid of [null, undefined, -1, 1.5, 81, 256, NaN, Infinity, '0']) {
    expect(supportedTokenDecimals(invalid)).toBeNull();
  }
  for (const invalid of [undefined, -1, 1.5, 81]) {
    const key = ['tokenMeta', CHAIN_ID, TOKEN_A.toLowerCase(), false];
    await act(async () => {
      client.setQueryData(scopedQueryKey(key), {
        public: { token_address: TOKEN_A, decimals: invalid },
        chain: null,
      });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    // The final loop iteration has no holding, so missing precision stays unknown.
    expect(metric('Decimals')).toBe('Unknown');
    expect(metric('Balance')).toBeNull();
    expect(button('Send').disabled).toBe(true);
    expect(button('Swap').disabled).toBe(true);
    expect(button('Receive').disabled).toBe(false);
    await act(async () => {
      button('Send').click();
      button('Swap').click();
    });
    expect(sent).toEqual([]);
    expect(navigated).toEqual([]);
  }
}

async function token_activity_symbol_identity_survives_transport_and_filters_contract_and_chain() {
  await mount(TOKEN_A);
  const key = ['tokenDetailHistory', CHAIN_ID, loadConstructions().address.toLowerCase()];
  const history = client.getQueryData<TokenDetailHistory>(key)!;
  expect(history.transfers).toHaveLength(2);
  expect(history.transfers[0]).toMatchObject({
    chainId: CHAIN_ID,
    tokenAddress: TOKEN_A.toLowerCase(),
    decimals: 0,
    amountRaw: '42',
  });
  expect(history.transfers[1]).toMatchObject({
    chainId: CHAIN_ID,
    tokenAddress: TOKEN_B.toLowerCase(),
    symbol: 'SAME',
  });
  expect(history.transactions[0]).toMatchObject({
    chainId: CHAIN_ID,
    tokenAddress: null,
    symbol: 'PAX',
  });
  expect(container.querySelectorAll('button').length).toBeGreaterThan(3);
  const sentRows = () =>
    Array.from(container.querySelectorAll('button')).filter((b) =>
      b.textContent?.startsWith('Sent')
    );
  expect(sentRows()).toHaveLength(1);
  await act(async () => sentRows()[0].click());
  expect(selectedTransactions).toEqual([HASH_A]);
  await act(async () => {
    client.setQueryData(key, {
      ...history,
      transfers: [
        ...history.transfers,
        { ...history.transfers[0], chainId: 125000001, hash: ethers.id('wrong-chain') },
      ],
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  expect(sentRows()).toHaveLength(1);
  await openToken(TOKEN_B);
  expect(sentRows()).toHaveLength(1);
  await act(async () => sentRows()[0].click());
  expect(selectedTransactions).toEqual([HASH_A, HASH_B]);
  await openToken('pax');
  expect(sentRows()).toHaveLength(1);
  await act(async () => sentRows()[0].click());
  expect(selectedTransactions).toEqual([HASH_A, HASH_B, HASH_NATIVE]);
  await openToken(TOKEN_A);
  await act(async () => {
    client.setQueryData(key, {
      ...history,
      transfers: [{ ...history.transfers[0], decimals: null }],
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  expect(sentRows()).toHaveLength(1);
  expect(sentRows()[0].textContent).toContain('Unknown amount SAME');
}

test(
  market_stats_window_is_independent_of_chart_period.name,
  market_stats_window_is_independent_of_chart_period
);
test(
  market_stats_window_reports_missing_coverage.name,
  market_stats_window_reports_missing_coverage
);
test(
  wpax_address_detection_uses_active_chain_and_exact_alias.name,
  wpax_address_detection_uses_active_chain_and_exact_alias
);
test(
  token_detail_zero_decimals_preserves_units_through_display_and_actions.name,
  token_detail_zero_decimals_preserves_units_through_display_and_actions
);
test(
  token_detail_precision_is_null_aware_and_validated.name,
  token_detail_precision_is_null_aware_and_validated
);
test(
  token_activity_symbol_identity_survives_transport_and_filters_contract_and_chain.name,
  token_activity_symbol_identity_survives_transport_and_filters_contract_and_chain
);

function walletContext(): WalletContextValue {
  if (!current) throw new Error('The wallet context has not rendered');
  return current;
}
