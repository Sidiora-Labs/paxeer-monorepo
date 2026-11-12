import {
  createPublicClient,
  defineChain,
  encodeFunctionData,
  getAddress,
  http,
  parseAbi,
} from 'viem';
import {
  Chains,
  type LaunchedChain,
  type ChainRevenueEpoch,
} from '../../../../../sdk/launchpad/packages/sdk/src/chains.ts';
import type { MobileServices } from './service-client.ts';
import type { WalletExecutionProvider } from './lifecycle.ts';
import type { TransactionRequest } from '../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

export const launchpadAddress = getAddress(
  '0x0000000000000000000000000000000000001017',
);
const registryAddress = getAddress(
  '0x000000000000000000000000000000000000101E',
);
export const chainLaunchAbi = parseAbi([
  'function homeChainOf(address pool) view returns (uint64 chainId)',
  'function chainRevenueEpoch(uint32 chain,uint64 epoch) view returns (uint32 registryNumber,uint64 epochNumber,uint256 epochAmount,uint64 snapshotBlock,uint256 eligibleSupply,uint256 claimedTotal,uint64 openedAt)',
  'function chainRevenueClaimable(uint32 chain,uint64 epoch,address holder) view returns (uint256 amount)',
  'function claimChainRevenueBatch(uint32 chain,uint64[] epochs) returns (uint256 totalAmount)',
]);
const registryAbi = parseAbi([
  'function getChain(uint32 id) view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight)',
]);

export type ChainRoute = Readonly<{
  chainId: number;
  registryNumber: string | null;
  rpcURL: string;
}>;
export function validHomeChainId(chainId: bigint): boolean {
  return chainId === 125n || (chainId >= 125000001n && chainId <= 4419967295n);
}
export function chainRoute(homeChain: bigint, rootRPC: string): ChainRoute {
  const url = new URL(rootRPC);
  if (
    url.protocol !== 'https:' &&
    !(
      url.protocol === 'http:' &&
      ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
    )
  )
    throw new Error('Approved chain RPC required');
  if (url.username || url.password || url.search || url.hash)
    throw new Error('Invalid chain RPC URL');
  if (homeChain === 0n)
    return { chainId: 125, registryNumber: null, rpcURL: rootRPC };
  const registryNumber = homeChain - 125000000n;
  if (registryNumber < 1n || registryNumber > 4294967295n)
    throw new Error('Invalid market home chain');
  return {
    chainId: Number(homeChain),
    registryNumber: registryNumber.toString(),
    rpcURL: `${url.href.replace(/\/$/, '')}/chain/${registryNumber}`,
  };
}

export function chainClient(route: ChainRoute) {
  const chain = defineChain({
    id: route.chainId,
    name: route.registryNumber
      ? `Paxeer chain ${route.registryNumber}`
      : 'Paxeer',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [route.rpcURL] } },
  });
  return createPublicClient({
    chain,
    transport: http(route.rpcURL, { retryCount: 0, timeout: 15000 }),
  });
}

export async function homeRoute(
  services: MobileServices,
  pool: string,
  signal: AbortSignal,
): Promise<ChainRoute> {
  signal.throwIfAborted();
  const rootRPC = services.config.walletConnect?.rpcURL;
  if (!rootRPC) throw new Error('Chain RPC unavailable');
  const root = chainClient(chainRoute(0n, rootRPC));
  if ((await root.getChainId()) !== 125)
    throw new Error('Wrong root RPC chain');
  const home = await root.readContract({
    address: launchpadAddress,
    abi: chainLaunchAbi,
    functionName: 'homeChainOf',
    args: [getAddress(pool)],
  });
  signal.throwIfAborted();
  return chainRoute(home, rootRPC);
}

export type ClaimableEpoch = Readonly<
  ChainRevenueEpoch & { claimableAmount: string }
>;
export type ChainMarket = Readonly<{
  chain: LaunchedChain;
  name: string;
  route: ChainRoute;
}>;

export async function revenueEpochs(
  services: MobileServices,
  chain: LaunchedChain,
  holder: string | undefined,
  signal: AbortSignal,
): Promise<ClaimableEpoch[]> {
  const sdk = new Chains(services.sdk);
  const root = chainClient(
    chainRoute(0n, services.config.walletConnect!.rpcURL),
  );
  const epochs: ClaimableEpoch[] = [];
  let cursor: string | undefined;
  do {
    const page = await sdk.revenue(
      chain.chain,
      {
        limit: 100,
        cursor,
        ...(holder ? { holder: holder.toLowerCase() } : {}),
      },
      { signal },
    );
    if (page.data.chain !== chain.chain || page.quality !== 'complete')
      throw new Error('Current chain revenue required');
    for (const epoch of page.data.epochs) {
      const amount = holder
        ? await root.readContract({
            address: launchpadAddress,
            abi: chainLaunchAbi,
            functionName: 'chainRevenueClaimable',
            args: [
              Number(chain.chain),
              BigInt(epoch.epoch),
              getAddress(holder),
            ],
          })
        : 0n;
      signal.throwIfAborted();
      epochs.push({ ...epoch, claimableAmount: amount.toString() });
    }
    cursor = page.nextCursor ?? undefined;
  } while (cursor);
  return epochs;
}

export async function chainMarket(
  services: MobileServices,
  pool: string,
  signal: AbortSignal,
): Promise<ChainMarket | null> {
  const route = await homeRoute(services, pool, signal);
  if (!route.registryNumber) return null;
  const sdk = new Chains(services.sdk);
  const detail = await sdk.get(route.registryNumber, { signal });
  const chain = detail.data.chain;
  if (
    detail.quality !== 'complete' ||
    chain.pool.toLowerCase() !== pool.toLowerCase() ||
    chain.chain !== route.registryNumber ||
    chain.chainId !== String(route.chainId)
  )
    throw new Error('Market and chain registry mismatch');
  const root = chainClient(
    chainRoute(0n, services.config.walletConnect!.rpcURL),
  );
  const record = await root.readContract({
    address: registryAddress,
    abi: registryAbi,
    functionName: 'getChain',
    args: [Number(chain.chain)],
  });
  if (record[0] !== Number(chain.chain) || record[4] !== BigInt(route.chainId))
    throw new Error('Chain registry mismatch');
  signal.throwIfAborted();
  return { chain, name: record[1], route };
}

export type ClaimableChainPosition = Readonly<{
  chain: LaunchedChain;
  epochs: readonly ClaimableEpoch[];
  total: string;
}>;
export async function claimableChainPositions(
  services: MobileServices,
  tokens: readonly string[],
  holder: string,
  signal: AbortSignal,
): Promise<ClaimableChainPosition[]> {
  if (!services.config.walletConnect) throw new Error('Chain RPC unavailable');
  const sdk = new Chains(services.sdk);
  const owned = new Set(tokens.map((token) => getAddress(token).toLowerCase()));
  const positions: ClaimableChainPosition[] = [];
  let cursor: string | undefined;
  do {
    const page = await sdk.list({ limit: 100, cursor }, { signal });
    if (page.quality !== 'complete')
      throw new Error('Current chain list required');
    for (const chain of page.data.items) {
      if (!owned.has(chain.token.toLowerCase())) continue;
      const epochs = await revenueEpochs(services, chain, holder, signal);
      const total = epochs.reduce(
        (sum, epoch) => sum + BigInt(epoch.claimableAmount),
        0n,
      );
      if (total > 0n)
        positions.push({ chain, epochs, total: total.toString() });
    }
    cursor = page.nextCursor ?? undefined;
  } while (cursor);
  return positions;
}

export function chainRevenueClaimRequest(
  chain: string,
  epochs: readonly string[],
  holder: string,
): TransactionRequest {
  if (
    !/^[1-9][0-9]*$/.test(chain) ||
    BigInt(chain) > 4294967295n ||
    epochs.length === 0 ||
    epochs.length > 100
  )
    throw new Error('Valid chain and epochs required');
  const selected = [...new Set(epochs)];
  if (
    selected.length !== epochs.length ||
    selected.some(
      (epoch) =>
        !/^[1-9][0-9]*$/.test(epoch) || BigInt(epoch) > 18446744073709551615n,
    )
  )
    throw new Error('Valid distinct epochs required');
  return {
    chainId: '125',
    from: getAddress(holder),
    to: launchpadAddress,
    data: encodeFunctionData({
      abi: chainLaunchAbi,
      functionName: 'claimChainRevenueBatch',
      args: [Number(chain), selected.map(BigInt)],
    }),
    valueRaw: '0',
    gasLimitRaw: null,
    maxFeePerGasRaw: null,
    maxPriorityFeePerGasRaw: null,
  };
}

export async function claimChainRevenueBatch(
  wallet: WalletExecutionProvider,
  services: MobileServices,
  chain: string,
  epochs: readonly string[],
  holder: string,
  signal: AbortSignal,
) {
  if (
    !wallet.sendTransaction ||
    wallet.connectedAccount?.toLowerCase() !== holder.toLowerCase()
  )
    throw new Error('Connected claim wallet required');
  const request = chainRevenueClaimRequest(chain, epochs, holder);
  const sdk = new Chains(services.sdk);
  const detail = await sdk.get(chain, { signal });
  if (detail.quality !== 'complete' || detail.data.chain.chain !== chain)
    throw new Error('Current chain required');
  const rootRPC = services.config.walletConnect?.rpcURL;
  if (!rootRPC) throw new Error('Chain RPC unavailable');
  const root = chainClient(chainRoute(0n, rootRPC));
  if ((await root.getChainId()) !== 125)
    throw new Error('Wrong claim RPC chain');
  const amounts = await Promise.all(
    epochs.map((epoch) =>
      root.readContract({
        address: launchpadAddress,
        abi: chainLaunchAbi,
        functionName: 'chainRevenueClaimable',
        args: [Number(chain), BigInt(epoch), getAddress(holder)],
      }),
    ),
  );
  signal.throwIfAborted();
  if (amounts.some((amount) => amount <= 0n))
    throw new Error('Selected epoch has no claimable revenue');
  return wallet.sendTransaction(request, signal);
}
