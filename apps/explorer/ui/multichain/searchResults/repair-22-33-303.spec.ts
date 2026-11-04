// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import dotenv from 'dotenv';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';

import type * as multichain from '@blockscout/multichain-aggregator-types';

import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import type { QueryType, SearchQueries } from './utils';
import { SEARCH_TABS_IDS } from './utils';

const originalEnvs = window.__envs;
const SEARCH_TERM = 'tenant & café.eth';
const CHAIN_ID = '125000001';
const SEARCH_PATH = '/api/v1/clusters/tenant/search/';
const domains: Array<multichain.Domain> = Array.from({ length: 6 }, (_, index) => ({
  name: `tenant-${ index }.eth`,
  address: `0x${ String(index + 1).padStart(40, '0') }`,
  protocol: undefined,
}));
const requests: Array<URL> = [];
const failures = new Set<string>();
let domainCount = 1;
const server = createServer((request, response) => {
  const url = new URL(request.url || '/', 'http://localhost');
  requests.push(url);
  response.setHeader('content-type', 'application/json');
  const category = url.pathname.slice(SEARCH_PATH.length);
  if (!url.pathname.startsWith(SEARCH_PATH) || ![ 'addresses', 'tokens', 'block-numbers', 'blocks', 'transactions', 'nfts', 'domains' ].includes(category)) {
    response.writeHead(404);
    response.end(JSON.stringify({ message: 'Unexpected fixture request' }));
    return;
  }
  if (failures.has(category)) {
    response.writeHead(503);
    response.end(JSON.stringify({ message: 'Search category temporarily unavailable' }));
    return;
  }
  const payload: multichain.SearchDomainsResponse = {
    items: category === 'domains' ? domains.slice(0, domainCount) : [],
    next_page_params: undefined,
  };
  response.end(JSON.stringify(payload));
});

const fixtureEnvs = dotenv.config({ path: './configs/envs/.env.vitest' });
if (fixtureEnvs.error || !fixtureEnvs.parsed) throw fixtureEnvs.error || new Error('Missing committed explorer test configuration');
server.listen(0, '127.0.0.1');
await once(server, 'listening');
const address = server.address();
if (!address || typeof address === 'string') throw new Error('Expected a loopback TCP address');
window.__envs = {
  ...fixtureEnvs.parsed,
  NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST: `http://127.0.0.1:${ address.port }`,
  NEXT_PUBLIC_MULTICHAIN_CLUSTER: 'tenant',
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_APP_ENV: 'testing',
};
// Reuse the committed jsdom viewport/observer fixture; application modules and HTTP remain real.
await import('ui/shared/layout/testWrapper');
const { 'default': SearchResultTabContent } = await import('./SearchResultTabContent');
const { 'default': useApiInfiniteQuery } = await import('lib/api/useApiInfiniteQuery');
const { 'default': theme } = await import('toolkit/theme/theme');
const { getResourceKey } = await import('lib/api/useApiQuery');

beforeAll(() => {
  fetchMock.disableMocks();
});

afterEach(() => {
  cleanup();
  failures.clear();
  requests.length = 0;
  domainCount = 1;
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
});

function SearchConsumer({ url }: { url: URL }) {
  const searchTerm = url.searchParams.get('q') || '';
  const chainId = url.searchParams.get('chain_id') || undefined;
  const queryParams = { q: searchTerm, chain_id: chainId === 'all' ? undefined : chainId };
  const queries: SearchQueries = {
    addresses: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_addresses', queryParams }),
    tokens: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_tokens', queryParams }),
    blockNumbers: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_block_numbers', queryParams }),
    blocks: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_blocks', queryParams }),
    transactions: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_transactions', queryParams }),
    nfts: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_nfts', queryParams }),
    domains: useApiInfiniteQuery({ resourceName: 'multichainAggregator:search_domains', queryParams }),
  };
  const tab = url.searchParams.get('tab');
  const queryType = (Object.keys(SEARCH_TABS_IDS) as Array<QueryType>).find(type => SEARCH_TABS_IDS[type] === tab);
  return React.createElement(SearchResultTabContent, {
    queries,
    queryType,
    isLoading: Object.values(queries).some(query => query.isPending),
    searchTerm,
    chainId,
    beforeContent: React.createElement('form', null,
      React.createElement('input', { 'aria-label': 'Search term', value: searchTerm, readOnly: true }),
      React.createElement('input', { 'aria-label': 'Selected chain', value: chainId || 'all', readOnly: true })),
  });
}

function searchUrl(tab = 'all', chainId: string | undefined = CHAIN_ID) {
  const url = new URL('/search-results', window.location.origin);
  url.searchParams.set('q', SEARCH_TERM);
  url.searchParams.set('tab', tab);
  if (chainId !== undefined) url.searchParams.set('chain_id', chainId);
  return url;
}

function mountSearch(url = searchUrl()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  const mounted = render(React.createElement(ChakraProvider, {
    value: theme,
    children: React.createElement(QueryClientProvider, { client }, React.createElement(SearchConsumer, { url })),
  }));
  return {
    client,
    stop: () => {
      mounted.unmount();
      client.clear();
    },
  };
}

function assertContext() {
  expect((screen.getByRole('textbox', { name: 'Search term' }) as HTMLInputElement).value).toBe(SEARCH_TERM);
  expect((screen.getByRole('textbox', { name: 'Selected chain' }) as HTMLInputElement).value).toBe(CHAIN_ID);
  expect(requests.length).toBeGreaterThan(0);
  for (const url of requests) {
    expect(url.pathname.startsWith(SEARCH_PATH)).toBe(true);
    expect(Object.fromEntries(url.searchParams)).toEqual({ q: SEARCH_TERM, chain_id: CHAIN_ID });
  }
}

async function selectedFailureShowsRetryAndRecoversWithSearchContext() {
  failures.add('domains');
  const mounted = mountSearch(searchUrl('names'));
  try {
    expect((await screen.findByRole('alert')).textContent).toContain('Unable to load names');
    expect(screen.queryByText('No names found')).toBeNull();
    const attempts = requests.filter(url => url.pathname.endsWith('/domains')).length;
    assertContext();
    failures.delete('domains');
    fireEvent.click(screen.getByRole('button', { name: 'Retry names' }));
    await screen.findByText(domains[0].name);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(requests.filter(url => url.pathname.endsWith('/domains'))).toHaveLength(attempts + 1);
    assertContext();
  } finally {
    mounted.stop();
  }
}

async function partialFailureKeepsSuccessfulResultsAndRetriesOnlyFailedCategory() {
  failures.add('tokens');
  const mounted = mountSearch();
  try {
    await screen.findByText(domains[0].name);
    expect((await screen.findByRole('alert')).textContent).toContain('Unable to load tokens');
    expect(screen.queryByText('No results found')).toBeNull();
    await waitFor(() => expect(mounted.client.isFetching()).toBe(0));
    const beforeRetry = requests.length;
    failures.delete('tokens');
    fireEvent.click(screen.getByRole('button', { name: 'Retry tokens' }));
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
    expect(screen.getByText(domains[0].name)).toBeDefined();
    expect(requests.slice(beforeRetry).map(url => url.pathname)).toEqual([ `${ SEARCH_PATH }tokens` ]);
    assertContext();
  } finally {
    mounted.stop();
  }
}

async function refetchFailureKeepsCachedResultsUntilRetryRecovers() {
  const mounted = mountSearch(searchUrl('names'));
  try {
    await screen.findByText(domains[0].name);
    failures.add('domains');
    await act(async() => {
      await mounted.client.invalidateQueries({
        queryKey: getResourceKey('multichainAggregator:search_domains', { queryParams: { q: SEARCH_TERM, chain_id: CHAIN_ID } }),
      });
    });
    await screen.findByRole('alert');
    expect(screen.getByText(domains[0].name)).toBeDefined();
    expect(screen.queryByText('No names found')).toBeNull();
    failures.delete('domains');
    fireEvent.click(screen.getByRole('button', { name: 'Retry names' }));
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
    expect(screen.getByText(domains[0].name)).toBeDefined();
    assertContext();
  } finally {
    mounted.stop();
  }
}

async function allFailuresAreVisibleAndNeverReportedAsEmpty() {
  for (const category of [ 'addresses', 'tokens', 'block-numbers', 'blocks', 'transactions', 'nfts', 'domains' ]) failures.add(category);
  const mounted = mountSearch();
  try {
    await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(7));
    expect(screen.queryByText('No results found')).toBeNull();
    expect(screen.getAllByRole('button', { name: /^Retry / })).toHaveLength(7);
    assertContext();
  } finally {
    mounted.stop();
  }
}

async function successfulEmptySearchStillShowsEmptyState() {
  domainCount = 0;
  for (const tab of [ 'all', 'names' ]) {
    const mounted = mountSearch(searchUrl(tab));
    try {
      await screen.findByText(tab === 'all' ? 'No results found' : 'No names found');
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.queryByRole('button', { name: /^Retry / })).toBeNull();
      assertContext();
    } finally {
      mounted.stop();
    }
  }
}

async function viewAllClickPreservesTenantQueryAndChainThroughDestinationRequest() {
  domainCount = 6;
  const mounted = mountSearch();
  let destination: URL | undefined;
  try {
    const link = await screen.findByRole('link', { name: 'View all' });
    expect(screen.getAllByText(/^tenant-\d\.eth$/)).toHaveLength(5);
    link.addEventListener('click', event => {
      event.preventDefault();
      destination = new URL((event.currentTarget as HTMLAnchorElement).href);
    });
    fireEvent.click(link);
    expect(destination?.pathname).toBe('/search-results');
    expect(destination && Object.fromEntries(destination.searchParams)).toEqual({ q: SEARCH_TERM, chain_id: CHAIN_ID, tab: 'names' });
  } finally {
    mounted.stop();
  }
  if (!destination) throw new Error('View all did not produce a destination');
  requests.length = 0;
  const detailed = mountSearch(destination);
  try {
    await screen.findByText(domains[5].name);
    expect(screen.getAllByText(/^tenant-\d\.eth$/)).toHaveLength(6);
    expect(screen.queryByRole('link', { name: 'View all' })).toBeNull();
    assertContext();
  } finally {
    detailed.stop();
  }
}

async function viewAllPreservesAllChainsAndUnfilteredSearch() {
  domainCount = 6;
  for (const chainId of [ 'all', undefined ]) {
    const url = searchUrl();
    if (chainId === undefined) url.searchParams.delete('chain_id');
    else url.searchParams.set('chain_id', chainId);
    const mounted = mountSearch(url);
    try {
      const link = await screen.findByRole('link', { name: 'View all' });
      const destination = new URL((link as HTMLAnchorElement).href);
      expect(destination.searchParams.get('q')).toBe(SEARCH_TERM);
      expect(destination.searchParams.get('chain_id')).toBe(chainId ?? null);
      expect(destination.searchParams.get('tab')).toBe('names');
      expect(requests.filter(request => request.pathname.endsWith('/domains')).every(request => !request.searchParams.has('chain_id'))).toBe(true);
    } finally {
      mounted.stop();
    }
  }
}

test(selectedFailureShowsRetryAndRecoversWithSearchContext.name, selectedFailureShowsRetryAndRecoversWithSearchContext);
test(partialFailureKeepsSuccessfulResultsAndRetriesOnlyFailedCategory.name, partialFailureKeepsSuccessfulResultsAndRetriesOnlyFailedCategory);
test(refetchFailureKeepsCachedResultsUntilRetryRecovers.name, refetchFailureKeepsCachedResultsUntilRetryRecovers);
test(allFailuresAreVisibleAndNeverReportedAsEmpty.name, allFailuresAreVisibleAndNeverReportedAsEmpty);
test(successfulEmptySearchStillShowsEmptyState.name, successfulEmptySearchStillShowsEmptyState);
test(viewAllClickPreservesTenantQueryAndChainThroughDestinationRequest.name, viewAllClickPreservesTenantQueryAndChainThroughDestinationRequest);
test(viewAllPreservesAllChainsAndUnfilteredSearch.name, viewAllPreservesAllChainsAndUnfilteredSearch);
