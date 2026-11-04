// @vitest-environment jsdom

import { expect, test } from 'vitest';

import type { TransferLive } from '@/components/bridge/transfer-actions';
import { phaseOf, settlementPatch } from './settlement';
import type { BridgeTransfer } from './transfers';
import { openBridgeRepairFixture } from './fixtures/repair-22-33-265';

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
  released: false,
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
  expect(phaseOf({ ...deposit, claimTxHash: '0xabcd' }, undefined, false)).toBe('settling');
  expect(phaseOf({ ...deposit, direction: 'out' }, creditedLive, false)).toBe('attesting');
  expect(phaseOf({ ...deposit, direction: 'out', releaseTxHash: '0xabcd' }, undefined, false)).toBe('settling');
}

function settlementPatchRecordsAChainObservedClaimOnce(): void {
  const now = 2000;
  const patch: ReturnType<typeof settlementPatch> = settlementPatch(deposit, creditedLive, now);
  expect(patch).toEqual({ claimedAt: now, settlementStatus: 'confirmed' });
  expect(deposit.claimedAt).toBeUndefined();
  expect(deposit.claimTxHash).toBeUndefined();

  const claimed = { ...deposit, ...patch };
  expect(claimed.claimedAt).toBe(now);
  expect(claimed.claimTxHash).toBeUndefined();
  expect(settlementPatch(claimed, creditedLive, now + 1)).toBeUndefined();
  expect(settlementPatch({ ...deposit, claimedAt: 0 }, creditedLive, now)).toEqual({ settlementStatus: 'confirmed' });
  expect(settlementPatch({ ...claimed, claimedAt: 0 }, creditedLive, now)).toBeUndefined();
  expect(settlementPatch(deposit, { ...creditedLive, credited: false }, now)).toBeUndefined();
  expect(settlementPatch(deposit, undefined, now)).toBeUndefined();
  expect(settlementPatch({ ...deposit, direction: 'out' }, creditedLive, now)).toBeUndefined();
}

async function settlementBroadcastReopensRetriesAndRequiresDestinationEvidence(): Promise<void> {
  const fixture = await openBridgeRepairFixture();
  try {
    for (const direction of [ 'in', 'out' ] as const) {
      const submitted = await fixture.submit(direction);
      let transfer = await fixture.reopen(submitted.id);
      const hash = await fixture.signSettlement(transfer);
      fixture.ledger.updateTransfer(transfer.id, fixture.settlement.settlementBroadcastPatch(transfer, hash, Date.now()));
      transfer = await fixture.reopen(transfer.id);
      expect(transfer.claimedAt).toBeUndefined();
      expect(transfer.releasedAt).toBeUndefined();
      expect(phaseOf(transfer, undefined, true)).toBe('settling');

      let live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('pending');
      expect(phaseOf(transfer, live, true)).toBe('settling');
      expect(fixture.actions.settledRefusal(transfer, live)).toBeUndefined();
      const readsBeforeReopen = fixture.requests.length;
      transfer = await fixture.reopen(transfer.id);
      live = await fixture.observe(transfer);
      expect(fixture.requests.slice(readsBeforeReopen).some((request) =>
        request.method === 'eth_getTransactionReceipt' && request.params[0] === hash)).toBe(true);
      expect(live.settlementStatus).toBe('pending');
      expect(phaseOf(transfer, live, true)).toBe('settling');

      fixture.ledger.updateTransfer(transfer.id, { settlementSubmittedAt: Date.now() - 130_000 });
      transfer = await fixture.reopen(transfer.id);
      live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('dropped');
      expect(phaseOf(transfer, live, true)).toBe('ready');
      expect(phaseOf(transfer, live, false)).toBe('attesting');
      expect(fixture.actions.settledRefusal(transfer, live)).toBeUndefined();
      transfer = await fixture.reopen(transfer.id);
      expect(transfer.settlementStatus).toBe('dropped');
      expect(phaseOf(transfer, undefined, true)).toBe('ready');

      fixture.setReceipt(hash, direction, 'reverted');
      live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('reverted');
      transfer = await fixture.reopen(transfer.id);
      expect(transfer.settlementStatus).toBe('reverted');
      expect(transfer.claimedAt).toBeUndefined();
      expect(transfer.releasedAt).toBeUndefined();
      expect(phaseOf(transfer, undefined, true)).toBe('ready');
      expect(fixture.actions.settledRefusal(transfer, live)).toBeUndefined();

      const replacement = await fixture.signSettlement(transfer, 4n);
      expect(replacement).not.toBe(hash);
      fixture.ledger.updateTransfer(transfer.id, fixture.settlement.settlementBroadcastPatch(transfer, replacement, Date.now()));
      fixture.setReceipt(replacement, direction, 'success');
      transfer = await fixture.reopen(transfer.id);
      live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('pending');
      expect(live.credited).toBe(false);
      expect(live.released).toBe(false);
      expect(phaseOf(transfer, live, true)).toBe('settling');
      expect(fixture.actions.settledRefusal(transfer, live)).toBeUndefined();
      expect((await fixture.reopen(transfer.id)).claimedAt).toBeUndefined();
      expect((await fixture.reopen(transfer.id)).releasedAt).toBeUndefined();

      fixture.confirm(transfer);
      live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('confirmed');
      expect(live.credited).toBe(direction === 'in');
      expect(live.released).toBe(direction === 'out');
      expect(phaseOf(transfer, live, true)).toBe('settled');
      transfer = await fixture.reopen(transfer.id);
      const confirmedAt = direction === 'in' ? transfer.claimedAt : transfer.releasedAt;
      expect(confirmedAt).toEqual(expect.any(Number));
      expect(settlementPatch(transfer, live, Date.now() + 1)).toBeUndefined();
      expect(fixture.actions.settledRefusal(transfer, live)).toBe(direction === 'in' ?
        'This deposit is already credited on Paxeer.' : 'This withdrawal is already released from the vault.');

      // A wallet replacement can settle without the originally recorded hash surviving.
      fixture.receipts.delete(hash);
      fixture.ledger.updateTransfer(transfer.id, direction === 'in' ? { claimTxHash: hash } : { releaseTxHash: hash });
      transfer = await fixture.reopen(transfer.id);
      live = await fixture.observe(transfer);
      expect(live.settlementStatus).toBe('confirmed');
      expect(phaseOf(transfer, live, true)).toBe('settled');
      expect(direction === 'in' ? (await fixture.reopen(transfer.id)).claimedAt :
        (await fixture.reopen(transfer.id)).releasedAt).toBe(confirmedAt);

      // Legacy optimistic timestamps cannot substitute for fresh destination evidence.
      fixture.clearConfirmation(transfer);
      fixture.ledger.updateTransfer(transfer.id, {
        claimedAt: 1, releasedAt: 1, settlementStatus: undefined, settlementSubmittedAt: Date.now() - 130_000,
      });
      transfer = await fixture.reopen(transfer.id);
      live = await fixture.observe(transfer);
      expect(phaseOf(transfer, live, true)).toBe('ready');
      transfer = await fixture.reopen(transfer.id);
      expect(transfer.claimedAt).toBeUndefined();
      expect(transfer.releasedAt).toBeUndefined();
      expect(fixture.actions.settledRefusal(transfer, live)).toBeUndefined();
    }
  } finally {
    await fixture.close();
  }
}

test(phaseOfSettlesADepositThePrecompileNullified.name, phaseOfSettlesADepositThePrecompileNullified);
test(settlementPatchRecordsAChainObservedClaimOnce.name, settlementPatchRecordsAChainObservedClaimOnce);
test(settlementBroadcastReopensRetriesAndRequiresDestinationEvidence.name, settlementBroadcastReopensRetriesAndRequiresDestinationEvidence);
