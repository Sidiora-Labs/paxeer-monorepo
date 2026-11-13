import { CHAIN_INDICATOR_IDS, type ChainIndicatorId } from 'types/homepage';

export function isHomepageChartId(value: string): value is ChainIndicatorId {
  return CHAIN_INDICATOR_IDS.some((id) => id === value);
}

export function hasHomepageChartDependencies(charts: ReadonlyArray<ChainIndicatorId | undefined> | undefined, statsApiHost?: string): boolean {
  return !charts?.includes('daily_operational_txs') || Boolean(statsApiHost);
}
