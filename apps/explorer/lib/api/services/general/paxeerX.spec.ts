import * as capabilitiesMock from 'mocks/paxeerX/capabilities';
import * as unifiedAccountMock from 'mocks/paxeerX/unifiedAccount';
import { PAXEER_X_ANCHORS_ITEM, PAXEER_X_RECEIPTS_ITEM, PAXEER_X_TX_STATUS } from 'stubs/paxeerXLists';
import { describe, expect, it } from 'vitest';

import type { GeneralApiPaxeerXResourceName, GeneralApiPaxeerXResourcePayload } from './paxeerX';
import { GENERAL_API_PAXEER_X_RESOURCES } from './paxeerX';
import type { GeneralApiPaxeerXListsResourceName, GeneralApiPaxeerXListsResourcePayload } from './paxeerXLists';
import { GENERAL_API_PAXEER_X_LISTS_RESOURCES } from './paxeerXLists';

describe('the Paxeer X resource definitions', () => {
  it('carries the unified account and capability paths the backend serves', () => {
    expect(GENERAL_API_PAXEER_X_RESOURCES.paxeer_x_unified_account.path).toBe('/api/v2/addresses/:hash/unified');
    expect(GENERAL_API_PAXEER_X_RESOURCES.paxeer_x_unified_account.pathParams).toEqual([ 'hash' ]);
    expect(GENERAL_API_PAXEER_X_RESOURCES.paxeer_x_capabilities.path).toBe('/api/v2/paxeer-x/capabilities');
  });

  it('takes the transaction hash as a path parameter and paginates the two list paths', () => {
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_tx_status.path).toBe('/api/v2/transactions/:hash/status');
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_tx_status.pathParams).toEqual([ 'hash' ]);
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_anchors.path).toBe('/api/v2/paxeer-x/archive/anchors');
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_anchors.paginated).toBe(true);
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipts.path).toBe('/api/v2/paxeer-x/archive/receipts');
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipts.paginated).toBe(true);
  });

  it('reads a single receipt by the id in its path', () => {
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipt.path).toBe('/api/v2/paxeer-x/archive/receipts/:id');
    expect(GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_receipt.pathParams).toEqual([ 'id' ]);
  });
});

describe('the Paxeer X payload map', () => {
  function unifiedBalanceFixtureMatchesNativeControllerContract() {
    const unifiedAccount: GeneralApiPaxeerXResourcePayload<'general:paxeer_x_unified_account'> = unifiedAccountMock.unifiedAccount;
    const capabilities: GeneralApiPaxeerXResourcePayload<'general:paxeer_x_capabilities'> = capabilitiesMock.addrOnly;
    const txStatus: GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_tx_status'> = PAXEER_X_TX_STATUS;
    const anchors: GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_anchors'> = {
      items: [ PAXEER_X_ANCHORS_ITEM ],
      next_page_params: null,
    };
    const receipts: GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipts'> = {
      items: [ PAXEER_X_RECEIPTS_ITEM ],
      next_page_params: null,
    };
    const receipt: GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_receipt'> = {
      ...PAXEER_X_RECEIPTS_ITEM,
      verification_status: 'checkpoint_finalised',
      payload_hash: '0x3ed9d81e7c1001bdda1caa1dc62c0acbbe3d2c671cdc20dc1e65efdaa4186967',
      transaction_hash: '0x8f9e7d6c5b4a39281706f5e4d3c2b1a0998877665544332211ffeeddccbbaa99',
      timestamp: '2023-05-22T18:00:36.000000Z',
    };

    expect(unifiedAccount.identities.evm).toBe(unifiedAccountMock.evmAddress);
    expect(unifiedAccount.balances).toEqual([
      {
        asset: {
          id: '0x2170Ed0880ac9A755fd29B2688956BD959F933F8',
          denom: 'USDX',
          symbol: 'USDX',
          decimals: 6,
        },
        total: '2500000',
      },
      {
        asset: { id: 'native', denom: 'HPX', symbol: 'HPX', decimals: 18 },
        total: '1000000000000000000',
      },
    ]);
    expect(unifiedAccount.activity).toEqual([
      {
        kind: 'transaction',
        hash: '0x1f0e9c7a3b5d2e4f6081a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7',
        block_number: 1_284_912,
        ordinal: 0,
        status: 'committed',
        result: 'error',
        timestamp: '2024-04-02T11:03:11.000000Z',
        asset: { id: 'native', denom: 'HPX', symbol: 'HPX', decimals: 18 },
        amount: '0',
        counterparty: '0x8Ba1f109551bD432803012645Ac136ddd64DBA72',
      },
      {
        kind: 'transaction',
        hash: '0x8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f00112233445566778899aabbccdd',
        block_number: 1_284_017,
        ordinal: 0,
        status: 'committed',
        result: 'ok',
        timestamp: '2024-04-02T10:12:35.000000Z',
        asset: { id: 'native', denom: 'HPX', symbol: 'HPX', decimals: 18 },
        amount: '1500000000000000000',
        counterparty: null,
      },
    ]);
    expect(unifiedAccount.activity_total).toBe(2);
    expect(unifiedAccount.next_page_params).toBeNull();
    expect(unifiedAccount.page_cursor).toBe(unifiedAccountMock.nativeUnifiedAccount.page_cursor);
    expect(unifiedAccount.first_page_cursor).toBe(unifiedAccount.page_cursor);
    expect(unifiedAccount.page_number).toBe(1);
    expect(capabilities.addr).toBe(true);
    expect(txStatus.rung).toBe(PAXEER_X_TX_STATUS.rung);
    expect(anchors.items[0].checkpoint_id).toBe(PAXEER_X_ANCHORS_ITEM.checkpoint_id);
    expect(receipts.items[0].frozen).toBe(PAXEER_X_RECEIPTS_ITEM.frozen);
    expect(receipt.id).toBe(PAXEER_X_RECEIPTS_ITEM.id);
    expect(receipt.verification_status).toBe('checkpoint_finalised');
  }

  it(unifiedBalanceFixtureMatchesNativeControllerContract);

  function signatureResourceInventoryIsCompleteInDefinitionOrder() {
    const names: Array<GeneralApiPaxeerXResourceName | GeneralApiPaxeerXListsResourceName> = [
      'general:paxeer_x_unified_account',
      'general:paxeer_x_capabilities',
      'general:paxeer_x_tx_status',
      'general:paxeer_x_tx_signature',
      'general:paxeer_x_anchors',
      'general:paxeer_x_receipts',
      'general:paxeer_x_receipt',
    ];

    const defined = [
      ...Object.keys(GENERAL_API_PAXEER_X_RESOURCES),
      ...Object.keys(GENERAL_API_PAXEER_X_LISTS_RESOURCES),
    ].map((name) => `general:${ name }`);

    expect(names).toEqual(defined);
  }

  it(signatureResourceInventoryIsCompleteInDefinitionOrder);
});
