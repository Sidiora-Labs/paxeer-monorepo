import { BaseError, ContractFunctionRevertedError, formatEther, parseAbi } from 'viem';
import type { PublicClient } from 'viem';

import type { ChainRecord } from './model';

export const launchpadAddress = '0x0000000000000000000000000000000000001017' as const;

export const chainLaunchAbi = parseAbi([
  'function launchedChain(uint32 chain) view returns (uint32 registryNumber, uint64 chainId, address pool, address token, uint256 nftId, address creator, uint64 blockNumber)',
  'function chainRevenueEpoch(uint32 chain, uint64 epoch) view returns (uint32 registryNumber, uint64 epochNumber, uint256 epochAmount, uint64 snapshotBlock, uint256 eligibleSupply, uint256 claimedTotal, uint64 openedAt)',
  'error NotLaunchedChain()',
]);

const tokenAbi = parseAbi([ 'function symbol() view returns (string)' ]);
const maxEpoch = (BigInt(1) << BigInt(64)) - BigInt(1);

export type RevenueEpochView = {
  number: bigint;
  amountPax: string;
  snapshotBlock: bigint;
};

export type ChainLaunchView = {
  token: `0x${ string }`;
  tokenSymbol: string;
  pool: `0x${ string }`;
  feeRightId: bigint;
  homeChainId: bigint;
  launchBlock: bigint;
  lastEpoch: RevenueEpochView | null;
};

export type LaunchReadResult = readonly [number, bigint, `0x${ string }`, `0x${ string }`, bigint, `0x${ string }`, bigint];
export type EpochReadResult = readonly [number, bigint, bigint, bigint, bigint, bigint, bigint];

export function isAbsentLaunch(error: unknown): boolean {
  if (!(error instanceof BaseError)) return false;
  const revert = error.walk((cause) => cause instanceof ContractFunctionRevertedError);
  return revert instanceof ContractFunctionRevertedError && revert.data?.errorName === 'NotLaunchedChain';
}

export function mapRevenueEpoch(chain: ChainRecord, number: bigint, result: EpochReadResult): RevenueEpochView {
  const [ epochChain, epochNumber, amount, snapshotBlock ] = result;
  if (epochChain !== chain.id || epochNumber !== number) {
    throw new Error('Revenue epoch belongs to a different virtual chain');
  }
  return { number: epochNumber, amountPax: formatEther(amount), snapshotBlock };
}

export function mapChainLaunch(
  chain: ChainRecord,
  result: LaunchReadResult,
  tokenSymbol: string,
  lastEpoch: RevenueEpochView | null,
): ChainLaunchView {
  const [ registryNumber, homeChainId, pool, token, feeRightId, , launchBlock ] = result;
  if (registryNumber !== chain.id || homeChainId !== BigInt(chain.chain_id)) {
    throw new Error('Launch belongs to a different virtual chain');
  }
  return { token, tokenSymbol, pool, feeRightId, homeChainId, launchBlock, lastEpoch };
}

export async function readChainLaunch(chain: ChainRecord, client: PublicClient): Promise<ChainLaunchView | null> {
  let launch;
  try {
    launch = await client.readContract({
      address: launchpadAddress,
      abi: chainLaunchAbi,
      functionName: 'launchedChain',
      args: [ chain.id ],
    });
  } catch (error) {
    if (isAbsentLaunch(error)) return null;
    throw error;
  }

  const [ , , , token ] = launch;
  mapChainLaunch(chain, launch, '', null);

  async function epoch(number: bigint): Promise<RevenueEpochView | null> {
    try {
      const result = await client.readContract({
        address: launchpadAddress,
        abi: chainLaunchAbi,
        functionName: 'chainRevenueEpoch',
        args: [ chain.id, number ],
      });
      return mapRevenueEpoch(chain, number, result);
    } catch (error) {
      if (isAbsentLaunch(error)) return null;
      throw error;
    }
  }

  const tokenSymbol = await client.readContract({ address: token, abi: tokenAbi, functionName: 'symbol' });
  let lower = BigInt(0);
  let upper = BigInt(1);
  let latest: RevenueEpochView | null = null;
  while (true) {
    const found = await epoch(upper);
    if (!found) break;
    latest = found;
    lower = upper;
    if (upper === maxEpoch) break;
    upper = upper > maxEpoch / BigInt(2) ? maxEpoch : upper * BigInt(2);
  }
  if (upper !== lower) {
    while (lower + BigInt(1) < upper) {
      const middle = lower + (upper - lower) / BigInt(2);
      const found = await epoch(middle);
      if (found) {
        latest = found;
        lower = middle;
      } else {
        upper = middle;
      }
    }
  }

  return mapChainLaunch(chain, launch, tokenSymbol, latest);
}
