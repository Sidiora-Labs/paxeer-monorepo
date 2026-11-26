'use client';
import { displayCoinText } from '@/utils/coin-text';
import Link from 'next/link';
import { cn } from '@/lib/utils';
import { Avatar } from '@/ui/primitives';
import type { LiveDirectoryEntry } from './use-live-directory';
import { LiveBadge } from './live-badge';
import { ViewerCountChip } from './viewer-count-chip';
import { MarketCapChip } from './market-cap-chip';
import { StreamSparkline } from './stream-sparkline';
import { coinLabel, streamHref } from './stream-card';

export interface HeroMetaPanelProps {
  entry: LiveDirectoryEntry;
  creatorLabel: string;
  className?: string;
}

/** Meta column beside the hero player: status, title, coin row with market cap and creator, and (from lg) the 24h price line. It shares the hero frame's surface. */
export function HeroMetaPanel({ entry, creatorLabel, className }: HeroMetaPanelProps) {
  const href = streamHref(entry);
  const name = coinLabel(entry);
  const symbol = displayCoinText(entry.symbol, entry.tokenAddress ?? entry.poolAddress);
  return (
    <div className={cn('flex min-w-0 flex-col gap-3 p-4 lg:justify-center lg:p-5', className)}>
      <div className="flex flex-wrap items-center gap-2">
        <LiveBadge size="md" />
        <ViewerCountChip count={entry.viewerCount} quality={entry.viewerQuality} size="md" />
      </div>
      <h2 className="line-clamp-2 break-words text-md font-semibold text-text">
        <Link prefetch={false} href={href} className="rounded-xs hover:underline">{entry.stream.title}</Link>
      </h2>
      <Link prefetch={false} href={href} className="-mx-2 flex min-w-0 items-center gap-3 rounded-md p-2 transition-colors duration-fast hover:bg-surface-hover">
        <Avatar src={entry.imageUrl} name={symbol} size="xl" shape="square" decorative />
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className="min-w-0 truncate text-base font-semibold text-text" title={symbol}>{symbol}</span>
            <MarketCapChip raw={entry.mcapRaw} className="bg-surface-2" />
          </span>
          <span className="block truncate text-xs text-text-faint" title={`${name} by ${creatorLabel}`}>
            <span className="text-text-muted">{name}</span>{' by '}
            {creatorLabel}
          </span>
        </span>
      </Link>
      <div className="hidden min-w-0 lg:block">
        <p className="sr-only">24h price</p>
        <StreamSparkline poolAddress={entry.poolAddress} variant="hero" />
      </div>
    </div>
  );
}
