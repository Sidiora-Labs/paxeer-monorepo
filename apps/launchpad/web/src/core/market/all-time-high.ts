import { fetchCandleHistoryOutcome, type CandleHistoryOutcome } from '@/core/clients/candle-stats';
import { statsMarketCap } from './pool-stats-display';
import type { PoolStats } from './use-stats-batch';

const day = 86_400;
export const allTimeHighRefreshMs = 60_000;

export interface AllTimeHighWindow { from: number; to: number }
export type AllTimeHighHistory =
  | { status: 'not_covered' }
  | { status: 'candles'; highestPrice: number; window: AllTimeHighWindow }
  | { status: 'no_candles'; window: AllTimeHighWindow }
  | { status: 'failed'; outcome: Exclude<CandleHistoryOutcome, { status: 'ok' | 'no_data' }> };

/** Daily buckets cover the whole life only for pools younger than 365 days. */
export function allTimeHighWindow(createdAt: string | null | undefined, now = Math.floor(Date.now() / 1000)): AllTimeHighWindow | null {
  const created = createdAt ? Date.parse(createdAt) / 1000 : NaN;
  if (!Number.isFinite(created) || !Number.isFinite(now) || created < 0 || created > now || now - created >= 365 * day) return null;
  return { from: Math.floor(created / day) * day, to: now };
}

export async function loadAllTimeHighHistory(
  poolAddress: string,
  createdAt: string | null | undefined,
  options: { now?: number; signal?: AbortSignal; read?: typeof fetchCandleHistoryOutcome } = {},
): Promise<AllTimeHighHistory> {
  const window = allTimeHighWindow(createdAt, options.now);
  if (!window) return { status: 'not_covered' };
  const outcome = await (options.read ?? fetchCandleHistoryOutcome)(poolAddress.toLowerCase(), '1D', window.from, window.to, { signal: options.signal });
  if (outcome.status === 'no_data') return { status: 'no_candles', window };
  if (outcome.status !== 'ok') return { status: 'failed', outcome };
  if (outcome.history.h.length === 0) return { status: 'no_candles', window };
  return { status: 'candles', highestPrice: Math.max(...outcome.history.h), window };
}

/** Approximate USDL for compact display only; raw statistics remain unchanged. */
export function allTimeHighMarketCap(history: AllTimeHighHistory | undefined, stats: PoolStats | null, tokenDecimals: number | null): number | null {
  const raw = statsMarketCap(stats).value;
  if (raw === null || !history || history.status === 'not_covered' || history.status === 'failed') return null;
  const current = Number(raw) / 1e6;
  if (history.status === 'no_candles') return current;
  if (!stats?.totalSupplyRaw || !/^(0|[1-9][0-9]{0,77})$/.test(stats.totalSupplyRaw)
    || tokenDecimals === null || !Number.isInteger(tokenDecimals) || tokenDecimals < 0 || tokenDecimals > 255) return null;
  const high = history.highestPrice * (Number(stats.totalSupplyRaw) / 10 ** tokenDecimals);
  if (!Number.isFinite(high) || high < 0) return null;
  return Math.max(current, high);
}
