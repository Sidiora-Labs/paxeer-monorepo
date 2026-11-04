import BigNumber from 'bignumber.js';

import type { Block } from 'types/api/block';
import type { ClusterChainConfig } from 'types/multichain';

import type { Unit } from 'ui/shared/value/utils';
import { GWEI, getUnitDecimals } from 'ui/shared/value/utils';

type Currency = ClusterChainConfig['app_config']['chain']['currency'];

export function getBlockTotalReward(block: Block, currency: Currency) {
  const reward = (block.rewards ?? []).reduce((sum, { reward }) => sum.plus(reward), new BigNumber(0));
  return reward.div(new BigNumber(10).pow(currency.decimals));
}

export function getBlockNativeValueProps(currency: Currency, amount: string | null | undefined, units: Unit = 'ether', gweiThreshold?: number) {
  const effectiveUnits = units === 'wei' && amount && gweiThreshold && BigNumber(amount).div(GWEI).gte(10 ** -gweiThreshold) ? 'gwei' : units;
  const weiName = currency.weiName || 'wei';
  const asset = (() => {
    if (effectiveUnits === 'ether') {
      return currency.symbol || 'ETH';
    }
    if (effectiveUnits === 'gwei') {
      return currency.gweiName || `G${ weiName }`;
    }
    return weiName;
  })();

  return {
    decimals: effectiveUnits === 'ether' ? currency.decimals : getUnitDecimals(effectiveUnits),
    asset,
  };
}
