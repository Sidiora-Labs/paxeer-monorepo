'use client';
import { useId, useState } from 'react';
import { ChevronUp } from 'lucide-react';
import { cn } from '@/lib/utils';
import { TabStrip } from '@/ui/primitives';
import { tabIds } from '@/ui/primitives/controls';
import { TrendingWidget } from './trending-widget';
import {
  CalloutsFeedBody,
  FeedPauseButton,
  useCalloutsFeed,
  type CalloutsFeed,
} from './callouts-widget';

export type HomeFeedTab = 'coins' | CalloutsFeed;

export const homeFeedTabs: { value: HomeFeedTab; label: string }[] = [
  { value: 'coins', label: 'Coins' },
  { value: 'latest', label: 'Callouts' },
  { value: 'hot', label: 'Top' },
];

/** Compact home discovery, with the board's coin ranking first. */
export function HomeFeed({ className }: { className?: string }) {
  const [feed, setFeed] = useState<HomeFeedTab>('coins');
  const id = useId();
  const ids = tabIds(id, feed);

  return (
    <div
      data-home-feed=""
      className={cn('mx-auto flex w-full max-w-120 shrink-0 flex-col px-4 pb-6 pt-4', className)}
    >
      <TabStrip
        label="Home discovery"
        tabs={homeFeedTabs.map(tab => ({ id: tab.value, label: tab.label }))}
        active={feed}
        onChange={setFeed}
        idPrefix={id}
      />
      <div role="tabpanel" id={ids.panel} aria-labelledby={ids.tab} className="flex flex-col">
        {feed === 'coins' ? (
          <TrendingWidget standalone columnId="home-coins" filter="" density="comfortable" view="list" />
        ) : (
          <CompactCallouts key={feed} feed={feed} />
        )}
      </div>
    </div>
  );
}

function CompactCallouts({ feed }: { feed: CalloutsFeed }) {
  const state = useCalloutsFeed(feed, '');
  return (
    <>
      <div className="flex min-h-10 shrink-0 items-center justify-end py-1.5">
        <FeedPauseButton state={state} />
      </div>
      {state.paused && state.pending > 0 ? (
        <button
          type="button"
          onClick={state.togglePause}
          className="inline-flex h-control-height-sm w-full items-center justify-center gap-1.5 rounded-sm bg-surface-2 text-xs font-medium text-text hover:bg-surface-hover"
        >
          <ChevronUp aria-hidden="true" className="size-icon-sm" />
          {state.pending} new {state.pending === 1 ? 'callout' : 'callouts'}
        </button>
      ) : null}
      <CalloutsFeedBody feed={feed} filter="" state={state} className="flex-none overflow-visible" />
    </>
  );
}

export default HomeFeed;
