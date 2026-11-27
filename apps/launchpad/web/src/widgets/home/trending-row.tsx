'use client';
import { launchpadGeometry } from '@/app/presentation-tokens';

import { displayCoinText } from '@/utils/coin-text';
import Link from 'next/link';
import { Globe, MessageCircle, Star, type LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';
import { AgeChip, Avatar, StageBadge, type Stage } from '@/ui/primitives';
import { StatChange } from '@/ui/shared/stat-value';
import { formatSpotPrice } from '@/core/market/pool-stats-display';
import { formatCompactUsd } from '@/utils/format';
import { safeExternalDestination } from '@/core/app-foundation/navigation';
import type { ColumnDensity } from './layout-store';

export interface SocialLink {
  kind: 'website' | 'twitter' | 'telegram';
  url: string;
}

export interface TrendingRowData {
  tokenAddress?: string | null;
  poolAddress: string;
  href: string;
  rank: number;
  name: string;
  symbol: string;
  image: string | null;
  createdAt?: number | null;
  /** Exact decimal USDL price per whole token (PoolStats.spotPriceWad); null before the first trade. */
  price: string | null;
  priceReason: string | null;
  /** Market cap (PoolStats.totalSupplyValuationUsdlRaw), raw USDL. */
  mcapRaw: string | null;
  mcapReason: string | null;
  /** Percent change for the selected timeframe, only on a complete baseline. */
  change: number | null;
  /** Why `change` is null: no v2 window, not enough history, no stats. */
  changeReason: string | null;
  socials: SocialLink[];
  stage: Stage | null;
}

export interface TrendingRowProps {
  row: TrendingRowData;
  density?: ColumnDensity;
  starred?: boolean;
  onToggleStar?: (next: boolean) => void;
  flash?: 'up' | 'down' | null;
  className?: string;
}

const socialIcon: Record<SocialLink['kind'], { Icon: LucideIcon; label: string }> = {
  website: { Icon: Globe, label: 'Website' },
  twitter: { Icon: MessageCircle, label: 'X profile' },
  telegram: { Icon: MessageCircle, label: 'Telegram' },
};

/** Keep only socials that pass the app's external destination allowlist. */
export function safeSocials(socials: { website?: string | null; twitter?: string | null; telegram?: string | null } | null | undefined): SocialLink[] {
  if (!socials) return [];
  const out: SocialLink[] = [];
  for (const kind of ['website', 'twitter', 'telegram'] as const) {
    const raw = socials[kind];
    if (!raw) continue;
    const safe = safeExternalDestination(raw);
    if (safe) out.push({ kind, url: safe });
  }
  return out;
}

export function changeWash(change: number | null): string {
  if (change === null || !Number.isFinite(change) || change === 0) return '';
  return change > 0 ? 'sl-row-wash-up' : 'sl-row-wash-down';
}

/**
 * Trending coin row (components.md coin row): image with stage badge, name and
 * symbol and age, market cap and direction. Price remains available to assistive
 * technology, with social destinations beside the metadata on wider screens.
 */
export function TrendingRow({ row, density = 'comfortable', starred = false, onToggleStar, flash = null, className }: TrendingRowProps) {
  row = { ...row, name: displayCoinText(row.name, row.tokenAddress ?? row.poolAddress), symbol: displayCoinText(row.symbol, row.tokenAddress ?? row.poolAddress) };
  const compact = density === 'compact';
  return (
    <li
      className={cn(
        'sl-hairline relative flex flex-wrap items-center gap-x-3 gap-y-2 px-3 hover:bg-surface-1',
        compact ? 'min-h-list-row' : 'min-h-list-row-lg',
        changeWash(row.change),
        className,
      )}
      data-pool={row.poolAddress}
    >
      <Avatar src={row.image} name={row.name} size={compact ? 'sm' : 'lg'} shape="square" decorative badge={row.stage && !compact ? <StageBadge stage={row.stage} size={launchpadGeometry.smallBadge} surface={1} /> : undefined} />
      <div className="flex min-w-touch-target-min flex-1 flex-col gap-0.5">
        <div className="flex min-w-0 items-baseline gap-1.5">
          <Link prefetch={false} href={row.href} title={row.name} className="truncate text-sm font-semibold text-text after:absolute after:inset-0 after:content-['']">
            {row.name}
          </Link>
        </div>
        <div className="flex min-w-0 items-center gap-2 text-xs text-text-muted">
          <span className="truncate" title={row.symbol}>{row.symbol}</span>
          <AgeChip iso={row.createdAt ?? null} className="shrink-0 text-text-muted" />
          <span className="sr-only">
            {row.price === null ? `Price: ${row.priceReason ?? 'No price yet'}` : `Price ${formatSpotPrice(row.price)}`}
          </span>
          {row.socials.length > 0 ? (
            <span className="relative z-10 hidden items-center gap-1 md:inline-flex">
              {row.socials.map(({ kind, url }) => {
                const { Icon, label } = socialIcon[kind];
                return (
                  <a key={kind} href={url} target="_blank" rel="noopener noreferrer" aria-label={`${row.name} ${label}`} className="inline-flex size-4 items-center justify-center rounded-xs text-text-faint hover:bg-surface-hover hover:text-text">
                    <Icon aria-hidden="true" className="size-3" />
                  </a>
                );
              })}
            </span>
          ) : null}
        </div>
      </div>
      <div className="ml-auto flex shrink-0 flex-col items-end gap-0.5 whitespace-nowrap">
        {row.mcapRaw
          ? <span data-numeric className="text-sm font-bold tabular-nums text-text">{`${formatCompactUsd(row.mcapRaw, { raw: true, compact: true })} MC`}</span>
          : <span data-numeric title={row.mcapReason ?? 'No market cap yet'} aria-label={`Market cap: ${row.mcapReason ?? 'No market cap yet'}`} className="text-sm tabular-nums text-text-faint">—</span>}
        {row.change === null ? (
          <span data-numeric title={row.changeReason ?? 'No change yet'} aria-label={`Change: ${row.changeReason ?? 'No change yet'}`} className="text-xs tabular-nums text-text-faint">—</span>
        ) : (
          <StatChange change={{ value: row.change, reason: row.changeReason }} variant="pill" arrow flash={flash} />
        )}
      </div>
      {onToggleStar ? (
        <button
          type="button"
          aria-label={starred ? `Remove ${row.name} from watchlist` : `Add ${row.name} to watchlist`}
          aria-pressed={starred}
          onClick={() => onToggleStar(!starred)}
          className={cn('relative z-10 inline-flex size-7 shrink-0 items-center justify-center rounded-sm hover:bg-surface-hover', starred ? 'text-text' : 'text-text-faint hover:text-text')}
        >
          <Star aria-hidden="true" className="size-icon-sm" fill={starred ? 'currentColor' : 'none'} />
        </button>
      ) : null}
    </li>
  );
}

export function TrendingRowSkeleton({ density = 'comfortable', count = 8 }: { density?: ColumnDensity; count?: number }) {
  const compact = density === 'compact';
  return (
    <ul aria-busy="true" className="flex flex-col">
      <li className="sr-only">Loading coins</li>
      {Array.from({ length: count }, (_, index) => (
        <li key={index} aria-hidden="true" className={cn('flex items-center gap-3 px-3', compact ? 'h-list-row' : 'h-list-row-lg')}>
          <span className={cn('sl-skeleton shrink-0 rounded-sm', compact ? 'size-avatar-sm' : 'size-avatar-lg')} />
          <span className="flex flex-1 flex-col gap-1.5">
            <span className="sl-skeleton h-3 w-28" />
            <span className="sl-skeleton h-2.5 w-16" />
          </span>
          <span className="flex flex-col items-end gap-1.5">
            <span className="sl-skeleton h-3 w-14" />
            <span className="sl-skeleton h-2.5 w-10" />
          </span>
        </li>
      ))}
    </ul>
  );
}
