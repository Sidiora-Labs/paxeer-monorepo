// @vitest-environment jsdom

import React from 'react';

import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';
import { screen, within } from 'vitest/lib';

import TableScrollArea from './TableScrollArea';

test('tableRegionPreservesHeadersAndKeyboardFocus', function tableRegionPreservesHeadersAndKeyboardFocus() {
  render(
    <TableScrollArea label="Block history">
      <table aria-label="Block history">
        <thead><tr><th scope="col">Height</th><th scope="col">Transactions</th></tr></thead>
        <tbody><tr><td>125</td><td>42</td></tr></tbody>
      </table>
    </TableScrollArea>,
  );

  const region = screen.getByRole('region', { name: 'Block history' });
  region.focus();
  expect(document.activeElement).toBe(region);
  const table = within(region).getByRole('table', { name: 'Block history' });
  expect(within(table).getAllByRole('columnheader').map((header) => header.textContent)).toEqual([ 'Height', 'Transactions' ]);
  expect(within(table).getAllByRole('cell').map((cell) => cell.textContent)).toEqual([ '125', '42' ]);
});
