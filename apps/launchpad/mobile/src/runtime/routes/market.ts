import {
  Operations,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

const IF_POOL_LIST = 'IF_POOL_LIST';

export function pools(sdk: SdkClient, signal: AbortSignal) {
  return new Operations(sdk).call(
    IF_POOL_LIST,
    { query: { sort: 'new', limit: 50 } },
    { signal },
  );
}

export function pool(sdk: SdkClient, address: string, signal: AbortSignal) {
  return new Operations(sdk).call(
    'IF_POOL_DETAIL',
    { path: { poolAddress: evmAddress(address) } },
    { signal },
  );
}

export function search(sdk: SdkClient, query: string, signal: AbortSignal) {
  return new Operations(sdk).call(
    'IF_SEARCH',
    { query: { q: query, type: 'pool', limit: 25 } },
    { signal },
  );
}

export const marketRoutes = { pools, pool, search };

function evmAddress(value: string) {
  if (!/^0x[0-9a-fA-F]{40}$/.test(value))
    throw new TypeError('Invalid wallet or pool address');
  return value;
}
