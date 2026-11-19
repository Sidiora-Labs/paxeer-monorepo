import { notFound } from 'next/navigation';
import { HttpError, Operations, SdkClient } from '@kindle/sdk';
import { QueryClient } from '@tanstack/react-query';
import TokenModule, { type CoinInitialSnapshot, type CoinSnapshotFailure } from '@/page-modules/token';
import { backendConfiguration } from '@/core/backend-origins';
import { marketCachePolicy } from '@/core/cache-policy';
import { PoolNotFoundError, resolvePoolSnapshot } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import { readHoldersPage } from '@/core/market/holders-reader';
import { getPoolStreams } from '@/core/clients/livestream-api';
import { readChainRevenue } from '@/widgets/holder-rewards/chain-revenue-reader';
import { homeChainClient } from '@/widgets/trade/home-chain';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { userFacingMessage } from '@/core/user-facing-error';
import type { Address } from 'viem';

interface Props { params: Promise<{ address: string }>; searchParams: Promise<Record<string, string | string[] | undefined>>; }

export const metadata = { title: 'Coin market' };

function snapshotFailure(error: unknown): CoinSnapshotFailure {
  return error instanceof PoolNotFoundError ? { kind: 'not-found' }
    : error instanceof HttpError ? { kind: 'http', status: error.status, envelope: error.envelope, code: error.code }
      : { kind: 'error', name: error instanceof Error ? error.name : 'Error', message: error instanceof Error ? error.message : String(error) };
}

export default async function CoinPage({ params, searchParams }: Props) {
  const { address } = await params;
  if (!/^0x[0-9a-fA-F]{40}$/.test(address)) notFound();
  const poolAddress = address.toLowerCase();
  const origins = backendConfiguration().origins;
  // The root layout renders the backend configuration state.
  if (!origins) return null;
  const referringUrl = new URL(`/coin/${poolAddress}`, process.env.NEXT_PUBLIC_CANONICAL_ORIGIN || 'https://sidiora.fun');
  for (const [name, value] of Object.entries(await searchParams)) {
    for (const item of Array.isArray(value) ? value : value === undefined ? [] : [value]) referringUrl.searchParams.append(name, item);
  }
  const fetcher: typeof fetch = (input, init) => fetch(input, {
    ...init, cache: 'no-store', referrer: referringUrl.href, referrerPolicy: 'strict-origin-when-cross-origin',
  });
  const client = new SdkClient({
    apiOrigin: origins.data, identityOrigin: origins.gateway, mediaOrigin: origins.metadata,
    streamOrigin: origins.stream, approvedStreamOrigins: [origins.stream], production: process.env.NODE_ENV === 'production',
  }, fetcher);
  const operations = new Operations(client);
  const cache = new QueryClient();
  let initialSnapshot: CoinInitialSnapshot;
  try {
    const data = await cache.fetchQuery({
      queryKey: queryKeys.poolSnapshot(poolAddress),
      queryFn: ({ signal }) => resolvePoolSnapshot(poolAddress, { operations }, signal),
      ...marketCachePolicy.poolSnapshot,
    });
    initialSnapshot = { data, failure: null, updatedAt: Date.now() };
  } catch (error) {
    initialSnapshot = { data: null, failure: snapshotFailure(error), updatedAt: Date.now() };
  } finally {
    cache.clear();
  }
  if (initialSnapshot.failure?.kind !== 'not-found') {
    const data = initialSnapshot.data;
    const resolvedPool = data?.pool.poolAddress ?? poolAddress;
    const root = homeChainClient({ registryNumber: 0, chainId: 125, rpcUrl: getPaxeerRpcUrl(), walletNetwork: null });
    const statsOrigin = origins.stats;
    await Promise.all([
      readChainRevenue(root, resolvedPool as Address, undefined).then(
        revenue => { initialSnapshot.revenue = { poolAddress: resolvedPool, data: revenue, error: '' }; },
        error => { initialSnapshot.revenue = { poolAddress: resolvedPool, data: null, error: userFacingMessage(error, 'Revenue epochs could not be loaded.') }; },
      ),
      ...(data ? [
        getPoolStreams(resolvedPool).then(
          streams => { initialSnapshot.streams = { poolAddress: resolvedPool, streams }; },
          () => { initialSnapshot.streams = { poolAddress: resolvedPool, streams: null }; },
        ),
        ...(statsOrigin ? [(async () => {
          const statsClient = new SdkClient({ ...client.config, apiOrigin: statsOrigin }, fetcher);
          const queries = new QueryClient();
          try {
            const holders = await queries.fetchQuery({
              queryKey: queryKeys.holders(statsOrigin, resolvedPool, null),
              queryFn: ({ signal }) => readHoldersPage(resolvedPool, null, signal, statsClient, fetcher),
              ...marketCachePolicy.holders,
            });
            initialSnapshot.holders = { origin: statsOrigin, data: holders, failure: null };
          } catch (error) {
            initialSnapshot.holders = { origin: statsOrigin, data: null, failure: snapshotFailure(error) };
          } finally { queries.clear(); }
        })()] : []),
      ] : []),
    ]);
    initialSnapshot.updatedAt = Date.now();
  }
  return <TokenModule poolAddress={poolAddress} initialSnapshot={initialSnapshot} />;
}
