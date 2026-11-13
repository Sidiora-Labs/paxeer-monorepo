import type { LineChartSection, LineChartInfo } from '@blockscout/stats-types';
import { Resolution } from '@blockscout/stats-types';

import type { GeneralApiChartResourceName, GeneralApiMiscResourcePayload } from 'lib/api/services/general/misc';

export type ExplorerChart = LineChartInfo & {
  [R in GeneralApiChartResourceName]: {
    resource: R;
    field: Exclude<keyof GeneralApiMiscResourcePayload<R>['chart_data'][number], 'date' | 'tvl'>;
  }
}[GeneralApiChartResourceName];

export type ExplorerChartItem = GeneralApiMiscResourcePayload<ExplorerChart['resource']>['chart_data'][number];

export const EXPLORER_CHARTS: Array<ExplorerChart> = [
  {
    id: 'newTxns', title: 'Daily transactions', description: 'Transactions recorded each day',
    resource: 'general:stats_charts_txs', field: 'transactions_count', resolutions: [ Resolution.DAY ],
  },
  {
    id: 'coinPrice', title: 'PAX price', description: 'Daily PAX closing price', units: 'USD',
    resource: 'general:stats_charts_market', field: 'closing_price', resolutions: [ Resolution.DAY ],
  },
  {
    id: 'marketCap', title: 'PAX market value', description: 'Daily PAX market capitalization', units: 'USD',
    resource: 'general:stats_charts_market', field: 'market_cap', resolutions: [ Resolution.DAY ],
  },
  {
    id: 'secondaryCoinPrice', title: 'SID price', description: 'Daily SID closing price', units: 'USD',
    resource: 'general:stats_charts_secondary_coin_market', field: 'closing_price', resolutions: [ Resolution.DAY ],
  },
  {
    id: 'secondaryCoinMarketCap', title: 'SID market value', description: 'Daily SID market capitalization', units: 'USD',
    resource: 'general:stats_charts_secondary_coin_market', field: 'market_cap', resolutions: [ Resolution.DAY ],
  },
];

export const EXPLORER_CHART_SECTIONS: Array<LineChartSection> = [
  { id: 'transactions', title: 'Transactions', charts: EXPLORER_CHARTS.filter((chart) => chart.field === 'transactions_count') },
  { id: 'market', title: 'Market', charts: EXPLORER_CHARTS.filter((chart) => chart.field !== 'transactions_count') },
];

export function explorerNumber(value: string | number | null | undefined): number | undefined {
  if (value === null || value === undefined || (typeof value === 'string' && value.trim() === '')) {
    return undefined;
  }
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}

export function explorerChartValue(item: ExplorerChartItem, field: ExplorerChart['field']): number | undefined {
  if (field === 'transactions_count') {
    return explorerNumber('transactions_count' in item ? item.transactions_count : undefined);
  }
  return explorerNumber('closing_price' in item ? item[field] : undefined);
}
