'use client';

/**
 * Market stats — PAX / WPAX only. Shows market cap, circulating supply, and
 * 24h high/low derived from the OHLC dataset.
 */

import { formatUsd } from '@/lib/format';
import type { OhlcCandle } from '@/lib/api';
import { SectionLabel, MetricCard } from './Atoms';
import type { MarketWindow } from './detailQueries';

const PAX_CIRCULATING = 500_000_000;

export interface MarketStatsCardProps {
  paxPrice: number;
  ohlcData: OhlcCandle[];
  window: MarketWindow | null;
}

export function MarketStatsCard({ paxPrice, ohlcData, window }: MarketStatsCardProps) {
  if (paxPrice <= 0) return null;

  const last24h = window
    ? ohlcData
        .filter((c) => c.timestamp >= window.from && c.timestamp < window.to)
        .sort((a, b) => a.timestamp - b.timestamp)
    : [];
  const has24hData = Boolean(
    window &&
      window.to - window.from === 86_400_000 &&
      window.intervalMs > 0 &&
      last24h.length > 0 &&
      last24h[0].timestamp === window.from &&
      last24h[last24h.length - 1].timestamp + window.intervalMs === window.to &&
      last24h.every(
        (c, i) =>
          Number.isFinite(c.high) &&
          Number.isFinite(c.low) &&
          (i === 0 || c.timestamp - last24h[i - 1].timestamp === window.intervalMs)
      )
  );

  return (
    <>
      <div className="col-span-2 px-1 pt-1">
        <SectionLabel text="Market" />
        {window && (
          <p className="text-[11px] text-pax-muted">
            {new Date(window.from).toISOString()} – {new Date(window.to).toISOString()}
          </p>
        )}
      </div>
      <MetricCard label="Market Cap" value={formatUsd(paxPrice * PAX_CIRCULATING)} />
      <MetricCard label="Circulating" value="500M PAX" />
      {has24hData ? (
        <>
          <MetricCard label="24h High" value={formatUsd(Math.max(...last24h.map((c) => c.high)))} />
          <MetricCard label="24h Low" value={formatUsd(Math.min(...last24h.map((c) => c.low)))} />
        </>
      ) : (
        <p className="col-span-2 text-xs text-pax-muted">
          24h range unavailable: incomplete coverage
        </p>
      )}
    </>
  );
}
