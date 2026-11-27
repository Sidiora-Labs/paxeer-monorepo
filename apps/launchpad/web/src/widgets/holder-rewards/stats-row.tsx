'use client';

import type { ReactNode } from 'react';
import type { Loadable } from '@/core/market/unsupported';
import { cn } from '@/lib/utils';
import { ErrorState, StateNotice } from '@/ui/primitives';
import type { HolderRewardsStatsResponse } from './holder-rewards-contract';
import { usdlAmountText } from './holder-rewards-presentation';

export interface StatsRowProps {
  stats: Loadable<HolderRewardsStatsResponse>;
}

type TileState =
  | { kind: 'loading'; caption?: string }
  | { kind: 'value'; value: string; unit?: string; caption?: string }
  | { kind: 'unavailable'; title: string; plural?: boolean };

const numeral = 'flex min-w-0 items-baseline overflow-x-auto text-xl font-bold tracking-tight tabular-nums';

function DisplayTile({ label, state, tone = 'text', className }: { label: string; state: TileState; tone?: 'text' | 'success'; className?: string }) {
  let body: ReactNode;
  if (state.kind === 'loading') {
    body = <span aria-hidden="true" className="sl-skeleton block h-[1lh] w-40 max-w-full" />;
  } else if (state.kind === 'unavailable') {
    // The figure slot keeps its height while totals are unavailable.
    body = <span aria-label="Not available yet" className="text-text-faint">{'—'}</span>;
  } else {
    body = (
      <>
        <span className={cn('shrink-0', tone === 'success' ? 'text-success' : 'text-text')}>{state.value}</span>
        {state.unit ? <span className="ml-1.5 shrink-0 whitespace-nowrap text-md font-semibold tracking-normal text-text-faint">{state.unit}</span> : null}
      </>
    );
  }
  return (
    <div className={cn('flex min-h-23.5 sm:min-h-31.5 min-w-0 flex-col gap-2 px-5 py-4', className)}>
      <dt className="text-2xs font-semibold uppercase tracking-[var(--sl-tracking-label)] text-text-faint">{label}</dt>
      <dd data-numeric className={numeral} aria-busy={state.kind === 'loading' || undefined}>{body}</dd>
      {state.kind === 'unavailable' ? (
        <dd className="flex min-w-0"><StateNotice kind="empty" surface={1}>{state.title} {state.plural ? 'are' : 'is'} not available yet</StateNotice></dd>
      ) : state.kind === 'value' && state.caption ? (
        <dd className="text-xs text-text-faint">{state.caption}</dd>
      ) : state.kind === 'loading' && state.caption ? (
        <dd aria-hidden="true" className="h-[var(--sl-leading-xs)]" />
      ) : null}
    </div>
  );
}

/**
 * Three figures from the holder rewards totals.
 *
 * "At today's prices": rewards are paid in USDL and USDL is the unit every
 * price on Paxeer is quoted in, so the cumulative amount valued at today's
 * price is that same USDL amount. It is derived from the served total and
 * shares its availability: when the total is absent the figure renders the
 * neutral unavailable notice, never an estimate.
 */
export default function StatsRow({ stats }: StatsRowProps) {
  const loading = stats.state === 'loading';
  const data = stats.state === 'ready' ? stats.data.data : null;
  const tile = (value: string | null, missing: { title: string; plural?: boolean }, extra?: { unit?: string; caption?: string }): TileState =>
    loading ? { kind: 'loading', caption: extra?.caption } : value === null ? { kind: 'unavailable', ...missing } : { kind: 'value', value, ...extra };
  const distributed = data ? usdlAmountText(data.cumulativeDistributedRaw) : null;
  return (
    <section aria-labelledby="holder-rewards-stats-title" className="flex min-w-0 flex-col gap-2">
      <h2 id="holder-rewards-stats-title" className="sr-only">Holder rewards totals</h2>
      <dl className="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
        <DisplayTile label="Holder rewards" tone="text"
          state={tile(distributed, { title: 'Holder rewards', plural: true }, { unit: 'USDL' })} />
        <DisplayTile label="At today's prices"
          state={tile(distributed, { title: "Today's value" }, { unit: 'USDL', caption: 'Valued in USDL' })} />
        <DisplayTile label="Wallets rewarded" className="sm:col-span-2 lg:col-span-1"
          state={tile(data ? data.walletsRewarded.toLocaleString('en-US') : null, { title: 'Wallet count' })} />
      </dl>
      <div className="min-h-[var(--sl-leading-xs)]">
        {stats.state === 'error' ? (
          <ErrorState variant="chip" message="Totals can't load right now" onRetry={stats.retry} className="self-start" />
        ) : data ? (
          <p className="px-1 text-xs text-text-faint">
            Updated <time dateTime={data.generatedAt}>{new Date(data.generatedAt).toLocaleString()}</time>
          </p>
        ) : null}
      </div>
    </section>
  );
}
