// @vitest-environment jsdom
import type { Address, Hex } from 'viem';

import { afterEach, beforeEach, expect, test } from 'vitest';

import { listTransfers, saveTransfer, transferIdOf, updateTransfer } from './transfers';
import type { BridgeTransfer } from './transfers';

const storageKey = 'paxeer-bridge:transfers:v1';
const accountA: Address = '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd';
const accountAUpper: Address = '0xABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD';
const accountB: Address = '0x1234567890123456789012345678901234567890';

function transferOf(account: Address, submittedAt: number): BridgeTransfer {
  const sourceTxHash: Hex = `0x${ submittedAt.toString(16).padStart(64, '0') }`;
  return {
    id: transferIdOf('in', sourceTxHash),
    direction: 'in',
    foreignChainKey: 'base',
    foreignChainId: '8453',
    symbol: 'USDC',
    decimals: 6,
    amount: '1000000',
    assetId: '0x0000000000000000000000000000000000000001',
    account,
    recipient: account,
    sourceTxHash,
    requiredConfirmations: 12,
    submittedAt,
  };
}

beforeEach(() => window.localStorage.clear());
afterEach(() => window.localStorage.clear());

function listTransfersDropsMalformedEntries(): void {
  const valid = transferOf(accountA, 1);
  const other = transferOf(accountB, 2);
  const malformed: Array<unknown> = [ null, false, 42, 'transfer', [], {} ];
  const invalidFields: Record<string, unknown> = {
    id: 1,
    account: 1,
    direction: 'sideways',
    foreignChainKey: null,
    foreignChainId: 8453,
    symbol: false,
    decimals: '6',
    amount: 1000000,
    assetId: {},
    recipient: null,
    sourceTxHash: [],
    requiredConfirmations: '12',
    submittedAt: '1',
  };
  for (const [ field, value ] of Object.entries(invalidFields)) {
    malformed.push({ ...valid, [field]: value });
    const missing: Record<string, unknown> = { ...valid };
    delete missing[field];
    malformed.push(missing);
  }
  window.localStorage.setItem(storageKey, JSON.stringify([ ...malformed, valid, other ]));

  expect(listTransfers(accountAUpper)).toEqual([ valid ]);
  expect(listTransfers(accountB)).toEqual([ other ]);
  expect(listTransfers()).toEqual([ valid, other ]);

  for (const raw of [ 'null', '{}', '"transfer"', '{invalid JSON' ]) {
    window.localStorage.setItem(storageKey, raw);
    expect(listTransfers(accountA)).toEqual([]);
    expect(listTransfers()).toEqual([]);
  }
  window.localStorage.removeItem(storageKey);
  expect(listTransfers()).toEqual([]);
}

function transferIdOfKeysBySourceHashAndDirection(): void {
  const hash: Hex = `0x${ 'AbCd'.repeat(16) }`;
  const lower: Hex = `0x${ 'abcd'.repeat(16) }`;

  expect(transferIdOf('in', hash)).toBe(`${ lower }:in`);
  expect(transferIdOf('out', hash)).toBe(`${ lower }:out`);
  expect(transferIdOf('in', hash)).toBe(transferIdOf('in', lower));
  expect(transferIdOf('out', hash)).toBe(transferIdOf('out', lower));
  expect(transferIdOf('in', hash)).not.toBe(transferIdOf('out', hash));
  expect(transferIdOf('in', hash)).not.toBe(transferIdOf('in', `0x${ '1234'.repeat(16) }`));
}

function saveTransferKeepsFiftyRecordsPerAccount(): void {
  const initialA = Array.from({ length: 50 }, (_, index) => transferOf(accountA, index + 1));
  const initialB = Array.from({ length: 50 }, (_, index) => transferOf(accountB, index + 101));
  for (let index = 0; index < 50; index++) {
    saveTransfer(initialA[index]);
    saveTransfer(initialB[index]);
  }
  const expectedB = [ ...initialB ].reverse();
  expect(listTransfers(accountA)).toEqual([ ...initialA ].reverse());
  expect(listTransfers(accountB)).toEqual(expectedB);
  expect(listTransfers()).toHaveLength(100);

  const newestA = transferOf(accountAUpper, 51);
  saveTransfer(newestA);
  const expectedA = [ newestA, ...initialA.slice(1).reverse() ];
  expect(listTransfers(accountA)).toEqual(expectedA);
  expect(listTransfers(accountAUpper)).toEqual(expectedA);
  expect(listTransfers(accountB)).toEqual(expectedB);
  expect(listTransfers(accountA).some((transfer) => transfer.id === initialA[0].id)).toBe(false);
  expect(listTransfers()).toHaveLength(100);

  saveTransfer(transferOf(accountA, 0));
  expect(listTransfers(accountA)).toEqual(expectedA);
  expect(listTransfers(accountB)).toEqual(expectedB);

  const updatedA = { ...newestA, amount: '2000000' };
  saveTransfer(updatedA);
  expect(listTransfers(accountA)).toEqual([ updatedA, ...expectedA.slice(1) ]);
  expect(listTransfers(accountB)).toEqual(expectedB);
  expect(listTransfers()).toHaveLength(100);

  const newestB = transferOf(accountB, 151);
  saveTransfer(newestB);
  expect(listTransfers(accountB)).toEqual([ newestB, ...expectedB.slice(0, 49) ]);
  expect(listTransfers(accountA)).toEqual([ updatedA, ...expectedA.slice(1) ]);
  updateTransfer(newestB.id, { claimedAt: 152 });
  expect(listTransfers(accountB)[0]).toEqual({ ...newestB, claimedAt: 152 });
  expect(listTransfers(accountA)).toHaveLength(50);
  expect(listTransfers(accountB)).toHaveLength(50);
  expect(listTransfers()).toHaveLength(100);
  expect(JSON.parse(window.localStorage.getItem(storageKey)!)).toHaveLength(100);
}

test(listTransfersDropsMalformedEntries);
test(transferIdOfKeysBySourceHashAndDirection);
test(saveTransferKeepsFiftyRecordsPerAccount);
