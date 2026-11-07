import { PAXEER_CONFIG } from './constants';
import { isVirtualChainId, VIRTUAL_CHAIN_ID_BASE } from './networks';
import { preferencesRepository } from '@/platform/storage/repositories';

export interface WalletReadOptions {
  fresh?: boolean;
  chainId?: number;
}

export const WALLET_CHAIN_HEADER = 'x-wallet-chain-id';
export const WALLET_CACHE_HEADER = 'x-wallet-cache-mode';

export function validateWalletChainId(chainId: number): number {
  if (chainId !== PAXEER_CONFIG.chainId && !isVirtualChainId(chainId)) {
    throw new TypeError('Wallet read chain identity is invalid');
  }
  return chainId;
}

export function selectedWalletChainId(): number {
  if (typeof window === 'undefined') return validateWalletChainId(PAXEER_CONFIG.chainId);
  const preferences = preferencesRepository.read();
  const chainId = preferences.activeChainId ?? PAXEER_CONFIG.chainId;
  validateWalletChainId(chainId);
  if (chainId !== PAXEER_CONFIG.chainId) {
    const network = preferences.networks.find((entry) => entry.chainId === chainId);
    const expectedPath = `/chain/${chainId - VIRTUAL_CHAIN_ID_BASE}`;
    if (
      !network ||
      network.kind !== 'virtual' ||
      new URL(network.rpcUrl, 'https://wallet.invalid').pathname.replace(/\/+$/, '') !==
        new URL(PAXEER_CONFIG.rpcUrl || '/', 'https://wallet.invalid').pathname.replace(
          /\/+$/,
          ''
        ) +
          expectedPath
    ) {
      throw new TypeError('Selected wallet chain does not match its RPC identity');
    }
  }
  return chainId;
}

export function walletReadUrl(input: string, options: WalletReadOptions = {}): string {
  const chainId = validateWalletChainId(options.chainId ?? selectedWalletChainId());
  const absolute = /^[a-z][a-z\d+.-]*:/i.test(input);
  const url = new URL(
    input,
    typeof window === 'undefined' ? 'http://wallet.invalid' : window.location.origin
  );
  if (url.searchParams.has('chain_id') || url.searchParams.has('fresh')) {
    throw new TypeError('Wallet read identity and freshness are reserved parameters');
  }
  url.searchParams.set('chain_id', String(chainId));
  if (options.fresh) url.searchParams.set('fresh', 'true');
  return absolute || typeof window !== 'undefined'
    ? url.toString()
    : `${url.pathname}${url.search}`;
}

export interface WalletReadRequest {
  chainId: number;
  fresh: boolean;
  upstreamPath: string;
  cacheKey: string;
}

/** The wallet BFF must consume this contract before consulting its response or in-flight caches. */
export function parseWalletReadRequest(url: URL, method = 'GET'): WalletReadRequest {
  if (!['GET', 'HEAD'].includes(method)) throw new TypeError('Wallet reads are read-only');
  if (
    url.searchParams.getAll('chain_id').length !== 1 ||
    !/^[1-9]\d{0,9}$/.test(url.searchParams.get('chain_id') ?? '')
  ) {
    throw new TypeError('Wallet read requires one canonical chain identity');
  }
  const chainId = validateWalletChainId(Number(url.searchParams.get('chain_id')));
  const freshValues = url.searchParams.getAll('fresh');
  if (freshValues.length > 1 || (freshValues.length === 1 && freshValues[0] !== 'true')) {
    throw new TypeError('Wallet cache bypass is invalid');
  }
  const query = new URLSearchParams(url.searchParams);
  query.delete('chain_id');
  query.delete('fresh');
  const path = url.pathname.replace(/^\/wallet\/api\/wallet(?=\/)/, '');
  if (!/^(?:\/health$|\/api\/v[12]\/)/.test(path))
    throw new TypeError('Wallet read path is invalid');
  const tenantPath =
    chainId === PAXEER_CONFIG.chainId ? '' : `/chain/${chainId - VIRTUAL_CHAIN_ID_BASE}`;
  const upstreamPath = `${tenantPath}${path}${query.size ? `?${query}` : ''}`;
  return {
    chainId,
    fresh: freshValues.length === 1,
    upstreamPath,
    cacheKey: `${method} ${chainId} ${upstreamPath}`,
  };
}

export function walletReadResponseHeaders(request: WalletReadRequest): Record<string, string> {
  return {
    [WALLET_CHAIN_HEADER]: String(request.chainId),
    [WALLET_CACHE_HEADER]: request.fresh ? 'bypass' : 'scoped',
    'Cache-Control': 'no-store',
  };
}

export function assertWalletReadResponse(response: Response, requestUrl: string): void {
  const url = new URL(requestUrl, 'http://wallet.invalid');
  if (response.headers.get(WALLET_CHAIN_HEADER) !== url.searchParams.get('chain_id')) {
    throw new Error('Wallet BFF did not acknowledge the requested chain identity');
  }
  if (
    url.searchParams.get('fresh') === 'true' &&
    response.headers.get(WALLET_CACHE_HEADER) !== 'bypass'
  ) {
    throw new Error('Wallet BFF did not acknowledge cache bypass');
  }
}
