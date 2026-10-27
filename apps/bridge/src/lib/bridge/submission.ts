import type { Address } from 'viem';

import type { BridgeAsset } from './chains';
import { erc20Abi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';

export function planDeposit({ asset, amount, allowance, vault, recipient }: {
  asset: Pick<BridgeAsset, 'address' | 'isNative'>;
  amount: bigint;
  allowance: bigint;
  vault: Address;
  recipient: Address;
}) {
  const recipient32 = paxeerRecipient(recipient);
  if (asset.isNative) {
    return [ {
      abi: paxeerXVaultAbi,
      address: vault,
      functionName: 'depositNative',
      args: [ recipient32 ],
      value: amount,
    } ] as const;
  }

  const deposit = {
    abi: paxeerXVaultAbi,
    address: vault,
    functionName: 'deposit',
    args: [ asset.address, amount, recipient32 ],
  } as const;

  if (allowance >= amount) return [ deposit ] as const;

  return [ {
    abi: erc20Abi,
    address: asset.address,
    functionName: 'approve',
    args: [ vault, amount ],
  }, deposit ] as const;
}
