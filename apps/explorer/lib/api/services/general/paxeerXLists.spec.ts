import type { PaxeerXArchiveAnchor, PaxeerXArchiveReceipt, PaxeerXArchiveReceiptsItem } from 'types/api/paxeerXArchive';

import { PAXEER_X_ANCHORS_ITEM, PAXEER_X_RECEIPTS_ITEM } from 'stubs/paxeerXLists';
import { expect, expectTypeOf, test } from 'vitest';

import type { GeneralApiPaxeerXListsResourcePayload } from './paxeerXLists';
import { GENERAL_API_PAXEER_X_LISTS_RESOURCES } from './paxeerXLists';

function kernel_resources_use_archive_paths() {
  const resources = GENERAL_API_PAXEER_X_LISTS_RESOURCES;

  expect(resources.paxeer_x_anchors.path).toBe('/api/v2/paxeer-x/archive/anchors');
  expect(resources.paxeer_x_anchors.paginated).toBe(true);
  expect(resources.paxeer_x_anchors.filterFields).toEqual([]);
  expect(resources.paxeer_x_receipts.path).toBe('/api/v2/paxeer-x/archive/receipts');
  expect(resources.paxeer_x_receipts.paginated).toBe(true);
  expect(resources.paxeer_x_receipts.filterFields).toEqual([]);
  expect(resources.paxeer_x_receipt.path).toBe('/api/v2/paxeer-x/archive/receipts/:id');
  expect(resources.paxeer_x_receipt.pathParams).toEqual([ 'id' ]);

  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_anchors'>['items'][number]>()
    .toEqualTypeOf<PaxeerXArchiveAnchor>();
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipts'>['items'][number]>()
    .toEqualTypeOf<PaxeerXArchiveReceiptsItem>();
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipt'>>()
    .toEqualTypeOf<PaxeerXArchiveReceipt>();
  expectTypeOf<PaxeerXArchiveAnchor['frozen']>().toEqualTypeOf<true>();
  expectTypeOf<PaxeerXArchiveReceiptsItem['frozen']>().toEqualTypeOf<true>();
  expectTypeOf<PaxeerXArchiveReceipt['frozen']>().toEqualTypeOf<true>();
  expectTypeOf<'status'>().not.toMatchTypeOf<keyof PaxeerXArchiveAnchor>();
  expectTypeOf<'status'>().not.toMatchTypeOf<keyof PaxeerXArchiveReceiptsItem>();
  expectTypeOf<'status'>().not.toMatchTypeOf<keyof PaxeerXArchiveReceipt>();

  expect(PAXEER_X_ANCHORS_ITEM.frozen).toBe(true);
  expect(PAXEER_X_ANCHORS_ITEM).not.toHaveProperty('status');
  expect(PAXEER_X_RECEIPTS_ITEM.frozen).toBe(true);
  expect(PAXEER_X_RECEIPTS_ITEM).not.toHaveProperty('status');
}

test(kernel_resources_use_archive_paths, kernel_resources_use_archive_paths);
