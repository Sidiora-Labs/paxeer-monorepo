import type { TokenInfo } from 'types/api/token';

import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { expect, test } from 'vitest';

import { getTokenChange, hasTokenChanges } from './tokenChange';

test('token changes accept finite numbers and numeric strings', function tokenChangesAcceptFiniteValues() {
  for (const value of [ 0, '0', -1.25, '2.5', ' 3.5 ', '1e-3' ]) {
    const token = { ...tokenInfo, price_change_percentage_24h: value };

    expect(getTokenChange(token)?.toNumber()).toBe(Number(value));
    expect(hasTokenChanges([ tokenInfo, token ])).toBe(true);
  }
});

test('token changes reject missing blank and non finite values', function tokenChangesRejectUnavailableValues() {
  const values: Array<TokenInfo['price_change_percentage_24h']> = [
    undefined, null, '', ' ', 'NaN', 'Infinity', '-Infinity', 'unavailable', NaN, Infinity, -Infinity, '1e999',
  ];

  expect(getTokenChange(tokenInfo)).toBeUndefined();
  expect(hasTokenChanges([])).toBe(false);
  for (const value of values) {
    const token = { ...tokenInfo, price_change_percentage_24h: value };

    expect(getTokenChange(token)).toBeUndefined();
    expect(hasTokenChanges([ tokenInfo, token ])).toBe(false);
  }
});
