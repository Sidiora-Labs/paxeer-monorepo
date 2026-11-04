import type { UseQueryResult } from '@tanstack/react-query';
import { useQuery } from '@tanstack/react-query';
import React from 'react';
import { createPublicClient, http } from 'viem';

import type { AddressCounters } from 'types/api/address';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import type { ResourceError } from 'lib/api/resources';
import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import { ADDRESS_COUNTERS } from 'stubs/address';

export type AddressCountersQuery = UseQueryResult<AddressCounters, ResourceError<{ status: number }>> & {
  isDegradedData: boolean;
  outgoingTransactionCount: string | undefined;
  rpcError: Error | null;
};

interface Params {
  hash: string;
  isEnabled?: boolean;
  isLoading?: boolean;
  isDegradedData?: boolean;
  chain?: ClusterChainConfig;
}

export default function useAddressCountersQuery({ hash, isLoading, isDegradedData, isEnabled = true, chain: chainProp }: Params): AddressCountersQuery {
  const multichainContext = useMultichainContext();
  const chain = chainProp || multichainContext?.chain;
  const chainConfig = chain?.app_config ?? config;
  const chainId = chain?.id ?? chainConfig.chain.id;
  const rpcUrl = chainConfig.chain.rpcUrls.find(Boolean);
  const publicClient = React.useMemo(() => rpcUrl ? createPublicClient({
    transport: http(rpcUrl, { retryCount: 0 }),
  }) : undefined, [ rpcUrl ]);
  // The counters of an address do not depend on the address itself, so the request leaves with it
  // instead of waiting for its answer. Only the RPC fallback waits, because whether it is needed is
  // read from that answer.
  const enabled = isEnabled && Boolean(hash);

  const apiQuery = useApiQuery<'general:address_counters', { status: number }>('general:address_counters', {
    pathParams: { hash },
    queryOptions: {
      enabled,
      placeholderData: ADDRESS_COUNTERS,
      refetchOnMount: false,
    },
    chain,
  });

  const rpcQuery = useQuery<number, Error>({
    queryKey: [ 'RPC', 'address_counters', { hash, chainId, chainSlug: chain?.slug, rpcUrl } ],
    queryFn: async() => {
      if (!publicClient) {
        throw new Error(`No public RPC configured for chain ${ chainId }`);
      }
      return publicClient.getTransactionCount({ address: hash as `0x${ string }` });
    },
    enabled: enabled && !isLoading && (isDegradedData || apiQuery.isError),
    retry: false,
    refetchOnMount: false,
  });

  return {
    ...apiQuery,
    isDegradedData: Boolean((isDegradedData || apiQuery.isError) && !apiQuery.data),
    outgoingTransactionCount: rpcQuery.data?.toString(),
    rpcError: rpcQuery.error,
  };
}
