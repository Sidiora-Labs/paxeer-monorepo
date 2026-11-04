import {
  PAXEER_CHAIN_ID,
  ethQuantity,
  type ExchangeModule,
  type ModuleProvider,
  type ModuleTransaction,
} from '@paxeer/wallet';
import { decodeFunctionData, parseAbi } from 'viem';
import { TOKENS } from '@/lib/constants';
import { parseSurfaceAmount, parseSurfaceId } from './useSurface';

export interface PerpsCollateral {
  readonly address: string;
  readonly decimals: number;
  readonly symbol: string;
}

export const PERPS_COLLATERAL_QUERY =
  'query WalletPerpsCollateral { collateralTokens { tokenAddress decimals isActive } }';

const COLLATERAL_CALLS = parseAbi([
  'function openPosition(uint256 marketId, address collateralToken, uint256 collateralAmount, uint256 leverage, bool isLong)',
  'function deposit(address token, uint256 amount)',
  'function withdraw(address token, uint256 amount)',
]);

export function perpsCollateralEndpoint(raw: string | undefined): string | null {
  if (!raw) return null;
  const origin = new URL(raw);
  if (
    !['http:', 'https:'].includes(origin.protocol) ||
    origin.username ||
    origin.password ||
    origin.pathname !== '/' ||
    origin.search ||
    origin.hash
  )
    throw new Error('The perps gateway must be an HTTP origin.');
  return new URL('/graphql', origin).href;
}

export async function readPerpsCollateral(
  endpoint: string | null
): Promise<readonly PerpsCollateral[]> {
  if (!endpoint) throw new Error('Perps collateral configuration is unavailable.');
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ query: PERPS_COLLATERAL_QUERY }),
    cache: 'no-store',
  });
  if (!response.ok) throw new Error('Perps collateral configuration is unavailable.');
  const answer: unknown = await response.json();
  if (
    typeof answer !== 'object' ||
    answer === null ||
    !('data' in answer) ||
    ('errors' in answer && (!Array.isArray(answer.errors) || answer.errors.length > 0))
  )
    throw new Error('Invalid perps collateral configuration.');
  const data = answer.data;
  if (
    typeof data !== 'object' ||
    data === null ||
    !('collateralTokens' in data) ||
    !Array.isArray(data.collateralTokens)
  )
    throw new Error('Invalid perps collateral configuration.');
  const seen = new Set<string>();
  const tokens: PerpsCollateral[] = [];
  for (const token of data.collateralTokens) {
    if (
      typeof token !== 'object' ||
      token === null ||
      typeof token.tokenAddress !== 'string' ||
      !/^0x[0-9a-fA-F]{40}$/.test(token.tokenAddress) ||
      BigInt(token.tokenAddress) === 0n ||
      !Number.isInteger(token.decimals) ||
      token.decimals < 0 ||
      token.decimals > 255 ||
      typeof token.isActive !== 'boolean' ||
      seen.has(token.tokenAddress.toLowerCase())
    )
      throw new Error('Invalid perps collateral configuration.');
    const address = token.tokenAddress.toLowerCase();
    seen.add(address);
    if (!token.isActive) continue;
    const known = Object.values(TOKENS).find((entry) => entry.address?.toLowerCase() === address);
    tokens.push({ address, decimals: token.decimals, symbol: known?.symbol ?? address });
  }
  if (tokens.length === 0) throw new Error('No active perps collateral is available.');
  return tokens;
}

export function requirePerpsCollateral(
  tokens: readonly PerpsCollateral[],
  address: string
): PerpsCollateral {
  const token = tokens.find((entry) => entry.address.toLowerCase() === address.toLowerCase());
  if (!token) throw new Error('Unsupported perps collateral.');
  return token;
}

export function buildPerpsPosition(
  module: ExchangeModule,
  tokens: readonly PerpsCollateral[],
  order: {
    marketId: string;
    collateralToken: string;
    collateralAmount: string;
    leverage: string;
    isLong: boolean;
  }
): { tx: ModuleTransaction; pull: bigint } {
  const token = requirePerpsCollateral(tokens, order.collateralToken);
  const pull = parseSurfaceAmount(order.collateralAmount, token.decimals);
  return {
    tx: module.openPosition({
      marketId: parseSurfaceId(order.marketId),
      collateralToken: token.address,
      collateralAmount: pull,
      leverage: parseSurfaceAmount(order.leverage, 18),
      isLong: order.isLong,
    }),
    pull,
  };
}

export async function perpsCollateralApproval(
  module: ExchangeModule,
  provider: ModuleProvider,
  endpoint: string | null,
  owner: string,
  transaction: ModuleTransaction,
  expected: PerpsCollateral
): Promise<ModuleTransaction | undefined> {
  const call = decodeFunctionData({
    abi: COLLATERAL_CALLS,
    data: transaction.data as `0x${string}`,
  });
  const address = call.functionName === 'openPosition' ? call.args[1] : call.args[0];
  const amount = call.functionName === 'openPosition' ? call.args[2] : call.args[1];
  const token = requirePerpsCollateral(await readPerpsCollateral(endpoint), address);
  if (token.decimals !== expected.decimals || token.address !== expected.address)
    throw new Error('Perps collateral configuration changed; review the amount again.');
  // Bind the configuration to the venue network before any allowance or signature request.
  if ((await ethQuantity(provider, 'eth_chainId')) !== BigInt(PAXEER_CHAIN_ID))
    throw new Error('Perps collateral is unavailable on this network.');
  if (call.functionName === 'withdraw') return undefined;
  return module.approval(owner, token.address, amount);
}
