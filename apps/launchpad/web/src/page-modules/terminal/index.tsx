'use client';
import type { ComponentType } from 'react';
import { EmptyState, Button } from '@/ui/primitives';
import TerminalFrame from '@/widgets/terminal/frame';
import TerminalCoinView from '@/widgets/terminal/coin-view';
import TerminalStatusBar from '@/widgets/terminal/status-bar';
import TerminalTrendingView from '@/widgets/terminal/views/trending';
import { parseTerminalSlug, type TerminalTabView } from '@/widgets/terminal/sub-nav';
import TerminalTrenchesView from '@/widgets/terminal/views/trenches';
import TerminalTrackView from '@/widgets/terminal/views/track';
import TerminalPortfolioView from '@/widgets/terminal/views/portfolio';
import TerminalRewardsView from '@/widgets/terminal/views/rewards';

const views: Record<TerminalTabView, ComponentType> = {
  trending: TerminalTrendingView,
  trenches: TerminalTrenchesView,
  track: TerminalTrackView,
  portfolio: TerminalPortfolioView,
  rewards: TerminalRewardsView,
};

/** /terminal (trending), /terminal/<view> and /terminal/<pool>; the app route validates the slug first. */
export default function TerminalModule({ slug }: { slug?: string }) {
  const target = parseTerminalSlug(slug);
  if (target.kind === 'pool') return <div className="min-h-dvh min-w-0"><TerminalCoinView key={target.pool} poolAddress={target.pool} /></div>;
  const status = <TerminalStatusBar poolAddress={null} />;
  if (target.kind === 'invalid') {
    return (
      <div className="min-h-dvh min-w-0"><TerminalFrame
        center={<div className="rounded-md bg-surface-1"><EmptyState variant="panel" title="Page not found" body="Open a coin by its address or pick a view from the tabs above." action={<Button href="/terminal" variant="secondary" size="sm">Back to trending</Button>} /></div>}
        status={status}
      /></div>
    );
  }
  const View = views[target.view];
  return <div className="min-h-dvh min-w-0"><TerminalFrame center={<View />} status={status} /></div>;
}
