import type * as stats from '@blockscout/stats-types';

import { base as producerStats } from 'mocks/stats';
import { HOMEPAGE_STATS } from 'stubs/stats';

import type { CounterValueSemantics } from './NumberWidgetsList';

export const utilizationCounter: stats.Counter = {
  id: 'networkUtilization',
  title: 'Network utilization',
  value: String(producerStats.network_utilization_percentage),
  units: '%',
  description: 'Share of the gas limit in use',
};

export const positiveChangeCounter: stats.Counter = {
  id: 'coinPriceIncrease',
  title: 'Coin price increase',
  value: String(HOMEPAGE_STATS.coin_price_change_percentage),
  units: '%',
  description: 'Coin price change percentage',
};

export const negativeChangeCounter: stats.Counter = {
  ...positiveChangeCounter,
  id: 'coinPriceDecrease',
  title: 'Coin price decrease',
  value: String(producerStats.coin_price_change_percentage),
};

export const counterSemantics: Readonly<Record<string, CounterValueSemantics>> = {
  [utilizationCounter.id]: 'absolute',
  [positiveChangeCounter.id]: 'change',
  [negativeChangeCounter.id]: 'change',
};

export const counters: stats.Counters = {
  counters: [
    { id: 'totalAddresses', title: 'Total addresses', value: '1804523', description: 'Addresses that ever appeared on chain' },
    { id: 'totalTxns', title: 'Total transactions', value: '196150000', description: 'Every transaction the chain carries' },
    { id: 'averageBlockTime', title: 'Average block time', value: '0.175', units: 's', description: 'Mean time between two blocks' },
    utilizationCounter,
    positiveChangeCounter,
    negativeChangeCounter,
  ],
};
