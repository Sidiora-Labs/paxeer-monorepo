import type { Address, Hex } from 'viem';

import type { BridgeAsset, BridgeChain } from './chains';
import { solanaRecipientHandle } from './solana';
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
    id: transferIdOf(direction, input.hash, input.foreign.chainId.toString()),
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

export type DepositRecordInput = SubmittedTransferInput & { vault: Address; recipient: string };
export type WithdrawalRecordInput = SubmittedTransferInput & { destination: string };

export function depositRecord(input: DepositRecordInput): BridgeTransfer {
  return {
    ...submittedRecord('in', input, input.recipient),
    vault: input.vault,
  };
}

export function withdrawalRecord(input: WithdrawalRecordInput): BridgeTransfer {
  const transfer = submittedRecord('out', input, input.destination);
  if (input.foreign.key !== 'solana') return transfer;
  const recipientHandle = solanaRecipientHandle(input.destination);
  if (!recipientHandle) throw new Error('Enter a valid Solana recipient.');
  return { ...transfer, recipientHandle };
}
