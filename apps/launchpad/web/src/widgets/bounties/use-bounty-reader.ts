'use client';
import { useEffect, useSyncExternalStore } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useGatewayOperations, useGatewaySessionOperations } from '@/core/market/operations';
import { accountEpoch, accountSession, assertAccount, subscribeAccount } from '@/widgets/profile/account-transport';
import { type BountyListQuery } from './bounties-contract';

const serverEpoch = () => 0;

/** Drafts and their submissions are visible only to the authenticated creator. */
export function useBountyReader() {
  const epoch = useSyncExternalStore(subscribeAccount, accountEpoch, serverEpoch);
  const session = accountSession();
  const publicOperations = useGatewayOperations();
  const privateOperations = useGatewaySessionOperations();
  const client = useQueryClient();
  useEffect(() => subscribeAccount(() => {
    const stale = { predicate: (query: { meta?: Record<string, unknown> }) => typeof query.meta?.communityScope === 'number' && query.meta.communityScope !== accountEpoch() };
    void client.cancelQueries(stale);
    client.removeQueries(stale);
  }), [client]);
  async function read<N extends 'BountyListResponse' | 'BountyGetResponse' | 'BountySubmissionsResponse'>(path: string, schema: N, signal: AbortSignal) {
    if (!session || !privateOperations) throw new Error('Sign in to read your drafts');
    assertAccount(session);
    const value = await privateOperations.client.requestV2(path, schema, { authenticated: true, idempotentRead: true, signal: AbortSignal.any([signal, session.signal]), timeoutMs: 8000 });
    assertAccount(session);
    return value;
  }
  return {
    scope: session ? epoch : 'public',
    meta: session ? { communityScope: epoch } : undefined,
    list(query: BountyListQuery, signal: AbortSignal) {
      const params = new URLSearchParams(Object.entries(query).map(([key, value]) => [key, String(value)]));
      return session ? read(`/v2/bounties?${params}`, 'BountyListResponse', signal) : publicOperations.call('IF_BOUNTY_LIST', { query }, { signal });
    },
    get(bountyId: string, signal: AbortSignal) {
      return session ? read(`/v2/bounties/${encodeURIComponent(bountyId)}`, 'BountyGetResponse', signal) : publicOperations.call('IF_BOUNTY_GET', { path: { bountyId } }, { signal });
    },
    submissions(bountyId: string, cursor: string | null, limit: number, signal: AbortSignal) {
      const query = cursor ? { limit, cursor } : { limit };
      const params = new URLSearchParams(Object.entries(query).map(([key, value]) => [key, String(value)]));
      return session ? read(`/v2/bounties/${encodeURIComponent(bountyId)}/submissions?${params}`, 'BountySubmissionsResponse', signal) : publicOperations.call('IF_BOUNTY_SUBMISSIONS', { path: { bountyId }, query }, { signal });
    },
  };
}
