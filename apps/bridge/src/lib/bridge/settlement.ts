import type { Hex } from 'viem';
import type { TransferLive, TransferPhase } from '@/components/bridge/transfer-actions';
import type { BridgeTransfer } from './transfers';

export type SettlementStatus = 'pending' | 'dropped' | 'reverted' | 'confirmed';

export function settlementBroadcastPatch(transfer: BridgeTransfer, hash: Hex, now: number): Partial<BridgeTransfer> {
  return {
    ...(transfer.direction === 'in' ? { claimTxHash: hash } : { releaseTxHash: hash }),
    claimedAt: undefined,
    releasedAt: undefined,
    settlementSubmittedAt: now,
    settlementStatus: 'pending',
  };
}

export function phaseOf(
  transfer: BridgeTransfer,
  live: TransferLive | undefined,
  attestationReady: boolean,
): TransferPhase {
  if (transfer.direction === 'in' ? live?.credited : live?.released) return 'settled';
  if (live?.sourceFailed) return 'failed';
  const status = live?.settlementStatus ?? transfer.settlementStatus;
  if ((transfer.claimTxHash || transfer.releaseTxHash) && (status === undefined || status === 'pending')) return 'settling';
  if (attestationReady) return 'ready';
  if (live && live.confirmations > 0 && live.confirmations >= live.requiredConfirmations) return 'attesting';
  return 'confirming';
}

export function settlementPatch(
  transfer: BridgeTransfer,
  live: Pick<TransferLive, 'credited'> & Partial<Pick<TransferLive, 'released' | 'settlementStatus'>> | undefined,
  now: number,
): Partial<BridgeTransfer> | undefined {
  if (!live) return undefined;
  const patch: Partial<BridgeTransfer> = {};
  const complete = transfer.direction === 'in' ? live.credited : live.released === true;
  if (complete) {
    if (transfer.direction === 'in' && transfer.claimedAt === undefined) patch.claimedAt = now;
    if (transfer.direction === 'out' && transfer.releasedAt === undefined) patch.releasedAt = now;
  } else {
    if (transfer.claimedAt !== undefined) patch.claimedAt = undefined;
    if (transfer.releasedAt !== undefined) patch.releasedAt = undefined;
  }
  const status = complete ? 'confirmed' : live.settlementStatus;
  if (status !== undefined && status !== transfer.settlementStatus) patch.settlementStatus = status;
  return Object.keys(patch).length ? patch : undefined;
}
