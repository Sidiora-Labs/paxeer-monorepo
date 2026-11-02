import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { describe, expect, it, test } from 'vitest';

import { activityKindLabel, assetLabel, formatAmount, IDENTITY_ORDER, listIdentities } from './utils';

function identity_order_has_three_kinds() {
  expect(IDENTITY_ORDER).toEqual([ 'evm', 'pax', 'did' ]);
  expect(listIdentities(paxeerXMock.unifiedAccount.identities)).toEqual([
    { kind: 'evm', label: 'EVM address', value: paxeerXMock.evmAddress },
    { kind: 'pax', label: 'Paxeer address', value: paxeerXMock.paxAddress },
    { kind: 'did', label: 'LayerX DID', value: paxeerXMock.did },
  ]);
}

test(identity_order_has_three_kinds);

describe('listIdentities', () => {
  it('keeps the three spellings in a stable order', () => {
    expect(listIdentities(paxeerXMock.unifiedAccount.identities).map((entry) => entry.kind))
      .toEqual([ 'evm', 'pax', 'did' ]);
  });

  it('drops the identities that are absent', () => {
    expect(listIdentities(paxeerXMock.unifiedAccountEvmOnly.identities).map((entry) => entry.kind))
      .toEqual([ 'evm' ]);
  });
});

describe('formatAmount', () => {
  it('scales by the asset decimals', () => {
    expect(formatAmount('1500000000000000000', paxeerXMock.nativeAsset)).toBe('1.5');
    expect(formatAmount('2500000', paxeerXMock.tokenAsset)).toBe('2.5');
  });

  it('leaves the amount unscaled when the asset carries no scale', () => {
    expect(formatAmount('1200', { ...paxeerXMock.tokenAsset, decimals: null })).toBe('1,200');
  });

  it('leaves the amount unscaled when the asset is unknown', () => {
    expect(formatAmount('1234', null)).toBe('1,234');
  });

  it('returns a non-numeric amount verbatim', () => {
    expect(formatAmount('not-a-number', paxeerXMock.nativeAsset)).toBe('not-a-number');
  });
});

describe('assetLabel', () => {
  it('prefers the symbol, then the denom', () => {
    expect(assetLabel(paxeerXMock.nativeAsset)).toBe('HPX');
    expect(assetLabel({ ...paxeerXMock.tokenAsset, symbol: null })).toBe(paxeerXMock.tokenAsset.denom);
    expect(assetLabel(null)).toBe('Unknown asset');
  });
});

describe('activityKindLabel', () => {
  it('names the chain-side rows and humanizes legacy kernel events', () => {
    expect(activityKindLabel('custody_deposit')).toBe('Custody deposit');
    expect(activityKindLabel('claim_finalised')).toBe('Claim finalised');
    expect(activityKindLabel('token_transfer')).toBe('Token transfer');
    expect(activityKindLabel('transaction')).toBe('Transaction');
  });

  it('humanizes a kind it does not know', () => {
    expect(activityKindLabel('module12/event3')).toBe('Module12/event3');
    expect(activityKindLabel('native_transfer')).toBe('Native transfer');
  });
});
