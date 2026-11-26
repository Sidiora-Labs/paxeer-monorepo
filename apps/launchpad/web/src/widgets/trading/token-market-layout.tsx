'use client';
import { surfaceSpacing } from '@/app/presentation-tokens';
import { Suspense, useEffect, useState } from 'react';
import dynamic from 'next/dynamic';
import PremiumErrorBoundary from '@/ui/common/error-boundary';
import { SearchX } from 'lucide-react';
import { Button, EmptyState, Skeleton } from '@/ui/primitives';
import { PoolNotFoundError, type PoolStats } from '@/core/market/pool-resolver';
import { formatCount } from '@/utils/format';
import { usePoolSnapshot } from '@/core/market/use-pool-snapshot';
import { useMarketLive } from '@/page-modules/token/market-session-context';
import { priceNumber } from '@/page-modules/token/market-presentation';
import { HotCalloutsPanel } from '@/widgets/social/hot-callouts-panel';
import TokenHeader from '@/widgets/coin/token-header';
import FreshnessNotice from '@/widgets/coin/freshness-notice';
import StatsAuditPanel from '@/widgets/coin/stats-audit-panel';
import PoolInfoCard from '@/widgets/coin/pool-info-card';
import CreatorCard from '@/widgets/coin/creator-card';
import McAthCard from '@/widgets/coin/mc-ath-card';
import CreatorRewardsCard from '@/widgets/coin/creator-rewards-card';
import ChainRevenue from '@/widgets/holder-rewards/chain-revenue';
import CoinTabs from '@/widgets/coin/coin-tabs';
import TokenDescription from './token-description';
import { MetadataViewer } from '@/features/metadata-media/viewer';
import type { InitialPoolStreams } from '@/core/clients/livestream-api';
import type { InitialChainRevenue } from '@/widgets/holder-rewards/chain-revenue-reader';
import TradePanel from './trade-panel';
import LiveStream from './live-stream';
import TvChart from './tv-chart';

function PriceChartSkeleton() {
  return <Skeleton variant="block" className="h-[calc(var(--sl-space)*164)] lg:h-[calc(var(--sl-space)*150)]" label="price chart" />;
}

function TradePanelSkeleton() {
  return <Skeleton variant="block" height={surfaceSpacing(80)} label="trade panel" />;
}

const MobileTradeDrawer = dynamic(() => import('./mobile-trade-drawer'), { ssr: false });

const gridClass = [
  'grid grid-cols-1 items-start gap-2',
  'lg:grid-cols-[minmax(0,1fr)_min(var(--spacing-right-rail-width),calc(var(--sl-shell-layout-space)*80))] lg:grid-rows-[auto_1fr]',
  'xl:grid-cols-[min(var(--spacing-callout-column-width),calc(var(--sl-shell-layout-space)*75))_minmax(0,1fr)_min(var(--spacing-right-rail-width),calc(var(--sl-shell-layout-space)*80))]',
  '2xl:grid-cols-[min(var(--spacing-callout-column-width-wide),calc(var(--sl-shell-layout-space)*85))_minmax(0,1fr)_min(var(--spacing-right-rail-width-wide),calc(var(--sl-shell-layout-space)*90))]',
].join(' ');
/**
 * The callouts feed scrolls inside a viewport-high column. It is deliberately not `position: sticky`:
 * the shell's `<main>` is the nearest scroll container, so a sticky offset there resolves against
 * main's top padding and pushes this column below the other two instead of keeping it flush.
 */
const calloutColumnClass = 'xl:flex xl:max-h-[calc(100dvh-var(--shell-top))] xl:flex-col xl:overflow-y-auto';

/** The coin route keeps its market sections in place while the server resolves the address. */
export function TokenMarketLoading() {
  return (
    <div aria-busy="true" aria-label="Loading coin market" className="@container/market flex w-full min-w-0 flex-col gap-2 p-2 pb-36 motion-reduce:[&_*]:animate-none motion-reduce:[&_*]:transition-none sm:p-3 lg:pb-3">
      <div className={gridClass}>
        <aside aria-label="Loading hot callouts" className={`hidden min-w-0 xl:col-start-1 xl:row-start-1 xl:row-span-2 ${calloutColumnClass}`}>
          <Skeleton variant="rows" rows={4} label="hot callouts" />
        </aside>
        <div className="flex min-w-0 flex-col gap-2 lg:col-start-1 lg:row-start-1 xl:col-start-2">
          <header className="flex min-h-18 items-center gap-3 rounded-md bg-surface-1 p-3">
            <Skeleton variant="block" width={surfaceSpacing(16)} height={surfaceSpacing(16)} label="coin image" />
            <div className="flex min-w-0 flex-1 flex-col gap-2 [&>*]:max-w-full"><Skeleton variant="text" width={surfaceSpacing(40)} /><Skeleton variant="text" width={surfaceSpacing(60)} /></div>
          </header>
          <PriceChartSkeleton />
        </div>
        <aside aria-label="Loading trade and coin details" className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-1 lg:gap-4 lg:col-start-2 lg:row-start-1 lg:row-span-2 xl:col-start-3">
          <div className="hidden min-w-0 lg:block"><TradePanelSkeleton /></div>
          <Skeleton variant="rows" rows={4} label="coin details" />
        </aside>
        <div className="order-1 min-w-0 lg:order-none lg:col-start-1 lg:row-start-2 xl:col-start-2">
          <Skeleton variant="block" height={surfaceSpacing(60)} label="coin activity" />
        </div>
      </div>
    </div>
  );
}

/** Holder count row (rail position 2, after the trade panel); live stats first, then the snapshot rows. */
function RailHolders({ poolAddress, liveStats }: { poolAddress: string; liveStats: PoolStats | null }) {
  const snapshot = usePoolSnapshot(poolAddress);
  const stats = liveStats ?? snapshot.data?.stats ?? null;
  const count = stats?.holderCount !== null && stats?.holderCount !== undefined ? formatCount(stats.holderCount) : null;
  return (
    <section aria-labelledby="rail-holders-title" className="flex h-10 items-center justify-between gap-2 rounded-md bg-surface-1 px-3">
      <h2 id="rail-holders-title" className="text-sm font-semibold text-text">Holders</h2>
      {snapshot.isPending && count === null
        ? <span aria-hidden="true" className="sl-skeleton inline-block h-4 w-10" />
        : <span data-numeric className={`text-sm font-bold tabular-nums ${count ? 'text-text-muted' : 'text-text-faint'}`}>{count ?? '—'}</span>}
    </section>
  );
}

/** Description panel below the market valuation. */
function DescriptionCard({ poolAddress }: { poolAddress: string }) {
  return (
    <section aria-labelledby="description-card-title" className="flex flex-col gap-2 rounded-md bg-surface-1 p-3">
      <h2 id="description-card-title" className="text-sm font-semibold text-text">Description</h2>
      <TokenDescription poolAddress={poolAddress} />
    </section>
  );
}

/**
 * Neutral state for an address no pool or token resolves to (the resolver answered 404). Not an
 * error: nothing failed, the coin simply does not exist, so there is no reconnect banner or retry.
 */
export function CoinNotFound({ poolAddress }: { poolAddress: string }) {
  return (
    <div data-market-pool={poolAddress.toLowerCase()} data-coin-state="not-found" className="flex w-full flex-col p-2 sm:p-3">
      <section aria-label="Coin not found" className="mx-auto w-full max-w-xl rounded-md bg-surface-1">
        <EmptyState
          icon={SearchX}
          headingLevel={2}
          title="Coin not found"
          body="No coin matches this address. Check the link, or browse the coins trading now."
          action={<Button variant="primary" size="md" href="/explore">Browse coins</Button>}
        />
      </section>
    </div>
  );
}

/** Coin page: hot callouts column, main column (header, chart, tabs) and the right rail; stacks to a single column with the trade drawer on small screens. */
export default function TokenMarketLayout({ poolAddress, initialStreams, initialRevenue }: { poolAddress: string; initialStreams?: InitialPoolStreams; initialRevenue?: InitialChainRevenue }) {
  const snapshot = usePoolSnapshot(poolAddress);
  const data = snapshot.data ?? null;
  const pool = data?.pool.poolAddress ?? null;
  const notFound = snapshot.isError && snapshot.error instanceof PoolNotFoundError;
  /**
   * One live session per coin page, keyed by the resolved pool. It starts only once resolution has
   * settled and never for a coin the resolver answered 404 for: an unknown address must not poll
   * `/v2/snapshots` for topics that cannot exist. A transient resolver failure (not a 404) still
   * starts the session on the URL address so live data can recover.
   */
  /**
   * Resolution has settled at least once for this address. Sticky on purpose: React Query puts a
   * failed query with no data back to `pending` whenever a new observer mounts and refetches it, and
   * the frames below mount such observers. Gating them on `isPending` alone would unmount them on
   * that refetch and remount them on the next failure, a tight refetch loop.
   */
  const [settledFor, setSettledFor] = useState<string | null>(null);
  useEffect(() => { if (!snapshot.isPending) setSettledFor(poolAddress); }, [snapshot.isPending, poolAddress]);
  const settled = !snapshot.isPending || settledFor === poolAddress;
  const live = useMarketLive(pool ?? poolAddress, { enabled: settled && !notFound, poolDetail: data });
  const price = priceNumber(live.state.price);
  const liveStats = live.state.stats;
  if (notFound) return <CoinNotFound poolAddress={poolAddress} />;
  /**
   * The pool the market frames render for. Once resolution settles without a 404 the URL already
   * names the pool, so a failed snapshot degrades values to dashes instead of removing the chart,
   * trade panel, tabs and trade drawer. While resolution is pending nothing that opens market
   * traffic mounts (an unknown address must not poll before its 404 arrives).
   */
  const marketPool = pool ?? (settled ? poolAddress : null);
  const trading = <>
    <div className="hidden min-w-0 lg:block">
      <PremiumErrorBoundary area="TradePanel">
        <Suspense fallback={<TradePanelSkeleton />}>
          {marketPool ? <TradePanel poolAddress={marketPool} /> : <TradePanelSkeleton />}
        </Suspense>
      </PremiumErrorBoundary>
    </div>
    {marketPool ? <MobileTradeDrawer poolAddress={marketPool} /> : null}
  </>;
  const notice = !settled ? null : <FreshnessNotice freshness={live.freshness} stream={live.state.stream} error={live.state.error} noTradesYet={live.state.noTradesYet} onReconnect={() => { void live.session.start(); }} />;
  return (
    <div
      data-market-pool={poolAddress.toLowerCase()}
      data-market-stream={live.state.stream}
      data-market-freshness={live.freshness}
      data-market-snapshot={pool ? 'ready' : settled ? 'failed' : 'pending'}
      className="@container/market flex w-full min-w-0 flex-col gap-2 p-2 pb-36 motion-reduce:[&_*]:animate-none motion-reduce:[&_*]:transition-none sm:p-3 lg:pb-3"
    >
      <div className={gridClass}>
        <aside aria-label="Hot callouts" className={`hidden min-w-0 xl:col-start-1 xl:row-start-1 xl:row-span-2 ${calloutColumnClass}`}>
          <PremiumErrorBoundary area="Callouts">
            <HotCalloutsPanel poolAddress={pool} initialFeed={pool ? 'coin' : 'hot'} />
          </PremiumErrorBoundary>
        </aside>
        <div className="flex min-w-0 flex-col gap-2 lg:col-start-1 lg:row-start-1 xl:col-start-2">
          <PremiumErrorBoundary area="Token header"><TokenHeader poolAddress={poolAddress} liveStats={liveStats} livePrice={price} now={live.now} notice={notice} noTradesYet={live.state.noTradesYet} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="TvChart">
            <Suspense fallback={<PriceChartSkeleton />}>
              {marketPool ? <TvChart poolAddress={marketPool} symbol={data?.token?.symbol} market={live.session} /> : <PriceChartSkeleton />}
            </Suspense>
          </PremiumErrorBoundary>
        </div>
        <aside aria-label="Trade and coin details" className="order-2 grid lg:order-none min-w-0 grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-1 lg:gap-4 lg:col-start-2 lg:row-start-1 lg:row-span-2 xl:col-start-3">
          {snapshot.isError ? trading : <MetadataViewer tokenAddress={data?.pool.tokenAddress ?? ''} tradePanel={trading} />}
          {pool ? (
            <PremiumErrorBoundary area="LiveStream">
              <Suspense fallback={null}>
                <div className="min-w-0 md:col-span-2 lg:col-span-1 empty:hidden"><LiveStream poolAddress={pool} initial={initialStreams} /></div>
              </Suspense>
            </PremiumErrorBoundary>
          ) : null}
          <div className="min-w-0 md:col-span-2 lg:col-span-1"><RailHolders poolAddress={poolAddress} liveStats={liveStats} /></div>
          <PremiumErrorBoundary area="Stats"><StatsAuditPanel poolAddress={poolAddress} liveStats={liveStats} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Pool info"><PoolInfoCard poolAddress={poolAddress} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Creator"><CreatorCard poolAddress={poolAddress} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Market cap"><McAthCard poolAddress={poolAddress} liveStats={liveStats} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Description"><DescriptionCard poolAddress={poolAddress} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Creator rewards"><CreatorRewardsCard poolAddress={poolAddress} /></PremiumErrorBoundary>
          <PremiumErrorBoundary area="Chain revenue"><ChainRevenue poolAddress={pool ?? poolAddress} initial={initialRevenue} /></PremiumErrorBoundary>
        </aside>
        <div className="order-1 min-w-0 lg:order-none lg:col-start-1 lg:row-start-2 xl:col-start-2">
          {marketPool ? (
            <PremiumErrorBoundary area="Coin tabs"><CoinTabs poolAddress={marketPool} live={live} price={price} ariaLabel={data ? 'Market activity' : 'Coin details'} /></PremiumErrorBoundary>
          ) : <Skeleton variant="block" height={surfaceSpacing(60)} label="coin details" />}
        </div>
      </div>
    </div>
  );
}
