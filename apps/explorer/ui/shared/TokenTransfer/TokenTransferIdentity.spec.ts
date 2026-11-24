import { isDeepStrictEqual } from 'node:util';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';

import type { Erc1155TotalPayload, TokenTransfer, TokenTransferResponse } from 'types/api/tokenTransfer';

import { MultichainContext } from 'lib/contexts/multichain';
import { erc20, erc721, erc1155A, erc1155D, erc7984 } from 'mocks/tokens/tokenTransfer';

import { getTokenTransferKey } from './helpers';
import { TokenTransferList } from './TokenTransferList';
import TokenTransferListItem from './TokenTransferListItem';
import { TokenTransferTable } from './TokenTransferTable';
import TokenTransferTableItem from './TokenTransferTableItem';

interface TransferRow {
  key: React.Key | null;
  transfer: TokenTransfer;
}

type TransferRenderer = (data: Array<TokenTransfer>, isLoading?: boolean) => React.ReactElement;

const renderList: TransferRenderer = (data, isLoading) => TokenTransferList({ data, isLoading });
const renderTable: TransferRenderer = (data, isLoading) => TokenTransferTable({ data, isLoading, top: 0 });

function collectRows(node: React.ReactNode, rows: Array<TransferRow>) {
  React.Children.forEach(node, child => {
    if (!React.isValidElement<{ children?: React.ReactNode }>(child)) {
      return;
    }

    if (React.isValidElement<TokenTransfer>(child) && (child.type === TokenTransferListItem || child.type === TokenTransferTableItem)) {
      rows.push({ key: child.key, transfer: child.props });
      return;
    }

    collectRows(child.props.children, rows);
  });
}

function renderedRows(renderer: TransferRenderer, data: Array<TokenTransfer>, isLoading?: boolean) {
  const rows: Array<TransferRow> = [];

  function CaptureTransferRows() {
    collectRows(renderer(data, isLoading), rows);
    return null;
  }

  renderToStaticMarkup(React.createElement(MultichainContext.Provider, {
    value: null,
    children: React.createElement(CaptureTransferRows),
  }));

  return rows;
}

const batchFixture = {
  items: [ erc1155A, erc1155D ],
  next_page_params: null,
} satisfies TokenTransferResponse;

function isBatchResponse(value: unknown): value is TokenTransferResponse {
  return isDeepStrictEqual(value, batchFixture);
}

async function readBatch() {
  const response = new Response(JSON.stringify(batchFixture), {
    headers: { 'Content-Type': 'application/json' },
  });

  const batch = await response.json();
  if (!isBatchResponse(batch)) {
    throw new Error('The decoded batch response must match the complete committed fixture');
  }

  return batch.items;
}

function batchTotal(transfer: TokenTransfer): Erc1155TotalPayload {
  const total = transfer.total;
  if (!total || !('token_id' in total) || !('value' in total) || !('decimals' in total) || !('token_instance' in total)) {
    throw new Error('The batch fixture must contain expanded ERC-1155 totals');
  }

  return total;
}

function tokenId(transfer: TokenTransfer) {
  return batchTotal(transfer).token_id;
}

async function erc1155ListIdentity() {
  const batch = await readBatch();
  const rows = renderedRows(renderList, batch);

  expect(batch).toHaveLength(2);
  expect(batch.map(item => [ item.transaction_hash, item.block_hash, item.log_index ]))
    .toEqual([ [ erc1155A.transaction_hash, erc1155A.block_hash, erc1155A.log_index ],
      [ erc1155A.transaction_hash, erc1155A.block_hash, erc1155A.log_index ] ]);
  expect(rows).toHaveLength(2);
  expect(rows.map(row => tokenId(row.transfer))).toEqual([ '123', '456' ]);
  expect(new Set(rows.map(row => row.key)).size).toBe(2);
  expect(rows.every(row => typeof row.key === 'string')).toBe(true);

  const reordered = renderedRows(renderList, [ ...batch ].reverse());
  expect(reordered.map(row => row.key)).toEqual(rows.map(row => row.key).reverse());
  expect(reordered.map(row => row.transfer.total)).toEqual(rows.map(row => row.transfer.total).reverse());
}

async function erc1155TableIdentity() {
  const batch = await readBatch();
  const initial = renderedRows(renderTable, batch);
  const mobile = renderedRows(renderList, batch);

  expect(initial).toHaveLength(2);
  expect(new Set(initial.map(row => row.key)).size).toBe(2);
  expect(initial.map(row => row.key)).toEqual(mobile.map(row => row.key));

  const firstPage = renderedRows(renderTable, batch.slice(0, 1));
  const secondPage = renderedRows(renderTable, batch.slice(1));
  expect(firstPage.map(row => row.key)).toEqual([ initial[0].key ]);
  expect(secondPage.map(row => row.key)).toEqual([ initial[1].key ]);
  expect(secondPage.map(row => tokenId(row.transfer))).toEqual([ '456' ]);

  const updated = batch.map(item => ({
    ...item,
    total: { ...batchTotal(item), value: tokenId(item) === '123' ? '500' : '600' },
  } satisfies TokenTransfer)).reverse();
  const rows = renderedRows(renderTable, updated);
  expect(rows.map(row => row.key)).toEqual(initial.map(row => row.key).reverse());
  expect(rows.map(row => row.transfer.total)).toEqual([
    { ...batchTotal(erc1155D), value: '600' },
    { ...batchTotal(erc1155A), value: '500' },
  ]);
  expect(renderedRows(renderList, updated).map(row => row.key)).toEqual(rows.map(row => row.key));
  expect(renderedRows(renderTable, []).length).toBe(0);
  expect(renderedRows(renderTable, batch).map(row => row.key)).toEqual(initial.map(row => row.key));
}

function transferIdentityCompatibility() {
  const transfers = [ erc20, erc721, erc1155A ];

  for (const renderer of [ renderList, renderTable ]) {
    const rows = renderedRows(renderer, transfers);
    expect(rows).toHaveLength(3);
    expect(new Set(rows.map(row => row.key)).size).toBe(3);
    expect(rows.map(row => row.transfer.total)).toEqual(transfers.map(item => item.total));

    const confidentialRows = renderedRows(renderer, [ erc7984 ]);
    expect(confidentialRows).toHaveLength(1);
    expect(confidentialRows[0].transfer.total).toBeNull();
    expect(typeof confidentialRows[0].key).toBe('string');

    const placeholders = renderedRows(renderer, [ erc1155A, erc1155A ], true);
    expect(placeholders).toHaveLength(2);
    expect(new Set(placeholders.map(row => row.key)).size).toBe(2);
  }

  expect(getTokenTransferKey({ ...erc1155A, total: { ...batchTotal(erc1155A), token_id: '0' } }))
    .not.toBe(getTokenTransferKey({ ...erc1155A, total: null }));
  expect(getTokenTransferKey({ ...erc1155A, transaction_hash: null }))
    .not.toBe(getTokenTransferKey(erc1155A));
  expect(getTokenTransferKey({ ...erc1155A, block_hash: 'another-block' }))
    .not.toBe(getTokenTransferKey(erc1155A));
  expect(getTokenTransferKey({ ...erc1155A, log_index: '2' }))
    .not.toBe(getTokenTransferKey(erc1155A));
}

test('erc1155_list_identity', erc1155ListIdentity);
test('erc1155_table_identity', erc1155TableIdentity);
test('transfer identity preserves other token types and loading placeholders', transferIdentityCompatibility);
