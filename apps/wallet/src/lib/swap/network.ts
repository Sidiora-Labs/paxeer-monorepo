import { ethers } from 'ethers';
import { CONTRACTS, TOKENS, SIDIORA_POOL_REGISTRY_ABI } from './sdk';
import type { SwapToken } from './constants';

export interface SwapNetwork {
  chainId: number;
  endpoint: string;
  provider: ethers.Provider;
  indexerUrl: string;
  metadataUrl: string;
}

export type TokenCapability =
  | { chainId: number; endpoint: string; state: 'pecor' | 'unsupported' | 'undiscovered' }
  | { chainId: number; endpoint: string; state: 'launchpad'; poolAddress: string };

export interface QuoteNetwork {
  chainId: number;
  endpoint: string;
  generation: number;
  expiresAt: number;
}

let generation = 0;
let selected: SwapNetwork | undefined;
const capabilities = new Map<string, TokenCapability>();

export function setSwapNetwork(network: SwapNetwork): void {
  if (!Number.isSafeInteger(network.chainId) || network.chainId <= 0 || !network.endpoint) {
    throw new TypeError('Invalid swap network');
  }
  if (
    selected?.provider === network.provider &&
    selected.chainId === network.chainId &&
    selected.endpoint === network.endpoint &&
    selected.indexerUrl === network.indexerUrl &&
    selected.metadataUrl === network.metadataUrl
  )
    return;
  selected = Object.freeze({ ...network });
  generation++;
  capabilities.clear();
}

export function getSwapNetwork(): SwapNetwork {
  if (!selected) throw new Error('Select an active swap network before discovery or quoting');
  return selected;
}

export function quoteNetwork(expiresAt = Date.now() + 30_000): QuoteNetwork {
  const network = getSwapNetwork();
  return Object.freeze({
    chainId: network.chainId,
    endpoint: network.endpoint,
    generation,
    expiresAt,
  });
}

export function assertQuoteNetwork(source: QuoteNetwork): void {
  const network = getSwapNetwork();
  if (
    source.chainId !== network.chainId ||
    source.endpoint !== network.endpoint ||
    source.generation !== generation
  )
    throw new Error('Swap network changed; request a new quote');
  if (!Number.isSafeInteger(source.expiresAt) || source.expiresAt <= Date.now()) {
    throw new Error('Swap quote expired; request a new quote');
  }
}

export function tokenCapability(token: SwapToken): TokenCapability {
  const network = getSwapNetwork();
  const address = token.address.toLowerCase();
  const known = capabilities.get(address);
  if (known) return known;
  return { chainId: network.chainId, endpoint: network.endpoint, state: 'undiscovered' };
}

export async function resolveTokenCapability(token: SwapToken): Promise<TokenCapability> {
  const network = getSwapNetwork();
  const source = quoteNetwork();
  const cached = tokenCapability(token);
  if (cached.state !== 'undiscovered') return cached;
  const actualChain = (await network.provider.getNetwork()).chainId;
  if (actualChain !== BigInt(network.chainId)) throw new Error('Swap provider chain mismatch');
  const registry = new ethers.Contract(
    CONTRACTS.PoolRegistry,
    SIDIORA_POOL_REGISTRY_ABI,
    network.provider
  );
  // A failed registry read remains undiscovered; only a successful zero result proves absence.
  const pool: string =
    token.isNative || !token.address
      ? ethers.ZeroAddress
      : await registry.getPoolByToken(token.address);
  assertQuoteNetwork(source);
  const core =
    token.isNative ||
    !token.address ||
    Object.values(TOKENS).some((t) => t.address.toLowerCase() === token.address.toLowerCase());
  const capability: TokenCapability =
    pool !== ethers.ZeroAddress
      ? {
          chainId: network.chainId,
          endpoint: network.endpoint,
          state: 'launchpad',
          poolAddress: pool,
        }
      : {
          chainId: network.chainId,
          endpoint: network.endpoint,
          state: core ? 'pecor' : 'unsupported',
        };
  capabilities.set(token.address.toLowerCase(), Object.freeze(capability));
  return capability;
}
