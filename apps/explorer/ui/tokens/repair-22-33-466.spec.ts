// @vitest-environment jsdom

import { createListCollection } from '@chakra-ui/react';
import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { TokensSorting, TokensSortingValue } from 'types/api/tokens';

import buildUrl from 'lib/api/buildUrl';
import { tokenInfoERC20b } from 'mocks/tokens/tokenInfo';
import { render } from 'ui/shared/layout/testWrapper';
import getSortParamsFromValue from 'ui/shared/sort/getSortParamsFromValue';
import getSortValueFromQuery from 'ui/shared/sort/getSortValueFromQuery';
import { expect, test } from 'vitest';

import { getOnchainMarketCap, TokenFiatValue, TOKEN_VALUE_PLACEHOLDER } from './TokensTableItem';
import { SORT_OPTIONS } from './utils';

function renderMarketCap(token: TokenInfo) {
  const value = getOnchainMarketCap(token);

  return render(React.createElement(TokenFiatValue, { value })).container.innerHTML;
}

test('tokenMarketCapUnknownDecimals', function tokenMarketCapUnknownDecimals() {
  const token = { ...tokenInfoERC20b, decimals: null };

  expect(token.total_supply).toBe('900000000000000000000000000');
  expect(token.exchange_rate).toBe('0.982');
  expect(getOnchainMarketCap(token)).toBeUndefined();
  expect(renderMarketCap(token)).toContain('data-token-value-placeholder');
  expect(renderMarketCap(token)).toContain(TOKEN_VALUE_PLACEHOLDER);
});

test('tokenMarketCapInvalidDecimals', function tokenMarketCapInvalidDecimals() {
  for (const decimals of [ '', ' ', 'NaN', 'Infinity', '-Infinity', '1.5', '-1' ]) {
    const token = { ...tokenInfoERC20b, decimals };

    expect(getOnchainMarketCap(token)).toBeUndefined();
    expect(renderMarketCap(token)).toContain('data-token-value-placeholder');
  }
});

test('tokenMarketCapZeroDecimals', function tokenMarketCapZeroDecimals() {
  const token = { ...tokenInfoERC20b, decimals: '0', total_supply: '1000', exchange_rate: '2.5' };

  expect(getOnchainMarketCap(token)?.toFixed()).toBe('2500');
  expect(renderMarketCap(token)).toContain('$2,500');
  expect(renderMarketCap(token)).not.toContain('data-token-value-placeholder');
});

test('tokenMarketCapKnownDecimals', function tokenMarketCapKnownDecimals() {
  const token = { ...tokenInfoERC20b, total_supply: '1000000000', exchange_rate: '2.5' };

  expect(getOnchainMarketCap(token)?.toFixed()).toBe('2500');
  expect(renderMarketCap(token)).toContain('$2,500');
  expect(renderMarketCap(token)).not.toContain('data-token-value-placeholder');
});

test('tokenSortLabelMatchesApiField', function tokenSortLabelMatchesApiField() {
  const collection = createListCollection({ items: SORT_OPTIONS });

  for (const order of [ 'asc', 'desc' ] as const) {
    const value = `circulating_market_cap-${ order }` as const;
    const option = collection.find(value);

    expect(option).toEqual({
      label: `Circulating market cap ${ order === 'asc' ? 'ascending' : 'descending' }`,
      value,
    });
    expect(collection.stringifyItem(option!)).toBe(option!.label);

    const sorting = getSortParamsFromValue<TokensSortingValue, TokensSorting['sort'], TokensSorting['order']>(option!.value);
    const url = new URL(buildUrl('general:tokens', undefined, sorting));

    expect(url.searchParams.get('sort')).toBe('circulating_market_cap');
    expect(url.searchParams.get('order')).toBe(order);
    expect(getSortValueFromQuery(Object.fromEntries(url.searchParams), SORT_OPTIONS)).toBe(value);
  }
});
