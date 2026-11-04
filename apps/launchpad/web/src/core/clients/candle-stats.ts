import { CHART_UDF_ROUTES, chartsOrigin } from '@/core/charts-udf';

export interface CandleHistory { t: number[]; o: number[]; h: number[]; l: number[]; c: number[]; v: number[] }

export type CandleHistoryFailure = {
  status: 'unavailable' | 'error';
  httpStatus: number | null;
  code: string | null;
  message: string;
  retryable: boolean | null;
  retryAfterMs: number | null;
  requestId: string | null;
  cause?: unknown;
};

export type CandleHistoryOutcome =
  | { status: 'ok'; history: CandleHistory }
  | { status: 'no_data'; nextTime: number | null }
  | { status: 'aborted'; reason: unknown }
  | CandleHistoryFailure;

export interface CandleStats {
  change24h: number | null;
  change1h: number | null;
  change5m: number | null;
  /** Extremes of daily bars whose timestamps are in extremesWindow, never lifetime extremes. */
  rollingYearHigh: number | null;
  rollingYearLow: number | null;
  extremesWindow: { from: number; to: number };
  history: { daily: CandleHistoryOutcome; hourly: CandleHistoryOutcome; fiveMinute: CandleHistoryOutcome };
}

export interface CandleStatsOptions {
  /** core/trading-charts origin; defaults to the page origin (next.config.ts rewrites /v2/udf/history). */
  chartsOrigin?: string;
  /** Epoch seconds the windows end at; defaults to now. */
  now?: number;
  signal?: AbortSignal;
}

const series = (value: unknown, length: number): value is number[] =>
  Array.isArray(value) && value.length === length && value.every(item => typeof item === 'number' && Number.isFinite(item));

function document(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function failure(message: string, httpStatus: number | null, cause?: unknown): CandleHistoryFailure {
  return { status: 'error', httpStatus, code: null, message, retryable: null, retryAfterMs: null, requestId: null, cause };
}

function retryDelay(response: Response, body: Record<string, unknown> | null): number | null {
  if (typeof body?.retryAfterMs === 'number' && Number.isFinite(body.retryAfterMs) && body.retryAfterMs >= 0) return body.retryAfterMs;
  const header = response.headers.get('Retry-After');
  if (header === null) return null;
  if (/^\d+$/.test(header)) return Number(header) * 1000;
  const timestamp = Date.parse(header);
  return Number.isFinite(timestamp) ? Math.max(0, timestamp - Date.now()) : null;
}

/** Read the canonical price history without turning failure or cancellation into an empty market. */
export async function fetchCandleHistoryOutcome(poolAddress: string, resolution: string, fromTs: number, toTs: number, options: CandleStatsOptions = {}): Promise<CandleHistoryOutcome> {
  let response: Response | undefined;
  try {
    options.signal?.throwIfAborted();
    const url = new URL(CHART_UDF_ROUTES.history, chartsOrigin(options.chartsOrigin));
    for (const [key, value] of Object.entries({ symbol: poolAddress.toLowerCase(), resolution, from: String(fromTs), to: String(toTs) })) url.searchParams.set(key, value);
    response = await fetch(url, { signal: options.signal, headers: { Accept: 'application/json', 'X-Schema-Version': '2' } });
    let body: Record<string, unknown> | null = null;
    try {
      body = document(await response.json());
    } catch (cause) {
      options.signal?.throwIfAborted();
      if (response.ok) return failure('Malformed candle history JSON', response.status, cause);
    }
    options.signal?.throwIfAborted();
    if (!response.ok) {
      return {
        status: response.status === 429 || response.status >= 500 ? 'unavailable' : 'error',
        httpStatus: response.status,
        code: typeof body?.code === 'string' ? body.code : null,
        message: typeof body?.message === 'string' ? body.message : `Candle history HTTP ${response.status}`,
        retryable: typeof body?.retryable === 'boolean' ? body.retryable : null,
        retryAfterMs: retryDelay(response, body),
        requestId: typeof body?.requestId === 'string' ? body.requestId : null,
      };
    }
    if (response.headers.get('X-Schema-Version') !== '2' || !body) return failure('Invalid candle history envelope', response.status);
    if (body.s === 'no_data') return { status: 'no_data', nextTime: typeof body.nextTime === 'number' && Number.isFinite(body.nextTime) ? body.nextTime : null };
    if (body.s !== 'ok' || body.mode !== 'price' || !Array.isArray(body.t) || body.t.length === 0) return failure('Invalid candle history envelope', response.status);
    const length = body.t.length;
    if (!series(body.t, length) || !series(body.o, length) || !series(body.h, length) || !series(body.l, length) || !series(body.c, length) || !series(body.v, length)) return failure('Invalid candle history series', response.status);
    if (body.t.some((time, index, times) => !Number.isSafeInteger(time) || time < fromTs || time >= toTs || (index > 0 && time <= times[index - 1]))) return failure('Invalid candle history timestamps', response.status);
    return { status: 'ok', history: { t: body.t, o: body.o, h: body.h, l: body.l, c: body.c, v: body.v } };
  } catch (cause) {
    if (options.signal?.aborted) return { status: 'aborted', reason: options.signal.reason };
    if (cause instanceof Error && cause.name === 'AbortError') return { status: 'aborted', reason: cause };
    return { ...failure('Candle history transport unavailable', response?.status ?? null, cause), status: 'unavailable' };
  }
}

export class CandleHistoryError extends Error {
  constructor(readonly outcome: Exclude<CandleHistoryOutcome, { status: 'ok' | 'no_data' }>) {
    super(outcome.status === 'aborted' ? 'Candle history aborted' : outcome.message);
    this.name = outcome.status === 'aborted' ? 'AbortError' : 'CandleHistoryError';
  }
}

/** Compatibility for bar consumers: only canonical no_data is null; failures retain their typed outcome. */
export async function fetchCandleHistory(poolAddress: string, resolution: string, fromTs: number, toTs: number, options: CandleStatsOptions = {}): Promise<CandleHistory | null> {
  const outcome = await fetchCandleHistoryOutcome(poolAddress, resolution, fromTs, toTs, options);
  if (outcome.status === 'ok') return outcome.history;
  if (outcome.status === 'no_data') return null;
  throw new CandleHistoryError(outcome);
}

function windowChange(outcome: CandleHistoryOutcome, interval: number, now: number): number | null {
  if (outcome.status !== 'ok') return null;
  const { t, c } = outcome.history;
  const last = t.length - 1;
  // Bounds are half-open: at a boundary the newest bar starts one interval earlier.
  if (now - t[last] > interval) return null;
  const baseline = t.indexOf(t[last] - interval);
  if (baseline < 0 || c[baseline] <= 0) return null;
  return ((c[last] - c[baseline]) / c[baseline]) * 100;
}

export async function fetchCandleStats(poolAddress: string, options: CandleStatsOptions = {}): Promise<CandleStats> {
  const now = options.now ?? Math.floor(Date.now() / 1000);
  const extremesWindow = { from: now - 365 * 86400, to: now };
  const [daily, hourly, fiveMinute] = await Promise.all([
    fetchCandleHistoryOutcome(poolAddress, '1D', extremesWindow.from, now, options),
    fetchCandleHistoryOutcome(poolAddress, '60', now - 2 * 3600, now, options),
    fetchCandleHistoryOutcome(poolAddress, '5', now - 600, now, options),
  ]);
  const positiveLows = daily.status === 'ok' ? daily.history.l.filter(value => value > 0) : [];
  return {
    change24h: windowChange(daily, 86400, now),
    change1h: windowChange(hourly, 3600, now),
    change5m: windowChange(fiveMinute, 300, now),
    rollingYearHigh: daily.status === 'ok' ? Math.max(...daily.history.h) : null,
    rollingYearLow: positiveLows.length > 0 ? Math.min(...positiveLows) : null,
    extremesWindow,
    history: { daily, hourly, fiveMinute },
  };
}
