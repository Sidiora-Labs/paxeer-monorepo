// @vitest-environment jsdom

import React from 'react';

import { SettingsContextProvider } from 'lib/contexts/settings';
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
