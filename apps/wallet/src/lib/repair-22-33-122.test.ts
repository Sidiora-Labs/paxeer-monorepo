// @vitest-environment jsdom
// @vitest-environment-options {"url":"http://127.0.0.1:32122"}

import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { createECDH } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { Wallet } from 'ethers';
import { createElement, act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider, QueryObserver } from '@tanstack/react-query';
import { beforeAll, afterAll, beforeEach, afterEach, expect, test } from 'vitest';
import {
  fetchPortfolio,
  fetchPortfolioFresh,
  fetchHoldings,
  fetchHoldingsFresh,
  fetchBalance,
  fetchBalanceFresh,
  fetchPortfolioTransactions,
  fetchTokenMetadata,
  fetchBalanceHistory,
  fetchPnlHistory,
  fetchSidCandles,
  SID_TOKEN_ADDRESS,
} from './api';
import { TOKENS, PAXEER_CONFIG } from './constants';
import { TOKENS as SDK_TOKENS, CHAIN_ID } from './swap/sdk/addresses';
import { SWAP_TOKENS } from './swap/constants';
import { queryKeys } from './queries/keys';
import { useReconcilePortfolio } from './queries/portfolio';
import { initBlockscout } from './data/client';
import type { Token, TokenBalance, AddressResponse } from './data';
import {
  walletReadUrl,
  parseWalletReadRequest,
  walletReadResponseHeaders,
  selectedWalletChainId,
} from './wallet-read-contract';
import { currencyRatesRepository, preferencesRepository } from '@/platform/storage/repositories';
import {
  initCurrencyService,
  getRate,
  getRatesFreshness,
  convertFromUsd,
  formatUsdAsFiat,
} from './currency';
import {
  NotificationDelivery,
  type CampaignAnnouncement,
  type NotificationSink,
} from './notification-delivery';
import {
  sendSubscriptionToServer,
  confirmPushRegistration,
  heartbeat,
  clearWalletAddressForPush,
  setWalletAddressForPush,
  isPushSubscriptionEnabled,
  PushRegistrationError,
  type PushSubscriptionRecord,
} from './push';
import {
  getBackgroundFailures,
  clearBackgroundFailure,
} from '@/platform/status/background-failures';
import { createPushStore, parseStoredSubscription, type PushStore } from '@/server/push-store';
import { verifyPushOwnership } from '@/server/push-ownership';
import transactionsDocument from '../../../../services/platform/hosted/indexer/src/explorer_fixtures/transactions.json';
import transfersDocument from '../../../../services/platform/hosted/indexer/src/explorer_fixtures/token_transfers_page1.json';
import transfersPageTwo from '../../../../services/platform/hosted/indexer/src/explorer_fixtures/token_transfers_page2.json';

const ADDRESS = transactionsDocument.items[0].to.hash.toLowerCase();
const OTHER = transactionsDocument.items[0].from.hash.toLowerCase();
const ROOT = PAXEER_CONFIG.chainId;
const TENANT = 125000001;
const ORIGIN = 'http://127.0.0.1:32122';
const BASE = '/wallet/api/wallet';

interface Exchange {
  path: string;
  chain?: number;
  fresh?: boolean;
  body: unknown;
  status?: number;
  acknowledge?: boolean;
}
let server: Server;
let directory: string;
let pushStore: PushStore;
let exchanges: Exchange[] = [];
let unexpected: string[] = [];
let observed: { chainId: number; fresh: boolean; upstreamPath: string; cacheKey: string }[] = [];
let pushReply: 'accept' | 'refuse' | 'invalid-ack' = 'accept';
let requests = 0;

function tokenDocument(): Token {
  return {
    address_hash: SDK_TOKENS.SID.address,
    name: SDK_TOKENS.SID.name,
    symbol: 'SID',
    decimals: String(SDK_TOKENS.SID.decimals),
    type: 'ERC-20',
    holders_count: '4',
    exchange_rate: '1',
    volume_24h: null,
    total_supply: '1000000',
    icon_url: null,
    circulating_market_cap: null,
  };
}
function balances(raw: string): TokenBalance[] {
  return [{ value: raw, token: tokenDocument(), token_id: null, token_instance: null }];
}
function addressDocument(raw: string): AddressResponse {
  return {
    hash: ADDRESS,
    is_contract: false,
    is_verified: false,
    is_scam: false,
    name: null,
    ens_domain_name: null,
    metadata: null,
    proxy_type: null,
    implementations: [],
    public_tags: [],
    private_tags: [],
    watchlist_names: [],
    coin_balance: raw,
    exchange_rate: '1',
    block_number_balance_updated_at: 5200,
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
function plan(path: string, body: unknown, options: Omit<Exchange, 'path' | 'body'> = {}) {
  exchanges.push({ path, body, ...options });
}
function planPortfolio(raw: string, fresh = false, chain = ROOT, acknowledge = true) {
  plan(`/api/v2/addresses/${ADDRESS}`, addressDocument(raw), { fresh, chain, acknowledge });
  plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances(raw), { fresh, chain, acknowledge });
  plan(
    `/api/v2/addresses/${ADDRESS}/counters`,
    { transactions_count: '2', token_transfers_count: '1' },
    { fresh, chain, acknowledge }
  );
}
function planBalance(raw: string, fresh = false) {
  plan(`/api/v2/addresses/${ADDRESS}`, addressDocument(raw), { fresh });
  plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances(raw), { fresh });
}
function planActivity(
  address: string,
  native: unknown[],
  token: unknown[],
  chain = ROOT,
  nativeNext: Record<string, unknown> | null = null,
  tokenNext: Record<string, unknown> | null = null
) {
  plan(
    `/api/v2/addresses/${address}/transactions?limit=50`,
    { items: native, next_page_params: nativeNext },
    { chain, fresh: true }
  );
  plan(
    `/api/v2/addresses/${address}/token-transfers?limit=50`,
    { items: token, next_page_params: tokenNext },
    { chain, fresh: true }
  );
}

beforeAll(async () => {
  directory = await mkdtemp(path.join(tmpdir(), 'wallet-22-33-122-'));
  pushStore = createPushStore(directory);
  server = createServer(async (request, response) => {
    requests += 1;
    try {
      const url = new URL(request.url ?? '/', ORIGIN);
      if (url.pathname === '/wallet/api/push/subscribe') {
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const input = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        verifyPushOwnership(input, Math.floor(Date.now() / 1000));
        const subscription = parseStoredSubscription({
          ...input,
          createdAt: Date.now(),
          lastActiveAt: Date.now(),
        });
        if (pushReply === 'refuse') {
          response
            .writeHead(503, { 'Content-Type': 'application/json' })
            .end(JSON.stringify({ error: 'Recorded persistence refusal' }));
          return;
        }
        await pushStore.upsertSubscription(subscription);
        response
          .writeHead(200, { 'Content-Type': 'application/json' })
          .end(JSON.stringify(pushReply === 'invalid-ack' ? { accepted: true } : { ok: true }));
        return;
      }
      let key = `${url.pathname}${url.search}`;
      let headers: Record<string, string> = {};
      let chain = ROOT;
      let fresh = false;
      if (url.pathname.startsWith(BASE)) {
        const contract = parseWalletReadRequest(url, request.method);
        observed.push(contract);
        headers = walletReadResponseHeaders(contract);
        chain = contract.chainId;
        fresh = contract.fresh;
        const upstream = new URL(contract.upstreamPath, ORIGIN);
        key = `${upstream.pathname.replace(/^\/chain\/\d+/, '')}${upstream.search}`;
      }
      if (url.pathname === '/wallet/api/candle/sid/history') {
        if (
          url.searchParams.get('chain_id') !== String(ROOT) ||
          url.searchParams.get('token_address') !== SDK_TOKENS.SID.address ||
          url.searchParams.get('symbol') !== 'SID'
        )
          throw new Error('SID chart identity drift');
        key = url.pathname;
      }
      const index = exchanges.findIndex(
        (exchange) =>
          exchange.path === key &&
          (exchange.chain ?? ROOT) === chain &&
          (exchange.fresh ?? false) === fresh
      );
      if (index < 0) throw new Error(`Unexpected ${request.method} ${url.pathname}${url.search}`);
      const exchange = exchanges.splice(index, 1)[0];
      if (exchange.acknowledge === false) headers = {};
      response.writeHead(exchange.status ?? 200, {
        'Content-Type': 'application/json',
        ...headers,
      });
      response.end(JSON.stringify(exchange.body));
    } catch (error) {
      unexpected.push(String(error));
      response
        .writeHead(500, { 'Content-Type': 'application/json' })
        .end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(32122, '127.0.0.1');
  await once(server, 'listening');
});

afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
  await rm(directory, { recursive: true, force: true });
});

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  for (const failure of getBackgroundFailures()) clearBackgroundFailure(failure.id);
  clearWalletAddressForPush();
  exchanges = [];
  observed = [];
  unexpected = [];
  requests = 0;
  pushReply = 'accept';
  initBlockscout({ baseUrl: `${ORIGIN}${BASE}` });
});
afterEach(() => {
  expect(unexpected).toEqual([]);
  expect(exchanges).toEqual([]);
});

async function finding42_fresh_reads_and_optimistic_reconciliation_require_bypass_acknowledgment() {
  planPortfolio('1000000000000000000');
  const old = await fetchPortfolio(ADDRESS);
  expect(old.native_balance.balance).toBe('1');
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const key = queryKeys.portfolio(ADDRESS);
  client.setQueryData(key, old);
  const portfolioObserver = new QueryObserver<typeof old>(client, {
    queryKey: key,
    enabled: false,
  });
  const balanceObserver = new QueryObserver(client, {
    queryKey: queryKeys.balance(ADDRESS),
    enabled: false,
  });
  const unsubscribePortfolio = portfolioObserver.subscribe(() => {});
  const unsubscribeBalance = balanceObserver.subscribe(() => {});
  let reconcile!: ReturnType<typeof useReconcilePortfolio>;
  function Probe() {
    reconcile = useReconcilePortfolio();
    return null;
  }
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () =>
      root.render(createElement(QueryClientProvider, { client }, createElement(Probe)))
    );
    planPortfolio('2000000000000000000', true);
    planBalance('2000000000000000000', true);
    await reconcile(ADDRESS);
    expect(client.getQueryData<typeof old>(key)?.native_balance.balance).toBe('2');
    expect(portfolioObserver.getCurrentResult().data?.native_balance.balance).toBe('2');
    expect(client.getQueryData(queryKeys.balance(ADDRESS))).toMatchObject({ native_balance: '2' });

    planPortfolio('3000000000000000000', true);
    expect((await fetchPortfolioFresh(ADDRESS)).native_balance.balance).toBe('3');
    planBalance('4000000000000000000', true);
    expect((await fetchBalanceFresh(ADDRESS)).native_balance).toBe('4');
    plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances('5000000'), { fresh: true });
    expect((await fetchHoldingsFresh(ADDRESS))[0].balance).toBe('5');
    planPortfolio('6000000000000000000', true);
    expect((await fetchPortfolio(ADDRESS, { fresh: true })).native_balance.balance).toBe('6');
    planBalance('7000000000000000000', true);
    expect((await fetchBalance(ADDRESS, { fresh: true })).native_balance).toBe('7');
    plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances('8000000'), { fresh: true });
    expect((await fetchHoldings(ADDRESS, { fresh: true }))[0].balance).toBe('8');

    plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances('0'), {
      fresh: true,
      acknowledge: false,
    });
    await expect(fetchHoldingsFresh(ADDRESS)).rejects.toThrow('acknowledge');
    expect(observed.slice(3).every((request) => request.fresh)).toBe(true);
    expect(client.getQueryData<typeof old>(key)?.native_balance.balance).toBe('2');
    expect(portfolioObserver.getCurrentResult().data?.native_balance.balance).toBe('2');
  } finally {
    await act(async () => root.unmount());
    container.remove();
    unsubscribePortfolio();
    unsubscribeBalance();
    client.clear();
  }
}

async function finding43_history_uses_bounded_independent_transaction_and_transfer_cursors() {
  const txCursor = { block_number: 1240, index: 1, items_count: 3 };
  const transferCursor = transfersDocument.next_page_params;
  plan(`/api/v2/addresses/${ADDRESS}/transactions?limit=3`, {
    ...transactionsDocument,
    items: transactionsDocument.items.slice(0, 3),
    next_page_params: txCursor,
  });
  plan(`/api/v2/addresses/${ADDRESS}/token-transfers?limit=3`, transfersDocument);
  const first = await fetchPortfolioTransactions(ADDRESS, 3);
  expect(first.transactions.map((item) => item.tx_hash)).toEqual(
    transactionsDocument.items.slice(0, 3).map((item) => item.hash)
  );
  expect(first.next_page_params).toEqual(txCursor);
  expect(first.next_transfer_page_params).toEqual(transferCursor);
  plan(
    `/api/v2/addresses/${ADDRESS}/transactions?block_number=1240&index=1&items_count=3&limit=3`,
    { ...transactionsDocument, items: transactionsDocument.items.slice(3) }
  );
  plan(
    `/api/v2/addresses/${ADDRESS}/token-transfers?block_number=1200&index=4&limit=3`,
    transfersPageTwo
  );
  const second = await fetchPortfolioTransactions(ADDRESS, 3, {
    transactions: first.next_page_params,
    transfers: first.next_transfer_page_params,
  });
  expect(second.transactions.map((item) => item.tx_hash)).toEqual(
    transactionsDocument.items.slice(3).map((item) => item.hash)
  );
  expect(second.token_transfers.map((item) => item.tx_hash)).toEqual(
    transfersPageTwo.items.map((item) => item.transaction_hash)
  );
  expect(second.next_page_params).toBeNull();
  expect(second.next_transfer_page_params).toBeNull();
  const before = requests;
  for (const limit of [0, 51, -1, 1.5, NaN])
    expect(() => fetchPortfolioTransactions(ADDRESS, limit)).toThrow(RangeError);
  expect(requests).toBe(before);
}

function selectTenant() {
  const rpcPath = new URL(PAXEER_CONFIG.rpcUrl || '/', 'https://rpc.example').pathname.replace(
    /\/+$/,
    ''
  );
  const origin = PAXEER_CONFIG.rpcUrl
    ? new URL(PAXEER_CONFIG.rpcUrl).origin
    : 'https://rpc.example';
  preferencesRepository.update((current) => ({
    ...current,
    activeChainId: TENANT,
    networks: [
      {
        chainId: TENANT,
        name: 'Recorded tenant',
        kind: 'virtual',
        rpcUrl: `${origin}${rpcPath}/chain/1`,
        explorerUrl: 'https://explorer.example/chains/1',
      },
    ],
  }));
}
async function finding9_selected_chain_scopes_address_token_history_transport_and_cache_keys() {
  const rootKey = queryKeys.portfolio(ADDRESS);
  selectTenant();
  expect(selectedWalletChainId()).toBe(TENANT);
  expect(queryKeys.portfolio(ADDRESS)).not.toEqual(rootKey);
  expect(queryKeys.txHistory(ADDRESS, 2)).not.toEqual(queryKeys.txHistory(ADDRESS, 3));
  planPortfolio('9000000000000000000', false, TENANT);
  expect((await fetchPortfolio(ADDRESS)).native_balance.balance).toBe('9');
  plan(`/api/v2/tokens/${SDK_TOKENS.SID.address}`, tokenDocument(), { chain: TENANT });
  expect((await fetchTokenMetadata(SDK_TOKENS.SID.address)).address).toBe(SDK_TOKENS.SID.address);
  plan(
    `/api/v2/addresses/${ADDRESS}/coin-balance-history`,
    { items: [], next_page_params: null },
    { chain: TENANT }
  );
  expect((await fetchBalanceHistory(ADDRESS)).items).toEqual([]);
  plan(
    `/api/v1/portfolio/${ADDRESS}/pnl?days=7`,
    { address: ADDRESS, history: [] },
    { chain: TENANT }
  );
  expect(await fetchPnlHistory(ADDRESS, 7)).toMatchObject({ address: ADDRESS });
  expect(
    observed.every(
      (request) => request.chainId === TENANT && request.upstreamPath.startsWith('/chain/1/')
    )
  ).toBe(true);
  const tenantRequest = parseWalletReadRequest(
    new URL(walletReadUrl(`${ORIGIN}${BASE}/api/v2/addresses/${ADDRESS}`))
  );
  const rootRequest = parseWalletReadRequest(
    new URL(walletReadUrl(`${ORIGIN}${BASE}/api/v2/addresses/${ADDRESS}`, { chainId: ROOT }))
  );
  expect(tenantRequest.cacheKey).not.toBe(rootRequest.cacheKey);
  expect(() =>
    parseWalletReadRequest(
      new URL(`${ORIGIN}${BASE}/api/v2/tokens?chain_id=125&chain_id=125000001`)
    )
  ).toThrow();
  expect(() =>
    parseWalletReadRequest(new URL(`${ORIGIN}${BASE}/api/v2/tokens?chain_id=125&fresh=false`))
  ).toThrow();
  preferencesRepository.update((current) => ({ ...current, activeChainId: 125000002 }));
  expect(() => selectedWalletChainId()).toThrow('match');
}

async function finding27_sid_balance_swap_metadata_and_chart_share_the_registered_contract() {
  expect(ROOT).toBe(CHAIN_ID);
  expect(TOKENS.SID.address).toBe(SDK_TOKENS.SID.address);
  expect(SID_TOKEN_ADDRESS).toBe(SDK_TOKENS.SID.address);
  plan(`/api/v2/addresses/${ADDRESS}/token-balances`, balances('1000000'));
  const holding = (await fetchHoldings(ADDRESS))[0];
  plan(`/api/v2/tokens/${holding.contract_address}`, tokenDocument());
  const metadata = await fetchTokenMetadata(holding.contract_address);
  expect(metadata.address).toBe(SID_TOKEN_ADDRESS);
  expect(holding.decimals).toBe(SDK_TOKENS.SID.decimals);
  expect(holding.balance).toBe('1');
  expect(metadata.decimals).toBe(holding.decimals);
  expect(SWAP_TOKENS.find((token) => token.symbol === 'SID')?.address).toBe(
    holding.contract_address.toLowerCase()
  );
  plan('/wallet/api/candle/sid/history', {
    s: 'ok',
    t: [1793538000],
    o: [1],
    h: [2],
    l: [0.5],
    c: [1.5],
    v: [12],
  });
  expect((await fetchSidCandles('60', 1)).ohlc[0].close).toBe(1.5);
  selectTenant();
  await expect(fetchSidCandles('60', 1)).rejects.toThrow('not registered');
}

async function finding21_fx_conversion_and_freshness_share_the_same_age_bound() {
  const now = Date.now();
  for (const [age, freshness] of [
    [1000, 'fresh'],
    [5 * 60 * 1000 + 1000, 'stale'],
    [60 * 60 * 1000 + 1000, 'unavailable'],
  ] as const) {
    currencyRatesRepository.write({
      base: 'USD',
      rates: { USD: 1, EUR: 0.9 },
      fetchedAt: now - age,
    });
    initCurrencyService();
    expect(getRatesFreshness()).toBe(freshness);
    expect(getRate('EUR')).toBe(freshness === 'unavailable' ? null : 0.9);
    expect(convertFromUsd(10, 'EUR')).toBe(freshness === 'unavailable' ? null : 9);
    expect(convertFromUsd(10, 'USD')).toBe(10);
    if (freshness === 'stale') expect(formatUsdAsFiat(10, 'en-US', 'EUR')).toContain('stale rate');
    if (freshness === 'unavailable')
      expect(formatUsdAsFiat(10, 'en-US', 'EUR')).toContain('USD (conversion unavailable)');
  }
  currencyRatesRepository.write({
    base: 'USD',
    rates: { USD: 1, EUR: 0.9 },
    fetchedAt: now + 60000,
  });
  initCurrencyService();
  expect(getRatesFreshness()).toBe('unavailable');
  expect(convertFromUsd(10, 'EUR')).toBeNull();
}

class NotificationInbox {
  readonly records = new Map<string, { title: string; options: NotificationOptions }>();
  readonly receive: NotificationSink = async (title, options) => {
    this.records.set(options.tag ?? title, { title, options });
  };
}
function campaign(id: string, startsAt: number, endsAt: number): CampaignAnnouncement {
  return {
    id,
    title: 'Recorded campaign schedule',
    startsAt,
    endsAt,
    eligibility: {
      text: 'Eligibility is determined by the published campaign rules; holding a token alone does not guarantee an allocation.',
      sourceUrl: 'https://campaign.example/rules',
      verifiedAt: startsAt - 1000,
    },
  };
}
async function finding19_dated_verified_campaigns_expire_and_seen_identity_survives_restart() {
  const now = Date.now();
  const inbox = new NotificationInbox();
  let delivery = new NotificationDelivery(inbox.receive);
  const expired = campaign('expired', now - 2000, now);
  plan('/wallet/api/notifications/campaign', expired);
  await delivery.checkCampaign(now);
  expect(inbox.records.size).toBe(0);
  const future = campaign('future', now + 1000, now + 2000);
  plan('/wallet/api/notifications/campaign', future);
  await delivery.checkCampaign(now);
  expect(inbox.records.size).toBe(0);
  const current = campaign('published-campaign', now - 1000, now + 60000);
  plan('/wallet/api/notifications/campaign', current);
  await delivery.checkCampaign(now);
  expect(inbox.records.size).toBe(1);
  expect(inbox.records.get('campaign:published-campaign')?.options.body).toContain(
    current.eligibility.text
  );
  expect(inbox.records.get('campaign:published-campaign')?.options.body).toContain(
    new Date(current.endsAt).toISOString()
  );
  delivery.stop();
  delivery = new NotificationDelivery(inbox.receive);
  plan('/wallet/api/notifications/campaign', current);
  await delivery.checkCampaign(now);
  expect(inbox.records.size).toBe(1);
  plan('/wallet/api/notifications/campaign', {
    ...current,
    id: 'unverified',
    eligibility: { ...current.eligibility, verifiedAt: now + 5000 },
  });
  await expect(delivery.checkCampaign(now)).rejects.toThrow('invalid');
  delivery.stop();
}

async function finding18_committed_incoming_events_are_scoped_and_deduplicated_after_restart() {
  const inbox = new NotificationInbox();
  let delivery = new NotificationDelivery(inbox.receive);
  planActivity(ADDRESS, [], []);
  await delivery.checkIncoming(ADDRESS, ROOT);
  expect(inbox.records.size).toBe(0);
  planActivity(ADDRESS, transactionsDocument.items, transfersDocument.items);
  plan(
    `/api/v2/addresses/${ADDRESS}/token-transfers?block_number=1200&index=4&limit=50`,
    transfersPageTwo,
    { fresh: true }
  );
  // A continuation is needed to prove that incoming transfers behind outbound rows are consumed.
  exchanges.find((exchange) => exchange.path.endsWith('/token-transfers?limit=50'))!.body =
    transfersDocument;
  await delivery.checkIncoming(ADDRESS, ROOT);
  expect(inbox.records.size).toBe(4);
  const delivered = [...inbox.records.values()];
  expect(
    delivered.every(
      (item) =>
        item.title === 'Transaction Received' &&
        item.options.data.walletAddress === ADDRESS &&
        item.options.data.chainId === ROOT
    )
  ).toBe(true);
  expect(
    delivered.some((item) =>
      item.options.data.eventId.includes(transactionsDocument.items[2].hash.toLowerCase())
    )
  ).toBe(false);
  expect(
    delivered.some((item) =>
      item.options.data.eventId.includes(transfersDocument.items[0].transaction_hash)
    )
  ).toBe(false);
  const count = inbox.records.size;
  delivery.stop();
  const restartedInbox = new NotificationInbox();
  delivery = new NotificationDelivery(restartedInbox.receive);
  planActivity(ADDRESS, transactionsDocument.items, transfersDocument.items);
  await delivery.checkIncoming(ADDRESS.toUpperCase().replace('0X', '0x'), ROOT);
  expect(restartedInbox.records.size).toBe(0);
  expect(inbox.records.size).toBe(count);
  planActivity(OTHER, transactionsDocument.items, transfersDocument.items);
  await delivery.checkIncoming(OTHER, ROOT);
  expect(restartedInbox.records.size).toBe(0);
  planActivity(ADDRESS, transactionsDocument.items, transfersDocument.items, TENANT);
  await delivery.checkIncoming(ADDRESS, TENANT);
  expect(restartedInbox.records.size).toBe(0);
  delivery.stop();
}

function pushSubscription(): PushSubscriptionRecord {
  const key = createECDH('prime256v1');
  key.setPrivateKey(Buffer.from('2'.padStart(64, '0'), 'hex'));
  return {
    endpoint: 'https://fcm.googleapis.com/fcm/send/recorded-wallet-22-33-122',
    keys: {
      p256dh: key.getPublicKey(undefined, 'uncompressed').toString('base64url'),
      auth: Buffer.alloc(16, 1).toString('base64url'),
    },
  };
}
async function finding16_push_registration_and_heartbeat_require_signed_persisted_acknowledgment() {
  const signer = new Wallet('0x' + '11'.repeat(32));
  const subscription = pushSubscription();
  const missingWallet = await sendSubscriptionToServer(subscription);
  expect(missingWallet).toEqual({ ok: false, code: 'WALLET_UNAVAILABLE', retryable: true });
  setWalletAddressForPush(signer.address, (text) => signer.signMessage(text));
  pushReply = 'refuse';
  const refusal = await heartbeat(subscription);
  expect(refusal).toEqual({ ok: false, code: 'SERVER_REFUSED', retryable: true, status: 503 });
  expect(isPushSubscriptionEnabled(subscription.endpoint)).toBe(false);
  expect(await pushStore.getSubscription(subscription.endpoint)).toBeNull();
  await expect(confirmPushRegistration(subscription)).rejects.toBeInstanceOf(PushRegistrationError);
  expect(getBackgroundFailures()).toContainEqual(
    expect.objectContaining({ code: 'PUSH_SUBSCRIBE_FAILED', retryable: true })
  );
  pushReply = 'invalid-ack';
  expect(await heartbeat(subscription)).toMatchObject({ ok: false, code: 'ACK_INVALID' });
  expect(isPushSubscriptionEnabled(subscription.endpoint)).toBe(false);
  pushReply = 'accept';
  expect(await confirmPushRegistration(subscription)).toEqual(subscription);
  expect(isPushSubscriptionEnabled(subscription.endpoint)).toBe(true);
  expect(await pushStore.getSubscription(subscription.endpoint)).toMatchObject({
    endpoint: subscription.endpoint,
    walletAddress: signer.address.toLowerCase(),
  });
  // Reopen the real durable server store before refreshing an existing subscription.
  pushStore = createPushStore(directory);
  clearWalletAddressForPush();
  expect(isPushSubscriptionEnabled(subscription.endpoint)).toBe(false);
  setWalletAddressForPush(signer.address, (text) => signer.signMessage(text));
  expect(await heartbeat(subscription)).toMatchObject({
    ok: true,
    walletAddress: signer.address.toLowerCase(),
  });
  expect(isPushSubscriptionEnabled(subscription.endpoint)).toBe(true);
  const recovered = await pushStore.getSubscription(subscription.endpoint);
  expect(recovered?.keys).toEqual(subscription.keys);
}

for (const proof of [
  finding42_fresh_reads_and_optimistic_reconciliation_require_bypass_acknowledgment,
  finding43_history_uses_bounded_independent_transaction_and_transfer_cursors,
  finding9_selected_chain_scopes_address_token_history_transport_and_cache_keys,
  finding27_sid_balance_swap_metadata_and_chart_share_the_registered_contract,
  finding21_fx_conversion_and_freshness_share_the_same_age_bound,
  finding19_dated_verified_campaigns_expire_and_seen_identity_survives_restart,
  finding18_committed_incoming_events_are_scoped_and_deduplicated_after_restart,
  finding16_push_registration_and_heartbeat_require_signed_persisted_acknowledgment,
])
  test(proof.name, proof);
