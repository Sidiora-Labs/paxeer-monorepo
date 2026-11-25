'use client';
import type { ReactNode } from 'react';
import Link from 'next/link';
import { cn } from '@/lib/utils';
import type { WalletIdentity } from '@/core/market/use-wallet-identities';
import { Avatar, RankBadge } from '@/ui/primitives';
import { formatUsdl, shortAddress } from '@/utils/format';
import { metricTone } from './cups/cups-contract';

export interface RankRowProps {
  rank: number;
  wallet: string;
  identity?: WalletIdentity;
  /** Signed raw USDL (6 decimals) metric; null renders an em dash with the quality note. */
  metricSigned: string | null;
  /** Spoken label for the metric column, e.g. "Profit". */
  metricLabel?: string;
  /** Raw USDL prize for the rank (cup results); omitted on rolling boards. */
  prizeRaw?: string | null;
  /** Row belongs to the signed-in viewer. */
  self?: boolean;
  size?: 'md' | 'lg';
  /** Secondary controls (Follow) rendered outside the row link. */
  actions?: ReactNode;
  qualityNote?: string | null;
  className?: string;
}

export function profileHref(wallet: string): string {
  return `/profile/${wallet.toLowerCase()}`;
}

const toneClass = { success: 'text-success', danger: 'text-danger', neutral: 'text-text' } as const;

/** Ranked trader row with a separate follow control and right-aligned figures. */
export function RankRow({ rank, wallet, identity, metricSigned, metricLabel = 'Profit', prizeRaw = null, self = false, size = 'md', actions, qualityNote = null, className }: RankRowProps) {
  const address = wallet.toLowerCase();
  const handle = identity?.displayName?.trim() || null;
  const short = identity?.shortAddress ?? shortAddress(address, 6, 4);
  const tone = metricSigned === null ? 'neutral' : metricTone(metricSigned);
  const large = size === 'lg';
  return (
    <tr className={cn('sl-hairline relative transition-colors duration-fast hover:bg-surface-1', large ? 'h-list-row-lg' : 'h-12', self && 'bg-surface-2', className)} data-rank={rank} data-self={self || undefined}>
      <td className="px-1 align-middle"><RankBadge rank={rank} className="justify-center" /></td>
      <td className="min-w-0 py-1 align-middle">
        <div className="flex min-w-0 items-center gap-3">
          <Avatar src={identity?.avatarUrl ?? null} name={handle ?? short} size={large ? 'xl' : 'md'} decorative />
          <div className="min-w-0 flex-1">
            <Link prefetch={false} href={profileHref(address)} title={handle ?? short} className="block min-w-0 text-md font-semibold leading-[var(--sl-leading-base)] text-text after:absolute after:inset-0 after:content-['']">
              <span className="block truncate">{handle ?? short}</span>
            </Link>
            {handle ? <p className="truncate font-mono text-xs text-text-muted">{short}</p> : null}
            {self ? <span className="text-2xs font-semibold uppercase tracking-wide text-text-muted">You</span> : null}
          </div>
        </div>
      </td>
      <td className="px-2 py-1 text-right align-middle">
        <span className={cn('text-num-md font-semibold tabular-nums', toneClass[tone])} data-numeric>
          <span className="sr-only">{metricLabel} </span>
          {metricSigned === null ? <span aria-label="no data yet">&mdash;</span> : formatUsdl(metricSigned, { signed: true })}
        </span>
        {qualityNote ? <span className="block text-2xs text-text-faint">{qualityNote}</span> : null}
        {prizeRaw !== null ? <span className="block text-xs tabular-nums text-text-muted" data-numeric><span>Prize </span>{formatUsdl(prizeRaw, { fractionDigits: 0 })}</span> : null}
      </td>
      <td className="align-middle"><div className="relative z-10 flex justify-end">{actions}</div></td>
    </tr>
  );
}
