// @vitest-environment jsdom
import { createServer, type Server, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { act, createElement, StrictMode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { initBlockscout } from '../client';
import type { AddressResponse, AddressCounters } from '../types/address';
import type { Token, TokenBalance, TokenInstanceInList, TokenTransfer } from '../types/token';
import type { Transaction } from '../types/transaction';
import { useAddress } from './useAddress';
import { useTokenBalances } from './useTokenBalances';
import { useNfts } from './useNfts';
import { useTokenTransfers } from './useTokenTransfers';
import { useTransactions } from './useTransactions';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const A = `0x${'a'.repeat(40)}`;
const B = `0x${'b'.repeat(40)}`;
const cursor = { items_count: 1, block_number: 42 };

function addressFixture(hash: string, version: number): AddressResponse {
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
    coin_balance: String(version),
    exchange_rate: null,
    block_number_balance_updated_at: version,
    creation_transaction_hash: null,
    creator_address_hash: null,
    creation_status: null,
    token: null,
    has_validated_blocks: false,
    has_logs: false,
    has_tokens: true,
    has_token_transfers: true,
    has_beacon_chain_withdrawals: false,
    watchlist_address_id: null,
  };
}
function countersFixture(version: number): AddressCounters {
  return {
    transactions_count: String(version),
    token_transfers_count: '2',
    gas_usage_count: '3',
    validations_count: '4',
  };
}
function tokenFixture(hash: string): Token {
  return {
    address_hash: hash,
    name: 'Recorded token',
    symbol: 'REC',
    decimals: '18',
    type: 'ERC-20',
    holders_count: '1',
    exchange_rate: null,
    volume_24h: null,
    total_supply: '100',
    icon_url: null,
    circulating_market_cap: null,
  };
}
function balanceFixture(hash: string, version: number): TokenBalance {
  return {
    value: String(version),
    token: tokenFixture(hash),
    token_id: null,
    token_instance: null,
  };
}
function nftFixture(hash: string, version: number): TokenInstanceInList {
  return {
    id: String(version),
    metadata: null,
    owner: addressFixture(hash, version),
    token: { ...tokenFixture(hash), type: 'ERC-721' },
    external_app_url: null,
    animation_url: null,
    image_url: null,
    is_unique: true,
    thumbnails: null,
    media_type: null,
    media_url: null,
    token_type: 'ERC-721',
    value: '1',
  };
}
function transferFixture(hash: string, version: number): TokenTransfer {
  return {
    transaction_hash: `0x${version.toString(16).padStart(64, '0')}`,
    from: addressFixture(hash, version),
    to: addressFixture(B, version),
    total: { value: String(version), decimals: '18' },
    token: tokenFixture(hash),
    type: 'token_transfer',
    timestamp: null,
    method: null,
    block_hash: `0x${'c'.repeat(64)}`,
    block_number: version,
    log_index: 0,
  };
}
function transactionFixture(hash: string, version: number): Transaction {
  return {
    hash: `0x${version.toString(16).padStart(64, '0')}`,
    result: 'success',
    status: 'ok',
    block_number: version,
    timestamp: null,
    from: addressFixture(hash, version),
    to: addressFixture(B, version),
    created_contract: null,
    confirmations: 1,
    confirmation_duration: [],
    value: String(version),
    fee: { type: 'actual', value: '1' },
    gas_price: '1',
    type: 2,
    gas_used: '21000',
    gas_limit: '21000',
    max_fee_per_gas: null,
    max_priority_fee_per_gas: null,
    base_fee_per_gas: null,
    priority_fee: null,
    transaction_burnt_fee: null,
    nonce: version,
    position: 0,
    revert_reason: null,
    raw_input: '0x',
    decoded_input: null,
    token_transfers: null,
    token_transfers_overflow: false,
    actions: [],
    exchange_rate: null,
    historic_exchange_rate: null,
    method: null,
    transaction_types: ['coin_transfer'],
    transaction_tag: null,
    has_error_in_internal_transactions: false,
    authorization_list: null,
  };
}

type HookState = {
  data: unknown;
  loading: boolean;
  error: Error | null;
  nextPageParams?: Record<string, unknown> | null;
  fetch: (hash: string) => Promise<void>;
  refresh: () => Promise<void>;
  fetchNextPage?: () => Promise<void>;
};
type HookCase = {
  useHook: () => HookState;
  suffix: string;
  query: Record<string, string>;
  paginated: boolean;
  empty: unknown;
  data: (hash: string, version: number) => unknown;
};
function useAddressState(): HookState {
  const value = useAddress();
  return { ...value, data: [value.address, value.counters] };
}
function useBalancesState(): HookState {
  const value = useTokenBalances();
  return { ...value, data: value.balances };
}
function useNftsState(): HookState {
  const value = useNfts();
  return { ...value, data: value.nfts, fetch: (hash) => value.fetch(hash, { type: 'ERC-721' }) };
}
function useTransfersState(): HookState {
  const value = useTokenTransfers();
  return {
    ...value,
    data: value.transfers,
    fetch: (hash) => value.fetch(hash, { filter: 'from', type: 'ERC-20' }),
  };
}
function useTransactionsState(): HookState {
  const value = useTransactions();
  return {
    ...value,
    data: value.transactions,
    fetch: (hash) => value.fetch(hash, { filter: 'to', order: 'desc' }),
  };
}
const addressCase: HookCase = {
  useHook: useAddressState,
  suffix: '',
  query: {},
  paginated: false,
  empty: [null, null],
  data: (hash, version) => [addressFixture(hash, version), countersFixture(version)],
};
const balancesCase: HookCase = {
  useHook: useBalancesState,
  suffix: '/token-balances',
  query: {},
  paginated: false,
  empty: [],
  data: (hash, version) => [balanceFixture(hash, version)],
};
const nftsCase: HookCase = {
  useHook: useNftsState,
  suffix: '/nft',
  query: { type: 'ERC-721' },
  paginated: true,
  empty: [],
  data: (hash, version) => [nftFixture(hash, version)],
};
const transfersCase: HookCase = {
  useHook: useTransfersState,
  suffix: '/token-transfers',
  query: { filter: 'from', type: 'ERC-20' },
  paginated: true,
  empty: [],
  data: (hash, version) => [transferFixture(hash, version)],
};
const transactionsCase: HookCase = {
  useHook: useTransactionsState,
  suffix: '/transactions',
  query: { filter: 'to', order: 'desc' },
  paginated: true,
  empty: [],
  data: (hash, version) => [transactionFixture(hash, version)],
};

type Exchange = {
  path: string;
  received: Promise<ServerResponse>;
  accept: (res: ServerResponse) => void;
  body: unknown;
};
let server: Server;
let root: Root;
let container: HTMLDivElement;
let latest: HookState;
let exchanges: Exchange[];
let responses: ServerResponse[];
let unexpected: string[];
let requests: string[];

beforeEach(async () => {
  exchanges = [];
  responses = [];
  unexpected = [];
  requests = [];
  server = createServer((req, res) => {
    responses.push(res);
    requests.push(`${req.method} ${req.url}`);
    const next = exchanges.shift();
    if (
      !next ||
      req.method !== 'GET' ||
      req.url !== next.path ||
      req.headers.accept !== 'application/json'
    ) {
      unexpected.push(`${req.method} ${req.url}`);
      res.writeHead(500).end('Unexpected request');
      next?.accept(res);
      return;
    }
    next.accept(res);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const bound = server.address();
  if (!bound || typeof bound === 'string') throw new Error('Missing loopback address');
  initBlockscout({ baseUrl: `http://127.0.0.1:${bound.port}` });
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  for (const response of responses) {
    if (!response.writableEnded) response.writeHead(503).end('Test cleanup');
  }
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
  container.remove();
  expect(unexpected).toEqual([]);
  expect(exchanges).toEqual([]);
});

async function mount(subject: HookCase) {
  function Probe() {
    latest = subject.useHook();
    return createElement(
      'output',
      null,
      JSON.stringify({ data: latest.data, loading: latest.loading, error: latest.error?.message })
    );
  }
  await act(async () => root.render(createElement(StrictMode, null, createElement(Probe))));
}
function plan(subject: HookCase, hash: string, version: number, page: boolean): Exchange[] {
  const query = new URLSearchParams(subject.query);
  if (page) for (const [key, value] of Object.entries(cursor)) query.set(key, String(value));
  const path = `/api/v2/addresses/${hash}${subject.suffix}${query.size ? `?${query}` : ''}`;
  const data = subject.data(hash, version);
  const bodies =
    subject === addressCase
      ? [
          { path, body: addressFixture(hash, version) },
          { path: `${path}/counters`, body: countersFixture(version) },
        ]
      : [
          {
            path,
            body: subject.paginated
              ? { items: data, next_page_params: page ? null : cursor }
              : data,
          },
        ];
  const pending = bodies.map(({ path, body }) => {
    let accept!: (res: ServerResponse) => void;
    const received = new Promise<ServerResponse>((resolve) => {
      accept = resolve;
    });
    return { path, body, received, accept };
  });
  exchanges.push(...pending);
  return pending;
}
async function begin(
  subject: HookCase,
  hash: string,
  version: number,
  action = () => latest.fetch(hash),
  page = false
) {
  const pending = plan(subject, hash, version, page);
  let finished!: Promise<void>;
  await act(async () => {
    finished = action();
  });
  await Promise.all(pending.map(({ received }) => received));
  return {
    async finish(status = 200) {
      await act(async () => {
        for (const exchange of pending) {
          const response = await exchange.received;
          response.writeHead(status, { 'Content-Type': 'application/json' });
          response.end(
            JSON.stringify(
              status === 200 ? exchange.body : { message: 'Recorded upstream failure' }
            )
          );
        }
        await finished;
      });
    },
  };
}
function expectPending(subject: HookCase) {
  expect(latest.data).toEqual(subject.empty);
  expect(latest.error).toBeNull();
  expect(latest.loading).toBe(true);
  if (subject.paginated) expect(latest.nextPageParams).toBeNull();
}
function expectData(subject: HookCase, hash: string, version: number) {
  expect(latest.data).toEqual(subject.data(hash, version));
  expect(latest.loading).toBe(false);
  expect(latest.error).toBeNull();
  if (subject.paginated) expect(latest.nextPageParams).toEqual(cursor);
  expect(container.textContent).toContain(JSON.stringify(subject.data(hash, version)));
}
async function exerciseAccountRequests(subject: HookCase) {
  await mount(subject);
  await (await begin(subject, A, 1)).finish();
  expectData(subject, A, 1);

  const oldSuccess = await begin(subject, A, 2, () => latest.refresh());
  const newAccount = await begin(subject, B, 3);
  expectPending(subject);
  await oldSuccess.finish();
  expectPending(subject);
  await newAccount.finish();
  expectData(subject, B, 3);

  const olderSameAccount = await begin(subject, B, 4);
  await (await begin(subject, B, 5)).finish();
  await olderSameAccount.finish();
  expectData(subject, B, 5);

  const oldFailure = await begin(subject, B, 6);
  const current = await begin(subject, A, 7);
  await oldFailure.finish(503);
  expectPending(subject);
  await current.finish();
  expectData(subject, A, 7);

  const originalA = await begin(subject, A, 8);
  const intermediateB = await begin(subject, B, 9);
  await (await begin(subject, A, 10)).finish();
  await originalA.finish();
  await intermediateB.finish(503);
  expectData(subject, A, 10);

  await (await begin(subject, B, 11)).finish(503);
  expect(latest.data).toEqual(subject.empty);
  expect(latest.loading).toBe(false);
  expect(latest.error).toMatchObject({ status: 503 });
  const recovery = await begin(subject, B, 12, () => latest.refresh());
  expectPending(subject);
  await recovery.finish();
  expectData(subject, B, 12);

  const disconnected = await begin(subject, B, 13);
  const requestCount = requests.length;
  await act(async () => {
    await latest.fetch('');
  });
  expect(latest.data).toEqual(subject.empty);
  expect(latest.loading).toBe(false);
  expect(latest.error).toBeNull();
  await disconnected.finish();
  await act(async () => {
    await latest.refresh();
    await latest.fetchNextPage?.();
  });
  expect(requests).toHaveLength(requestCount);
  expect(latest.data).toEqual(subject.empty);
  expect(latest.loading).toBe(false);
  if (subject.paginated) expect(latest.nextPageParams).toBeNull();
}
async function exercisePages(subject: HookCase) {
  await (await begin(subject, A, 20)).finish();
  const nextPage = latest.fetchNextPage!;
  const oldPage = await begin(subject, A, 21, nextPage, true);
  const count = requests.length;
  await act(async () => {
    await nextPage();
  });
  expect(requests).toHaveLength(count);
  const switched = await begin(subject, B, 22);
  await act(async () => {
    await nextPage();
  });
  expect(requests).toHaveLength(count + 1);
  expectPending(subject);
  await oldPage.finish();
  expectPending(subject);
  await switched.finish();
  expectData(subject, B, 22);

  const pageFailure = await begin(subject, B, 23, nextPage, true);
  await (await begin(subject, A, 24)).finish();
  await pageFailure.finish(503);
  expectData(subject, A, 24);

  const replacedPage = await begin(subject, A, 25, nextPage, true);
  await (await begin(subject, A, 26, () => latest.refresh())).finish();
  await replacedPage.finish();
  expectData(subject, A, 26);

  await (await begin(subject, A, 27, nextPage, true)).finish(503);
  expect(latest.data).toEqual(subject.data(A, 26));
  expect(latest.error).toMatchObject({ status: 503 });
  expect(latest.loading).toBe(false);
  expect(latest.nextPageParams).toEqual(cursor);
  await (await begin(subject, A, 28, nextPage, true)).finish();
  expect(latest.data).toEqual([
    ...(subject.data(A, 26) as unknown[]),
    ...(subject.data(A, 28) as unknown[]),
  ]);
  expect(latest.nextPageParams).toBeNull();
  expect(latest.error).toBeNull();
  expect(latest.loading).toBe(false);
  const finalCount = requests.length;
  await act(async () => {
    await nextPage();
  });
  expect(requests).toHaveLength(finalCount);
}
async function finding46_useAddress_guards_account_data_counters_errors_and_loading() {
  await exerciseAccountRequests(addressCase);
}
async function finding47_useTokenBalances_guards_account_data_errors_and_loading() {
  await exerciseAccountRequests(balancesCase);
}
async function finding50_useNfts_guards_account_requests_and_page_appends() {
  await exerciseAccountRequests(nftsCase);
  await exercisePages(nftsCase);
}
async function finding49_useTokenTransfers_guards_account_requests_and_page_appends() {
  await exerciseAccountRequests(transfersCase);
  await exercisePages(transfersCase);
}
async function finding48_useTransactions_guards_account_requests_and_page_appends() {
  await exerciseAccountRequests(transactionsCase);
  await exercisePages(transactionsCase);
}

test(
  finding46_useAddress_guards_account_data_counters_errors_and_loading.name,
  finding46_useAddress_guards_account_data_counters_errors_and_loading
);
test(
  finding47_useTokenBalances_guards_account_data_errors_and_loading.name,
  finding47_useTokenBalances_guards_account_data_errors_and_loading
);
test(
  finding50_useNfts_guards_account_requests_and_page_appends.name,
  finding50_useNfts_guards_account_requests_and_page_appends
);
test(
  finding49_useTokenTransfers_guards_account_requests_and_page_appends.name,
  finding49_useTokenTransfers_guards_account_requests_and_page_appends
);
test(
  finding48_useTransactions_guards_account_requests_and_page_appends.name,
  finding48_useTransactions_guards_account_requests_and_page_appends
);
