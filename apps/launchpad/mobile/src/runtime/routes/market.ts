import { type SdkClient } from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export function pools(sdk: SdkClient, signal: AbortSignal) {
  return sdk.request('/v1/pools?limit=50', 'PoolListResponse', {
    signal,
    idempotentRead: true,
  });
}

export function pool(sdk: SdkClient, address: string, signal: AbortSignal) {
  return sdk.request(`/v1/pools/${evmAddress(address)}`, 'PoolDetailResponse', {
    signal,
    idempotentRead: true,
  });
}

export function search(sdk: SdkClient, query: string, signal: AbortSignal) {
  return sdk.request(
    `/v1/search?type=pool&limit=50&q=${encodeURIComponent(query)}`,
    'SearchResponse',
    { signal, idempotentRead: true },
  );
}

export const marketRoutes = { pools, pool, search };

function evmAddress(value: string) {
  if (!/^0x[0-9a-fA-F]{40}$/.test(value))
    throw new TypeError('Invalid wallet or pool address');
  return value;
}
