// @vitest-environment jsdom

import React from 'react';

import * as tokenMock from 'mocks/tokens/tokenInfo';
import { render } from 'ui/shared/layout/testWrapper';
import { describe, expect, it, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TokensListItem from './TokensListItem';
import { TOKEN_VALUE_PLACEHOLDER } from './TokensTableItem';

const FIELDS = [
  'Price',
  'Market cap',
  'Volume 24h',
  'Holders',
];

describe('TokensListItem', () => {
  it('omits the change row when the page has no change values', function mobileOmitsUnavailableChanges() {
    const { container } = render(<TokensListItem token={ tokenMock.tokenInfo } index={ 0 } page={ 1 } showChange={ false }/>);

    expect(container.querySelector('[data-token-field="Change 24h"]')).toBeNull();
    expect(Array.from(container.querySelectorAll('[data-token-field]')).map((row) => row.getAttribute('data-token-field')))
      .toEqual(FIELDS);
  });

  it('shows the token change when the page carries values', function mobileShowsReportedChanges() {
    for (const [ value, direction, expected ] of [ [ '2.5', 'up', '2.50%' ], [ -1.25, 'down', '1.25%' ], [ 0, 'up', '0.00%' ] ] as const) {
      const { container, unmount } = render(
        <TokensListItem token={{ ...tokenMock.tokenInfo, price_change_percentage_24h: value }} index={ 0 } page={ 1 } showChange/>,
      );

      expect(Array.from(container.querySelectorAll('[data-token-field]')).map((row) => row.getAttribute('data-token-field')))
        .toEqual([ 'Price', 'Change 24h', 'Market cap', 'Volume 24h', 'Holders' ]);
      expect(container.querySelector(`[data-token-field="Change 24h"] [data-token-change="${ direction }"]`)?.textContent).toBe(expected);
      unmount();
    }
  });

  it('uses a placeholder for missing changes in a page with values', function mobileShowsMissingChangePlaceholder() {
    const { container } = render(<TokensListItem token={ tokenMock.tokenInfo } index={ 0 } page={ 1 } showChange/>);

    expect(container.querySelector('[data-token-field="Change 24h"]')?.textContent).toBe(`Change 24h${ TOKEN_VALUE_PLACEHOLDER }`);
    expect(container.querySelector('[data-token-field="Change 24h"] [data-token-value-placeholder]')).not.toBeNull();
  });

  it('omits the change row while loading', function mobileOmitsLoadingChanges() {
    const { container } = render(
      <TokensListItem token={{ ...tokenMock.tokenInfo, price_change_percentage_24h: 2.5 }} index={ 0 } page={ 1 } isLoading showChange/>,
    );

    expect(container.querySelector('[data-token-field="Change 24h"]')).toBeNull();
    expect(container.querySelectorAll('[data-token-field]')).toHaveLength(FIELDS.length);
  });

  it('carries the same fields as the desktop columns in the same order', () => {
    const { container } = render(<TokensListItem token={ tokenMock.tokenInfo } index={ 0 } page={ 1 }/>);

    expect(Array.from(container.querySelectorAll('[data-token-field]')).map((row) => row.getAttribute('data-token-field')))
      .toEqual(FIELDS);
  });

  it('heads the item with the rank and the token', () => {
    const { container } = render(<TokensListItem token={ tokenMock.tokenInfo } index={ 1 } page={ 2 }/>);

    expect(container.querySelector('[data-token-rank]')?.textContent).toBe('52');
    expect(container.textContent).toContain('ARIANEE (ARIA)');
  });

  it('writes the figures the payload carries and dashes the ones it does not', () => {
    const { container } = render(<TokensListItem token={ tokenMock.tokenInfo } index={ 0 } page={ 1 } coinPrice="1.5"/>);

    expect(container.querySelector('[data-token-field="Price"]')?.textContent)
      .toBe('Price$2.0101');
    expect(container.querySelector('[data-token-field="Change 24h"]')).toBeNull();
    expect(container.querySelector('[data-token-field="Volume 24h"]')?.textContent)
      .toBe(`Volume 24h${ TOKEN_VALUE_PLACEHOLDER }`);
    expect(container.querySelector('[data-token-field="Market cap"]')?.textContent)
      .toBe('Market cap$117,629,601.62');
    expect(container.querySelector('[data-token-field="Holders"]')?.textContent).toBe('Holders46,554');
  });
});
