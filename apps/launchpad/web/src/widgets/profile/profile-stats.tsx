'use client';

import { surfaceSpacing } from '@/app/presentation-tokens';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { formatCount } from '@/utils/format';
import { Skeleton, UnavailableState } from '@/ui/primitives';
import { useFollowStats } from '@/widgets/social/use-follow';
import { Dash, useProfileIssue } from './profile-issues';

export interface ProfileStatsProps {
  wallet: string;
  /** ISO timestamp; the joined date is shown only when the backend publishes it. */
  createdAt: string | null;
  className?: string;
}

export function formatJoinedDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function Stat({ value, label }: { value: ReactNode; label: string }) {
  return (
    <div className="flex min-w-16 flex-col">
      <dd data-numeric className="order-1 text-sm font-semibold tabular-nums text-text">{value}</dd>
      <dt className="order-2 text-xs text-text-faint">{label}</dt>
    </div>
  );
}

/** Follower and following counts from the social follow graph, plus the joined date when present. */
export default function ProfileStats({ wallet, createdAt, className }: ProfileStatsProps) {
  const stats = useFollowStats(wallet);
  const collected = useProfileIssue('follow-stats', stats.state === 'error' ? { label: 'Follower counts', error: stats.error, retry: stats.retry } : null);
  return (
    <div className={cn('grid min-w-0 grid-cols-[minmax(0,1fr)]', className)}>
      <dl aria-hidden="true" className="invisible col-start-1 row-start-1 flex min-h-10 flex-wrap items-start gap-x-8 gap-y-2 pointer-events-none">
        <Stat value={<Skeleton width={surfaceSpacing(8)} height={surfaceSpacing(5)} />} label="Followers" />
        <Stat value={<Skeleton width={surfaceSpacing(8)} height={surfaceSpacing(5)} />} label="Following" />
      </dl>
      <dl className="col-start-1 row-start-1 flex min-h-10 flex-wrap items-start gap-x-8 gap-y-2">
      {stats.state === 'loading' ? (
        <>
          <Stat value={<Skeleton width={surfaceSpacing(8)} height={surfaceSpacing(5)} />} label="Followers" />
          <Stat value={<Skeleton width={surfaceSpacing(8)} height={surfaceSpacing(5)} />} label="Following" />
        </>
      ) : stats.state === 'ready' ? (
        <>
          <Stat value={formatCount(stats.data.followers)} label={Number(stats.data.followers) === 1 ? 'Follower' : 'Followers'} />
          <Stat value={formatCount(stats.data.following)} label="Following" />
        </>
      ) : stats.state === 'unsupported' ? (
        <div className="flex items-baseline">
          <UnavailableState variant="inline" title="Follower counts" plural operation={stats.operation} status={stats.status} />
        </div>
      ) : collected ? (
        <>
          <Stat value={<Dash label="Followers" />} label="Followers" />
          <Stat value={<Dash label="Following" />} label="Following" />
        </>
      ) : (
        <p role="alert" className="text-xs text-danger">Can't load followers right now</p>
      )}
      {createdAt ? <Stat value={formatJoinedDate(createdAt)} label="Joined" /> : null}
      </dl>
    </div>
  );
}
