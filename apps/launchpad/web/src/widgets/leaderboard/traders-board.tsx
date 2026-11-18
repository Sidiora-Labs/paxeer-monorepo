'use client';
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useAccount } from 'wagmi';
import { Users } from 'lucide-react';
import { cn } from '@/lib/utils';
import { queryKeys } from '@/core/query-keys';
import { leaderboardLimit, useLeaderboard, type LeaderboardWindow } from '@/core/market/use-leaderboard';
import { useWalletIdentities } from '@/core/market/use-wallet-identities';
import { Button, EmptyState, ErrorState, SectionHeader, UnavailableState } from '@/ui/primitives';
import { RankRowSkeleton } from '@/ui/shared/skeletons';
import { relativeAge } from '@/utils/format';
import { FollowButton } from '@/widgets/social/follow-button';
import { periodLabels, periodWindows, type RollingPeriod } from './period-control';
import { RankRow } from './rank-row';

export interface TradersBoardProps {
  period: RollingPeriod;
  className?: string;
}

const qualityNotes = { complete: null, partial: 'Still counting', unavailable: 'Not tracked yet' } as const;

/**
 * Rolling trader board over IF_LEADERBOARDS (category trader, window 24h /
 * 7d / 30d, limit 100). Identities come from the batched profile lookup;
 * every wallet keeps its short-address fallback. The endpoint's real 404
 * today renders the unavailable state with a retry and no rows.
 */
export function TradersBoard({ period, className }: TradersBoardProps) {
  const window: LeaderboardWindow = periodWindows[period];
  const [page, setPage] = useState<{ window: LeaderboardWindow; cursor?: string }>({ window });
  const cursor = page.window === window ? page.cursor : undefined;
  const board = useLeaderboard('trader', window, leaderboardLimit, cursor);
  const wallets = board.state === 'ready' ? board.data.data.snapshot.entries.map(entry => entry.wallet) : [];
  const { identities } = useWalletIdentities(wallets);
  const account = useAccount();
  const viewer = account.address?.toLowerCase() ?? null;
  const client = useQueryClient();
  const retry = () => { void client.invalidateQueries({ queryKey: queryKeys.leaderboard('trader', window, leaderboardLimit) }); };
  const empty = board.state === 'ready' && board.data.data.snapshot.entries.length === 0;

  return (
    <section aria-labelledby="traders-board-title" className={cn('min-h-60 rounded-lg bg-surface-1 p-3 sm:p-4', className)}>
      <SectionHeader
        title={<span id="traders-board-title">Top traders</span>}
        description={`${periodLabels[period]}, ranked by profit`}
        loading={board.state === 'loading'}
        className="[&_h2]:min-h-[2lh]"
      />
      <div className="mt-3 grid min-w-0">
        <div aria-hidden={!empty || undefined} inert={!empty || undefined} className={cn('col-start-1 row-start-1 min-w-0', !empty && 'invisible')}>
          <EmptyState icon={Users} title="No traders yet" body={`Top traders from the ${periodLabels[period].toLowerCase()} show up here once people trade`} action={<Button href="/explore" variant="secondary" size="sm">Browse coins</Button>} />
        </div>
        <div className="col-start-1 row-start-1 min-w-0">
          {board.state === 'loading' ? <RankRowSkeleton count={1} /> : null}
          {board.state === 'unsupported' ? (
            <UnavailableState title="Leaderboard" operation={board.operation} status={board.status} message="Trader rankings are coming soon" onRetry={retry} />
          ) : null}
          {board.state === 'error' ? <ErrorState title="Couldn't load the leaderboard" message="Something went wrong, try again" onRetry={board.retry} /> : null}
          {board.state === 'ready' && board.data.data.snapshot.entries.length > 0 ? (
            <>
              <ol className="flex flex-col gap-0.5" aria-label={`Top traders, ${periodLabels[period].toLowerCase()}`}>
                {board.data.data.snapshot.entries.map(entry => {
                  const wallet = entry.wallet.toLowerCase();
                  const identity = identities[wallet];
                  const metric = entry.quality === 'unavailable' ? null : entry.metricSigned ?? entry.metricRaw;
                  return (
                    <RankRow
                      key={wallet}
                      rank={entry.rank}
                      wallet={wallet}
                      identity={identity}
                      metricSigned={metric}
                      qualityNote={qualityNotes[entry.quality]}
                      self={viewer === wallet}
                      actions={<FollowButton wallet={wallet} handle={identity?.displayName ?? null} variant="icon" />}
                    />
                  );
                })}
              </ol>
              <p className="mt-3 font-mono text-2xs text-text-faint">
                Updated {relativeAge(board.data.generatedAt)} ago
              </p>
            </>
          ) : null}
        </div>
      </div>
      {board.state === 'ready' && board.data.nextCursor ? <Button variant="secondary" size="sm" onClick={() => setPage({ window, cursor: board.data.nextCursor ?? undefined })}>Next page</Button> : null}
      {cursor ? <Button variant="ghost" size="sm" onClick={() => setPage({ window })}>First page</Button> : null}
    </section>
  );
}
