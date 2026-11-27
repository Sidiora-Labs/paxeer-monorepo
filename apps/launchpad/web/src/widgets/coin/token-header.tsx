'use client';
import { displayCoinText } from '@/utils/coin-text';
import './coin-market.css';
import { surfaceSpacing } from '@/app/presentation-tokens';
import { useState, type ReactNode } from 'react';
import { ArrowLeftRight, AtSign, Coins, Copy, Globe, Link as LinkIcon, MessageCircle, Send, Share2, Star } from 'lucide-react';
import { toast } from 'sonner';
import { AgeChip, Avatar, DropdownMenu, IconButton, ShortAddress, Skeleton, UnavailableState } from '@/ui/primitives';
import { usePoolSnapshot } from '@/core/market/use-pool-snapshot';
import type { PoolStats } from '@/core/market/pool-resolver';
import { formatShare, formatSpotPrice, statsChange, statsCirculatingMarketCap, statsMarketCap, statsShare, statsSpotPrice, type StatValue } from '@/core/market/pool-stats-display';
import { StatChange, StatDash } from '@/ui/shared/stat-value';
import { useTokenMetadata } from '@/hooks/market/use-token-metadata';
import { safeExternalDestination } from '@/core/app-foundation/navigation';
import { useWatchlist } from '@/hooks/ui/use-watchlist';
import { ReportMenu } from '@/widgets/social/report-menu';
import { safeSocialUrl } from '@/widgets/trading/token-description';
import { formatCount, formatPrice, shortAddress, formatCompactUsd } from '@/utils/format';

type Headline = 'mcap' | 'price';

export function tokenHeaderPrice(stats: PoolStats | null, livePrice: number | null): StatValue<string> {
  const spot = statsSpotPrice(stats);
  if (spot.value !== null) return { value: formatSpotPrice(spot.value), reason: spot.reason };
  if (livePrice !== null && Number.isFinite(livePrice)) return { value: formatPrice(livePrice), reason: null };
  return { value: null, reason: spot.reason };
}

function socialIcon(label: string, url: string) {
  const key = `${label} ${url}`.toLowerCase();
  if (/x\.com|twitter/.test(key)) return AtSign;
  if (/t\.me|telegram/.test(key)) return Send;
  if (/discord/.test(key)) return MessageCircle;
  if (/web|site|home/.test(key)) return Globe;
  return LinkIcon;
}

function socialHref(url: string): string | null {
  return safeExternalDestination(url) ?? safeSocialUrl(url);
}

async function shareCurrentPage(title: string) {
  const url = window.location.href;
  try {
    if (typeof navigator.share === 'function') { await navigator.share({ title, url }); return; }
    await navigator.clipboard.writeText(url);
    toast.success('Link copied');
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return;
    toast.error("Couldn't share this link, try again");
  }
}

export interface TokenHeaderProps {
  poolAddress: string;
  /** Latest stats from the page's single live market session; the snapshot row is the fallback. */
  liveStats: PoolStats | null;
  /** Live quote price in USDL per token, already reduced to a number. */
  livePrice: number | null;
  now: number;
  /** Compact status chip (market freshness) shown beside the value; never a page-wide banner. */
  notice?: ReactNode;
  noTradesYet?: boolean;
}

const headerClass = 'coin-header grid min-w-0 gap-3 p-3';

function explorerHref(address: string): string | null {
  return safeExternalDestination(`https://paxscan.paxeer.app/address/${address}`);
}

/** Compact header statistic with its label above its value. */
function HeaderStat({ label, value, unit }: { label: string; value: string | null; unit?: string }) {
  return (
    <div className="flex min-w-0 flex-col">
      <dt className="text-2xs font-medium uppercase tracking-wider text-text-faint">{label}</dt>
      <dd data-numeric className={`truncate text-sm font-bold tabular-nums ${value ? 'text-text' : 'text-text-faint'}`}>
        {value === null ? <span aria-label="No data">—</span> : <>{value}{unit ? <span className="ml-1 text-xs font-normal text-text-faint">{unit}</span> : null}</>}
      </dd>
    </div>
  );
}

/** Headline valuation (market cap or price, toggled) plus the 24h volume and holder count; every value may be null and renders as a dash. */
function ValuationCluster({ headline, onToggle, value, reason, secondary, change, volume24h, holders, notice }: { headline: Headline; onToggle: () => void; value: string | null; reason: string | null; secondary?: string | null; change: StatValue<number> | null; volume24h: string | null; holders: string | null; notice?: ReactNode }) {
  const label = headline === 'mcap' ? 'Market cap' : 'Price';
  return (
    <div data-coin-header-row="value" className="coin-header-values relative flex min-w-0 items-center gap-5 overflow-x-auto whitespace-nowrap">
      <div className="flex shrink-0 items-center gap-2">
        <button
          type="button"
          onClick={onToggle}
          aria-label={`${label} ${value ?? `not available yet${reason ? `: ${reason}` : ''}`}. Switch to ${headline === 'mcap' ? 'price' : 'market cap'}`}
          title={value === null ? reason ?? undefined : secondary ?? undefined}
          className="-mx-1 flex shrink-0 flex-col items-end rounded-sm px-1 text-right transition-colors hover:bg-surface-hover"
        >
          <span className="flex items-center gap-1 text-2xs font-medium uppercase tracking-wider text-text-faint">{label}<ArrowLeftRight aria-hidden="true" className="size-3" /></span>
          <span data-numeric data-headline={headline} className={`text-xl font-bold tabular-nums ${value ? 'text-text' : 'text-text-faint'}`}>{value ?? '—'}</span>
        </button>
        {change ? <span data-change="24h" className="pb-1"><StatChange change={change} variant="pill" arrow size="sm" align="start" decimals={2} /></span> : null}
      </div>
      {notice}
      <dl className="flex shrink-0 items-end gap-5">
        <HeaderStat label="24h vol" value={volume24h} />
        <HeaderStat label="Holders" value={holders} />
      </dl>
    </div>
  );
}

export default function TokenHeader({ poolAddress, liveStats, livePrice, now, notice, noTradesYet = false }: TokenHeaderProps) {
  const snapshot = usePoolSnapshot(poolAddress);
  const watchlist = useWatchlist();
  const [headline, setHeadline] = useState<Headline>('mcap');
  const data = snapshot.data ?? null;
  const token = data?.token ?? null;
  const stats = liveStats ?? data?.stats ?? null;
  const image = useTokenMetadata(data?.pool.tokenAddress ?? null);
  const logo = image.data?.images?.logo ?? null;

  if (snapshot.isPending) {
    return (
      <header data-coin-header="loading" aria-busy="true" className="grid min-h-18 gap-3 p-3">
        <Skeleton variant="block" width={surfaceSpacing(16)} height={surfaceSpacing(16)} label="coin image" />
        <div className="flex min-w-0 flex-1 flex-col gap-2 [&>*]:max-w-full"><Skeleton variant="text" width={surfaceSpacing(40)} /><Skeleton variant="text" width={surfaceSpacing(60)} /></div>
      </header>
    );
  }
  if (snapshot.isError || !data) {
    // The snapshot failed but the URL already names the pool: keep the header frame (identity row,
    // address menu, watch/share, valuation row) and degrade only the values to dashes.
    const status = (snapshot.error as { status?: unknown } | null)?.status;
    const starredByUrl = watchlist.check(poolAddress);
    return (
      <header data-coin-header="degraded" aria-label="Coin header" className={headerClass}>
        <div data-coin-header-row="identity" className="flex min-w-0 items-center gap-3">
          <span aria-hidden="true" className="flex size-avatar-lg shrink-0 items-center justify-center rounded-md bg-surface-3 text-text-faint"><Coins className="size-6" /></span>
          <h1 className="min-w-0 flex-1 truncate text-lg font-semibold text-text" title={poolAddress}><span className="sr-only">Coin </span>{shortAddress(poolAddress)}</h1>
          <span className="hidden md:inline-flex"><ShortAddress address={poolAddress} explorerHref={explorerHref(poolAddress)} onCopied={() => toast.success('Address copied')} /></span>
          <IconButton label={starredByUrl ? 'Remove from watchlist' : 'Add to watchlist'} pressed={starredByUrl} icon={<Star className={starredByUrl ? 'fill-current text-accent' : undefined} />} size="sm" onClick={() => watchlist.toggle(poolAddress)} />
          <span className="hidden md:inline-flex"><IconButton label="Share coin link" icon={<Share2 />} size="sm" onClick={() => { void shareCurrentPage('Coin on Sidiora'); }} /></span>
          <DropdownMenu label="Coin address" size="sm" items={[
            { id: 'address', label: 'View pool address', href: explorerHref(poolAddress) ?? undefined },
            { id: 'share', label: 'Share coin link', icon: <Share2 />, onSelect: () => { void shareCurrentPage('Coin on Sidiora'); } },
          ]} />
        </div>
        <ValuationCluster headline={headline} onToggle={() => setHeadline(current => (current === 'mcap' ? 'price' : 'mcap'))} value={null} reason={null} change={null} volume24h={null} holders={null} notice={<div className="flex shrink-0 items-center gap-2">{notice}<UnavailableState variant="chip" wrap title="This coin" plural="auto" operation="IF_POOL_DETAIL" status={typeof status === 'number' ? status : null} onRetry={() => { void snapshot.refetch(); }} /></div>} />
      </header>
    );
  }

  const symbol = displayCoinText(data.metadata?.displaySymbol ?? token?.symbol, data.pool.tokenAddress);
  const name = displayCoinText(data.metadata?.displayName ?? token?.name, data.pool.tokenAddress);
  const pool = data.pool.poolAddress;
  const starred = watchlist.check(pool);
  // The headline uses total supply; the circulating valuation is available on hover.
  const mcapStat = statsMarketCap(stats);
  const circulating = statsCirculatingMarketCap(stats).value;
  const mcap = mcapStat.value === null ? null : formatCompactUsd(mcapStat.value, { raw: true, compact: true });
  const price = tokenHeaderPrice(stats, livePrice);
  const change24h = stats ? statsChange(stats, '24h') : null;
  const volume24h = stats ? formatCompactUsd(stats.volume24hRaw, { raw: true, compact: true }) : null;
  const holders = stats?.holderCount !== null && stats?.holderCount !== undefined ? formatCount(stats.holderCount) : null;
  const devShare = stats ? statsShare(stats, 'creator') : null;
  const socials = (data.metadata?.socialLinks ?? []).map(link => ({ ...link, href: socialHref(link.url) })).filter(link => link.href !== null);
  const headlineValue = headline === 'mcap' ? mcap : price.value;
  const headlineReason = headline === 'mcap' ? mcapStat.reason : price.reason;

  return (
    <header data-coin-header="ready" aria-label={`${symbol} header`} className={headerClass}>
      <div data-coin-header-row="identity" className="flex min-w-0 items-center gap-3">
        <Avatar src={logo} name={symbol} size="lg" shape="square" decorative />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <h1 title={name} className="truncate text-lg font-semibold text-text">{name}</h1>
          <div className="flex min-w-0 items-center gap-2 text-sm text-text-muted">
            <span className="truncate" title={symbol}>{symbol}</span>
            <span className="hidden shrink-0 md:inline-flex">{data.createdAt ? <AgeChip iso={data.createdAt} now={now} /> : null}</span>
            {devShare ? <span data-dev-share className="hidden shrink-0 items-center gap-1 text-xs tabular-nums md:inline-flex">Creator {devShare.value !== null ? formatShare(devShare.value) : <StatDash reason={devShare.reason ?? 'Not available yet'} subject="Creator share" align="start" />}</span> : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-0.5">
          <IconButton label={starred ? 'Remove from watchlist' : 'Add to watchlist'} pressed={starred} icon={<Star className={starred ? 'fill-current text-accent' : undefined} />} size="sm" onClick={() => watchlist.toggle(pool)} />
          <span className="hidden md:inline-flex"><IconButton label="Share coin link" icon={<Share2 />} size="sm" onClick={() => { void shareCurrentPage(`${symbol} on Sidiora`); }} /></span>
          <ReportMenu subject={{ targetType: 'token', targetId: data.pool.tokenAddress, label: symbol }} size="sm" items={[
            { id: 'share', label: 'Share coin link', icon: <Share2 />, onSelect: () => { void shareCurrentPage(`${symbol} on Sidiora`); } },
            { id: 'copy-address', label: 'Copy coin address', icon: <Copy />, onSelect: () => { void navigator.clipboard.writeText(data.pool.tokenAddress).then(() => toast.success('Address copied')).catch(() => toast.error("Couldn't copy address, try again")); } },
            ...(explorerHref(data.pool.tokenAddress) ? [{ id: 'explorer', label: 'View coin address', icon: <LinkIcon />, href: explorerHref(data.pool.tokenAddress)! }] : []),
            ...socials.map(link => { const Icon = socialIcon(link.label, link.url); return { id: link.url, label: link.label, icon: <Icon />, href: link.href! }; }),
          ]} />
        </div>
      </div>
      <ValuationCluster headline={headline} onToggle={() => setHeadline(current => (current === 'mcap' ? 'price' : 'mcap'))} value={headlineValue} reason={headlineReason} secondary={headline === 'mcap' && circulating ? `Circulating market cap ${formatCompactUsd(circulating, { raw: true, compact: true })}` : null} change={noTradesYet ? null : change24h} volume24h={volume24h} holders={holders} notice={notice} />
    </header>
  );
}
