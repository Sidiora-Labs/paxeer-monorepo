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
import { RankingTable } from './ranking-table';

export interface TradersBoardProps {
  period: RollingPeriod;
  className?: string;
}

const qualityNotes = { complete: null, partial: 'Still counting', unavailable: 'Not tracked yet' } as const;

/** Rolling trader rankings retain wallet identities, profit quality and pagination. */
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
    <section aria-labelledby="traders-board-title" className={cn('min-w-0', className)}>
      <SectionHeader
        title={<span id="traders-board-title">Top traders</span>}
        description={`${periodLabels[period]}, ranked by profit`}
        loading={board.state === 'loading'}
      />
      <div className="mt-3 min-w-0">
        <RankingTable label={`Top traders, ${periodLabels[period].toLowerCase()}`} state={
          board.state === 'loading' ? <RankRowSkeleton count={1} /> :
          board.state === 'unsupported' ? <UnavailableState title="Leaderboard" operation={board.operation} status={board.status} notServed={board.notServed} error={board.error} message="Trader rankings are coming soon" onRetry={board.retry ?? retry} /> :
          board.state === 'error' ? <ErrorState title="Couldn't load the leaderboard" message="Something went wrong, try again" onRetry={board.retry} /> :
          empty ? <EmptyState icon={Users} title="No traders yet" body={`Top traders from the ${periodLabels[period].toLowerCase()} show up here once people trade`} action={<Button href="/explore" variant="secondary" size="sm">Browse coins</Button>} /> : null
        }>
          {board.state === 'ready' ? board.data.data.snapshot.entries.map(entry => {
            const wallet = entry.wallet.toLowerCase();
            const identity = identities[wallet];
            const metric = entry.quality === 'unavailable' ? null : entry.metricSigned ?? entry.metricRaw;
            return <RankRow key={wallet} rank={entry.rank} wallet={wallet} identity={identity} metricSigned={metric} qualityNote={qualityNotes[entry.quality]} self={viewer === wallet} actions={<FollowButton wallet={wallet} handle={identity?.displayName ?? null} variant="icon" />} />;
          }) : null}
        </RankingTable>
        {board.state === 'ready' && !empty ? <p className="mt-3 font-mono text-2xs tabular-nums text-text-faint">Updated {relativeAge(board.data.generatedAt)} ago</p> : null}
      </div>
      {board.state === 'ready' && board.data.nextCursor ? <Button variant="secondary" size="sm" onClick={() => setPage({ window, cursor: board.data.nextCursor ?? undefined })}>Next page</Button> : null}
      {cursor ? <Button variant="ghost" size="sm" onClick={() => setPage({ window })}>First page</Button> : null}
    </section>
  );
}
