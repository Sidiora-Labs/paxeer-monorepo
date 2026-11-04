// @vitest-environment jsdom

import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import path from 'node:path';
import React from 'react';

import type { GeneralApiPaxeerXListsResourcePayload } from 'lib/api/services/general/paxeerXLists';
import { GENERAL_API_PAXEER_X_LISTS_RESOURCES } from 'lib/api/services/general/paxeerXLists';
import useFetch from 'lib/hooks/useFetch';
import { PAXEER_X_KERNEL_RECEIPTS_ITEM, PAXEER_X_RECEIPTS_ITEM } from 'stubs/paxeerXLists';
import { render, Wrapper } from 'ui/paxeerX/account/testWrapper';
import PaxeerXReceiptDetails from 'ui/paxeerX/receipts/PaxeerXReceiptDetails';
import PaxeerXReceiptsListItem from 'ui/paxeerX/receipts/PaxeerXReceiptsListItem';
import PaxeerXReceiptsTable from 'ui/paxeerX/receipts/PaxeerXReceiptsTable';
import { expect, expectTypeOf, test } from 'vitest';
import { renderHook } from 'vitest/lib';

import type {
  PaxeerXArchiveReceipt,
  PaxeerXArchiveReceiptsItem,
  PaxeerXArchiveReceiptsResponse,
  PaxeerXArchiveVerificationStatus,
} from './paxeerXArchive';

const backend = path.resolve(__dirname, '../../../../services/explorer/backend');

test('archiveReceiptNullabilityMatchesProducerFields', function archiveReceiptNullabilityMatchesProducerFields() {
  const view = readFileSync(path.join(backend, 'apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/archive_view.ex'), 'utf8');
  const receiptMap = view.split('defp prepare_receipt(%Receipt{} = receipt) do')[1]?.split('\n  end')[0];
  expect(receiptMap).toBeDefined();
  const emittedKeys = Array.from((receiptMap ?? '').matchAll(/"(\w+)" =>/g), (match) => match[1]);
  expect(emittedKeys.sort()).toEqual([
    'account', 'block_number', 'frozen', 'id', 'origin', 'payload_hash', 'provenance',
    'timestamp', 'transaction_hash', 'verification', 'verification_status',
  ].sort());
  expectTypeOf<keyof PaxeerXArchiveReceipt>().toEqualTypeOf<
    'account' | 'block_number' | 'frozen' | 'id' | 'origin' | 'payload_hash' | 'provenance' |
    'timestamp' | 'transaction_hash' | 'verification' | 'verification_status'
  >();
  expect(view).toContain('Enum.map(receipts, &prepare_receipt/1)');
  expect(view).toContain('def render("receipt.json", %{receipt: receipt}), do: prepare_receipt(receipt)');
  expectTypeOf<PaxeerXArchiveReceipt>().toEqualTypeOf<PaxeerXArchiveReceiptsItem>();
  expectTypeOf<PaxeerXArchiveReceiptsResponse['items'][number]>().toEqualTypeOf<PaxeerXArchiveReceipt>();
});

test('archiveReceiptNullabilityPreservesEvmContract', function archiveReceiptNullabilityPreservesEvmContract() {
  type Evm = Extract<PaxeerXArchiveReceipt, { origin: 'evm' }>;
  expectTypeOf<Evm['block_number']>().toEqualTypeOf<number>();
  expectTypeOf<Evm['transaction_hash']>().toEqualTypeOf<string>();
  expectTypeOf<Evm['timestamp']>().toEqualTypeOf<string | null>();
  expectTypeOf<Evm['verification']>().toEqualTypeOf<PaxeerXArchiveVerificationStatus>();
  expectTypeOf<Evm['verification_status']>().toEqualTypeOf<PaxeerXArchiveVerificationStatus>();
  expectTypeOf<Evm['provenance']>().toEqualTypeOf<{
    evm: { transaction_hash: string; log_index: number; block_hash: string; block_number: number };
  }>();
  const view = readFileSync(path.join(backend, 'apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/archive_view.ex'), 'utf8');
  const provenance = view.split('defp provenance(%Receipt{origin: :evm} = receipt) do')[1]?.split('\n  end')[0];
  expect(provenance).toBeDefined();
  expect(Array.from((provenance ?? '').matchAll(/"(\w+)" =>/g), (match) => match[1]))
    .toEqual([ 'evm', 'transaction_hash', 'log_index', 'block_hash', 'block_number' ]);
  expect(view).toContain('{:ok, %Block{timestamp: timestamp}} -> timestamp');
  expect(view).toContain('_missing -> nil');
});

test('archiveReceiptNullabilityPreservesKernelContract', function archiveReceiptNullabilityPreservesKernelContract() {
  type Kernel = Extract<PaxeerXArchiveReceipt, { origin: 'kernel' }>;
  expectTypeOf<Kernel['block_number']>().toEqualTypeOf<null>();
  expectTypeOf<Kernel['transaction_hash']>().toEqualTypeOf<null>();
  expectTypeOf<Kernel['timestamp']>().toEqualTypeOf<null>();
  expectTypeOf<Kernel['verification']>().toEqualTypeOf<'sequencer_verified'>();
  expectTypeOf<Kernel['verification_status']>().toEqualTypeOf<'batch_included'>();
  expectTypeOf<Kernel['provenance']>().toEqualTypeOf<{
    kernel: {
      batch_number: string;
      batch_id: string;
      sequence: string;
      activity_id: string;
      result_code: number;
      receipt_sha256: string;
      canonical_sha256: string;
      batch_raw_sha256: string;
      state_root: string;
      proof: { header_hex: string; signature_hex: string };
    };
  }>();
  const view = readFileSync(path.join(backend, 'apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/archive_view.ex'), 'utf8');
  const provenance = view.split('defp provenance(%Receipt{origin: :kernel} = receipt) do')[1]?.split('\n  end')[0];
  expect(provenance).toBeDefined();
  expect(Array.from((provenance ?? '').matchAll(/"(\w+)" =>/g), (match) => match[1])).toEqual([
    'kernel', 'batch_number', 'batch_id', 'sequence', 'activity_id', 'result_code', 'receipt_sha256',
    'canonical_sha256', 'batch_raw_sha256', 'state_root', 'proof', 'header_hex', 'signature_hex',
  ]);
  expect(provenance).toContain('"batch_number" => to_string(receipt.kernel_batch_number)');
  expect(provenance).toContain('"sequence" => to_string(receipt.kernel_sequence)');
  expect(view).toContain('defp timestamp(%Receipt{}), do: nil');
  const migration = readFileSync(path.join(backend, 'apps/explorer/priv/repo/migrations/20261001000300_add_kernel_receipt_provenance.exs'), 'utf8');
  expect(migration).toContain('receipt_id = kernel_activity_id AND status = \'batch_included\'');
  expect(migration).toContain('AND transaction_hash IS NULL AND log_index IS NULL AND block_hash IS NULL');
  expect(migration).toContain('AND block_number IS NULL AND block_consensus IS NULL');
  expect(migration).toContain('kernel_verification = \'sequencer_verified\'');
});

test('archiveReceiptNullabilityReachesFrontendResources', function archiveReceiptNullabilityReachesFrontendResources() {
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipt'>>()
    .toEqualTypeOf<PaxeerXArchiveReceipt>();
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipts'>>()
    .toEqualTypeOf<PaxeerXArchiveReceiptsResponse>();
  expectTypeOf<PaxeerXArchiveReceipt['origin']>().toEqualTypeOf<'evm' | 'kernel'>();
  expectTypeOf<PaxeerXArchiveReceipt['frozen']>().toEqualTypeOf<true>();
  expectTypeOf<PaxeerXArchiveReceipt['account']>().toEqualTypeOf<string | null>();
  expectTypeOf<PaxeerXArchiveReceipt['payload_hash']>().toEqualTypeOf<string | null>();
  expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipts.path).toBe('/api/v2/paxeer-x/archive/receipts');
  expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipt.path).toBe('/api/v2/paxeer-x/archive/receipts/:id');
});

test('archiveReceiptNullabilityTransportsBothOriginsToConsumers', async function archiveReceiptNullabilityTransportsBothOriginsToConsumers() {
  const evmWithoutTimestamp = { ...PAXEER_X_RECEIPTS_ITEM, timestamp: null } satisfies PaxeerXArchiveReceipt;
  const items: Array<PaxeerXArchiveReceipt> = [ PAXEER_X_RECEIPTS_ITEM, PAXEER_X_KERNEL_RECEIPTS_ITEM ];
  const listPath = GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipts.path;
  const detailPath = (id: string) => GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipt.path.replace(':id', id);
  const requests: Array<string> = [];
  const server = createServer((request, response) => {
    requests.push(request.url ?? '');
    response.setHeader('Content-Type', 'application/json');
    if (request.url === listPath) {
      response.end(JSON.stringify({ items, next_page_params: null } satisfies PaxeerXArchiveReceiptsResponse));
      return;
    }
    const receipt = [ evmWithoutTimestamp, PAXEER_X_KERNEL_RECEIPTS_ITEM ].find((item) => request.url === detailPath(item.id));
    if (receipt) {
      response.end(JSON.stringify(receipt));
      return;
    }
    response.statusCode = 404;
    response.end(JSON.stringify({ message: 'Not found' }));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP listener');
  }
  const endpoint = `http://127.0.0.1:${ address.port }`;
  fetchMock.disableMocks();
  const hook = renderHook(() => useFetch(), { wrapper: Wrapper });
  try {
    const list = await hook.result.current<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipts'>, never>(endpoint + listPath);
    if (!('items' in list)) {
      throw new Error('Expected an archive receipts response');
    }
    expect(list).toEqual({ items, next_page_params: null });
    const table = render(React.createElement(PaxeerXReceiptsTable, { items: list.items }));
    const evmRow = table.container.querySelector(`[data-receipt="${ PAXEER_X_RECEIPTS_ITEM.id }"]`);
    const kernelRow = table.container.querySelector(`[data-receipt="${ PAXEER_X_KERNEL_RECEIPTS_ITEM.id }"]`);
    expect(evmRow?.querySelector(`a[href="/block/${ PAXEER_X_RECEIPTS_ITEM.block_number }"]`)).toBeTruthy();
    expect(kernelRow?.querySelectorAll('td')[2]?.textContent).toBe('Pending');
    expect(kernelRow?.querySelector('a[href^="/block/"]')).toBeNull();
    table.unmount();
    for (const item of list.items) {
      const detail = await hook.result.current<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipt'>, never>(endpoint + detailPath(item.id));
      if (!('origin' in detail)) {
        throw new Error('Expected an archive receipt response');
      }
      expect(detail).toEqual(item.origin === 'evm' ? evmWithoutTimestamp : item);
      expect(detail.origin).toBe(item.origin);
      expect(detail.provenance).toEqual(item.provenance);
      const mobile = render(React.createElement(PaxeerXReceiptsListItem, { item: detail }));
      expect(mobile.container.querySelector(`a[href="/paxeer-x/receipts/${ item.id }"]`)).toBeTruthy();
      if (detail.origin === 'kernel') {
        expect(mobile.container.textContent).toContain('Pending');
        expect(mobile.container.querySelector('a[href^="/block/"]')).toBeNull();
      } else {
        expect(mobile.container.querySelector(`a[href="/block/${ detail.block_number }"]`)).toBeTruthy();
      }
      mobile.unmount();
      const rendered = render(React.createElement(PaxeerXReceiptDetails, { data: detail }));
      expect(rendered.container.querySelector('[data-field="id"]')?.textContent).toBe(item.id);
      expect(rendered.container.querySelector('[data-verification-status]')?.getAttribute('data-verification-status'))
        .toBe(item.verification_status);
      const values = Array.from(rendered.container.querySelectorAll('[data-scan-value]'));
      expect(values[6]?.textContent).toBe('—');
      if (detail.origin === 'kernel') {
        expect(values[4]?.textContent).toBe('—');
        expect(values[5]?.textContent).toBe('—');
        expect(rendered.container.querySelector('a[href^="/tx/"]')).toBeNull();
        expect(rendered.container.querySelector('a[href^="/block/"]')).toBeNull();
      } else {
        expect(rendered.container.querySelector(`a[href="/tx/${ detail.transaction_hash }"]`)).toBeTruthy();
        expect(rendered.container.querySelector(`a[href="/block/${ detail.block_number }"]`)).toBeTruthy();
      }
      rendered.unmount();
    }
    expect(requests).toEqual([ listPath, ...items.map((item) => detailPath(item.id)) ]);
  } finally {
    hook.unmount();
    fetchMock.enableMocks();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}, 60_000);
