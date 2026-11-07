// @vitest-environment jsdom
import { createServer, type Server, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider, type UseQueryResult } from '@tanstack/react-query';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { ethers } from 'ethers';
import { initBlockscout } from '@/lib/data/client';
import { fetchPortfolioTransactions } from '@/lib/api';
import { preferencesRepository } from '@/platform/storage/repositories';
import { announceRpcChanged } from '@/lib/constants';
import { parseWalletReadRequest, walletReadResponseHeaders } from '@/lib/wallet-read-contract';
import { virtualNetwork, VIRTUAL_CHAIN_ID_BASE } from '@/lib/networks';
import { TransactionRow } from '@/widgets/transactions/TransactionRow';
import { queryKeys } from './keys';
import { getWalletQueryScope, scopedQueryKey } from './scope';
import { useReconcilePortfolio, usePortfolioQuery } from './portfolio';
import { useTxHistoryQuery, type TxHistoryResult } from './transactions';
import { useTokenMetaQuery, TOKEN_META_NEGATIVE_TTL, TOKEN_META_TTL } from './tokenDetail';
import { useTxDetailQuery } from './txDetail';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const owner = '0x1111111111111111111111111111111111111111';
const recipient = '0x2222222222222222222222222222222222222222';
const token = '0x3333333333333333333333333333333333333333';
const pool = '0x4444444444444444444444444444444444444444';
const hash = ethers.id('query repair transfer');
const codec = new ethers.Interface([
  'event Transfer(address indexed from, address indexed to, uint256 value)',
]);
const nativeFetch = globalThis.fetch;
let server: Server;
let origin: string;
let client: QueryClient;
let root: Root | undefined;
let container: HTMLDivElement;
let chainStatus: number;
let publicStatus: number;
let transferStatus: number;
let malformedChain: boolean;
let malformedPublic: boolean;
let malformedTransfers: boolean;
let units: bigint;
let decimals: string;
let requests: string[];
let heldResponses: ServerResponse[];
let holdHistory: boolean;
let tenant: string;
let heldPortfolios: ServerResponse[];
let holdPortfolio: boolean;

function rawTransfer() {
  const encoded = codec.encodeEventLog(codec.getEvent('Transfer')!, [owner, recipient, units]);
  const decoded = codec.parseLog(encoded)!;
  return {
    transaction_hash: hash,
    from: { hash: decoded.args[0] },
    to: { hash: decoded.args[1] },
    total: { value: decoded.args[2].toString(), decimals },
    token: tokenMetadata(),
    type: 'token_transfer',
    timestamp: '2026-01-01T00:00:00Z',
    method: 'transfer',
    block_hash: hash,
    block_number: 1,
    log_index: 0,
  };
}

function tokenMetadata() {
  return {
    address_hash: token,
    name: 'Whole token',
    symbol: 'UNIT',
    decimals,
    type: 'ERC-20',
    holders_count: '1',
    exchange_rate: null,
    volume_24h: null,
    total_supply: units.toString(),
    icon_url: null,
    circulating_market_cap: null,
  };
}

function transaction() {
  return {
    hash,
    from: { hash: owner },
    to: { hash: recipient },
    value: '0',
    gas_used: '21000',
    gas_price: '1',
    fee: { value: '21000' },
    status: 'ok',
    timestamp: '2026-01-01T00:00:00Z',
    block_number: 1,
    transaction_types: ['token_transfer'],
    token_transfers: [],
  };
}

function respond(response: ServerResponse, body: unknown, status = 200) {
  response.writeHead(status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(body));
}

beforeAll(async () => {
  server = createServer((request, response) => {
    const url = new URL(request.url ?? '', 'http://wallet.invalid');
    const path = url.pathname;
    requests.push(path);
    if (path.startsWith('/api/v2/')) {
      const read = parseWalletReadRequest(url);
      for (const [name, value] of Object.entries(walletReadResponseHeaders(read))) {
        response.setHeader(name, value);
      }
    }
    if (path === `/api/v2/addresses/${owner}/transactions`) {
      if (holdHistory) {
        heldResponses.push(response);
        return;
      }
      respond(response, { items: [transaction()], next_page_params: null });
    } else if (path === `/api/v2/addresses/${owner}/token-transfers`) {
      respond(response, { items: [rawTransfer()], next_page_params: null });
    } else if (path === `/api/v2/addresses/${owner}`) {
      if (holdPortfolio) {
        heldPortfolios.push(response);
        return;
      }
      respond(response, { coin_balance: tenant === 'A' ? '100' : '200', exchange_rate: '1' });
    } else if (path === `/api/v2/addresses/${owner}/token-balances`) {
      respond(response, []);
    } else if (path === `/api/v2/addresses/${owner}/counters`) {
      respond(response, { transactions_count: '1' });
    } else if (path === `/api/v2/tokens/${token}`) {
      respond(response, malformedChain ? {} : tokenMetadata(), chainStatus);
    } else if (path === `/wallet/api/sdk/metadata/metadata/${token}.json`) {
      respond(
        response,
        malformedPublic
          ? { token_address: token, decimals: -1 }
          : {
              token_address: token,
              pool_address: pool,
              symbol: 'UNIT',
              decimals: 0,
            },
        publicStatus
      );
    } else if (path === `/api/v2/transactions/${hash}`) {
      respond(response, transaction());
    } else if (path === `/api/v2/transactions/${hash}/token-transfers`) {
      respond(
        response,
        malformedTransfers
          ? { items: null }
          : {
              items: [rawTransfer()],
              next_page_params: null,
            },
        transferStatus
      );
    } else {
      respond(response, { error: `Unexpected fixture path: ${path}` }, 500);
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Fixture has no TCP address');
  origin = `http://127.0.0.1:${address.port}`;
  // Node fetch needs the origin resolution that browsers provide for relative URLs.
  // Every request still traverses native HTTP to the committed loopback producer.
  globalThis.fetch = (input, init) =>
    nativeFetch(typeof input === 'string' ? new URL(input, origin) : input, init);
  initBlockscout({ baseUrl: origin });
});

afterAll(async () => {
  globalThis.fetch = nativeFetch;
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement('div');
  document.body.append(container);
  root = undefined;
  chainStatus = publicStatus = transferStatus = 200;
  malformedChain = malformedPublic = malformedTransfers = false;
  decimals = '0';
  units = 7n;
  requests = [];
  tenant = 'A';
  holdHistory = holdPortfolio = false;
  heldResponses = [];
  heldPortfolios = [];
});

afterEach(async () => {
  await act(async () => root?.unmount());
  heldResponses.forEach((response) => response.destroy());
  heldPortfolios.forEach((response) => response.destroy());
  client.clear();
  container.remove();
});

async function mount(component: () => React.ReactNode) {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(QueryClientProvider, { client }, createElement(component)));
  });
}

async function settled(assertion: () => void) {
  const deadline = Date.now() + 4_000;
  for (;;) {
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

async function refresh<T>(action: () => Promise<T>) {
  await act(async () => {
    await action();
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
}

function selectTenant(id: number) {
  const generated = virtualNetwork(VIRTUAL_CHAIN_ID_BASE + id, `Tenant ${id}`);
  const network = {
    ...generated,
    rpcUrl: new URL(generated.rpcUrl, `https://api${id}.mainnet-beta.paxeer.network`).href,
    explorerUrl: new URL(generated.explorerUrl, `https://explorer${id}.example`).href,
  };
  preferencesRepository.update((previous) => ({
    ...previous,
    activeChainId: network.chainId,
    networks: [...previous.networks.filter((saved) => saved.chainId !== network.chainId), network],
  }));
  announceRpcChanged();
}

async function finding_10_isolates_equal_addresses_and_cancels_old_reads() {
  let history: UseQueryResult<TxHistoryResult>;
  function HistoryConsumer() {
    const query = useTxHistoryQuery(owner);
    useLayoutEffect(() => {
      history = query;
    });
    return createElement('output', null, query.data?.transfers[0]?.amountRaw ?? 'pending');
  }
  await act(async () => selectTenant(1));
  const firstScope = getWalletQueryScope();
  const factories = [
    () => queryKeys.portfolio(owner),
    () => queryKeys.balance(owner),
    () => queryKeys.holdings(owner),
    () => queryKeys.txHistory(owner),
    () => queryKeys.transfers(owner),
    () => queryKeys.txCount(owner),
    () => queryKeys.txDetail(hash),
    () => queryKeys.txTransfers(hash),
    () => queryKeys.paxPrice(),
    () => queryKeys.pnlHistory(owner, 7),
    () => queryKeys.portfolioChart(owner, '1D'),
    () => queryKeys.rankings('trending'),
  ];
  const firstKeys = factories.map((factory) => factory());
  await mount(HistoryConsumer);
  await settled(() => expect(container.textContent).toBe('7'));
  expect(history!.data?.transfers[0]?.decimals).toBe(0);

  holdHistory = true;
  const oldRefetch = history!.refetch();
  await settled(() => expect(heldResponses).toHaveLength(1));
  units = 11n;
  await act(async () => selectTenant(2));
  await settled(() => expect(heldResponses).toHaveLength(2));
  expect(factories.map((factory) => factory())).not.toEqual(firstKeys);
  for (let i = 0; i < factories.length; i++) expect(factories[i]()).not.toEqual(firstKeys[i]);
  expect(container.textContent).toBe('pending');
  holdHistory = false;
  respond(heldResponses[1], { items: [transaction()], next_page_params: null });
  await settled(() => expect(container.textContent).toBe('11'));
  expect(client.getQueryState(queryKeys.txHistory(owner, 50, firstScope))?.fetchStatus).toBe(
    'idle'
  );
  respond(heldResponses[0], { items: [transaction()], next_page_params: null });
  await act(async () => {
    await oldRefetch;
  });
  expect(container.textContent).toBe('11');
  expect(
    client.getQueryData<TxHistoryResult>(queryKeys.txHistory(owner, 50, firstScope))?.transfers[0]
      .amountRaw
  ).toBe('7');
  expect(
    client.getQueryData<TxHistoryResult>(queryKeys.txHistory(owner))?.transfers[0].amountRaw
  ).toBe('11');
}

test(
  finding_10_isolates_equal_addresses_and_cancels_old_reads.name,
  finding_10_isolates_equal_addresses_and_cancels_old_reads
);

async function finding_10_reconcile_cannot_publish_across_a_switch() {
  let reconcile: ReturnType<typeof useReconcilePortfolio>;
  function PortfolioConsumer() {
    const query = usePortfolioQuery(owner);
    const refresh = useReconcilePortfolio();
    useLayoutEffect(() => {
      reconcile = refresh;
    });
    return createElement('output', null, query.data?.native_balance.balance_raw ?? 'pending');
  }
  await mount(PortfolioConsumer);
  await settled(() => expect(container.textContent).toBe('100'));
  holdPortfolio = true;
  const pending = reconcile!(owner);
  await settled(() => expect(heldPortfolios).toHaveLength(2));
  const oldScope = getWalletQueryScope();
  tenant = 'B';
  holdPortfolio = false;
  await act(async () => selectTenant(3));
  await settled(() => expect(container.textContent).toBe('200'));
  heldPortfolios.forEach((response) =>
    respond(response, { coin_balance: '999', exchange_rate: '1' })
  );
  await act(async () => {
    await pending;
  });
  expect(container.textContent).toBe('200');
  expect(
    client.getQueryData<{ native_balance: { balance_raw: string } }>(
      queryKeys.portfolio(owner, oldScope)
    )?.native_balance.balance_raw
  ).toBe('100');
}

test(
  finding_10_reconcile_cannot_publish_across_a_switch.name,
  finding_10_reconcile_cannot_publish_across_a_switch
);

async function finding_13_zero_decimal_raw_transfer_reaches_history_view() {
  const facade = await fetchPortfolioTransactions(owner);
  expect(facade.token_transfers[0].token_decimals).toBe(0);
  expect(facade.token_transfers[0].amount_raw).toBe('7');
  function HistoryView() {
    const query = useTxHistoryQuery(owner);
    const row = query.data?.transfers[0];
    return row
      ? createElement(TransactionRow, { row, walletAddress: owner, showSymbolInLabel: true })
      : null;
  }
  await mount(HistoryView);
  await settled(() => expect(container.textContent).toMatch(/-7\.0\s*UNIT/));
  expect(container.textContent).toContain('Sent UNIT');
  expect(container.textContent).not.toContain('0.0000 UNIT');
  const row = client.getQueryData<TxHistoryResult>(queryKeys.txHistory(owner))!.transfers[0];
  expect(row).toMatchObject({ amountRaw: '7', decimals: 0 });
  expect(
    renderToStaticMarkup(createElement(TransactionRow, { row, walletAddress: owner }))
  ).toContain('7.0');
}

test(
  finding_13_zero_decimal_raw_transfer_reaches_history_view.name,
  finding_13_zero_decimal_raw_transfer_reaches_history_view
);

async function finding_13_invalid_precision_is_an_error_and_zero_raw_is_preserved() {
  let current: ReturnType<typeof useTxHistoryQuery>;
  function Consumer() {
    const query = useTxHistoryQuery(owner);
    useLayoutEffect(() => {
      current = query;
    });
    return null;
  }
  units = 0n;
  await mount(Consumer);
  await settled(() => expect(current!.isSuccess).toBe(true));
  expect(current!.data?.transfers[0]).toMatchObject({ amountRaw: '0', decimals: 0 });
  decimals = 'invalid';
  await refresh(() => current!.refetch());
  expect(current!.isError).toBe(true);
  expect(current!.error?.message).toContain('Token decimals');
  expect(current!.data?.transfers[0]).toMatchObject({ amountRaw: '0', decimals: 0 });
}

test(
  finding_13_invalid_precision_is_an_error_and_zero_raw_is_preserved.name,
  finding_13_invalid_precision_is_an_error_and_zero_raw_is_preserved
);

async function finding_44_transport_and_decode_failures_retry_and_retain_metadata() {
  let current: ReturnType<typeof useTokenMetaQuery>;
  function MetadataConsumer() {
    const query = useTokenMetaQuery(token, false, false);
    useLayoutEffect(() => {
      current = query;
    });
    return createElement('output', null, query.data?.public?.pool_address ?? 'pending');
  }
  publicStatus = 503;
  await mount(MetadataConsumer);
  await settled(() => expect(current!.isError).toBe(true));
  expect(current!.data).toBeUndefined();
  expect(current!.error?.message).toContain('503');
  publicStatus = 200;
  await refresh(() => current!.refetch());
  await settled(() => expect(container.textContent).toBe(pool));
  const valid = current!.data;
  chainStatus = 503;
  await refresh(() => current!.refetch());
  expect(current!.isError).toBe(true);
  expect(current!.data).toEqual(valid);
  chainStatus = 200;
  malformedChain = true;
  await refresh(() => current!.refetch());
  expect(current!.error?.message).toBe('Invalid chain token metadata');
  expect(current!.data).toEqual(valid);
  malformedChain = false;
  malformedPublic = true;
  await refresh(() => current!.refetch());
  expect(current!.error?.message).toBe('Invalid public token metadata');
  expect(current!.data).toEqual(valid);
  malformedPublic = false;
  await refresh(() => current!.refetch());
  expect(current!.isSuccess).toBe(true);
  expect(current!.data?.public?.pool_address).toBe(pool);
}

test(
  finding_44_transport_and_decode_failures_retry_and_retain_metadata.name,
  finding_44_transport_and_decode_failures_retry_and_retain_metadata
);

async function finding_44_not_found_expires_and_rediscovers_on_remount() {
  let current: ReturnType<typeof useTokenMetaQuery>;
  function MetadataConsumer() {
    const query = useTokenMetaQuery(token, false, false);
    useLayoutEffect(() => {
      current = query;
    });
    return null;
  }
  chainStatus = publicStatus = 404;
  await mount(MetadataConsumer);
  await settled(() => expect(current!.isSuccess).toBe(true));
  expect(current!.data).toEqual({ chain: null, public: null });
  const key = scopedQueryKey(['tokenMeta', token, false]);
  const query = client.getQueryCache().find({ queryKey: key })!;
  expect(query.gcTime).toBe(TOKEN_META_TTL);
  await act(async () => root!.unmount());
  root = undefined;
  client.setQueryData(key, current!.data, { updatedAt: Date.now() - TOKEN_META_NEGATIVE_TTL - 1 });
  chainStatus = publicStatus = 200;
  await mount(MetadataConsumer);
  await settled(() => expect(current!.data?.public?.pool_address).toBe(pool));
  expect(requests.filter((path) => path.includes('/metadata/'))).toHaveLength(2);
}

test(
  finding_44_not_found_expires_and_rediscovers_on_remount.name,
  finding_44_not_found_expires_and_rediscovers_on_remount
);

async function finding_45_partial_transfer_failure_preserves_data_and_retries_independently() {
  let current: ReturnType<typeof useTxDetailQuery>;
  function DetailConsumer() {
    const query = useTxDetailQuery(hash);
    useLayoutEffect(() => {
      current = query;
    });
    return createElement('output', null, query.data?.transferStatus ?? 'pending');
  }
  transferStatus = 503;
  await mount(DetailConsumer);
  await settled(() => expect(current!.data?.transferStatus).toBe('unavailable'));
  expect(current!.error?.message).toContain('503');
  expect(current!.data?.tx.hash).toBe(hash);
  expect(current!.data?.transferError).toBeInstanceOf(Error);
  const txReads = () => requests.filter((path) => path === `/api/v2/transactions/${hash}`).length;
  expect(txReads()).toBe(1);
  transferStatus = 200;
  await refresh(() => current!.refetchTransfers());
  await settled(() => expect(current!.data?.transferStatus).toBe('complete'));
  expect(current!.data?.tokenTransfers[0].total).toMatchObject({ value: '7' });
  const valid = current!.data?.tokenTransfers;
  transferStatus = 503;
  await refresh(() => current!.refetchTransfers());
  expect(current!.data?.transferStatus).toBe('unavailable');
  expect(current!.data?.tokenTransfers).toEqual(valid);
  expect(current!.isError).toBe(true);
  transferStatus = 200;
  malformedTransfers = true;
  await refresh(() => current!.refetchTransfers());
  expect(current!.data?.transferStatus).toBe('unavailable');
  expect(current!.data?.transferError?.message).toBe('Invalid transaction token transfer response');
  expect(current!.data?.tokenTransfers).toEqual(valid);
  malformedTransfers = false;
  units = 12n;
  await refresh(() => current!.refetchTransfers());
  expect(current!.data?.transferStatus).toBe('complete');
  expect(current!.data?.tokenTransfers[0].total).toMatchObject({ value: '12' });
  expect(current!.isError).toBe(false);
  expect(txReads()).toBe(1);
}

test(
  finding_45_partial_transfer_failure_preserves_data_and_retries_independently.name,
  finding_45_partial_transfer_failure_preserves_data_and_retries_independently
);
