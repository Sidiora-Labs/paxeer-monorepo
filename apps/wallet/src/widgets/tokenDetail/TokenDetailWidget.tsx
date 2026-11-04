'use client';

/**
 * Token detail widget — orchestrator for the per-token deep-dive screen.
 *
 * Composes:
 * - {@link PriceHero}        — price + period change
 * - {@link PriceChart}       — area chart + period selector + scrub
 * - {@link TokenActions}     — Send / Receive / Swap row
 * - {@link PositionCard}     — wallet's holding (when > 0)
 * - {@link PerformanceCard}  — Sidiora 24h volume / trades
 * - {@link MarketStatsCard}  — PAX/WPAX market cap + 24h high/low
 * - {@link SecurityCard}     — top-10 holders + risk score
 * - {@link SocialLinks}      — website / X / Telegram
 * - {@link InfoCard}         — name / symbol / contract / decimals / etc.
 * - {@link ActivityFeed}     — token-scoped tx history
 *
 * All data routes through TanStack Query hooks (`@/lib/queries`) so caches
 * are shared with other screens (no duplicate fetches when navigating between
 * portfolio → token detail).
 */

import { useMemo, useState, useCallback } from 'react';
import { useWallet, useWalletState } from '@/providers/WalletProvider';
import { formatBalance } from '@/lib/format';
import {
  usePortfolioQuery,
  usePaxPriceQuery,
  useTokenMetaQuery,
  useTokenPoolDataQuery,
  useTokenCandlesQuery,
  useCrossversePriceQuery,
  type TimePeriod,
} from '@/lib/queries';
import { getCrossverseSymbol } from '@/lib/api';
import type { AppRoute } from '@/widgets/shell/useAppRoute';
import { PriceHero, type PriceChangeInfo } from './PriceHero';
import { PriceChart, type ChartPoint } from './PriceChart';
import { TokenActions } from './TokenActions';
import { PositionCard } from './PositionCard';
import { PerformanceCard } from './PerformanceCard';
import { MarketStatsCard } from './MarketStatsCard';
import { SecurityCard } from './SecurityCard';
import { SocialLinks } from './SocialLinks';
import { InfoCard } from './InfoCard';
import { ActivityFeed } from './ActivityFeed';
import { CHAIN_ID, TOKENS } from '@/lib/swap/sdk/addresses';
import { PAXEER_CONFIG } from '@/lib/constants';
import {
  supportedTokenDecimals,
  useMarketDayQuery,
  useTokenDetailHistoryQuery,
  type TokenDetailHistoryRow,
} from './detailQueries';

const WPAX_BY_CHAIN: Readonly<Partial<Record<number, string>>> = {
  [CHAIN_ID]: TOKENS.WPAX9.address.toLowerCase(),
};

export function resolveTokenDetailIdentity(routeTokenId: string, chainId: number | null) {
  const normalizedId = routeTokenId.trim().toLowerCase();
  const routeAlias = normalizedId === 'pax' || normalizedId === 'wpax' ? normalizedId : null;
  const wrappedAddress = chainId === null ? undefined : WPAX_BY_CHAIN[chainId];
  const tokenId = routeAlias === 'wpax' && wrappedAddress ? wrappedAddress : normalizedId;
  return {
    tokenId,
    isPax: routeAlias === 'pax',
    isWpax: wrappedAddress !== undefined && tokenId === wrappedAddress,
  };
}

export interface TokenDetailWidgetProps {
  tokenId: string;
  onNavigate: (route: AppRoute) => void;
  onTxDetail?: (hash: string) => void;
  onSendToken?: (tokenAddress: string) => void;
}

export function TokenDetailWidget({
  tokenId: routeTokenId,
  onNavigate,
  onTxDetail,
  onSendToken,
}: TokenDetailWidgetProps) {
  const { activeAccount } = useWalletState();
  const { chainId } = useWallet();
  const { tokenId, isPax, isWpax } = resolveTokenDetailIdentity(routeTokenId, chainId);
  const cvSymbol = !isPax && !isWpax ? getCrossverseSymbol(tokenId) : null;

  const [period, setPeriod] = useState<TimePeriod>('1D');

  // ── Data queries ─────────────────────────────────────────────────────
  const portfolioQuery = usePortfolioQuery(activeAccount?.address);
  const paxPriceQuery = usePaxPriceQuery();
  const txHistoryQuery = useTokenDetailHistoryQuery(activeAccount?.address, chainId);
  const tokenMetaQuery = useTokenMetaQuery(tokenId, isPax, isWpax, {
    queryKey: ['tokenMeta', chainId, tokenId, isWpax],
  });
  const cvPriceQuery = useCrossversePriceQuery(cvSymbol);

  const publicMeta = tokenMetaQuery.data?.public ?? null;
  const chainMeta = tokenMetaQuery.data?.chain ?? null;
  const poolAddress = publicMeta?.pool_address ?? null;

  const poolDataQuery = useTokenPoolDataQuery(poolAddress);
  const candlesQuery = useTokenCandlesQuery(tokenId, period, poolAddress, isPax, isWpax, {
    queryKey: ['tokenCandles', chainId, tokenId, period, poolAddress, isPax, isWpax],
  });
  const marketDayQuery = useMarketDayQuery(
    tokenId,
    chainId,
    isPax ? 'PAX' : isWpax ? 'WPAX' : null
  );

  // ── Derived data ─────────────────────────────────────────────────────
  const portfolio = portfolioQuery.data ?? null;
  const paxPrice = paxPriceQuery.data?.latest ?? 0;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const ohlcData = candlesQuery.data ?? [];
  const poolStats = poolDataQuery.data?.stats ?? null;
  const holderDist = poolDataQuery.data?.holders ?? null;

  const loading = portfolioQuery.isPending || (!isPax && tokenMetaQuery.isPending);
  const chartLoading = candlesQuery.isFetching;

  const holding = useMemo(() => {
    if (isPax) return null;
    return (portfolio?.token_holdings || []).find(
      (h: any) => h.contract_address?.toLowerCase() === tokenId.toLowerCase()
    );
  }, [portfolio, tokenId, isPax]);

  const tokenSymbol = isPax
    ? 'PAX'
    : isWpax
      ? 'WPAX'
      : publicMeta?.symbol || holding?.symbol || chainMeta?.symbol || '???';
  const tokenName = isPax
    ? 'Paxeer'
    : isWpax
      ? 'Wrapped PAX'
      : publicMeta?.name || holding?.name || chainMeta?.name || 'Unknown Token';
  const tokenDecimals = isPax
    ? 18
    : supportedTokenDecimals(publicMeta?.decimals ?? holding?.decimals ?? chainMeta?.decimals);

  const balanceRaw = isPax
    ? portfolio?.native_balance?.balance_raw || '0'
    : holding?.balance_raw || '0';
  const balanceNum =
    tokenDecimals === null ? null : Number(formatBalance(balanceRaw, tokenDecimals, 8));

  // Price routing: PAX price > crossverse price > pool stats (WAD) > holding price > fallback.
  const poolPriceUsd = poolStats ? parseFloat(poolStats.price) / 1e18 : 0;
  const cvPrice = cvPriceQuery.data?.latest ?? 0;
  const apiPrice =
    isPax || isWpax
      ? paxPrice
      : cvPrice > 0
        ? cvPrice
        : poolPriceUsd > 0
          ? poolPriceUsd
          : Number(holding?.price_usd || '0');
  const fallbackPrice =
    !isPax && holding?.value_usd && holding?.balance && Number(holding.balance) > 0
      ? Number(holding.value_usd) / Number(holding.balance)
      : 0;
  const priceUsd = apiPrice > 0 ? apiPrice : fallbackPrice;
  const valueUsd = balanceNum === null ? null : balanceNum * priceUsd;

  const chartPoints = useMemo<ChartPoint[]>(() => {
    return ohlcData.map((c) => ({ time: c.timestamp, price: c.close }));
  }, [ohlcData]);

  const periodChange = useMemo<PriceChangeInfo | null>(() => {
    if (chartPoints.length < 2) return null;
    const first = chartPoints[0].price;
    const last = chartPoints[chartPoints.length - 1].price;
    const pct = first > 0 ? ((last - first) / first) * 100 : 0;
    return { pct, dollar: last - first, positive: pct >= 0 };
  }, [chartPoints]);

  // ── Token-scoped activity (filter the global tx history) ─────────────
  const tokenActivity = useMemo<TokenDetailHistoryRow[]>(() => {
    const rows = isPax
      ? (txHistoryQuery.data?.transactions ?? [])
      : (txHistoryQuery.data?.transfers ?? []);
    return rows
      .filter(
        (row) =>
          row.chainId === chainId &&
          (isPax ? row.tokenAddress === null : row.tokenAddress?.toLowerCase() === tokenId)
      )
      .slice(0, 30);
  }, [isPax, txHistoryQuery.data, tokenId, chainId]);

  // ── Pool / public metadata derived values ────────────────────────────
  const marketCap = poolStats?.marketCap ? parseFloat(poolStats.marketCap) / 1e6 : 0;
  const holderCount = poolStats?.holderCount || holderDist?.totalHolders || 0;
  const top10Pct = holderDist?.top10Pct
    ? parseFloat(holderDist.top10Pct) / 100
    : poolStats?.top10Concentration
      ? parseFloat(poolStats.top10Concentration) / 100
      : 0;
  const riskRating = poolStats?.riskRating ?? null;
  const createdAt = publicMeta?.created_at || poolStats?.createdAt || null;
  const totalSupply =
    publicMeta?.total_supply != null
      ? Number(publicMeta.total_supply)
      : chainMeta?.total_supply != null
        ? Number(chainMeta.total_supply)
        : null;
  const isSidioraToken = Boolean(poolAddress);

  // ── Scrub state (drives PriceHero + PriceChart cursor) ───────────────
  const [scrubPoint, setScrubPoint] = useState<ChartPoint | null>(null);
  const isScrubbing = scrubPoint !== null;

  const scrubChange = useMemo<PriceChangeInfo | null>(() => {
    if (!scrubPoint || chartPoints.length < 2) return null;
    const first = chartPoints[0].price;
    const diff = scrubPoint.price - first;
    const pct = first > 0 ? (diff / first) * 100 : 0;
    return { pct, dollar: diff, positive: diff >= 0 };
  }, [scrubPoint, chartPoints]);

  const handleScrubMove = useCallback((p: ChartPoint) => setScrubPoint(p), []);
  const handleScrubLeave = useCallback(() => setScrubPoint(null), []);

  const displayPrice = isScrubbing ? scrubPoint!.price : priceUsd;
  const heroChange = isScrubbing ? scrubChange : periodChange;
  const positive = heroChange?.positive ?? true;

  return (
    <div className="flex flex-col">
      <div className="pb-28">
        <PriceHero
          loading={loading}
          displayPrice={displayPrice}
          scrubTime={scrubPoint?.time ?? null}
          change={heroChange}
          isScrubbing={isScrubbing}
        />

        <PriceChart
          points={chartPoints}
          loading={loading}
          chartLoading={chartLoading}
          positive={positive}
          period={period}
          onPeriodChange={setPeriod}
          onScrubMove={handleScrubMove}
          onScrubLeave={handleScrubLeave}
        />

        <TokenActions
          tokenId={tokenId}
          onNavigate={onNavigate}
          onSendToken={onSendToken}
          precisionKnown={tokenDecimals !== null}
        />

        <div className="grid grid-cols-2 gap-2.5 px-3 mt-4">
          {!loading && balanceNum !== null && valueUsd !== null && (
            <PositionCard balanceNum={balanceNum} valueUsd={valueUsd} tokenSymbol={tokenSymbol} />
          )}

          {isSidioraToken && <PerformanceCard stats={poolStats} />}

          {(isPax || isWpax) && (
            <MarketStatsCard
              paxPrice={paxPrice}
              ohlcData={marketDayQuery.data?.candles ?? []}
              window={marketDayQuery.data?.window ?? null}
            />
          )}

          <SecurityCard top10Pct={top10Pct} riskRating={riskRating} />

          <SocialLinks
            website={publicMeta?.socials?.website ?? null}
            twitter={publicMeta?.socials?.twitter ?? null}
            telegram={publicMeta?.socials?.telegram ?? null}
          />

          <InfoCard
            isPax={isPax}
            tokenId={tokenId}
            tokenName={tokenName}
            tokenSymbol={tokenSymbol}
            tokenDecimals={tokenDecimals}
            totalSupply={totalSupply}
            marketCap={marketCap}
            holderCount={holderCount}
            createdAt={createdAt}
          />

          <ActivityFeed
            rows={tokenActivity}
            walletAddress={activeAccount?.address ?? ''}
            loading={txHistoryQuery.isPending}
            unavailable={chainId !== PAXEER_CONFIG.chainId || txHistoryQuery.isError}
            onTxDetail={onTxDetail}
          />
        </div>
      </div>
    </div>
  );
}
