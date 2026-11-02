import type { TransferLive, TransferPhase } from '@/components/bridge/transfer-actions';
import type { BridgeTransfer } from './transfers';

export function phaseOf(
  transfer: BridgeTransfer,
  live: TransferLive | undefined,
  attestationReady: boolean,
): TransferPhase {
  if (transfer.direction === 'in' && (transfer.claimTxHash || live?.credited)) return 'settled';
  if (transfer.direction === 'out' && transfer.releaseTxHash) return 'settled';
  if (live?.sourceFailed) return 'failed';
  if (attestationReady) return 'ready';
  if (live && live.confirmations > 0 && live.confirmations >= live.requiredConfirmations) return 'attesting';
  return 'confirming';
}

export function settlementPatch(
  transfer: BridgeTransfer,
  live: Pick<TransferLive, 'credited'> | undefined,
  now: number,
): Pick<BridgeTransfer, 'claimedAt'> | undefined {
  if (transfer.direction === 'in' && live?.credited && transfer.claimedAt === undefined) {
    return { claimedAt: now };
  }
  return undefined;
}
