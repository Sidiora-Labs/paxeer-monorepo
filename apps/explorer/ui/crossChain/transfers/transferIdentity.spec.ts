// @vitest-environment jsdom

import React from 'react';

import type { GetTransfersResponse, InterchainTransfer } from '@blockscout/interchain-indexer-types';
import { MessageStatus } from '@blockscout/interchain-indexer-types';

import { listResponse, transferA, transferB } from 'mocks/crossChain/transfers';
import { expect, test } from 'vitest';

import { TokenTransfersCrossChainContent } from './TokenTransfersCrossChainContent';
import TokenTransfersCrossChainListItem from './TokenTransfersCrossChainListItem';
import { TokenTransfersCrossChainTable } from './TokenTransfersCrossChainTable';
import TokenTransfersCrossChainTableItem from './TokenTransfersCrossChainTableItem';
import { getItemKey } from './utils';

function transportTransfers(items: Array<InterchainTransfer>): GetTransfersResponse {
  const payload: GetTransfersResponse = { ...listResponse, items };
  const response = JSON.parse(JSON.stringify(payload)) as GetTransfersResponse;
  expect(response).toEqual(payload);
  return response;
}

function collectRows(tree: React.ReactNode, rowType: typeof TokenTransfersCrossChainListItem | typeof TokenTransfersCrossChainTableItem) {
  const rows: Array<{ key: React.Key | null; data: InterchainTransfer }> = [];

  React.Children.forEach(tree, (child) => {
    if (!React.isValidElement<{ data: InterchainTransfer; children?: React.ReactNode }>(child)) {
      return;
    }

    if (child.type === rowType) {
      rows.push({ key: child.key, data: child.props.data });
    } else {
      rows.push(...collectRows(child.props.children, rowType));
    }
  });

  return rows;
}

function consumeTransfers(items: Array<InterchainTransfer>, isLoading = false) {
  return [
    collectRows(TokenTransfersCrossChainContent({ items, isLoading, isError: false }), TokenTransfersCrossChainListItem),
    collectRows(TokenTransfersCrossChainTable({ data: items, isLoading }), TokenTransfersCrossChainTableItem),
  ];
}

function sameSourceRecipientsAndDestinations() {
  const { items } = transportTransfers([
    transferA,
    { ...transferA, recipient: { hash: transferA.source_token.address_hash } },
    { ...transferA, destination_chain: transferA.source_chain },
    { ...transferA, destination_token: transferA.source_token },
    { ...transferA, destination_amount: '1' },
    { ...transferA, destination_transaction_hash: transferB.destination_transaction_hash },
    { ...transferA, source_transaction_hash: transferB.source_transaction_hash },
    { ...transferA, bridge: transferB.bridge },
    { ...transferA, send_timestamp: '2022-01-13T12:06:25.000Z' },
  ]);

  expect(new Set(items.map((item) => getItemKey(item))).size).toBe(items.length);
  for (const rows of consumeTransfers(items)) {
    expect(rows.map((row) => row.data)).toEqual(items);
    expect(new Set(rows.map((row) => row.key)).size).toBe(items.length);
    expect(rows.map((row) => row.key)).toEqual(items.map((item) => getItemKey(item, 0)));
  }
}

function exactDuplicateOccurrences() {
  const { items } = transportTransfers([ transferA, transferA, transferA ]);

  expect(items[0]).not.toBe(items[1]);
  expect(items[0]).toEqual(items[1]);
  const expectedKeys = items.map((item, occurrence) => getItemKey(item, occurrence));

  for (const isLoading of [ true, false ]) {
    for (const rows of consumeTransfers(items, isLoading)) {
      expect(rows).toHaveLength(3);
      expect(rows.map((row) => row.data)).toEqual(items);
      expect(new Set(rows.map((row) => row.key)).size).toBe(3);
      expect(rows.map((row) => row.key)).toEqual(expectedKeys);
    }
  }
}

function stableKeysAcrossListUpdates() {
  const recipientTransfer = { ...transferA, recipient: { hash: transferA.source_token.address_hash } };
  const initial = transportTransfers([ transferA, recipientTransfer, transferA ]);
  const updated = transportTransfers([ transferB, recipientTransfer, transferA, transferA ]);
  const removed = transportTransfers([ recipientTransfer, transferA, transferA ]);
  const originalViews = consumeTransfers(initial.items);

  for (const response of [ updated, removed ]) {
    consumeTransfers(response.items).forEach((rows, view) => {
      const original = originalViews[view];
      const offset = response.items.length - initial.items.length;

      expect(rows.map((row) => row.data)).toEqual(response.items);
      expect(rows[offset].key).toBe(original[1].key);
      expect(rows[offset + 1].key).toBe(original[0].key);
      expect(rows[offset + 2].key).toBe(original[2].key);
      expect(new Set(rows.map((row) => row.key)).size).toBe(response.items.length);
    });
  }
}

function compositeIdentityPreservesFieldBoundaries() {
  const first = { ...transferA, message_id: 'message-part', sender: { hash: 'sender' } };
  const second = { ...transferA, message_id: 'message', sender: { hash: 'part-sender' } };
  expect(getItemKey(first)).not.toBe(getItemKey(second));
  expect(getItemKey({ ...transferA, sender: undefined, source_amount: 'sender' }))
    .not.toBe(getItemKey({ ...transferA, sender: { hash: 'sender' }, source_amount: undefined }));

  const statusUpdate = {
    ...transferA,
    status: MessageStatus.MESSAGE_STATUS_FAILED,
    receive_timestamp: '2022-01-13T12:06:31.000Z',
    recipient: { ...transferA.recipient, ens_domain_name: 'recipient.eth' },
  };
  expect(getItemKey(statusUpdate)).toBe(getItemKey(transferA));
}

test(sameSourceRecipientsAndDestinations, sameSourceRecipientsAndDestinations);
test(exactDuplicateOccurrences, exactDuplicateOccurrences);
test(stableKeysAcrossListUpdates, stableKeysAcrossListUpdates);
test(compositeIdentityPreservesFieldBoundaries, compositeIdentityPreservesFieldBoundaries);
