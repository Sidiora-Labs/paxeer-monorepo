// @vitest-environment jsdom

import React from 'react';

import * as tokenMock from 'mocks/tokens/tokenInfo';
import { render } from 'ui/shared/layout/testWrapper';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TokensTable from './TokensTable';
import { TOKEN_VALUE_PLACEHOLDER } from './TokensTableItem';

const items = [ tokenMock.tokenInfo, tokenMock.tokenInfoERC20b ];

const COLUMNS = [
  '#',
  'Token',
  'Price',
  'Market cap',
  'Volume 24h',
  'Holders',
];

const rowCells = (container: HTMLElement, index: number) => {
  const row = container.querySelectorAll('[data-token-row]')[index];

  return Array.from(row.querySelectorAll('td')).map((cell) => cell.textContent);
};

describe('TokensTable', () => {
  it('omits change for a page without finite values', function tableOmitsUnavailableChanges() {
    const invalidItems = [
      ...items,
      ...[ null, '', ' ', 'NaN', 'Infinity', '-Infinity', 'unavailable', NaN, Infinity ].map((value, index) => ({
        ...tokenMock.tokenInfo,
        address_hash: `0x${ (index + 1).toString(16).padStart(40, '0') }`,
        price_change_percentage_24h: value,
      })),
    ];
    const { container } = render(<TokensTable items={ invalidItems } page={ 1 }/>);

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent)).toEqual(COLUMNS);
    expect(container.querySelector('[data-market-field="change"]')).toBeNull();
    expect(Array.from(container.querySelectorAll('[data-token-row]')).map((row) => row.querySelectorAll('td').length))
      .toEqual(invalidItems.map(() => COLUMNS.length));
  });

  it('shows each reported change and placeholders in a mixed page', function tableShowsReportedChanges() {
    const mixedItems = [
      items[0],
      { ...items[1], price_change_percentage_24h: '-3.25' },
      { ...tokenMock.tokenInfoERC20c, price_change_percentage_24h: 0 },
    ];
    const { container } = render(<TokensTable items={ mixedItems } page={ 1 }/>);

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent))
      .toEqual([ '#', 'Token', 'Price', 'Change 24h', 'Market cap', 'Volume 24h', 'Holders' ]);
    const cells = container.querySelectorAll('[data-market-field="change"]');
    expect(Array.from(cells).map((cell) => cell.textContent)).toEqual([ TOKEN_VALUE_PLACEHOLDER, '3.25%', '0.00%' ]);
    expect(cells[1].querySelector('[data-token-change="down"]')).not.toBeNull();
    expect(cells[2].querySelector('[data-token-change="up"]')).not.toBeNull();
    expect(container.querySelector('thead th:nth-child(4) button')).toBeNull();
  });

  it('keeps the change column out of loading rows', function tableOmitsLoadingChanges() {
    const { container, rerender } = render(
      <TokensTable items={ [ { ...items[0], price_change_percentage_24h: 2.5 } ] } page={ 1 } isLoading/>,
    );

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent)).toEqual(COLUMNS);
    expect(container.querySelector('[data-market-field="change"]')).toBeNull();
    rerender(<TokensTable items={ [ { ...items[0], price_change_percentage_24h: 2.5 } ] } page={ 1 }/>);
    expect(container.querySelector('[data-market-field="change"]')?.textContent).toBe('2.50%');
    rerender(<TokensTable items={ items } page={ 1 }/>);
    expect(container.querySelector('[data-market-field="change"]')).toBeNull();
  });

  it('heads the table with the scan columns in their order', () => {
    const { container } = render(<TokensTable items={ items } page={ 1 }/>);

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent)).toEqual(COLUMNS);
  });

  it('numbers the rows from the page they belong to', () => {
    const { container } = render(<TokensTable items={ items } page={ 3 }/>);

    expect(Array.from(container.querySelectorAll('[data-token-rank]')).map((cell) => cell.textContent))
      .toEqual([ '101', '102' ]);
  });

  it('fills the row with the token, the price, the capitalisations and the holders the payload carries', () => {
    const { container } = render(<TokensTable items={ items } page={ 1 }/>);

    const cells = rowCells(container, 0);

    expect(cells[1]).toContain('ARIANEE (ARIA)');
    expect(cells[1]).toContain('ERC-20');
    expect(cells[2]).toContain('$2.0101');
    expect(cells[3]).toBe('$117,629,601.62');
    expect(cells[5]).toBe('46,554');
    expect(rowCells(container, 1)[3]).toBe('$115,060,192.36');
  });

  it('dashes the columns the list endpoint answers with no figure', () => {
    const { container } = render(<TokensTable items={ items } page={ 1 }/>);

    const cells = rowCells(container, 0);

    expect(container.querySelector('[data-market-field="change"]')).toBeNull();
    expect(cells[4]).toBe(TOKEN_VALUE_PLACEHOLDER);
  });

  it('keeps the API price without deriving a coin-denominated price', () => {
    const { container } = render(<TokensTable items={ items } page={ 1 } coinPrice="1.5"/>);

    expect(container.querySelector('[data-token-row] [data-token-price]')?.textContent).toBe('$2.0101');
    expect(container.querySelector('[data-token-native-price]')).toBeNull();
  });

  it('asks for a sort from the price, the circulating capitalisation and the holders and from no other column', () => {
    const setSorting = vi.fn();
    const { container } = render(
      <TokensTable items={ items } page={ 1 } sorting="default" setSorting={ setSorting }/>,
    );

    const headers = Array.from(container.querySelectorAll('thead th')) as Array<HTMLElement>;

    const asked = headers.map((header) => {
      setSorting.mockClear();
      fireEvent.click((header.firstElementChild as HTMLElement | null) ?? header);

      return setSorting.mock.calls.map(([ value ]) => value)[0];
    });

    expect(asked).toEqual([
      undefined,
      undefined,
      'fiat_value-desc',
      'circulating_market_cap-desc',
      undefined,
      'holders_count-desc',
    ]);
  });
});
