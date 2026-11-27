'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { cn } from '@/lib/utils';
import { displayCoinText } from '@/utils/coin-text';
import { formatCompactUsd } from '@/utils/format';
import { Avatar } from './avatar';
import { AgeChip, DirectionChip } from './badges';

export interface CoinRowProps {
  href: string;
  image: string | null;
  name: string;
  symbol: string;
  tokenAddress?: string | null;
  createdAt: string | number | null;
  mcapRaw: string | bigint | null;
  mcapDecimals?: number;
  percentChange: number | null;
  badges?: ReactNode;
  loading?: boolean;
  className?: string;
}

export function CoinRow({
  href, image, name: rawName, symbol: rawSymbol, tokenAddress, createdAt,
  mcapRaw, mcapDecimals = 6, percentChange, badges, loading = false, className,
}: CoinRowProps) {
  const name = displayCoinText(rawName, tokenAddress);
  const symbol = displayCoinText(rawSymbol, tokenAddress);
  const classes = cn('sl-hairline flex min-h-list-row min-w-0 items-center gap-3 px-3 py-2', className);
  if (loading) {
    return (
      <div aria-busy="true" className={classes}>
        <span className="sr-only">Loading coin</span>
        <Avatar src={null} name={name} shape="square" size="lg" loading decorative />
        <span aria-hidden="true" className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="sl-skeleton h-4 w-2/3" />
          <span className="sl-skeleton h-4 w-1/3" />
        </span>
        <span aria-hidden="true" className="sl-skeleton h-8 w-16" />
      </div>
    );
  }
  return (
    <Link prefetch={false} href={href} className={cn(
      classes, 'transition-colors duration-fast hover:bg-surface-1',
      percentChange !== null && percentChange > 0 && 'sl-row-wash-up',
      percentChange !== null && percentChange < 0 && 'sl-row-wash-down'
    )}>
      <Avatar src={image} name={name} shape="square" size="lg" decorative />
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-1">
          <span className="truncate text-md font-semibold text-text" title={name}>{name}</span>
          {badges}
        </span>
        <span className="flex min-w-0 items-center gap-2 text-xs text-text-muted">
          <span className="truncate" title={symbol}>${symbol}</span>
          <AgeChip iso={createdAt} className="shrink-0 text-text-muted" />
        </span>
      </span>
      <span className="flex shrink-0 flex-col items-end gap-0.5 text-right">
        <span data-numeric className="text-num-md font-bold tabular-nums text-text">
          {mcapRaw === null ? '—' : formatCompactUsd(mcapRaw, { raw: true, decimals: mcapDecimals, compact: true })}
        </span>
        <DirectionChip value={percentChange} />
      </span>
    </Link>
  );
}
