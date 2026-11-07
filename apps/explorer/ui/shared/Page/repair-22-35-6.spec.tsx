// @vitest-environment jsdom

import React from 'react';

import type { TokenInfo } from 'types/api/token';

import * as addressMock from 'mocks/address/address';
import { forContract } from 'mocks/address/counters';
import { base as addressTabCounters } from 'mocks/address/tabCounters';
import { base as block } from 'mocks/blocks/block';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { base as transaction } from 'mocks/txs/tx';
import { Tag } from 'toolkit/chakra/tag';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import Address from 'ui/pages/Address';
import Block from 'ui/pages/Block';
import Token from 'ui/pages/Token';
import Transaction from 'ui/pages/Transaction';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, expect, test, vi } from 'vitest';
import { fireEvent, waitFor } from 'vitest/lib';

import PageTitle from './PageTitle';

const routerSpy = vi.hoisted(() => ({ push: vi.fn(() => Promise.resolve(true)) }));

vi.mock('next/router', async() => {
  const base = (await import('ui/shared/layout/testWrapper')).nextRouterModule();
  return { ...base, useRouter: () => ({ ...base.useRouter(), push: routerSpy.push }) };
});

vi.setConfig({ testTimeout: 60_000 });

let marketToken: TokenInfo;

beforeEach(() => {
  routerState.pathname = '/address/[hash]';
  routerState.query = { hash: addressMock.hash };
  routerSpy.push.mockClear();
  marketToken = { ...tokenInfo, exchange_rate: null, circulating_market_cap: null, holders_count: null };
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    const path = new URL(request.url).pathname;
    let body: unknown;
    if (path === `/api/v2/addresses/${ addressMock.hash }`) {
      body = addressMock.contract;
    } else if (path.endsWith('/tabs-counters')) {
      body = addressTabCounters;
    } else if (path.endsWith('/counters') && path.includes('/addresses/')) {
      body = forContract;
    } else if (path === `/api/v2/tokens/${ tokenInfo.address_hash }`) {
      body = marketToken;
    } else if (path === `/api/v2/addresses/${ tokenInfo.address_hash }`) {
      body = { ...addressMock.token, hash: tokenInfo.address_hash };
    } else if (path === `/api/v2/transactions/${ transaction.hash }`) {
      body = transaction;
    } else if (path === `/api/v2/blocks/${ block.height }`) {
      body = block;
    } else if (path.endsWith('/counters')) {
      body = { transfers_count: '18', token_holders_count: '12' };
    } else if (path.endsWith('/token-balances')) {
      body = [];
    } else {
      body = { items: [], next_page_params: null, addresses: {}, days: 0 };
    }
    return Promise.resolve({ body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } });
  });
});

function entityHeaderRendersKindTitleIdentifierAndTags() {
  const { container, getByRole } = render(
    <PageTitle kind="transaction" title="Transaction 42" identifier={ transaction.hash } tags={ <><Tag>Success</Tag><Tag>Transfer</Tag></> }/>,
  );
  expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('TRANSACTION');
  expect(getByRole('heading', { level: 1 }).textContent).toBe('Transaction 42');
  const identifier = container.querySelector('[data-entity-identifier]') as HTMLElement;
  expect(identifier.textContent).toContain(transaction.hash);
  expect(identifier.querySelector('[aria-label="copy"]')).not.toBeNull();
  expect(identifier.querySelector('[aria-label="Show QR code"]')).not.toBeNull();
  const tags = container.querySelector('[data-entity-tags]') as HTMLElement;
  expect(tags.textContent).toBe('SuccessTransfer');
  expect(identifier.compareDocumentPosition(tags) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

function sectionGroupRendersLabelRowsAndResponsiveGrid() {
  const { container, getByRole } = render(
    <DetailedInfo.Group label="SUMMARY">
      <DetailedInfo.ItemLabel hint="The value sent by this transaction">Value</DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue>2 ETH</DetailedInfo.ItemValue>
      <DetailedInfo.ItemLabel>Fee</DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue>0.001 ETH</DetailedInfo.ItemValue>
    </DetailedInfo.Group>,
  );
  expect(getByRole('heading', { level: 2 }).textContent).toBe('SUMMARY');
  expect(Array.from(container.querySelectorAll('[data-detailed-info-label]')).map((row) => row.textContent)).toEqual([ 'Value', 'Fee' ]);
  expect(Array.from(container.querySelectorAll('[data-detailed-info-value]')).map((row) => row.textContent)).toEqual([ '2 ETH', '0.001 ETH' ]);
  expect(container.querySelector('[data-detailed-info-label] svg')).not.toBeNull();
  const styles = Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText)).join('\n');
  expect(styles).toContain('220px minmax(0, 1fr)');
  expect(styles).toContain('minmax(0, 1fr)');
  expect(styles).toContain('--chakra-colors-hairline');
}

async function addressPageRendersEntityTabSetAndOverview() {
  const { container } = render(<Address/>);
  await waitFor(() => expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('CONTRACT'));
  const tabs = container.querySelector('[data-entity-tabs]') as HTMLElement;
  const topTabs = tabs.querySelector('[role="tablist"]') as HTMLElement;
  await waitFor(() => {
    expect(Array.from(topTabs.querySelectorAll('[data-tab-label]')).map((label) => label.textContent)).toEqual([
      'Overview', 'History', 'Transfers', 'Portfolio', 'Analytics', 'Contract', 'More',
    ]);
    expect(topTabs.querySelector('[data-tab="token_transfers"] [data-tab-count]')?.textContent)
      .toBe(`(${ addressTabCounters.token_transfers_count })`);
  });
  expect(container.querySelector('[data-section-group="ADDRESS INFO"]')).not.toBeNull();
  expect(container.querySelector('[data-address-balances]')).not.toBeNull();
  expect(topTabs.compareDocumentPosition(container.querySelector('[data-address-details]') as Node) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  const historyTab = topTabs.querySelector('[data-tab="history"]') as HTMLButtonElement;
  await waitFor(() => expect(historyTab.disabled).toBe(false));
  fireEvent.click(historyTab);
  await waitFor(() => expect(routerSpy.push).toHaveBeenCalledWith(
    { pathname: '/address/[hash]', query: { hash: addressMock.hash, tab: 'history' } }, undefined, { shallow: true },
  ));
}

async function addressLegacyTabIdsRemainReachable() {
  const { container, rerender } = render(<Address/>);
  await waitFor(() => expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('CONTRACT'));
  const cases = [
    [ 'txs', 'history' ],
    [ 'txs_local', 'history' ],
    [ 'internal_txns', 'history' ],
    [ 'token_transfers', 'token_transfers' ],
    [ 'token_transfers_local', 'token_transfers' ],
    [ 'tokens', 'portfolio' ],
    [ 'tokens_erc20', 'portfolio' ],
    [ 'tokens_nfts', 'portfolio' ],
    [ 'tokens_nfts_collection', 'portfolio' ],
    [ 'tokens_nfts_list', 'portfolio' ],
    [ 'coin_balance_history', 'portfolio' ],
    [ 'logs', 'more' ],
    [ 'blocks_validated', 'more' ],
  ];
  for (const [ legacyId, groupId ] of cases) {
    routerState.query = { hash: addressMock.hash, tab: legacyId };
    rerender(<Address/>);
    await waitFor(() => {
      const topTabs = container.querySelector('[data-entity-tabs] [role="tablist"]') as HTMLElement;
      expect(topTabs.querySelector('[aria-selected="true"]')?.getAttribute('data-tab')).toBe(groupId);
      expect(container.querySelector('[role="tabpanel"]')).not.toBeNull();
    });
    expect(routerState.query.tab).toBe(legacyId);
  }
  const paths = fetchMock.mock.calls.map(([ url ]) => String(url));
  expect(paths.some((path) => path.includes('/internal-transactions'))).toBe(true);
  expect(paths.some((path) => path.includes('/nft'))).toBe(true);
  expect(paths.some((path) => path.includes('/logs'))).toBe(true);
}

async function tokenMarketsShowDashesForMissingApiFields() {
  routerState.pathname = '/token/[hash]';
  routerState.query = { hash: tokenInfo.address_hash };
  const { container } = render(<Token/>);
  await waitFor(() => expect(container.querySelector('h1')?.textContent).toBe(`${ tokenInfo.name } (${ tokenInfo.symbol })`));
  expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('TOKEN');
  const values = container.querySelectorAll('[data-token-markets] [data-stat-value]');
  expect(Array.from(values).map((value) => value.textContent)).toEqual([ '—', '—', '—', '—' ]);
  const topTabs = container.querySelector('[role="tablist"]') as HTMLElement;
  expect(Array.from(topTabs.querySelectorAll('[role="tab"]')).map((tab) => tab.textContent)).toEqual([ 'Markets', 'History', 'Holders', 'Metadata' ]);
  expect(container.querySelector('[data-token-markets] svg')).toBeNull();
}

async function tokenMarketsKeepSuppliedFieldsAndZeroValues() {
  marketToken = { ...tokenInfo, exchange_rate: '0', circulating_market_cap: '0', volume_24h: '1234.56', holders_count: '0' };
  routerState.pathname = '/token/[hash]';
  routerState.query = { hash: tokenInfo.address_hash };
  const { container } = render(<Token/>);
  await waitFor(() => {
    expect(Array.from(container.querySelectorAll('[data-token-markets] [data-stat-value]')).map((value) => value.textContent))
      .toEqual([ '$0', '$0', '$1,234.56', '0' ]);
  });
  expect(container.textContent).not.toContain('Onchain market cap');
}

async function transactionOverviewRendersAllLabelledGroups() {
  routerState.pathname = '/tx/[hash]';
  routerState.query = { hash: transaction.hash };
  const { container } = render(<Transaction/>);
  await waitFor(() => expect(container.querySelector('[data-section-group="SUMMARY"]')).not.toBeNull());
  expect(Array.from(container.querySelectorAll('[data-section-group]')).map((group) => group.getAttribute('data-section-group')))
    .toEqual([ 'SUMMARY', 'TRANSFERS', 'BALANCE CHANGES', 'DETAILS' ]);
  const tabs = container.querySelector('[role="tablist"]') as HTMLElement;
  expect(Array.from(tabs.querySelectorAll('[role="tab"]')).map((tab) => tab.textContent))
    .toEqual([ 'Overview', 'Token transfers', 'Internal', 'Logs', 'State', 'Raw' ]);
}

async function blockDetailsRemainReachableThroughExistingIndexRoute() {
  routerState.pathname = '/block/[height_or_hash]';
  routerState.query = { height_or_hash: String(block.height), tab: 'index' };
  const { container } = render(<Block/>);
  await waitFor(() => expect(container.querySelector('[data-section-group="BLOCK INFO"]')?.textContent).toContain(block.hash));
  expect(container.querySelector('h1')?.textContent).toBe(`Block #${ block.height }`);
  expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('BLOCK');
  expect(container.querySelector('[data-section-group="BLOCK INFO"]')?.textContent).toContain('Parent hash');
  expect(container.querySelector('[data-tab="index"]')?.getAttribute('aria-selected')).toBe('true');
}

function entityColoursResolveInBothModes() {
  function atPath(source: unknown, path: string): unknown {
    return path.split('.').reduce<unknown>((node, part) => (
      typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[part] : undefined
    ), source);
  }
  function resolve(path: string, mode: '_light' | '_dark', seen: Set<string> = new Set()): string {
    expect(seen.has(path)).toBe(false);
    const node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
    const normalized = atPath(node, 'DEFAULT') ?? node;
    const value = atPath(normalized, 'value');
    const raw = typeof value === 'string' ? value : atPath(value, mode);
    expect(raw).toBeTypeOf('string');
    const reference = /^\{(.+)\}$/.exec(String(raw));
    return reference ? resolve(reference[1], mode, new Set(seen).add(path)) : String(raw);
  }
  const roles = [
    'text.primary', 'text.secondary', 'text.muted', 'bg.surface', 'hairline', 'accent', 'positive', 'positiveSoft', 'negative', 'negativeSoft',
  ];
  for (const role of roles) {
    for (const mode of [ '_light', '_dark' ] as const) {
      expect(resolve(`colors.${ role }`, mode)).toMatch(/^(#[\da-f]{3,8}|rgba?\([\d\s.,%]+\))$/i);
    }
  }
}

test(entityHeaderRendersKindTitleIdentifierAndTags.name, entityHeaderRendersKindTitleIdentifierAndTags);
test(sectionGroupRendersLabelRowsAndResponsiveGrid.name, sectionGroupRendersLabelRowsAndResponsiveGrid);
test(addressPageRendersEntityTabSetAndOverview.name, addressPageRendersEntityTabSetAndOverview);
test(addressLegacyTabIdsRemainReachable.name, addressLegacyTabIdsRemainReachable);
test(tokenMarketsShowDashesForMissingApiFields.name, tokenMarketsShowDashesForMissingApiFields);
test(tokenMarketsKeepSuppliedFieldsAndZeroValues.name, tokenMarketsKeepSuppliedFieldsAndZeroValues);
test(transactionOverviewRendersAllLabelledGroups.name, transactionOverviewRendersAllLabelledGroups);
test(blockDetailsRemainReachableThroughExistingIndexRoute.name, blockDetailsRemainReachableThroughExistingIndexRoute);
test(entityColoursResolveInBothModes.name, entityColoursResolveInBothModes);
