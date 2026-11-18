// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { TokenInfo } from 'types/api/token';

import type { ResourcePayload } from 'lib/api/resources';
import { getResourceKey } from 'lib/api/useApiQuery';
import { Provider } from 'toolkit/chakra/provider';
import { expect, test } from 'vitest';

import TickerStrip from './TickerStrip';

function createToken(exchangeRate: string | null, change?: string | number): TokenInfo & { price_change_percentage_24h?: string | number } {
  return {
    address_hash: '0x0000000000000000000000000000000000000001',
    type: 'ERC-20',
    symbol: 'PAX',
    name: 'Paxeer',
    decimals: '18',
    holders_count: '1',
    exchange_rate: exchangeRate,
    total_supply: '1000000000000000000',
    icon_url: null,
    circulating_market_cap: null,
    reputation: 'ok',
    price_change_percentage_24h: change,
  };
}

function renderTicker(items: Array<TokenInfo>) {
  const client = new QueryClient();
  client.setQueryData<ResourcePayload<'general:tokens'>>(getResourceKey('general:tokens'), { items, next_page_params: null });
  const container = document.createElement('div');
  try {
    container.innerHTML = renderToStaticMarkup(
      <Provider>
        <QueryClientProvider client={ client }><TickerStrip/></QueryClientProvider>
      </Provider>,
    );
    return container;
  } finally {
    client.clear();
  }
}

test('ticker rounds sixteen fraction digits to six', function tickerRoundsSixteenFractionDigitsToSix() {
  const container = renderTicker([ createToken('0.1234567890123456') ]);
  expect(container.querySelector('[data-price]')?.textContent).toBe('$0.123457');
});

test('ticker pads one fraction digit to two', function tickerPadsOneFractionDigitToTwo() {
  const container = renderTicker([ createToken('1.2') ]);
  expect(container.querySelector('[data-price]')?.textContent).toBe('$1.20');
});

test('ticker preserves thousands grouping', function tickerPreservesThousandsGrouping() {
  const container = renderTicker([ createToken('1234567.1234567890123456') ]);
  expect(container.querySelector('[data-price]')?.textContent).toBe('$1,234,567.123457');
});

test('ticker rounds day changes to at most two fraction digits', function tickerRoundsDayChangesToAtMostTwoFractionDigits() {
  for (const [ change, expected ] of [
    [ '4.23456789', '+4.23%' ],
    [ '-3.125', '-3.13%' ],
    [ 4.2, '+4.2%' ],
    [ 0, '0%' ],
  ] as const) {
    const container = renderTicker([ createToken('1.25', change) ]);
    expect(container.querySelector('[data-change]')?.textContent).toBe(expected);
  }
});

test('ticker omits tokens without rates', function tickerOmitsTokensWithoutRates() {
  const unpricedToken = { ...createToken(null), address_hash: '0x0000000000000000000000000000000000000002', symbol: 'UNPRICED' };
  const container = renderTicker([ createToken('1.25'), unpricedToken ]);
  expect(container.querySelectorAll('[data-ticker-token]')).toHaveLength(1);
  expect(container.querySelector(`[data-ticker-token="${ unpricedToken.address_hash }"]`)).toBeNull();
  expect(container.textContent).not.toContain('UNPRICED');
  expect(renderTicker([ unpricedToken ]).querySelector('[data-label="ticker-strip"]')).toBeNull();
});

test('ticker keeps dashes for non finite rates', function tickerKeepsDashesForNonFiniteRates() {
  for (const rate of [ 'NaN', 'Infinity', '-Infinity', 'unavailable' ]) {
    const container = renderTicker([ createToken(rate) ]);
    expect(container.querySelector('[data-price]')?.textContent).toBe('—');
    expect(container.querySelector('[data-change]')?.textContent).toBe('—');
  }
});

test('ticker rounds prices half up at six fraction digits', function tickerRoundsPricesHalfUpAtSixFractionDigits() {
  for (const [ rate, expected ] of [
    [ '1.2345645', '$1.234565' ],
    [ '0.0000005', '$0.000001' ],
    [ '999.9999995', '$1,000.000000' ],
    [ '0', '$0.00' ],
    [ '1.234', '$1.234' ],
  ]) {
    const container = renderTicker([ createToken(rate) ]);
    expect(container.querySelector('[data-price]')?.textContent).toBe(expected);
  }
});
