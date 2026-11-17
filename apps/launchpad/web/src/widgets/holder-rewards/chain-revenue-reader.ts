import type { Address, PublicClient } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';

export interface ChainRevenueEpoch {
  epoch: bigint;
  amount: bigint;
  claimable: bigint | null;
  openedAt: bigint;
  claimed: boolean;
}

export async function readChainRevenue(root: PublicClient, pool: Address, holder: Address | undefined): Promise<{ chain: number; epochs: ChainRevenueEpoch[] }> {
  const chain = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'launchedChainOfPool', args: [pool] });
  if (chain === 0) return { chain: 0, epochs: [] };
  const epochs: ChainRevenueEpoch[] = [];
  for (let epoch = 1n; ; epoch++) {
    let record: readonly [number, bigint, bigint, bigint, bigint, bigint, bigint];
    try {
      record = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'chainRevenueEpoch', args: [chain, epoch] });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (/NotLaunchedChain/i.test(message)) break;
      throw error;
    }
    let isClaimed = false;
    let claimable: bigint | null = null;
    if (holder) {
      try {
        claimable = await root.readContract({ address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'chainRevenueClaimable', args: [chain, epoch, holder] });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (/AlreadyClaimed/i.test(message)) { isClaimed = true; claimable = 0n; }
        else if (/ZeroAmount/i.test(message)) claimable = 0n;
        else throw error;
      }
    }
    epochs.push({ epoch, amount: record[2], claimable, openedAt: record[6], claimed: isClaimed });
  }
  return { chain, epochs };
}

export interface InitialChainRevenue {
  poolAddress: string;
  data: { chain: number; epochs: ChainRevenueEpoch[] } | null;
  error: string;
}
