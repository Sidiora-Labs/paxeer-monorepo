import type { EssentialDappsChainConfig } from 'types/client/marketplace';

import config from 'configs/app';
import * as essentialDappsChainsConfigNodejs from 'configs/essential-dapps-chains/config.nodejs';
import { isBrowser } from 'toolkit/utils/isBrowser';

const marketplaceFeature = config.features.marketplace;

const essentialDappsChains: () => { chains: Array<EssentialDappsChainConfig> } | undefined = () => {
  if (!marketplaceFeature.isEnabled || !marketplaceFeature.essentialDapps) {
    return;
  }

  if (isBrowser()) {
    return window.__essentialDappsChains;
  }

  return essentialDappsChainsConfigNodejs.getValue();
};

export default essentialDappsChains;
