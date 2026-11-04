import type { BridgeTransfer } from '@/lib/bridge/transfers';
import { listTransfers, saveTransfer, updateTransfer } from '@/lib/bridge/transfers';
import type { Address } from 'viem';

export type CurrentBridgeTransfer = BridgeTransfer & { bridgeSchemaVersion: 2 };

/** v2 uses source finality; settlement evidence belongs to a separate observation. */
export function migrateBridgeTransfer(transfer: BridgeTransfer): CurrentBridgeTransfer {
  return {
    ...transfer,
    requiredConfirmations: transfer.direction === 'out' ? 1 : transfer.requiredConfirmations,
    bridgeSchemaVersion: 2,
  };
}

export function listBridgeTransfers(account?: Address): Array<CurrentBridgeTransfer> {
  return listTransfers(account).map((transfer) => {
    const migrated = migrateBridgeTransfer(transfer);
    if ((transfer as Partial<CurrentBridgeTransfer>).bridgeSchemaVersion !== 2 ||
      transfer.requiredConfirmations !== migrated.requiredConfirmations) saveTransfer(migrated);
    return migrated;
  });
}

export function saveBridgeTransfer(transfer: BridgeTransfer): CurrentBridgeTransfer {
  const migrated = migrateBridgeTransfer(transfer);
  saveTransfer(migrated);
  return migrated;
}

export { updateTransfer };
