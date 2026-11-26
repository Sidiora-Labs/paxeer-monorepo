'use client';

import { displayCoinText } from '@/utils/coin-text';
import { useState, useEffect, useMemo } from 'react';
import { useAccount, useReadContract } from 'wagmi';
import { useQuery } from '@tanstack/react-query';
import { isAddress, zeroAddress, type Address } from 'viem';
import { Coins } from 'lucide-react';
import { useReadQuoterGetPoolsByCreator } from '@/core/network/contracts';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { metadataApiUrl } from '@/core/sdk-config';
import TxFeed from '@/widgets/trading/tx-feed';
import HoldersPanel from '@/widgets/trading/holders-panel';
import { Button, EmptyState, ErrorState, Skeleton } from '@/ui/primitives';
import { SignedOutState } from '@/ui/shared/signed-out-state';
import { poolPath } from '@/core/app-foundation/navigation';

interface PoolMeta {
  poolAddress: Address;
  tokenAddress: Address;
  name: string | null;
  symbol: string | null;
}

const poolIdentityAbi = [
  {
    type: 'function',
    name: 'getPoolMetadata',
    stateMutability: 'view',
    inputs: [{ name: 'pool', type: 'address' }],
    outputs: [
      {
        name: '',
        type: 'tuple',
        components: [
          { name: 'creator', type: 'address' },
          { name: 'token', type: 'address' },
          { name: 'optical', type: 'address' },
          { name: 'nftId', type: 'uint256' },
          { name: 'createdAt', type: 'uint256' },
          { name: 'createdBlock', type: 'uint256' },
        ],
      },
    ],
  },
] as const;

export async function fetchCreatorPoolMetadata(
  poolAddress: Address,
  tokenAddress: Address,
  signal?: AbortSignal,
): Promise<PoolMeta> {
  if (tokenAddress === zeroAddress || tokenAddress.toLowerCase() === poolAddress.toLowerCase()) {
    throw new Error('Pool token identity is unavailable');
  }
  const response = await fetch(metadataApiUrl(`/metadata/${tokenAddress}`), { signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(15000)]) : AbortSignal.timeout(15000) });
  if (!response.ok) throw new Error('Coin metadata is unavailable');
  const data: unknown = await response.json();
  if (
    !data ||
    typeof data !== 'object' ||
    !('token_address' in data) ||
    typeof data.token_address !== 'string' ||
    data.token_address.toLowerCase() !== tokenAddress.toLowerCase() ||
    !('pool_address' in data) ||
    typeof data.pool_address !== 'string' ||
    data.pool_address.toLowerCase() !== poolAddress.toLowerCase() ||
    !('name' in data) ||
    (data.name !== null && typeof data.name !== 'string') ||
    !('symbol' in data) ||
    (data.symbol !== null && typeof data.symbol !== 'string')
  ) {
    throw new Error('Coin metadata does not identify this pool and token');
  }
  return { poolAddress, tokenAddress, name: data.name, symbol: data.symbol };
}

function useCreatorPoolMetadata(creator: Address, poolAddress: Address) {
  const identity = useReadContract({
    chainId: CORE_DEPLOYMENT.chainId,
    address: CORE_DEPLOYMENT.poolRegistry,
    abi: poolIdentityAbi,
    functionName: 'getPoolMetadata',
    args: [poolAddress],
  });
  const tokenAddress = identity.data?.token;
  const metadata = useQuery({
    queryKey: [
      'creator-pool-metadata', CORE_DEPLOYMENT.chainId, creator, poolAddress, tokenAddress,
    ],
    enabled: tokenAddress !== undefined,
    queryFn: ({ signal }) => {
      if (!tokenAddress) throw new Error('Pool token identity is unavailable');
      return fetchCreatorPoolMetadata(poolAddress, tokenAddress, signal);
    },
    retry: false,
  });
  return {
    ...metadata,
    isPending: identity.isPending || metadata.isPending,
    isError: identity.isError || metadata.isError,
    retry: async () => {
      if (identity.isError) await identity.refetch();
      else await metadata.refetch();
    },
  };
}

function CreatorPoolButton({
  creator,
  poolAddress,
  selected,
  onSelect,
}: {
  creator: Address;
  poolAddress: Address;
  selected: boolean;
  onSelect: () => void;
}) {
  const metadata = useCreatorPoolMetadata(creator, poolAddress);
  const symbol = metadata.data ? displayCoinText(metadata.data.symbol, metadata.data.tokenAddress) : poolAddress;
  return (
    <button
      type="button"
      onClick={onSelect}
      title={symbol}
      className={`min-h-touch-target-min min-w-0 max-w-48 truncate rounded-lg px-3 py-2 text-2xs font-semibold transition ${
        selected
          ? 'bg-surface-3 text-text'
          : 'bg-surface-2 text-text-faint hover:bg-surface-hover hover:text-text'
      }`}
    >
      {symbol}
    </button>
  );
}

export function CreatorPoolInfo({ creator, poolAddress }: {
  creator: Address;
  poolAddress: Address;
}) {
  const metadata = useCreatorPoolMetadata(creator, poolAddress);
  const name = metadata.data ? displayCoinText(metadata.data.name, metadata.data.tokenAddress) : '—';
  const symbol = metadata.data ? displayCoinText(metadata.data.symbol, metadata.data.tokenAddress) : '—';
  return (
    <div className="rounded-lg bg-surface-2 p-4">
      <h3 className="text-sm font-semibold mb-2">Coin info</h3>
      {metadata.isError ? <p role="alert">Coin info unavailable</p> : null}
      {metadata.isError ? <Button variant="link" size="sm" onClick={() => void metadata.retry()}>Try again</Button> : metadata.isPending ? <p role="status">Loading coin info...</p> : null}
      <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-2 text-2xs [&>span:nth-child(even)]:text-right [&>span:nth-child(even)]:tabular-nums">
        <span className="text-text-faint">Pool address</span>
        <span className="truncate">{poolAddress}</span>
        <span className="text-text-faint">Token address</span>
        <span className="truncate">{metadata.data?.tokenAddress ?? '—'}</span>
        <span className="text-text-faint">Name</span>
        <span className="min-w-0 truncate" title={name}>{name}</span>
        <span className="text-text-faint">Ticker</span>
        <span className="min-w-0 truncate" title={symbol}>{symbol}</span>
      </div>
      <Button variant="link" size="sm" href={poolPath(125, poolAddress)}>View coin</Button>
      {metadata.data ? <Button variant="link" size="sm" href={`/tokens/${metadata.data.tokenAddress}/metadata`}>Edit coin details</Button> : null}
    </div>
  );
}

function CreatorPools({ creator }: { creator: Address }) {
  const [selectedPool, setSelectedPool] = useState<Address | null>(null);

  const {
    data: pools,
    isPending,
    isError,
    error,
    refetch,
  } = useReadQuoterGetPoolsByCreator({ creator });

  const poolList = useMemo<Address[]>(
    () =>
      Array.isArray(pools)
        ? pools.filter((pool): pool is Address => typeof pool === 'string' && isAddress(pool))
        : [],
    [pools],
  );

  const activePool =
    selectedPool && poolList.includes(selectedPool)
      ? selectedPool
      : poolList[0] ?? null;

  useEffect(() => {
    if (selectedPool !== activePool) setSelectedPool(activePool);
  }, [activePool, selectedPool]);

  return (
    <>
      {isPending ? (
        <Skeleton
          variant="rows"
          rows={3}
          label="your coins"
          className="rounded-lg bg-surface-1 p-2"
        />
      ) : isError ? (
        <ErrorState
          title="Your coins can't load right now"
          message="Something went wrong, try again"
          onRetry={() => {
            void refetch();
          }}
          className="bg-canvas"
        />
      ) : poolList.length === 0 ? (
        <EmptyState
          icon={Coins}
          title="No coins created yet"
          body="Launch a coin and it shows up here with its holders and trades."
          action={
            <Button href="/create" variant="primary" size="sm">
              Create coin
            </Button>
          }
          className="bg-canvas"
        />
      ) : (
        <>
          <div className="flex gap-2 overflow-x-auto scrollbar-none pb-2">
            {poolList.map((addr) => (
              <CreatorPoolButton
                key={addr}
                creator={creator}
                poolAddress={addr}
                selected={activePool === addr}
                onSelect={() => setSelectedPool(addr)}
              />
            ))}
          </div>

          {activePool && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <div className="space-y-4">
                <CreatorPoolInfo creator={creator} poolAddress={activePool} />
                <HoldersPanel poolAddress={activePool} />
              </div>
              <TxFeed poolAddress={activePool} />
            </div>
          )}
        </>
      )}
    </>
  );
}

export default function MyTokenView() {
  const { address } = useAccount();
  return (
    <div className="p-6 text-text space-y-4">
      <h1 className="text-md font-semibold">My coins</h1>
      {address ? (
        <CreatorPools key={address.toLowerCase()} creator={address} />
      ) : (
        <SignedOutState
          icon={Coins}
          title="Sign in to see your coins"
          body="Coins you launch with your wallet show up here with their holders and trades."
          className="bg-canvas"
        />
      )}
    </div>
  );
}
