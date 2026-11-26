'use client';
import { type Loadable } from '@/core/market/unsupported';
import { plainCopy, userFacingMessage } from '@/core/user-facing-error';
import { NoticeBar } from '@/ui/primitives';

/**
 * Notice for one enrichment source of a table (statistics, prices, metadata)
 * that is not served or failed. Copy is plain; the operation id and status
 * are never rendered.
 */
export function SourceNotice({ label, state, onRetry, placeholder = 'an em dash' }: { label: string; state: Loadable<unknown>; onRetry?: () => void; placeholder?: string }) {
  if (state.state === 'unsupported') {
    void placeholder;
    const retry = state.notServed ? undefined : state.retry ?? onRetry;
    return (
      <div>
        <NoticeBar tone="warning" announce action={retry ? <button type="button" onClick={retry} className="text-sm font-bold text-text hover:underline">Try again</button> : undefined}>
          {label} not available yet
        </NoticeBar>
      </div>
    );
  }
  if (state.state === 'error') {
    return (
      <NoticeBar tone="danger" announce action={<button type="button" onClick={state.retry} className="text-sm font-bold text-text hover:underline">Try again</button>}>
        {label}: {userFacingMessage(state.error, plainCopy.marketFeed)}
      </NoticeBar>
    );
  }
  return null;
}

export default SourceNotice;
