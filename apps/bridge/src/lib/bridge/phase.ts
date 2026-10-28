import type { TransferLive, TransferPhase } from '@/components/bridge/transfer-actions';
import type { TransferStep } from '@/components/trading/deposit-status';
import type { TransferRecordStatus } from '@/components/trading/transfer-history';
import { phaseOf as settlementPhaseOf } from './settlement';
import type { BridgeTransfer } from './transfers';

export function phaseOf(
  transfer: BridgeTransfer,
  live: TransferLive | undefined,
  attestationReady: boolean,
): TransferPhase {
  if (transfer.direction === 'in' && transfer.claimedAt !== undefined) return 'settled';
  return settlementPhaseOf(transfer, live, attestationReady);
}

export function stepOf(phase: TransferPhase): TransferStep {
  switch (phase) {
    case 'settled': return 'credited';
    case 'settling': return 'submitted';
    case 'confirming':
    case 'attesting':
    case 'ready':
    case 'failed': return phase;
  }
}

export function historyStatusOf(phase: TransferPhase): TransferRecordStatus {
  switch (phase) {
    case 'settled': return 'completed';
    case 'failed': return 'failed';
    case 'confirming':
    case 'attesting':
    case 'ready':
    case 'settling': return 'processing';
  }
}
