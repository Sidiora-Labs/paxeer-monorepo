import { expect, test } from 'vitest';

import type { TransferLive } from '@/components/bridge/transfer-actions';
import { phaseOf, settlementPatch } from './settlement';
import type { BridgeTransfer } from './transfers';

const deposit: BridgeTransfer = {
  id: '0x1234:in',
  direction: 'in',
  foreignChainKey: 'base',
  foreignChainId: '8453',
  symbol: 'USDC',
  decimals: 6,
  amount: '1000000',
  assetId: '0x0000000000000000000000000000000000000001',
  account: '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd',
  recipient: '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd',
  sourceTxHash: '0x1234',
  requiredConfirmations: 12,
  submittedAt: 1000,
};

const creditedLive: TransferLive = {
  confirmations: 12,
  requiredConfirmations: 12,
  credited: true,
  attestationReady: false,
  sourceFailed: false,
};

function phaseOfSettlesADepositThePrecompileNullified(): void {
  expect(deposit.claimTxHash).toBeUndefined();
  expect(phaseOf(deposit, creditedLive, false)).toBe('settled');
  expect(phaseOf(deposit, creditedLive, true)).toBe('settled');
  expect(phaseOf(deposit, { ...creditedLive, sourceFailed: true }, false)).toBe('settled');

  const uncredited = { ...creditedLive, credited: false };
  expect(phaseOf(deposit, uncredited, false)).toBe('attesting');
  expect(phaseOf(deposit, uncredited, true)).toBe('ready');
  expect(phaseOf(deposit, { ...uncredited, sourceFailed: true }, true)).toBe('failed');
  expect(phaseOf(deposit, { ...uncredited, confirmations: 0 }, false)).toBe('confirming');
  expect(phaseOf(deposit, undefined, false)).toBe('confirming');
  expect(phaseOf({ ...deposit, claimTxHash: '0xabcd' }, undefined, false)).toBe('settled');
  expect(phaseOf({ ...deposit, direction: 'out' }, creditedLive, false)).toBe('attesting');
  expect(phaseOf({ ...deposit, direction: 'out', releaseTxHash: '0xabcd' }, undefined, false)).toBe('settled');
}

function settlementPatchRecordsAChainObservedClaimOnce(): void {
  const now = 2000;
  const patch = settlementPatch(deposit, creditedLive, now);
  expect(patch).toEqual({ claimedAt: now });
  expect(deposit.claimedAt).toBeUndefined();
  expect(deposit.claimTxHash).toBeUndefined();

  const claimed = { ...deposit, ...patch };
  expect(claimed.claimedAt).toBe(now);
  expect(claimed.claimTxHash).toBeUndefined();
  expect(settlementPatch(claimed, creditedLive, now + 1)).toBeUndefined();
  expect(settlementPatch({ ...deposit, claimedAt: 0 }, creditedLive, now)).toBeUndefined();
  expect(settlementPatch(deposit, { ...creditedLive, credited: false }, now)).toBeUndefined();
  expect(settlementPatch(deposit, undefined, now)).toBeUndefined();
  expect(settlementPatch({ ...deposit, direction: 'out' }, creditedLive, now)).toBeUndefined();
}

test(phaseOfSettlesADepositThePrecompileNullified);
test(settlementPatchRecordsAChainObservedClaimOnce);
