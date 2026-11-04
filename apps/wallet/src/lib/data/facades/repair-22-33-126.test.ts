import { once } from 'node:events';
import { createServer, type Server } from 'node:http';
import { QueryClient } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { BlockscoutApiError, initBlockscout } from '../client';
import type { Address, AddressCounters, AddressResponse } from '../types/address';
import type { TokenBalance, TokenTransfer } from '../types/token';
import type { Transaction } from '../types/transaction';
import {
  getWalletBalance,
  getWalletHoldings,
  getWalletPortfolio,
  getWalletTransactions,
} from './wallet';

const WALLET = '0x1111111111111111111111111111111111111111';
const SENDER = '0x2222222222222222222222222222222222222222';
const HASH = `0x${'ab'.repeat(32)}`;
const TIMESTAMP = '2026-01-01T00:00:00.000Z';
const counters: AddressCounters = {
  transactions_count: '1',
  token_transfers_count: '1',
  gas_usage_count: '21000',
  validations_count: '0',
};

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

function tokenBalance(value: string, decimals: string, rate: string | null): TokenBalance {
  return {
    value,
    token_id: null,
    token_instance: null,
    token: {
      address_hash: `0x${'33'.repeat(20)}`,
      name: 'Recorded token',
      symbol: 'UNIT',
      decimals,
      type: 'ERC-20',
      holders_count: '1',
      exchange_rate: rate,
      volume_24h: null,
      total_supply: value,
      icon_url: null,
      circulating_market_cap: null,
    },
  };
}

const transferToken = tokenBalance('7', '0', '1').token;
if (!transferToken) throw new Error('Transfer fixture requires token metadata');
const transfer: TokenTransfer = {
  transaction_hash: HASH,
  from: address(SENDER),
  to: address(WALLET),
  total: { value: '7', decimals: '0' },
  token: transferToken,
  type: 'token_transfer',
  timestamp: TIMESTAMP,
  method: 'transfer',
  block_hash: `0x${'cd'.repeat(32)}`,
  block_number: 42,
  log_index: 3,
};
const transaction: Transaction = {
  hash: HASH,
  result: 'success',
  status: 'ok',
  block_number: 42,
  timestamp: TIMESTAMP,
  from: address(SENDER),
  to: address(WALLET),
  created_contract: null,
  confirmations: 1,
  confirmation_duration: [1],
  value: '1000000000000000000',
  fee: { type: 'actual', value: '21000' },
  gas_price: '1',
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
  token_transfers: [transfer],
  token_transfers_overflow: false,
  actions: null,
  exchange_rate: '1',
  historic_exchange_rate: '1',
  method: 'transfer',
  transaction_types: ['token_transfer'],
  transaction_tag: null,
  has_error_in_internal_transactions: false,
  authorization_list: null,
};

let server: Server;
let native: AddressResponse;
let tokens: TokenBalance[];
let transfers: TokenTransfer[];
let transferStatus: number;
let transferBarrier: (() => Promise<void>) | null;
let requests: string[];
let queryClient: QueryClient;

beforeEach(async () => {
  native = {
    ...address(WALLET),
    coin_balance: '0',
    exchange_rate: '1',
    block_number_balance_updated_at: 42,
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
  tokens = [];
  transfers = [transfer];
  transferStatus = 200;
  transferBarrier = null;
  requests = [];
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    requests.push(url.pathname + url.search);
    response.setHeader('content-type', 'application/json');
    const base = `/api/v2/addresses/${WALLET}`;
    switch (url.pathname) {
      case base:
        response.end(JSON.stringify(native));
        break;
      case `${base}/token-balances`:
        response.end(JSON.stringify(tokens));
        break;
      case `${base}/counters`:
        response.end(JSON.stringify(counters));
        break;
      case `${base}/transactions`:
        response.end(
          JSON.stringify({ items: [transaction], next_page_params: { block_number: 41 } })
        );
        break;
      case `${base}/token-transfers`:
        await transferBarrier?.();
        response.statusCode = transferStatus;
        response.end(
          JSON.stringify(
            transferStatus === 200
              ? { items: transfers, next_page_params: null }
              : { message: 'Token transfer index unavailable' }
          )
        );
        break;
      default:
        response.statusCode = 404;
        response.end(JSON.stringify({ message: `Unexpected route: ${url.pathname}` }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const bound = server.address();
  if (!bound || typeof bound === 'string') throw new Error('HTTP fixture did not bind');
  initBlockscout({ baseUrl: `http://127.0.0.1:${bound.port}` });
});

afterEach(async () => {
  queryClient.clear();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});

async function finding12_preserves_large_balances_and_zero_decimals() {
  native.coin_balance = '9007199254740993123456789000000000';
  native.exchange_rate = '1.25';
  tokens = [
    tokenBalance('9007199254740993', '0', '1'),
    tokenBalance('9007199254740993123456789000000000', '18', '0.1'),
    tokenBalance('0', '0', '0.01'),
  ];
  const portfolio = await getWalletPortfolio(WALLET);
  const holdings = await getWalletHoldings(WALLET);
  const summary = await getWalletBalance(WALLET);
  expect(portfolio.native_balance).toMatchObject({
    balance_raw: native.coin_balance,
    balance: '9007199254740993.123456789',
    value_usd: '11258999068426241.40',
  });
  expect(holdings).toEqual(portfolio.token_holdings);
  expect(holdings.map((holding) => [holding.balance, holding.value_usd])).toEqual([
    ['9007199254740993', '9007199254740993.00'],
    ['9007199254740993.123456789', '900719925474099.31'],
    ['0', '0.00'],
  ]);
  expect(summary.native_balance_usd).toBe('11258999068426241.40');
  expect(summary.token_balance_usd).toBe('9907919180215092.31');
  expect(portfolio.valuation.spot_value_usd).toBe('21166918248641333.72');
  expect(summary.valuation.spot_value_usd).toBe(portfolio.valuation.spot_value_usd);
  expect(requests).toContain(`/api/v2/addresses/${WALLET}/token-balances`);
}

async function finding12_rounds_once_at_each_currency_boundary() {
  tokens = [tokenBalance('1', '0', '0.004'), tokenBalance('1', '0', '0.004')];
  const portfolio = await getWalletPortfolio(WALLET);
  const summary = await getWalletBalance(WALLET);
  expect(portfolio.token_holdings.map((holding) => holding.value_usd)).toEqual(['0.00', '0.00']);
  expect(portfolio.valuation.spot_value_usd).toBe('0.01');
  expect(summary.token_balance_usd).toBe('0.01');
  tokens = [
    tokenBalance('1', '0', '2.675'),
    tokenBalance('15', '0', '1e-3'),
    tokenBalance('1', '0', '.5'),
    tokenBalance('2', '0', '1e3'),
  ];
  expect((await getWalletHoldings(WALLET)).map((holding) => holding.value_usd)).toEqual([
    '2.68',
    '0.02',
    '0.50',
    '2000.00',
  ]);
}

async function finding14_distinguishes_partial_valuation_and_unknown_freshness() {
  native.coin_balance = '1000000000000000000';
  native.exchange_rate = '2';
  tokens = [tokenBalance('10', '0', '3'), tokenBalance('7', '0', null)];
  const portfolio = await getWalletPortfolio(WALLET);
  const summary = await getWalletBalance(WALLET);
  expect(portfolio.total_value_usd).toBeNull();
  expect(portfolio.token_holdings[1].value_usd).toBeNull();
  expect(portfolio.valuation).toEqual({
    status: 'partial',
    spot_status: 'partial',
    priced_assets: 2,
    total_assets: 3,
    priced_value_usd: '32.00',
    spot_value_usd: null,
    perps_status: 'unavailable',
    freshness: {
      fetched_at: portfolio.computed_at,
      price_status: 'unknown',
      prices_updated_at: null,
      native_balance_block: 42,
    },
  });
  expect(Number.isNaN(Date.parse(portfolio.computed_at))).toBe(false);
  expect(summary.native_balance_usd).toBe('2.00');
  expect(summary.token_balance_usd).toBeNull();
  expect(summary.perps_value_usd).toBeNull();
  expect(summary.total_balance_usd).toBeNull();
  expect(summary.valuation).toEqual({
    ...portfolio.valuation,
    freshness: { ...portfolio.valuation.freshness, fetched_at: summary.computed_at },
  });
}

async function finding14_distinguishes_unavailable_prices_from_priced_zero() {
  native.coin_balance = '1000000000000000000';
  native.exchange_rate = null;
  tokens = [tokenBalance('9', '0', null)];
  const unavailable = await getWalletBalance(WALLET);
  expect(unavailable.native_balance_usd).toBeNull();
  expect(unavailable.token_balance_usd).toBeNull();
  expect(unavailable.valuation).toMatchObject({
    status: 'unavailable',
    spot_status: 'unavailable',
    priced_assets: 0,
    total_assets: 2,
    priced_value_usd: null,
    spot_value_usd: null,
  });
  native.coin_balance = '0';
  native.exchange_rate = '1';
  tokens = [tokenBalance('5', '0', '0')];
  const zeroPortfolio = await getWalletPortfolio(WALLET);
  const zeroSummary = await getWalletBalance(WALLET);
  expect(zeroPortfolio.native_balance.value_usd).toBe('0.00');
  expect(zeroPortfolio.token_holdings[0].value_usd).toBe('0.00');
  expect(zeroPortfolio.valuation).toMatchObject({
    status: 'partial',
    spot_status: 'complete',
    priced_assets: 2,
    total_assets: 2,
    priced_value_usd: '0.00',
    spot_value_usd: '0.00',
    perps_status: 'unavailable',
  });
  expect(zeroPortfolio.total_value_usd).toBeNull();
  expect(zeroSummary.native_balance_usd).toBe('0.00');
  expect(zeroSummary.token_balance_usd).toBe('0.00');
  expect(zeroSummary.total_balance_usd).toBeNull();
  expect(zeroSummary.perps_value_usd).toBeNull();
}

async function finding14_rejects_invalid_quotes_and_missing_balance_metadata() {
  tokens = [null, 'NaN', 'Infinity', '-1', '1junk', ''].map((rate) => tokenBalance('1', '0', rate));
  const missingDecimals = tokenBalance('1', '0', '1');
  if (!missingDecimals.token) throw new Error('Missing token fixture');
  missingDecimals.token.decimals = null;
  tokens.push(missingDecimals);
  native.coin_balance = null;
  const portfolio = await getWalletPortfolio(WALLET);
  expect(portfolio.native_balance.value_usd).toBeNull();
  expect(portfolio.token_holdings.every((holding) => holding.value_usd === null)).toBe(true);
  expect(portfolio.valuation.priced_assets).toBe(0);
  expect(portfolio.valuation.spot_status).toBe('unavailable');
}

async function finding15_propagates_structured_transfer_failure() {
  transferStatus = 503;
  await expect(getWalletTransactions(WALLET)).rejects.toMatchObject({
    name: 'BlockscoutApiError',
    status: 503,
    body: JSON.stringify({ message: 'Token transfer index unavailable' }),
  });
  expect(requests).toContain(`/api/v2/addresses/${WALLET}/transactions`);
  expect(requests).toContain(`/api/v2/addresses/${WALLET}/token-transfers`);
}

async function finding15_retains_known_transfers_during_refresh_failure_and_retry() {
  const options = {
    queryKey: ['wallet-history', WALLET],
    queryFn: () => getWalletTransactions(WALLET, { items_count: 1 }),
  };
  const known = await queryClient.fetchQuery(options);
  expect(known.transactions[0].token_transfers).toEqual(known.token_transfers);
  expect(known.token_transfers[0]).toMatchObject({
    tx_hash: HASH,
    amount: '7',
    amount_raw: '7',
    token_decimals: 0,
    direction: 'in',
  });
  expect(known.next_page_params).toEqual({ block_number: 41 });
  let release: () => void = () => {
    throw new Error('Transfer barrier not established');
  };
  let requested: () => void = () => {
    throw new Error('Request signal not established');
  };
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  const started = new Promise<void>((resolve) => {
    requested = resolve;
  });
  transferBarrier = () => {
    requested();
    return pending;
  };
  transferStatus = 503;
  const refresh = queryClient.fetchQuery(options);
  const failure = expect(refresh).rejects.toBeInstanceOf(BlockscoutApiError);
  try {
    await started;
    expect(queryClient.getQueryState(options.queryKey)?.fetchStatus).toBe('fetching');
    expect(queryClient.getQueryData(options.queryKey)).toEqual(known);
  } finally {
    release();
  }
  await failure;
  expect(queryClient.getQueryState(options.queryKey)?.status).toBe('error');
  expect(queryClient.getQueryState(options.queryKey)?.error).toMatchObject({ status: 503 });
  expect(queryClient.getQueryData(options.queryKey)).toEqual(known);
  transferBarrier = null;
  transferStatus = 200;
  transfers = [{ ...transfer, log_index: 4, total: { value: '8', decimals: '0' } }];
  const retried = await queryClient.fetchQuery(options);
  expect(retried.token_transfers[0]).toMatchObject({ amount: '8', log_index: 4 });
  expect(queryClient.getQueryState(options.queryKey)?.status).toBe('success');
  expect(queryClient.getQueryState(options.queryKey)?.error).toBeNull();
  expect(requests).toContain(`/api/v2/addresses/${WALLET}/transactions?items_count=1`);
}

async function finding15_preserves_successful_empty_transfer_history() {
  transfers = [];
  const result = await getWalletTransactions(WALLET);
  expect(result.transactions).toHaveLength(1);
  expect(result.token_transfers).toEqual([]);
}

test(
  finding12_preserves_large_balances_and_zero_decimals.name,
  finding12_preserves_large_balances_and_zero_decimals
);
test(
  finding12_rounds_once_at_each_currency_boundary.name,
  finding12_rounds_once_at_each_currency_boundary
);
test(
  finding14_distinguishes_partial_valuation_and_unknown_freshness.name,
  finding14_distinguishes_partial_valuation_and_unknown_freshness
);
test(
  finding14_distinguishes_unavailable_prices_from_priced_zero.name,
  finding14_distinguishes_unavailable_prices_from_priced_zero
);
test(
  finding14_rejects_invalid_quotes_and_missing_balance_metadata.name,
  finding14_rejects_invalid_quotes_and_missing_balance_metadata
);
test(
  finding15_propagates_structured_transfer_failure.name,
  finding15_propagates_structured_transfer_failure
);
test(
  finding15_retains_known_transfers_during_refresh_failure_and_retry.name,
  finding15_retains_known_transfers_during_refresh_failure_and_retry
);
test(
  finding15_preserves_successful_empty_transfer_history.name,
  finding15_preserves_successful_empty_transfer_history
);
