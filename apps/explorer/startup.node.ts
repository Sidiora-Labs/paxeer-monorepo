import config from 'configs/app';
import * as essentialDappsChainsConfig from 'configs/essential-dapps-chains/config.nodejs';
import * as multichainConfig from 'configs/multichain/config.nodejs';

const marketplaceFeature = config.features.marketplace;

if (config.features.multichain.isEnabled) {
  await multichainConfig.load();
}

if (marketplaceFeature.isEnabled && marketplaceFeature.essentialDapps) {
  const essentialDappsChains = await essentialDappsChainsConfig.load();
  if (!essentialDappsChains) {
    throw new Error('Cannot load required essential dapp chains configuration');
  }
}
