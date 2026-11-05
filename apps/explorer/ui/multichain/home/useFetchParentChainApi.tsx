import React from 'react';

import { getFeaturePayload } from 'configs/app/features/types';

import config from 'configs/app';
import useApiFetch from 'lib/api/useApiFetch';
import { useMultichainContext } from 'lib/contexts/multichain';

interface Params {
  path: '/stats' | '/stats/charts/market';
}

export default function useFetchParentChainApi() {
  const apiFetch = useApiFetch();
  const context = useMultichainContext();
  const chain = context?.chain;
  const network = chain?.app_config ?? config;
  const market = React.useMemo(() => {
    if (!network.apis.general || !network.chain.id || !network.chain.currency.symbol) {
      return;
    }

    return {
      chainId: network.chain.id,
      symbol: network.chain.currency.symbol,
      endpoint: network.apis.general.endpoint,
      basePath: network.apis.general.basePath,
      tenant: network.app.baseUrl,
      cluster: getFeaturePayload(network.features.multichain)?.cluster,
    };
  }, [ network ]);

  const fetch = React.useCallback(({ path }: Params) => {
    if (!market) {
      return Promise.reject(new Error('Native asset market feed is not configured'));
    }

    return apiFetch(path === '/stats' ? 'general:stats' : 'general:stats_charts_market', { chain });
  }, [ apiFetch, chain, market ]);

  return Object.assign(fetch, { market, queryKey: [ 'parent_chain', market ?? null ] });
}
