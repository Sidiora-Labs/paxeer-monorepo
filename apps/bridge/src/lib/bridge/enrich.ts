import type { TransactionReceipt } from 'viem';

import { matchDepositLog, nonceFromBridgeOutLogs } from './import';
import type { BridgeTransfer } from './transfers';

export function enrichRecord(transfer: BridgeTransfer, receipt: TransactionReceipt): BridgeTransfer {
  if (transfer.direction === 'in' && transfer.logIndex === undefined) {
    const hit = matchDepositLog(transfer, receipt.logs);
    if (hit && hit.logIndex !== null) {
      return { ...transfer, logIndex: String(hit.logIndex), nonce: String(hit.args.nonce) };
    }
  }
  if (transfer.direction === 'out' && transfer.bridgeOutNonce === undefined) {
    const nonce = nonceFromBridgeOutLogs(receipt.logs);
    if (nonce !== undefined) return { ...transfer, bridgeOutNonce: String(nonce) };
  }
  return transfer;
}
