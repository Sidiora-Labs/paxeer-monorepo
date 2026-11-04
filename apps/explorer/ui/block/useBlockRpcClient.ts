import React from 'react';
import { createPublicClient, http } from 'viem';

import { useMultichainContext } from 'lib/contexts/multichain';
import { publicClient } from 'lib/web3/client';

export default function useBlockRpcClient() {
  const multichainContext = useMultichainContext();
  const chain = multichainContext?.chain;
  const rpcUrl = chain?.app_config.chain.rpcUrls.find(Boolean);
  const rpcClient = React.useMemo(() => {
    if (!chain) {
      return publicClient;
    }

    if (!rpcUrl) {
      return;
    }

    return createPublicClient({
      transport: http(rpcUrl),
      batch: { multicall: true },
    });
  }, [ chain, rpcUrl ]);

  return { rpcClient, chainId: chain?.id };
}
