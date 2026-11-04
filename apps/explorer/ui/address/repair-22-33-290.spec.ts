// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, configure, fireEvent, render, waitFor, within } from '@testing-library/react';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/router';
import Router from 'next/router';
import { parse } from 'papaparse';
import React from 'react';

import type { AddressCoinBalanceHistoryChart, AddressCoinBalanceHistoryResponse, AddressNFTsResponse } from 'types/api/address';

import * as address from 'mocks/address/address';
import { nativeUnifiedAccount } from 'mocks/paxeerX/unifiedAccount';
import * as transfers from 'mocks/tokens/tokenTransfer';
import * as internal from 'mocks/txs/internalTxs';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

import { AddressRepairPeer } from './repair-22-33-290/peer';

configure({ asyncUtilTimeout: 30_000 });

const h = React.createElement;
const peer = new AddressRepairPeer();
const clients = new Set<QueryClient>();
const socketOptions = { reconnectAfterMs: () => 30, rejoinAfterMs: () => 30 };
const port = await peer.start();
// The gate owns every endpoint it uses. Config is loaded only after the loopback port is known.
window.__envs = {
  ...window.__envs,
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String(port),
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_APP_ENV: 'testing',
  NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
};
fetchMock.disableMocks();
const { Wrapper } = await import('ui/shared/layout/testWrapper');
const { SocketProvider } = await import('lib/socket/context');
const { 'default': useSocketChannel } = await import('lib/socket/useSocketChannel');
const { getResourceKey } = await import('lib/api/useApiQuery');
const { 'default': AddressCoinBalance } = await import('./AddressCoinBalance');
const { 'default': AddressContract } = await import('./AddressContract');
const { 'default': AddressInternalTxs } = await import('./AddressInternalTxs');
const { 'default': AddressTokenTransfers } = await import('./AddressTokenTransfers');
const { 'default': AddressTokens } = await import('./AddressTokens');
const { 'default': AddressDetails } = await import('./AddressDetails');
const { 'default': useAddressQuery } = await import('./utils/useAddressQuery');
const { 'default': useAddressCountersQuery } = await import('./utils/useAddressCountersQuery');
HTMLElement.prototype.scrollIntoView = function scrollIntoView() {};

beforeEach(() => {
  peer.reset();
  document.cookie = 'address_nft_display_type=; max-age=0; path=/';
  document.cookie = 'address_nft_display_type=; max-age=0; path=/explorer';
});

afterEach(async() => {
  cleanup();
  await Promise.all(Array.from(clients, client => client.cancelQueries()));
  clients.forEach(client => client.clear());
  clients.clear();
});
afterAll(async() => {
  cleanup();
  await peer.close();
  fetchMock.enableMocks();
});

// This in-memory route store supplies the normal Next context and applies real shallow transitions.
// Query hooks and both page controls read the same route state after each navigation.
function mount(ui: React.ReactElement, query: NextRouter<'/address/[hash]'>['query'], socket = false, client = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
})) {
  clients.add(client);
  function RouteStore({ children }: { children: React.ReactNode }) {
    const [ currentQuery, setQuery ] = React.useState(query);
    const navigate: NextRouter['push'] = async(url) => {
      const next = typeof url === 'string' ? Object.fromEntries(new URL(url, 'http://localhost').searchParams) : url.query || {};
      const normalized = Object.fromEntries(Object.entries(next).map(([ key, value ]) => {
        if (Array.isArray(value)) {
          return [ key, value.map(String) ];
        }
        return [ key, value === undefined ? undefined : String(value) ];
      }));
      if (typeof normalized.hash !== 'string') {
        throw new Error('Address navigation requires a hash');
      }
      setQuery({ ...normalized, hash: normalized.hash });
      return true;
    };
    const router: NextRouter<'/address/[hash]'> = {
      basePath: '', pathname: '/address/[hash]', route: '/address/[hash]', asPath: `/address/${ address.hash }`,
      query: currentQuery, isReady: true, isFallback: false, isPreview: false, isLocaleDomain: false,
      push: navigate, replace: navigate, reload() {}, back() {}, forward() {}, prefetch: async() => {},
      beforePopState() {}, events: Router.events,
    };
    return h(RouterContext.Provider, { value: router }, h(Wrapper, null,
      h(QueryClientProvider, { client }, h(SocketProvider, {
        url: socket ? `ws://127.0.0.1:${ port }/socket` : undefined, options: socketOptions, children,
      }))));
  }
  return { ...render(h(RouteStore, null, ui)), client };
}

function history(client: QueryClient) {
  return client.getQueryData<AddressCoinBalanceHistoryResponse>(getResourceKey('general:address_coin_balance', { pathParams: { hash: address.hash } }));
}

function chart(client: QueryClient) {
  return client.getQueryData<AddressCoinBalanceHistoryChart>(getResourceKey('general:address_coin_balance_chart', { pathParams: { hash: address.hash } }));
}

async function settled(client: QueryClient) {
  await waitFor(() => expect(client.isFetching()).toBe(0), { timeout: 30_000 });
}

function SharedAddressChannel() {
  useSocketChannel({ topic: `addresses:${ address.hash.toLowerCase() }`, isDisabled: false });
  return null;
}

async function coinHistoryRestartRecovery() {
  for (let index = 0; index < 55; index++) {
    peer.changeBalance(BigInt(100), false);
  }
  const view = mount(h(React.Fragment, null, h(SharedAddressChannel), h(AddressCoinBalance)),
    { hash: address.hash, tab: 'coin_balance_history' }, true);
  await waitFor(() => {
    expect(history(view.client)).toEqual(peer.history());
    expect(chart(view.client)).toEqual(peer.chart());
    expect(peer.joins.size).toBe(1);
  }, { timeout: 30_000 });
  await settled(view.client);
  peer.disconnect();
  await waitFor(() => expect(view.container.textContent).toContain('Connection lost'));
  peer.changeBalance(BigInt(300), false);
  peer.changeBalance(BigInt(-20), false);
  peer.changeBalance(BigInt(75), false);
  peer.historyUnavailable = true;
  peer.chartUnavailable = true;
  await waitFor(() => expect(peer.joins.size).toBe(1));
  const requestCount = peer.requests.length;
  peer.rejoin();
  await waitFor(() => expect(peer.requests.length).toBeGreaterThan(requestCount));
  await settled(view.client);
  expect(view.container.textContent).toContain('Connection lost');
  // A live event cannot certify recovery when either authoritative read is still unavailable.
  peer.historyUnavailable = false;
  const firstAttempt = peer.requests.length;
  peer.changeBalance(BigInt(25));
  await waitFor(() => expect(peer.requests.length).toBeGreaterThan(firstAttempt));
  await settled(view.client);
  expect(view.container.textContent).toContain('Connection lost');
  peer.chartUnavailable = false;
  peer.deliver(peer.changes.get(peer.block)!);
  await waitFor(() => {
    expect(history(view.client)).toEqual(peer.history());
    expect(chart(view.client)).toEqual(peer.chart());
    expect(view.container.textContent).not.toContain('Connection lost');
  });
  // Reopening the production component with its existing query cache also recovers missed changes.
  view.unmount();
  peer.changeBalance(BigInt(40), false);
  peer.changeBalance(BigInt(-5), false);
  const reopened = mount(h(AddressCoinBalance), { hash: address.hash, tab: 'coin_balance_history' }, true, view.client);
  await waitFor(() => {
    expect(history(reopened.client)).toEqual(peer.history());
    expect(chart(reopened.client)).toEqual(peer.chart());
    expect(reopened.container.textContent).not.toContain('Connection lost');
  });
}

test('coinHistoryRestartRecovery', async() => {
  expect.hasAssertions();
  await coinHistoryRestartRecovery();
}, 60_000);

async function coinHistoryBoundedStreamCursorAndVisibleCount() {
  for (let index = 0; index < 70; index++) {
    peer.changeBalance(BigInt(100), false);
  }
  const view = mount(h(AddressCoinBalance), { hash: address.hash, tab: 'coin_balance_history' }, true);
  await waitFor(() => {
    expect(history(view.client)).toEqual(peer.history());
    expect(peer.joins.size).toBe(1);
  }, { timeout: 30_000 });
  const duplicate = peer.changes.get(peer.block)!;
  const requestCount = peer.requests.length;
  for (let delivery = 0; delivery < 5; delivery++) {
    peer.deliver(duplicate);
  }
  await waitFor(() => expect(peer.requests.length).toBeGreaterThan(requestCount));
  await settled(view.client);
  expect(history(view.client)).toEqual(peer.history());
  for (let index = 0; index < 120; index++) {
    peer.changeBalance(BigInt(index + 1));
  }
  await waitFor(() => expect(history(view.client)).toEqual(peer.history()));
  await settled(view.client);
  expect(history(view.client)?.items).toHaveLength(50);
  expect(new Set(history(view.client)?.items.map(row => row.block_number)).size).toBe(50);
  expect(view.container.querySelector('[data-title]')?.textContent).toBe('50 visible balance changes');
  const cursor = peer.history().next_page_params!;
  fireEvent.click(view.getAllByLabelText('Next page')[0]);
  await waitFor(() => {
    const page = peer.requests.find(url =>
      url.pathname.endsWith('/coin-balance-history') && url.searchParams.get('block_number') === String(cursor.block_number));
    expect(page).toBeDefined();
    expect(view.container.querySelector('tbody')?.textContent).toContain(String(cursor.block_number - 1));
  });
  expect(view.container.querySelector('[data-title]')?.textContent).toBe('50 visible balance changes');
  expect(view.container.textContent).not.toContain('A total of');
}

test('coinHistoryBoundedStreamCursorAndVisibleCount', async() => {
  expect.hasAssertions();
  await coinHistoryBoundedStreamCursorAndVisibleCount();
}, 60_000);

async function contractHttpWithoutSocketAndDuringStalledJoin() {
  for (const socket of [ false, true ]) {
    peer.stalled = true;
    const view = mount(h(AddressContract, { addressData: address.contract }), { hash: address.contract.hash, tab: 'contract_code' }, socket);
    await waitFor(() => {
      const result = view.client.getQueryData<{ source_code: string }>(getResourceKey('general:contract', { pathParams: { hash: address.contract.hash } }));
      expect(result?.source_code).toBe('source_code');
      expect(view.container.textContent).toContain('Contract source code');
    }, { timeout: 30_000 });
    if (socket) {
      await waitFor(() => expect(Array.from(peer.joins.values()).some(frame => frame[2] === `addresses:${ address.contract.hash.toLowerCase() }`)).toBe(true));
      expect(peer.stalled).toBe(true);
    }
    view.unmount();
  }
}

test('contractHttpWithoutSocketAndDuringStalledJoin', async() => {
  expect.hasAssertions();
  await contractHttpWithoutSocketAndDuringStalledJoin();
}, 60_000);

function DetailsHarness() {
  const addressQuery = useAddressQuery({ hash: address.hash });
  const countersQuery = useAddressCountersQuery({ hash: address.hash, isLoading: addressQuery.isPlaceholderData });
  return h(AddressDetails, { addressQuery, countersQuery, isLoading: addressQuery.isPlaceholderData });
}

async function unifiedIdentityNativeResponseValuesAndLink() {
  const view = mount(h(DetailsHarness), { hash: address.hash });
  await waitFor(() => expect(view.container.querySelectorAll('[data-identity]')).toHaveLength(3), { timeout: 30_000 });
  expect(Array.from(view.container.querySelectorAll('[data-identity]')).map(node => node.getAttribute('data-identity'))).toEqual([ 'evm', 'pax', 'did' ]);
  for (const [ kind, value ] of Object.entries(nativeUnifiedAccount.identities)) {
    expect(view.container.querySelector(`[data-identity="${ kind }"]`)?.textContent).toBe(value);
  }
  expect(view.container.querySelector('[data-unified-account-link]')?.getAttribute('href')).toBe(`/paxeer-x/account/${ address.hash }`);
  expect(peer.requests.some(url => url.pathname.endsWith('/unified'))).toBe(true);
}

test('unifiedIdentityNativeResponseValuesAndLink', async() => {
  expect.hasAssertions();
  await unifiedIdentityNativeResponseValuesAndLink();
}, 60_000);

function pageCsv(container: HTMLElement) {
  const link = within(container).getByText('Download Page Data').closest('a')!;
  expect(link.getAttribute('download')).toMatch(/_page.csv$/);
  expect(link.getAttribute('href')).toMatch(/^data:text\/csv;charset=utf-8,/);
  const result = parse<Record<string, string>>(decodeURIComponent(link.getAttribute('href')!.split(',').slice(1).join(',')), { header: true });
  expect(result.errors).toEqual([]);
  return result.data;
}

async function internalPageCsvFilteredSecondPage() {
  const view = mount(h(AddressInternalTxs), { hash: address.hash, tab: 'internal_txs', filter: 'from' });
  await waitFor(() => {
    const state = view.client.getQueryState(getResourceKey('general:address_internal_txs', {
      pathParams: { hash: address.hash }, queryParams: { filter: 'from' },
    }));
    expect({ status: state?.status, error: state?.error }).toEqual({ status: 'success', error: null });
  }, { timeout: 30_000 });
  await waitFor(() => expect(view.container.querySelector('a[download]')?.getAttribute('href')).toMatch(/^data:/), { timeout: 30_000 });
  expect(pageCsv(view.container).map(row => row.transaction_hash)).toEqual([ internal.base.transaction_hash ]);
  fireEvent.click(view.getAllByLabelText('Next page')[0]);
  await waitFor(() => expect(pageCsv(view.container).map(row => row.transaction_hash)).toEqual([ internal.typeStaticCall.transaction_hash ]));
  const request = peer.requests.find(url => url.pathname.endsWith('/internal-transactions') && url.searchParams.has('index'))!;
  expect(request.searchParams.get('filter')).toBe('from');
  expect(request.searchParams.get('index')).toBe('1');
  const dateLink = view.getByText('Date-range CSV Export').closest('a')!;
  expect(dateLink.getAttribute('href')).toContain('filterValue=from');
  expect(dateLink.getAttribute('href')).not.toContain('index=');
}

test('internalPageCsvFilteredSecondPage', async() => {
  expect.hasAssertions();
  await internalPageCsvFilteredSecondPage();
}, 60_000);

async function tokenPageCsvPreservesTypeTokenDirectionAndPage() {
  const token = transfers.erc20.token;
  if (!token) {
    throw new Error('The committed ERC-20 transfer fixture must carry token metadata');
  }
  const view = mount(h(AddressTokenTransfers), {
    hash: address.hash, tab: 'token_transfers', type: 'ERC-20', filter: 'from', token: token.address_hash,
  });
  await waitFor(() => expect(view.container.querySelector('a[download]')?.getAttribute('href')).toMatch(/^data:/), { timeout: 30_000 });
  expect(pageCsv(view.container).map(row => row.transaction_hash)).toEqual([ transfers.erc20.transaction_hash ]);
  fireEvent.click(view.getAllByLabelText('Next page')[0]);
  await waitFor(() => expect(pageCsv(view.container).map(row => row.transaction_hash)).toEqual([ internal.typeStaticCall.transaction_hash ]));
  const rows = pageCsv(view.container);
  expect(JSON.parse(rows[0].token)).toMatchObject({ type: 'ERC-20', address_hash: token.address_hash });
  const request = peer.requests.find(url => url.pathname.endsWith('/token-transfers') && url.searchParams.has('index'))!;
  expect(request.searchParams.get('type')).toBe('ERC-20');
  expect(request.searchParams.get('token')).toBe(token.address_hash);
  expect(request.searchParams.get('filter')).toBe('from');
  expect(view.getByText('Date-range CSV Export (all tokens)')).toBeTruthy();
}

test('tokenPageCsvPreservesTypeTokenDirectionAndPage', async() => {
  expect.hasAssertions();
  await tokenPageCsvPreservesTypeTokenDirectionAndPage();
}, 60_000);

async function nftPageVisibleCountCollectionListAndFilterChange() {
  const view = mount(h(AddressTokens), { hash: address.hash, tab: 'tokens_nfts' });
  await waitFor(() => expect(view.container.textContent).toContain('1 visible collections'), { timeout: 30_000 });
  fireEvent.click(view.getAllByLabelText('Next page')[0]);
  await waitFor(() => expect(peer.requests.some(url =>
    url.pathname.endsWith('/nft/collections') && url.searchParams.has('token_contract_address_hash'))).toBe(true));
  await settled(view.client);
  expect(view.container.textContent).toContain('1 visible collections');
  fireEvent.click(view.getByText('List'));
  await waitFor(() => expect(view.container.textContent).toContain('1 visible NFTs'));
  // Display mode preserves page 2; explicitly return to page 1 before applying a new type selection.
  fireEvent.click(view.getAllByText('First', { selector: 'button' })[0]);
  await settled(view.client);
  await waitFor(() => expect(view.getAllByLabelText('Next page')[0].hasAttribute('disabled')).toBe(false));
  fireEvent.click(view.getAllByLabelText('Next page')[0]);
  await waitFor(() => expect(peer.requests.some(url => url.pathname.endsWith('/nft') && url.searchParams.has('id'))).toBe(true));
  await settled(view.client);
  expect(view.container.textContent).toContain('1 visible NFTs');
  fireEvent.click(view.getByText('Filter'));
  fireEvent.click(await view.findByLabelText('ERC-721'));
  await waitFor(() => {
    expect(peer.requests.some(url => url.pathname.endsWith('/nft') && url.searchParams.get('type') === 'ERC-721' && !url.searchParams.has('id'))).toBe(true);
    const filtered = view.client.getQueryData<AddressNFTsResponse>(getResourceKey('general:address_nfts', {
      pathParams: { hash: address.hash }, queryParams: { type: [ 'ERC-721' ] },
    }));
    expect(filtered?.items).toHaveLength(1);
    expect(filtered?.items[0].token.type).toBe('ERC-721');
    expect(view.container.textContent).toContain('1 visible NFTs');
  });
  expect(view.container.textContent).not.toContain('A total of');
}

test('nftPageVisibleCountCollectionListAndFilterChange', async() => {
  expect.hasAssertions();
  await nftPageVisibleCountCollectionListAndFilterChange();
}, 60_000);
