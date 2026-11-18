// @vitest-environment jsdom

import React from 'react';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import { SettingsContextProvider } from 'lib/contexts/settings';
import { chainA } from 'mocks/multichain/chains';
import * as txMock from 'mocks/txs/tx';
import { TableBody, TableCell, TableRoot, TableRow } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, waitFor } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TxsTable from './TxsTable';

const noop = () => undefined;

const failed = { ...txMock.withDecodedRevertReason, hash: txMock.base2.hash };
const pending = { ...txMock.pending, hash: txMock.base3.hash };
const txs = [ txMock.base, failed, pending ];

const renderTable = () => render(
  <TxsTable
    txs={ txs }
    sort="default"
    onSortToggle={ noop }
    top={ 0 }
    showBlockInfo
    stickyHeader={ false }
  />,
);

function tableStyleValues(table: HTMLTableElement, property = 'min-width') {
  const values: Array<{ media: string; value: string }> = [];

  function readRules(rules: CSSRuleList, media = '') {
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSStyleRule) {
        const value = rule.style.getPropertyValue(property);
        if (value && table.matches(rule.selectorText)) {
          values.push({ media, value });
        }
      }
      if ('cssRules' in rule) {
        readRules((rule as CSSGroupingRule).cssRules, rule instanceof CSSMediaRule ? rule.conditionText : media);
      }
    }
  }

  for (const sheet of Array.from(document.styleSheets)) {
    readRules(sheet.cssRules);
  }

  return values;
}

describe('TxsTable', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it.each([
    { sortable: false, sticky: false },
    { sortable: true, sticky: false },
    { sortable: false, sticky: true },
    { sortable: true, sticky: true },
  ])('keeps the age switch inline and all headers aligned with options %j', async function ageHeaderStaysInline({ sortable, sticky }) {
    const { container } = render(
      <SettingsContextProvider>
        <TxsTable txs={ txs } sort="default" onSortToggle={ sortable ? noop : undefined } top={ 0 } showBlockInfo stickyHeader={ sticky }/>
      </SettingsContextProvider>,
    );
    const toggle = container.querySelector('button[aria-label="Toggle time format"]') as HTMLButtonElement;
    const ageHeader = toggle.closest('th') as HTMLTableCellElement;
    const row = ageHeader.firstElementChild as HTMLElement;
    const headers = Array.from(container.querySelectorAll('thead th'));

    expect(row.textContent).toBe('Age');
    expect(row.contains(toggle)).toBe(true);
    expect(getComputedStyle(row).display).toBe('flex');
    expect(getComputedStyle(row).flexDirection).toBe('row');
    expect(getComputedStyle(row).flexWrap).toBe('nowrap');
    expect(getComputedStyle(row).alignItems).toBe('center');
    for (const header of headers) {
      expect(getComputedStyle(header).whiteSpace).toBe('nowrap');
      expect(getComputedStyle(header).verticalAlign).toBe('middle');
    }
    expect(headers.some((header) => header.textContent === 'Block')).toBe(true);

    fireEvent.click(toggle);

    await waitFor(() => {
      expect(row.textContent).toBe('AgeLocal');
      expect(toggle.getAttribute('data-selected')).toBe('true');
    });
    expect(getComputedStyle(row).flexWrap).toBe('nowrap');

    fireEvent.click(toggle);

    await waitFor(() => {
      expect(row.textContent).toBe('Age');
    });
  });

  it('heads the table with the scan columns in order', () => {
    const { container } = renderTable();

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent))
      .toEqual([ '', 'Transaction hash', 'Action', 'Block', 'Age', 'From / To', 'Amount ETH', 'Txn fee ETH' ]);
  });

  it('reserves room for a price update action with a three digit market count', function priceUpdateActionWidth() {
    const { container } = renderTable();
    const headers = Array.from(container.querySelectorAll('thead th'));

    expect(getComputedStyle(headers[2]).width).toBe('14rem');
  });

  it('reserves counterparty space from the hash amount and fee columns and preserves the other widths', function priceUpdateCounterpartyWidth() {
    const { container } = renderTable();
    const headers = Array.from(container.querySelectorAll('thead th'));

    expect(getComputedStyle(headers[5]).width).toBe('');
    expect(getComputedStyle(container.querySelector('table')!).minWidth).toBe('76rem');
    expect([ 0, 1, 3, 4, 6, 7 ].map((index) => getComputedStyle(headers[index]).width))
      .toEqual([
        'var(--chakra-spacing-12)', 'var(--chakra-spacing-40)', 'var(--chakra-spacing-28)', 'var(--chakra-spacing-32)',
        'var(--chakra-spacing-28)', '7.5rem',
      ]);
  });

  it.each([
    { block: true, value: true, fee: true, fixed: 56.5 },
    { block: false, value: true, fee: true, fixed: 49.5 },
    { block: true, value: false, fee: true, fixed: 49.5 },
    { block: true, value: true, fee: false, fixed: 49 },
    { block: false, value: false, fee: true, fixed: 42.5 },
    { block: false, value: true, fee: false, fixed: 42 },
    { block: true, value: false, fee: false, fixed: 42 },
    { block: false, value: false, fee: false, fixed: 35 },
  ])('gives the remaining width to counterparties with visible columns %j', function counterpartyWidthWithOptionalColumns({ block, value, fee, fixed }) {
    const hiddenFields = config.UI.views.tx.hiddenFields!;
    const previous = { ...hiddenFields };

    try {
      hiddenFields.value = !value;
      hiddenFields.tx_fee = !fee;

      for (const chain of [ undefined, chainA ]) {
        for (const sortable of [ false, true ]) {
          const { container, unmount } = render(
            <MultichainContext.Provider value={ chain ? { chain } : null }>
              <TxsTable txs={ txs } sort="default" onSortToggle={ sortable ? noop : undefined } top={ 0 } showBlockInfo={ block } stickyHeader={ false }/>
            </MultichainContext.Provider>,
          );
          const headers = Array.from(container.querySelectorAll('thead th'));
          const counterparty = headers.find((header) => header.textContent === 'From / To')!;
          const table = container.querySelector('table')!;
          const chainColumns = Number(Boolean(chain));
          const minWidth = `${ fixed + chainColumns * 2 + 19.5 }rem`;

          expect(getComputedStyle(counterparty).width).toBe('');
          expect(getComputedStyle(table).minWidth).toBe(minWidth);
          expect(tableStyleValues(table)).toEqual([
            { media: '', value: minWidth },
            { media: 'screen and (min-width: 62.5rem)', value: minWidth },
          ]);
          expect(getComputedStyle(headers[1 + chainColumns]).width).toBe('var(--chakra-spacing-40)');
          expect(getComputedStyle(headers[2 + chainColumns]).width).toBe('14rem');
          expect(headers.map((header) => getComputedStyle(header).width)).toEqual([
            'var(--chakra-spacing-12)',
            ...(chain ? [ 'var(--chakra-spacing-8)' ] : []),
            'var(--chakra-spacing-40)', '14rem',
            ...(block ? [ 'var(--chakra-spacing-28)' ] : []),
            'var(--chakra-spacing-32)', '',
            ...(value ? [ 'var(--chakra-spacing-28)' ] : []),
            ...(fee ? [ '7.5rem' ] : []),
          ]);
          expect(headers).toHaveLength(5 + chainColumns + Number(block) + Number(value) + Number(fee));
          unmount();
        }
      }
    } finally {
      Object.assign(hiddenFields, previous);
    }
  });

  it('keeps the table fixed and full width for price updates only', function priceUpdateTableLayout() {
    const { container } = render(
      <TxsTable txs={ [ txMock.priceUpdate ] } sort="default" top={ 0 } showBlockInfo stickyHeader={ false }/>,
    );
    const table = container.querySelector('table')!;
    const headers = table.querySelectorAll('thead th');

    expect(tableStyleValues(table, 'table-layout')).toEqual([ { media: '', value: 'fixed' } ]);
    expect(tableStyleValues(table, 'width')).toEqual([ { media: '', value: 'var(--chakra-sizes-full)' } ]);
    expect(getComputedStyle(table).minWidth).toBe('76rem');
    expect(getComputedStyle(headers[5]).width).toBe('');
    expect(getComputedStyle(headers[0]).position).toBe('sticky');
    expect(getComputedStyle(headers[0]).left).toBe('var(--chakra-spacing-0)');
  });

  it('keeps both ends of each counterparty address and the price feed link', function counterpartyAddressText() {
    const ordinary = { ...txMock.base, from: { ...txMock.base.from, ens_domain_name: null } };
    const { container } = render(
      <TxsTable txs={ [ txMock.priceUpdate, ordinary ] } sort="default" top={ 0 } showBlockInfo stickyHeader={ false }/>,
    );
    const cells = Array.from(container.querySelectorAll('tbody tr')).map((row) => row.querySelectorAll('td')[5]);

    expect(Array.from(cells[0].querySelectorAll('[data-entity-content]')).map((content) => content.textContent)).toEqual([ '0x7c...9f01' ]);
    expect(cells[0].querySelector('a[href="/feed"]')?.textContent).toBe('Price feed');
    expect(Array.from(cells[1].querySelectorAll('[data-entity-content]')).map((content) => content.textContent))
      .toEqual([ '0x04...84B9', '0xd7...5859' ]);
  });

  it.each([ 18, 999 ])('carries the full price update action in the title when clipped with %i markets', function priceUpdateActionTitle(count) {
    const { container } = render(
      <TxsTable
        txs={ [ { ...txMock.priceUpdate, price_update: { ...txMock.priceUpdate.price_update, observation_count: count } } ] }
        sort="default"
        top={ 0 }
        showBlockInfo
        stickyHeader={ false }
      />,
    );
    const cell = container.querySelectorAll('tbody tr td')[2];

    expect(cell.textContent).toBe(`Price update · ${ count } markets`);
    expect(cell.getAttribute('title')).toBe(cell.textContent);
  });

  it('takes the scan table styling from the table recipe', () => {
    const { container } = renderTable();
    const { container: plain } = render(
      <TableRoot>
        <TableBody>
          <TableRow>
            <TableCell>cell</TableCell>
          </TableRow>
        </TableBody>
      </TableRoot>,
    );

    expect(container.querySelector('table')?.className)
      .not.toBe(plain.querySelector('table')?.className);
  });

  it('opens every row with the preview button and links the hash', () => {
    const { container } = renderTable();

    const rows = Array.from(container.querySelectorAll('tbody tr'));

    expect(rows).toHaveLength(txs.length);
    expect(rows.map((row) => Boolean(row.querySelector('[data-scan-preview]')))).toEqual([ true, true, true ]);
    expect(rows.map((row) => row.querySelector('a[href^="/tx/"]')?.getAttribute('href')))
      .toEqual(txs.map((tx) => `/tx/${ tx.hash }`));
  });

  it('reads the decoded method in the action column and falls back to the raw selector', () => {
    const { container } = renderTable();

    expect(Array.from(container.querySelectorAll('tbody tr [data-scan-method]')).map((chip) => chip.getAttribute('data-scan-method')))
      .toEqual([ txMock.base.method, txMock.base.method, txMock.base.raw_input.slice(0, 10) ]);
  });

  it('keeps the failed and pending marker in the hash cell', () => {
    const { container } = renderTable();

    const hashCells = Array.from(container.querySelectorAll('tbody tr')).map((row) => row.querySelectorAll('td')[1]);

    expect(hashCells.map((cell) => cell?.querySelector('[data-status]')?.getAttribute('data-status') ?? null))
      .toEqual([ null, 'error', 'pending' ]);
  });
});
