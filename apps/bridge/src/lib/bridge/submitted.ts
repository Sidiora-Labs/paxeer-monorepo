import type { Address, Hex } from 'viem';

import type { BridgeAsset, BridgeChain } from './chains';
import { transferIdOf, type BridgeTransfer, type TransferDirection } from './transfers';

type SubmittedTransferInput = {
  hash: Hex;
  foreign: Pick<BridgeChain, 'key' | 'chainId'>;
  asset: Pick<BridgeAsset, 'symbol' | 'decimals' | 'assetId'>;
  account: Address;
  amount: bigint;
  requiredConfirmations: number;
};

function submittedRecord(direction: TransferDirection, input: SubmittedTransferInput, recipient: string): BridgeTransfer {
  return {
    id: transferIdOf(direction, input.hash),
    direction,
    foreignChainKey: input.foreign.key,
    foreignChainId: input.foreign.chainId.toString(),
    symbol: input.asset.symbol,
    decimals: input.asset.decimals,
    amount: input.amount.toString(),
    assetId: input.asset.assetId,
    account: input.account,
    recipient,
    sourceTxHash: input.hash,
    requiredConfirmations: input.requiredConfirmations,
    submittedAt: Date.now(),
  };
}

export function depositRecord(input: SubmittedTransferInput & { vault: Address; recipient: string }): BridgeTransfer {
  return {
    ...submittedRecord('in', input, input.recipient),
    vault: input.vault,
  };
}

export function withdrawalRecord(input: SubmittedTransferInput & { destination: string }): BridgeTransfer {
  return submittedRecord('out', input, input.destination);
}
