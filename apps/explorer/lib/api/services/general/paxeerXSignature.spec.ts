import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';

import { expect, expectTypeOf, test } from 'vitest';

import type { GeneralApiPaxeerXListsResourcePayload } from './paxeerXLists';
import { GENERAL_API_PAXEER_X_LISTS_RESOURCES } from './paxeerXLists';

test('signature_resource_path_is_signature_route', () => {
  const resource = GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_tx_signature;

  expect(resource.path).toBe('/api/v2/transactions/:hash/signature');
  expect(resource.pathParams).toEqual([ 'hash' ]);
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_tx_signature'>>().toEqualTypeOf<PaxeerXTxSignature>();
});
