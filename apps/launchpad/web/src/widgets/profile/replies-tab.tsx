'use client';

import Link from 'next/link';
import { MessageSquare } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { poolPath } from '@/core/app-foundation/navigation';
import { loadable } from '@/core/market/unsupported';
import { readWalletComments } from '@/widgets/social/wallet-comments';
import { Button, EmptyState, ErrorState, Skeleton, UnavailableState } from '@/ui/primitives';
import { formatCount, shortAddress } from '@/utils/format';
import { CollectedPlaceholder, useProfileIssue } from './profile-issues';

export interface RepliesTabProps {
  wallet: string;
  /** The signed-in owner gets a call to action on an empty list; visitors get neutral copy. */
  isOwner?: boolean;
}

const repliesOperation = 'GET /users/{wallet}/comments (media/social)';
const REPLIES_LIMIT = 50;

/** The wallet's comments across coin threads, from media/social; its real 404 renders the unavailable state. */
function RepliesContent({ wallet, isOwner = false }: RepliesTabProps) {
  const address = wallet.toLowerCase();
  const query = useQuery({
    queryKey: queryKeys.profileReplies(address),
    queryFn: ({ signal }) => readWalletComments(address, signal),
    enabled: /^0x[0-9a-f]{40}$/.test(address),
    ...marketCachePolicy.profileReplies,
  });
  const state = loadable(query, repliesOperation);
  const collected = useProfileIssue('replies', state.state === 'error' ? { label: 'Replies', error: state.error, retry: state.retry } : null);

  if (state.state === 'loading') return <Skeleton variant="rows" rows={4} label="replies" />;
  if (state.state === 'unsupported') {
    return <UnavailableState title="Replies" plural operation={state.operation} status={state.status} notServed={state.notServed} error={state.error} message="Check back in a bit" onRetry={() => { void query.refetch(); }} />;
  }
  if (state.state === 'error' && collected) return <CollectedPlaceholder label="Replies" />;
  if (state.state === 'error') return <ErrorState title="Can't load replies right now" message="Check your connection and try again" onRetry={state.retry} />;
  const { data: { items: comments }, nextCursor } = state.data;
  if (comments.length === 0) {
    return (
      <EmptyState
        icon={MessageSquare}
        title="No replies yet"
        body={isOwner ? 'Replies you post on coins show up here' : "This wallet hasn't replied to any coins yet"}
        action={isOwner ? <Button href="/explore" variant="secondary" size="sm">Browse coins</Button> : undefined}
      />
    );
  }
  return (
    <section aria-label="Replies" className="flex flex-col bg-surface-1">
      <ul className="flex flex-col">
        {comments.map(comment => (
          <li key={comment.commentId} className="sl-hairline px-3 py-2">
            <div className="flex items-center justify-between gap-3 text-xs text-text-faint">
              <Link prefetch={false} href={poolPath(125, comment.poolAddress)} className="truncate font-mono text-text-muted hover:text-text">{comment.parentCommentId ? 'Replied on' : 'Posted on'} {shortAddress(comment.poolAddress)}</Link>
              <span className="shrink-0 tabular-nums">{BigInt(comment.likeCount) > 0n ? `${formatCount(comment.likeCount)} likes` : ''}</span>
            </div>
            <p className="mt-1 whitespace-pre-wrap break-words text-sm text-text">{comment.body}</p>
          </li>
        ))}
      </ul>
      {nextCursor ? <p className="px-3 py-2 text-xs text-text-faint">Showing the latest {REPLIES_LIMIT} replies.</p> : null}
    </section>
  );
}

export function RepliesTab(props: RepliesTabProps) {
  return <RepliesContent {...props} />;
}
