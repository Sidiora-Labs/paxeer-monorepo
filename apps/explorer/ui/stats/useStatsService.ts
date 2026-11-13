import type { ExternalChainExtended } from 'types/externalChains';

import config from 'configs/app';
import { useMultichainContext } from 'lib/contexts/multichain';

export default function useStatsService(chainProp?: ExternalChainExtended): boolean {
  const context = useMultichainContext();
  const chain = chainProp || context?.chain;
  const apis = chain?.app_config?.apis ?? config.apis;

  return 'stats' in apis && Boolean(apis.stats);
}
