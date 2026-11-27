import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { decodeV2Contract } from '@kindle/sdk';
import { afterEach, expect, test, vi } from 'vitest';
import type { PoolSnapshot } from '@/core/market/pool-resolver';
import * as holdersRead from '@/core/market/use-holders';
import * as identitiesRead from '@/core/market/use-wallet-identities';
import { queryKeys } from '@/core/query-keys';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { shortAddress } from '@/utils/format';
import HoldersTable from './holders-table';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const response = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body);
const detail = response.data;
const wallet = detail.pool.creator.toLowerCase();
const snapshot: PoolSnapshot = {
  pool: { chainId: '125', poolAddress: detail.pool.poolAddress, tokenAddress: detail.token.address },
  token: { ...detail.token, decimals: 6, decimalsQuality: 'verified', totalSupplyRaw: '1000000000', supplyQuality: 'verified' },
  stats: { ...detail.stats, spotPriceWad: '1000000000000000000' },
  metadata: null,
  creator: wallet,
  createdAt: null,
  generatedAt: response.generatedAt,
  resolvedBy: 'detail',
  unavailable: { stats: null, metadata: null, registry: null },
};
const clients: QueryClient[] = [];

afterEach(() => {
  vi.restoreAllMocks();
  for (const client of clients.splice(0)) client.clear();
});

function renderHolders(displayName: string | null = null) {
  const page: holdersRead.HoldersPage = {
    rows: [{ wallet, tokenAddress: detail.token.address, balanceRaw: '250000000', share: { numerator: '250000000', denominator: '1000000000' } }],
    response: null,
    canonical: null,
    legacyShare: null,
    legacySource: null,
    nextCursor: null,
  };
  vi.spyOn(holdersRead, 'useHolders').mockReturnValue({ state: 'ready', data: page });
  const identity: identitiesRead.WalletIdentity = {
    wallet, displayName, avatarUrl: null, shortAddress: shortAddress(wallet), profile: null, resolution: 'profile',
  };
  vi.spyOn(identitiesRead, 'useWalletIdentities').mockReturnValue({
    identities: { [wallet]: identity }, isPending: false, label: () => displayName ?? shortAddress(wallet),
  });
  const client = new QueryClient();
  clients.push(client);
  client.setQueryData(queryKeys.poolSnapshot(snapshot.pool.poolAddress), snapshot);
  const markup = JSDOM.fragment(renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <HoldersTable poolAddress={snapshot.pool.poolAddress} />
    </QueryClientProvider>,
  ));
  expect(markup.querySelectorAll('tbody tr')).toHaveLength(1);
  expect(markup.querySelector('[data-holder-rank]')?.textContent).toBe('1');
  return markup;
}

function holdersTableKeepsWordsWhole() {
  const markup = renderHolders();
  expect(markup.querySelector('table')?.classList.contains('coin-holders-table')).toBe(true);
  const css = readFileSync(new URL('./coin-market.css', import.meta.url), 'utf8');
  expect(css).toMatch(/\.coin-holders-table\s*\{\s*overflow-wrap:\s*normal;\s*\}/);
  expect(markup.querySelector('[role="region"]')?.classList.contains('overflow-x-auto')).toBe(true);
  expect(markup.querySelector('table')?.classList.contains('min-w-160')).toBe(true);
}
test(holdersTableKeepsWordsWhole.name, holdersTableKeepsWordsWhole);

function holdersHeadingsStayOnOneLine() {
  const headings = renderHolders().querySelectorAll('th[scope="col"]');
  expect(headings).toHaveLength(4);
  for (const [index, label] of ['Rank / Holder', 'Held', '% supply', 'Position'].entries()) {
    expect(headings[index].textContent).toContain(label);
    expect(headings[index].classList.contains('whitespace-nowrap')).toBe(true);
  }
}
test(holdersHeadingsStayOnOneLine.name, holdersHeadingsStayOnOneLine);

function holdersFiguresStayOnOneLine() {
  for (const displayName of [null, 'A holder with a long display name']) {
    const markup = renderHolders(displayName);
    const figures = markup.querySelectorAll('tbody td[data-numeric]');
    expect(figures).toHaveLength(3);
    expect(Array.from(figures, cell => cell.textContent)).toEqual(['250', '25.00%', '250.00 USDL']);
    for (const cell of figures) expect(cell.classList.contains('whitespace-nowrap')).toBe(true);
    const address = Array.from(markup.querySelectorAll('tbody span')).find(span => span.textContent === shortAddress(wallet));
    expect(address).toBeDefined();
    expect(address!.classList.contains('whitespace-nowrap')).toBe(true);
    expect(markup.querySelector('[data-dev-badge]')?.classList.contains('whitespace-nowrap')).toBe(true);
    if (displayName) {
      const name = Array.from(markup.querySelectorAll('tbody span')).find(span => span.textContent === displayName);
      expect(name?.classList.contains('truncate')).toBe(true);
    }
  }
}
test(holdersFiguresStayOnOneLine.name, holdersFiguresStayOnOneLine);

function holdersGroupHeadingWrapsBetweenItsTerms() {
  const heading = renderHolders().querySelector('th[scope="colgroup"]');
  expect(heading?.getAttribute('colspan')).toBe('3');
  const terms = heading!.querySelectorAll('.whitespace-nowrap');
  expect(Array.from(terms, term => term.textContent)).toEqual(['Profit /', 'Avg entry /', 'Bought']);
  const group = terms[0].parentElement!;
  expect(group.textContent).toBe('Profit / Avg entry / Bought');
  expect(group.classList.contains('whitespace-nowrap')).toBe(false);
  expect(group.childNodes).toHaveLength(5);
  expect(group.childNodes[1].textContent).toBe(' ');
  expect(group.childNodes[3].textContent).toBe(' ');
}
test(holdersGroupHeadingWrapsBetweenItsTerms.name, holdersGroupHeadingWrapsBetweenItsTerms);
