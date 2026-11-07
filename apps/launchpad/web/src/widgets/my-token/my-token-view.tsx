'use client';

import { useState, useEffect, useMemo } from 'react';
import { useAccount, useReadContract } from 'wagmi';
import { useQuery } from '@tanstack/react-query';
import { isAddress, zeroAddress, type Address } from 'viem';
import { Coins } from 'lucide-react';
import { useReadQuoterGetPoolsByCreator } from '@/core/network/contracts';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { formatAddress } from '@/utils/format';
import { metadataApiUrl } from '@/core/sdk-config';
import TxFeed from '@/widgets/trading/tx-feed';
import HoldersPanel from '@/widgets/trading/holders-panel';
import { Button, EmptyState, ErrorState, Skeleton } from '@/ui/primitives';
import { SignedOutState } from '@/ui/shared/signed-out-state';

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
  const response = await fetch(metadataApiUrl(`/metadata/${tokenAddress}`), { signal });
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
  return { ...metadata, isError: identity.isError || metadata.isError };
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
  return (
    <button
      onClick={onSelect}
      className={`px-3 py-2 rounded-lg text-2xs font-bold whitespace-nowrap transition ${
        selected
          ? 'bg-info-soft text-info'
          : 'bg-surface-2 text-text-faint hover:bg-surface-hover hover:text-text'
      }`}
    >
      {metadata.data?.symbol || formatAddress(poolAddress, 4)}
    </button>
  );
}

function CreatorPoolInfo({ creator, poolAddress }: {
  creator: Address;
  poolAddress: Address;
}) {
  const metadata = useCreatorPoolMetadata(creator, poolAddress);
  return (
    <div className="rounded-lg bg-surface-2 p-4">
      <h3 className="text-sm font-bold mb-2">Coin info</h3>
      {metadata.isError ? <p role="alert">Coin info unavailable</p> : null}
      <div className="grid grid-cols-2 gap-2 text-2xs">
        <span className="text-text-faint">Pool address</span>
        <span className="truncate">{poolAddress}</span>
        <span className="text-text-faint">Token address</span>
        <span className="truncate">{metadata.data?.tokenAddress || '...'}</span>
        <span className="text-text-faint">Name</span>
        <span>{metadata.data?.name || '...'}</span>
        <span className="text-text-faint">Ticker</span>
        <span>{metadata.data?.symbol || '...'}</span>
      </div>
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
          className="rounded-lg bg-surface-1"
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
          className="rounded-lg bg-surface-1"
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
      <h1 className="text-md font-bold">My coins</h1>
      {address ? (
        <CreatorPools key={address.toLowerCase()} creator={address} />
      ) : (
        <SignedOutState
          icon={Coins}
          title="Sign in to see your coins"
          body="Coins you launch with your wallet show up here with their holders and trades."
          className="rounded-lg bg-surface-1"
        />
      )}
    </div>
  );
}
