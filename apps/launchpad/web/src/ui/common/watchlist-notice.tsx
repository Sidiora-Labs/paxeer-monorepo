'use client';
import { useState } from 'react';
import type { useWatchlist } from '@/hooks/ui/use-watchlist';
import { userFacingMessage } from '@/core/user-facing-error';
import { shortAddress } from '@/utils/format';
import { Button, NoticeBar } from '@/ui/primitives';

type WatchlistActions = Pick<ReturnType<typeof useWatchlist>, 'error' | 'pending' | 'refresh' | 'retry' | 'dismiss'>;

export function WatchlistNotice({ state }: { state: WatchlistActions }) {
  const [failure, setFailure] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  async function run(action: () => Promise<void>) {
    setBusy(true); setFailure(null);
    try { await action(); }
    catch (error) { setFailure(userFacingMessage(error)); }
    finally { setBusy(false); }
  }
  const error = failure ?? state.error;
  return (
    <>
      {error ? (
        <NoticeBar tone="danger" announce action={<Button variant="ghost" size="sm" disabled={busy} onClick={() => { void run(state.refresh); }}>Refresh watchlist</Button>}>
          {error}
        </NoticeBar>
      ) : null}
      {state.pending.map(operation => (
        <NoticeBar key={operation.mutationId} tone={error ? 'warning' : 'info'} announce action={<>
          <Button variant="ghost" size="sm" disabled={busy} onClick={() => { void run(() => state.retry(operation.mutationId, true)); }}>Apply change to latest watchlist</Button>
          <Button variant="ghost" size="sm" disabled={busy} onClick={() => state.dismiss(operation.mutationId)}>Dismiss change</Button>
        </>}>
          {operation.add ? 'Adding' : 'Removing'} {shortAddress(operation.poolAddress)} {operation.add ? 'to' : 'from'} your watchlist{error ? '' : ', waiting for confirmation'}
        </NoticeBar>
      ))}
    </>
  );
}
