import type { TransactionReceipt } from 'viem';

import { matchDepositLog, nonceFromBridgeOutLogs } from './import';
import { transferIdOf, type BridgeTransfer } from './transfers';

export function enrichRecord(transfer: BridgeTransfer, receipt: TransactionReceipt): BridgeTransfer {
  if (receipt.status !== 'success' || receipt.transactionHash.toLowerCase() !== transfer.sourceTxHash.toLowerCase()) {
    return transfer;
  }
  if (transfer.direction === 'in' && (transfer.logIndex === undefined || transfer.nonce === undefined)) {
    const hit = matchDepositLog(transfer, receipt.logs);
    if (hit && hit.logIndex !== null) {
      const logIndex = String(hit.logIndex);
      return {
        ...transfer,
        id: transferIdOf('in', transfer.sourceTxHash, transfer.foreignChainId, logIndex),
        logIndex,
        nonce: String(hit.args.nonce),
      };
    }
  }
  if (transfer.direction === 'out' && transfer.bridgeOutNonce === undefined) {
    const nonce = nonceFromBridgeOutLogs(receipt.logs);
    if (nonce !== undefined) {
      const bridgeOutNonce = String(nonce);
      return {
        ...transfer,
        id: transferIdOf('out', transfer.sourceTxHash, transfer.foreignChainId, bridgeOutNonce),
        bridgeOutNonce,
      };
    }
  }
  return transfer;
}
