"use client";

// AnalyticsLiquidStat — protocol-wide cycleable stat card.
//
// TVL comes from authoritative protocol snapshots, including collateral that
// has no recent balance events. A daily percentage requires both endpoints.

import { useMemo } from "react";
import { LiquidStat, type LiquidStatState } from "@/ui/tokens/LiquidStat";
import { useProtocolSnapshots } from "@/hooks/perps/useAnalytics";
import { useEnrichedGlobalStats } from "@/hooks/perps/useEnrichedStats";
import {
  useRealtimePrices,
  type TickerMap,
} from "@/hooks/perps/useRealtimePrices";
import type {
  EnrichedGlobalStats,
  ProtocolSnapshotRecord,
} from "@/types/perps";

const SPARK_POINTS = 15;
const SPARK_MIN = 20;
const SPARK_MAX = 100;
const DAY_MS = 86_400_000;

// ── Helpers ─────────────────────────────────────────────────────────────

function normalizeSpark(raw: number[]): number[] {
  if (raw.length === 0) {
    return new Array(SPARK_POINTS).fill((SPARK_MIN + SPARK_MAX) / 2);
  }
  const min = Math.min(...raw);
  const max = Math.max(...raw);
  const span = max - min;
  const flat = (SPARK_MIN + SPARK_MAX) / 2;
  const scaled =
    span <= 0
      ? raw.map(() => flat)
      : raw.map(
          (v) => SPARK_MIN + ((v - min) / span) * (SPARK_MAX - SPARK_MIN),
        );
  if (scaled.length >= SPARK_POINTS) return scaled.slice(-SPARK_POINTS);
  const pad = new Array(SPARK_POINTS - scaled.length).fill(scaled[0]!);
  return [...pad, ...scaled];
}

/** Only complete aggregate snapshots can seed a protocol TVL timeline. */
function buildTvlTimeline(
  records: ProtocolSnapshotRecord[],
): { ts: number; tvl: number }[] | null {
  const out: { ts: number; tvl: number }[] = [];
  for (const record of records) {
    const ts = Date.parse(record.blockTimestamp);
    const tvl = Number(record.tvlUsd) / 1e18;
    if (
      !/^\d+$/.test(record.tvlUsd) ||
      !Number.isFinite(tvl) ||
      !Number.isFinite(ts)
    ) {
      return null;
    }
    out.push({ ts, tvl });
  }
  return out.sort((a, b) => a.ts - b.ts);
}

// ── Component ──────────────────────────────────────────────────────────

export function AnalyticsLiquidStat() {
  const { data: snapshots } = useProtocolSnapshots();
  const { data: enriched } = useEnrichedGlobalStats();
  const { tickers } = useRealtimePrices();
  return (
    <AnalyticsLiquidStatContent
      snapshots={snapshots}
      enriched={enriched}
      tickers={tickers}
    />
  );
}

interface AnalyticsLiquidStatContentProps {
  snapshots: ProtocolSnapshotRecord[] | undefined;
  enriched: EnrichedGlobalStats | undefined;
  tickers: TickerMap;
}

export function AnalyticsLiquidStatContent({
  snapshots,
  enriched,
  tickers,
}: AnalyticsLiquidStatContentProps) {
  const timeline = useMemo(
    () => (snapshots ? buildTvlTimeline(snapshots) : null),
    [snapshots],
  );
  const latest = timeline?.[timeline.length - 1];
  // Without a snapshot at the daily endpoint, the period's change is unknown.
  const baseline = latest
    ? timeline?.find((p) => p.ts === latest.ts - DAY_MS)
    : undefined;
  const tvlDelta =
    latest && baseline && baseline.tvl > 0
      ? ((latest.tvl - baseline.tvl) / baseline.tvl) * 100
      : null;

  const states: LiquidStatState[] | null = useMemo(() => {
    if (snapshots === undefined || !enriched) return null;
    const tickerList = Object.values(tickers);
    if (tickerList.length === 0) return null;

    // ── 24h Volume ──
    // Orderly aggregates are canonical protocol-wide volume. Source of truth.
    const orderly = enriched.orderly;
    const vol1d = orderly?.perp_volume_last_1_day ?? 0;
    const vol7d = orderly?.perp_volume_last_7_days ?? 0;
    const vol7dAvg = vol7d > 0 ? vol7d / 7 : 0;
    const volDelta = vol7dAvg > 0 ? ((vol1d - vol7dAvg) / vol7dAvg) * 100 : 0;
    // Spark = per-market 24h volume in USD notional (h24_amount), sorted
    // asc so the line crests toward the volume leader on the right edge.
    const volBars = tickerList
      .map((t) => (t as { "24h_amount"?: number })["24h_amount"] ?? 0)
      .filter((v) => v > 0)
      .sort((a, b) => a - b)
      .slice(-SPARK_POINTS);
    const volSpark = normalizeSpark(volBars);

    // ── Active Markets ──
    // enriched.onchain.totalMarkets is the source-of-truth count on-chain.
    const totalMarkets = enriched.onchain.totalMarkets;
    // No temporal history for totalMarkets; honest "flat" spark at the
    // current value. When markets get added this will step up organically.
    const marketsSpark = normalizeSpark(
      new Array(SPARK_POINTS).fill(totalMarkets),
    );
    // Delta intentionally 0 — real data, real constraint. When snapshot
    // backfill lands, replace with temporal diff.
    const marketsDelta = 0;

    const out: LiquidStatState[] = [];
    if (latest && tvlDelta !== null && timeline) {
      out.push({
        label: "TVL",
        value: latest.tvl,
        delta: tvlDelta,
        sym: "$",
        data: normalizeSpark(timeline.slice(-SPARK_POINTS).map((p) => p.tvl)),
      });
    }
    out.push(
      {
        label: "24h Volume",
        value: vol1d,
        delta: volDelta,
        sym: "$",
        data: volSpark,
      },
      {
        label: "Active Markets",
        value: totalMarkets,
        delta: marketsDelta,
        data: marketsSpark,
      },
    );
    return out;
  }, [snapshots, enriched, tickers, timeline, latest, tvlDelta]);

  if (!states) {
    return (
      <div
        aria-hidden
        style={{
          width: 360,
          height: 260,
          borderRadius: 18,
          background:
            "linear-gradient(165deg, rgba(0,30,92,0.35) 0%, rgba(0,76,237,0.25) 100%)",
          border: "1px solid rgba(143,168,255,0.18)",
        }}
        className="animate-pulse"
      />
    );
  }

  if (tvlDelta === null) {
    return (
      <div>
        <p role="status" className="text-xs text-muted-foreground mb-3">
          {latest
            ? `TVL: $${latest.tvl.toLocaleString("en-US", { maximumFractionDigits: 2 })}`
            : "TVL unavailable"}
          {" · 24h change unavailable"}
        </p>
        <LiquidStat states={states} />
      </div>
    );
  }

  return <LiquidStat states={states} />;
}

export default AnalyticsLiquidStat;
