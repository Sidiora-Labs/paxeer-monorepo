'use client';
import { useEffect, useRef, useState } from 'react';
import { usePoolSnapshot } from '@/core/market/use-pool-snapshot';
import type { PoolStats } from '@/core/market/pool-resolver';
import { formatCompactUsd } from '@/utils/format';
import { statsCirculatingMarketCap, statsMarketCap } from '@/core/market/pool-stats-display';
import { StatDash } from '@/ui/shared/stat-value';
import { allTimeHighMarketCap } from '@/core/market/all-time-high';
import { useAllTimeHigh } from '@/core/market/use-all-time-high';

/** Market cap and its share of the all-time high from daily candles covering the pool's whole life. */
export default function McAthCard({ poolAddress, liveStats }: { poolAddress: string; liveStats: PoolStats | null }) {
  const section = useRef<HTMLElement>(null);
  const [inView, setInView] = useState(false);
  useEffect(() => {
    if (!section.current) return;
    if (typeof IntersectionObserver === 'undefined') { setInView(true); return; }
    const observer = new IntersectionObserver(entries => setInView(entries.some(entry => entry.isIntersecting)));
    observer.observe(section.current);
    return () => observer.disconnect();
  }, []);
  const snapshot = usePoolSnapshot(poolAddress);
  const stats = liveStats ?? snapshot.data?.stats ?? null;
  // Market cap is price x total supply (PoolStats.totalSupplyValuationUsdlRaw).
  const mcapStat = statsMarketCap(stats);
  const circulating = statsCirculatingMarketCap(stats).value;
  const mcap = mcapStat.value === null ? null : formatCompactUsd(mcapStat.value, { raw: true, compact: true });
  const loading = snapshot.isPending && !stats;
  const history = useAllTimeHigh(poolAddress, snapshot.data?.createdAt, inView);
  const token = snapshot.data?.token;
  const high = history.covered ? allTimeHighMarketCap(history.data, stats, token?.decimalsQuality === 'verified' ? token.decimals : null) : null;
  const peak = useRef<{ pool: string; value: number }>({ pool: poolAddress.toLowerCase(), value: 0 });
  if (peak.current.pool !== poolAddress.toLowerCase()) peak.current = { pool: poolAddress.toLowerCase(), value: 0 };
  if (high !== null) peak.current.value = Math.max(peak.current.value, high);
  const ath = high === null ? null : history.data?.status === 'no_candles' ? high : peak.current.value;
  const athLoading = snapshot.isPending || (history.covered && history.isPending);
  const showAth = athLoading || ath !== null;
  const athLabel = ath === null ? null : formatCompactUsd(BigInt(Math.round(ath * 1e6)), { raw: true, compact: true });
  const current = mcapStat.value === null ? null : Number(mcapStat.value) / 1e6;
  const share = ath !== null && current !== null ? (ath > 0 ? Math.min(100, Math.max(0, current / ath * 100)) : 100) : null;
  const shareLabel = share === null ? null : share > 0 && share < 0.01 ? 'less than 0.01' : String(Number(share.toFixed(2)));
  return (
    <section ref={section} aria-labelledby="mc-ath-title" className="flex flex-col gap-1.5 rounded-md bg-surface-1 px-3 py-2.5">
      <h2 id="mc-ath-title" className="sr-only">{showAth ? 'Market cap vs ATH' : 'Market cap'}</h2>
      <dl className="flex flex-wrap items-center gap-3 text-xs">
        <div className="flex shrink-0 items-baseline gap-1">
          <dt className="text-text-faint">MC</dt>
          <dd data-numeric className={`text-sm font-bold tabular-nums ${mcap ? 'text-text' : 'text-text-faint'}`} aria-busy={loading || undefined}>
            {loading
              ? <span aria-hidden="true" className="sl-skeleton inline-block h-3.5 w-12 align-middle" />
              : mcap !== null
                ? <span title={circulating ? `Circulating market cap ${formatCompactUsd(circulating, { raw: true, compact: true })}` : undefined}>{mcap}</span>
                : stats ? <StatDash reason={mcapStat.reason ?? 'Market cap not available yet'} subject="Market cap" align="start" /> : '—'}
          </dd>
        </div>
        {showAth && <>
          {athLoading
            ? <div aria-busy="true" aria-label="Loading all-time high" className="sl-skeleton h-1 min-w-8 flex-1 rounded-pill" />
            : <div role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={share ?? undefined}
                aria-label={`Market cap ${mcap}, all-time high ${athLabel}; ${shareLabel} percent of all-time high`}
                className="h-1 min-w-8 flex-1 overflow-hidden rounded-pill bg-track">
                <div className="h-full rounded-pill bg-success" style={{ width: `${Math.max(2, share ?? 0)}%` }} />
              </div>}
          <div className="flex shrink-0 items-baseline gap-1">
            <dt className="text-text-faint">ATH</dt>
            <dd data-numeric aria-busy={athLoading || undefined} className="text-sm font-bold tabular-nums text-text">
              {athLoading ? <span aria-hidden="true" className="sl-skeleton inline-block h-3.5 w-12 align-middle" /> : athLabel}
            </dd>
          </div>
        </>}
      </dl>
    </section>
  );
}
