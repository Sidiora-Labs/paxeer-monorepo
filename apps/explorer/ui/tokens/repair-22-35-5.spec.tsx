// @vitest-environment jsdom

import React from 'react';

import { getTokenTypes, isFungibleTokenType, NFT_TOKEN_TYPE_IDS } from 'lib/token/tokenTypes';
import { tokenInfo, tokenInfoERC20a, tokenInfoERC20c } from 'mocks/tokens/tokenInfo';
import TokensPage from 'ui/pages/Tokens';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { LIST_TABLE_STYLES } from 'ui/shared/Page/ListPage';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from 'vitest/lib';

import colors from '../../toolkit/theme/foundations/colors';
import semanticTokens from '../../toolkit/theme/foundations/semanticTokens';
import TokensListItem from './TokensListItem';
import TokensTable from './TokensTable';
import { TokenChangePercent, TOKEN_VALUE_PLACEHOLDER } from './TokensTableItem';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

const items = [ tokenInfoERC20a, { ...tokenInfo, volume_24h: '12345.67' }, tokenInfoERC20c ];
const requestedUrls: Array<URL> = [];

beforeEach(() => {
  routerState.pathname = '/tokens';
  routerState.query = {};
  requestedUrls.length = 0;
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    requestedUrls.push(new URL(request.url));
    return Promise.resolve({
      body: JSON.stringify({ items, next_page_params: null }),
      headers: { 'Content-Type': 'application/json' },
    });
  });
});

afterEach(() => {
  document.documentElement.classList.remove('light', 'dark');
});

async function renderTokensPage() {
  const result = render(<TokensPage/>);
  await waitFor(() => expect(result.container.querySelectorAll('[data-token-row]')).toHaveLength(items.length), { timeout: 20_000 });
  return result;
}

async function marketsPageHasSevenColumnsAndOnlyReportedValues() {
  const { container } = await renderTokensPage();
  expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Tokens');
  expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent)).toEqual([
    '#', 'Token', 'Price', 'Change 24h', 'Market cap', 'Volume 24h', 'Holders',
  ]);
  expect(container.querySelector('[data-list-page] table')).toBeTruthy();
  expect(container.querySelector('[data-scan-table-card]')).toBeNull();
  const rows = container.querySelectorAll('[data-token-row]');
  expect(rows[0].querySelector('[data-market-field="price"]')?.textContent).toBe('—');
  expect(rows[0].querySelector('[data-market-field="change"]')?.textContent).toBe('—');
  expect(rows[0].querySelector('[data-market-field="volume"]')?.textContent).toBe('—');
  expect(rows[1].querySelector('[data-market-field="price"]')?.textContent).toBe('$2.0101');
  expect(rows[1].querySelector('[data-market-field="market-cap"]')?.textContent).toBe('$117,629,601.62');
  expect(rows[1].querySelector('[data-market-field="volume"]')?.textContent).toBe('$12,345.67');
  expect(rows[1].querySelector('[data-token-holders]')?.textContent).toBe('46,554');
  expect(rows[2].querySelector('[data-market-field="market-cap"]')?.textContent).toBe('—');
  expect(container.querySelector('[data-token-native-price]')).toBeNull();
  expect(rows[1].querySelector('[data-entity] button[aria-label="copy"]')).toBeTruthy();
}

test(marketsPageHasSevenColumnsAndOnlyReportedValues, marketsPageHasSevenColumnsAndOnlyReportedValues, 60_000);

async function categoryChipsUseTheExistingApiFilters() {
  await renderTokensPage();
  const chips = within(screen.getByRole('group', { name: 'Token category' }));
  expect(chips.getAllByRole('button').map((chip) => chip.textContent)).toEqual([ 'All', 'Fungible', 'NFT', 'Bridged' ]);
  const fungible = chips.getByRole('button', { name: 'Fungible' });
  fireEvent.click(fungible);
  const fungibleTypes = Object.keys(getTokenTypes(false)).filter(isFungibleTokenType);
  await waitFor(() => expect(requestedUrls.some((url) => url.searchParams.get('type') === fungibleTypes.join(','))).toBe(true));
  expect(fungible.getAttribute('aria-pressed')).toBe('true');
  fireEvent.click(chips.getByRole('button', { name: 'NFT' }));
  await waitFor(() => expect(requestedUrls.some((url) => url.searchParams.get('type') === NFT_TOKEN_TYPE_IDS.join(','))).toBe(true));
  expect(chips.getByRole('button', { name: 'NFT' }).getAttribute('aria-pressed')).toBe('true');
  const beforeReset = requestedUrls.length;
  fireEvent.click(chips.getByRole('button', { name: 'All' }));
  await waitFor(() => expect(requestedUrls.slice(beforeReset).some((url) => url.pathname.endsWith('/tokens') && !url.searchParams.has('type'))).toBe(true));
  expect(chips.getByRole('button', { name: 'All' }).getAttribute('aria-pressed')).toBe('true');
}

test(categoryChipsUseTheExistingApiFilters, categoryChipsUseTheExistingApiFilters, 60_000);

async function numericSortsReachTheExistingApiSortHooks() {
  const { container } = await renderTokensPage();
  for (const [ label, field ] of [ [ 'Price', 'fiat_value' ], [ 'Market cap', 'circulating_market_cap' ], [ 'Holders', 'holders_count' ] ]) {
    const header = within(container).getByRole('columnheader', { name: label });
    fireEvent.click(within(header).getByRole('button'));
    await waitFor(() => expect(requestedUrls.some((url) => url.searchParams.get('sort') === field && url.searchParams.get('order') === 'desc')).toBe(true));
    fireEvent.click(within(header).getByRole('button'));
    await waitFor(() => expect(requestedUrls.some((url) => url.searchParams.get('sort') === field && url.searchParams.get('order') === 'asc')).toBe(true));
  }
  expect(within(container).getByRole('columnheader', { name: 'Change 24h' }).querySelector('button')).toBeNull();
  expect(within(container).getByRole('columnheader', { name: 'Volume 24h' }).querySelector('button')).toBeNull();
}

test(numericSortsReachTheExistingApiSortHooks, numericSortsReachTheExistingApiSortHooks, 60_000);

function atPath(source: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[key] : undefined, source);
}

function resolveColour(path: string, mode: string, visited = new Set<string>()): string {
  if (visited.has(path)) {
    throw new Error(`Circular colour reference: ${ path }`);
  }
  const next = new Set(visited).add(path);
  const [ name, opacity ] = path.split('/');
  let node = atPath(semanticTokens, name) ?? atPath({ colors }, name);
  if (typeof node === 'object' && node !== null && !('value' in node)) {
    node = (node as Record<string, unknown>).DEFAULT;
  }
  const value = atPath(node, 'value');
  const raw = typeof value === 'string' ? value : atPath(value, mode);
  if (typeof raw !== 'string' || !raw.length) {
    throw new Error(`Unresolved colour ${ path } in ${ mode }`);
  }
  const reference = /^\{(.+)\}$/.exec(raw);
  const result = reference ? resolveColour(reference[1], mode, next) : raw;
  if (opacity !== undefined) {
    const rgb = [ 1, 3, 5 ].map((start) => parseInt(result.slice(start, start + 2), 16));
    return `rgba(${ rgb.join(',') },${ Number(opacity) / 100 })`;
  }
  return result;
}

function percentageColoursUseSemanticTokensInBothModes() {
  for (const mode of [ 'light', 'dark' ]) {
    document.documentElement.classList.add(mode);
    const { container, unmount } = render(<><TokenChangePercent value={ 2.5 }/><TokenChangePercent value={ -1.25 }/></>);
    const positive = container.querySelector('[data-token-change="up"]') as HTMLElement;
    const negative = container.querySelector('[data-token-change="down"]') as HTMLElement;
    expect(positive.textContent).toBe('2.50%');
    expect(negative.textContent).toBe('1.25%');
    const styles = Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText)).join('\n');
    for (const [ element, token ] of [ [ positive, 'positive' ], [ negative, 'negative' ] ] as const) {
      expect(Array.from(element.classList).some((className) => styles.includes(`.${ className }`) &&
        Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules))
          .some((rule) => rule.cssText.includes(`.${ className }`) && rule.cssText.includes(`var(--chakra-colors-${ token })`)))).toBe(true);
    }
    for (const token of [
      'canvas', 'raised', 'hairline', 'accent', 'accent.soft', 'focusRing', 'text.primary', 'text.secondary', 'text.muted', 'positive', 'negative',
    ]) {
      expect(resolveColour(`colors.${ token }`, `_${ mode }`)).toMatch(/^(#[0-9a-f]{6}|rgba?\()/i);
    }
    unmount();
    document.documentElement.classList.remove(mode);
  }
}

test(percentageColoursUseSemanticTokensInBothModes, percentageColoursUseSemanticTokensInBothModes);

function mobileMarketsKeepTheFieldsAndMissingValuePolicy() {
  const { container } = render(<TokensListItem token={ tokenInfoERC20a } page={ 1 } index={ 0 }/>);
  expect(container.querySelector('[data-list-item-mobile]')).toBeTruthy();
  expect(Array.from(container.querySelectorAll('[data-token-field]')).map((row) => row.getAttribute('data-token-field')))
    .toEqual([ 'Price', 'Change 24h', 'Market cap', 'Volume 24h', 'Holders' ]);
  expect(container.querySelector('[data-token-field="Price"]')?.textContent).toBe(`Price${ TOKEN_VALUE_PLACEHOLDER }`);
  expect(container.querySelector('[data-token-field="Change 24h"]')?.textContent).toBe(`Change 24h${ TOKEN_VALUE_PLACEHOLDER }`);
  expect(container.querySelector('[data-token-native-price]')).toBeNull();
}

test(mobileMarketsKeepTheFieldsAndMissingValuePolicy, mobileMarketsKeepTheFieldsAndMissingValuePolicy);

function tablesUseDenseRowsAndAccessibleHoverCopy() {
  const { container } = render(<TokensTable items={ [ tokenInfo ] } page={ 1 }/>);
  expect(LIST_TABLE_STYLES['& thead, & thead tr']).toMatchObject({ height: '36px' });
  expect(LIST_TABLE_STYLES['& tbody tr']).toMatchObject({ height: '44px', bg: 'canvas', _hover: { bg: 'raised' } });
  expect(LIST_TABLE_STYLES['& td[data-is-numeric]']).toMatchObject({ textStyle: 'number', textAlign: 'right' });
  expect(LIST_TABLE_STYLES['& th:first-of-type, & td:first-of-type']).toMatchObject({ position: 'sticky', left: 0 });
  expect(LIST_TABLE_STYLES['& tr:hover button[aria-label=copy], & tr:focus-within button[aria-label=copy]']).toMatchObject({ opacity: 1 });
  expect(container.querySelectorAll('[data-token-row] td[data-is-numeric]')).toHaveLength(6);
  expect(container.querySelector('[data-token-row] button[aria-label="copy"]')).toBeTruthy();
  const styles = Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText)).join('\n');
  expect(styles).toContain('height: 44px');
  expect(styles).toContain('text-transform: uppercase');
  expect(styles).toContain('var(--chakra-fonts-mono)');
}

test(tablesUseDenseRowsAndAccessibleHoverCopy, tablesUseDenseRowsAndAccessibleHoverCopy);
