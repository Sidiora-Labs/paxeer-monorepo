'use client';
import type { ReactNode } from 'react';
import { Trophy } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Loadable } from '@/core/market/unsupported';
import { CountdownProgress, EmptyState, ErrorState, StatusBadge, UnavailableState } from '@/ui/primitives';
import { formatUsdl } from '@/utils/format';
import { CupRulesDisclosure } from './cup-rules-disclosure';
import { cupBadge, cupModeLabel, type Cup } from './cups-contract';

export interface CupHeroProps {
  cup: Loadable<Cup | null>;
  onRetry: () => void;
  control?: ReactNode;
  className?: string;
}

export function CupHero({ cup, onRetry, control, className }: CupHeroProps) {
  const record = cup.state === 'ready' ? cup.data : null;
  const badge = record ? cupBadge(record.status) : null;
  const empty = cup.state === 'ready' && record === null || cup.state === 'unsupported' && cup.notServed;
  return (
    <section
      aria-label="Competition"
      aria-busy={cup.state === 'loading' || undefined}
      data-cup-state={record?.status ?? (cup.state === 'ready' ? 'none' : cup.state)}
      className={cn('flex min-w-0 flex-col gap-3', className)}
    >
      {control ? <div className="flex min-w-0">{control}</div> : null}
      {cup.state === 'loading' ? (
        <div className="flex min-w-0 flex-col gap-2 py-4">
          <span className="sr-only">Loading competition</span>
          <span aria-hidden="true" className="sl-skeleton h-6 w-2/3" />
          <span aria-hidden="true" className="sl-skeleton h-8 w-1/2" />
          <span aria-hidden="true" className="sl-skeleton h-4 w-3/4" />
        </div>
      ) : empty ? (
        <EmptyState headingLevel={2} icon={Trophy} title="No competition scheduled" body="Prizes and results show up here once cups start" />
      ) : cup.state === 'error' ? (
        <ErrorState headingLevel={2} title="Couldn't load the cup" error={cup.error} onRetry={cup.retry} />
      ) : cup.state === 'unsupported' ? (
        <UnavailableState headingLevel={2} title="Competitions" plural operation={cup.operation} status={cup.status} notServed={cup.notServed} error={cup.error} onRetry={cup.retry ?? onRetry} />
      ) : record && badge ? (
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 id="cup-hero-title" className="text-md font-semibold text-text">{record.name}</h2>
            <StatusBadge status={badge.status} size={record.status === 'live' ? 'md' : 'sm'}>{badge.label}</StatusBadge>
            <span className="text-xs text-text-muted">{cupModeLabel[record.mode]}</span>
          </div>
          <dl>
            <dt className="text-xs text-text-muted">Prize pool</dt>
            <dd className="text-2xl font-bold tabular-nums text-text" data-numeric>{formatUsdl(record.prizePoolRaw, { fractionDigits: 0 })}</dd>
          </dl>
          {record.status === 'scheduled' ? (
            <EmptyState icon={Trophy} title="Standings open when the cup starts" body={<>Starts in <CountdownProgress endsAt={record.startsAt} variant="text" className="inline" /></>} />
          ) : (
            <p className="text-sm tabular-nums text-text-muted">
              {record.status === 'live' ? <>Ends in <CountdownProgress endsAt={record.endsAt} startsAt={record.startsAt} variant="text" className="inline" /></> : <>Ended {new Date(record.endsAt).toLocaleDateString(undefined, { dateStyle: 'medium' })}, final results below</>}
            </p>
          )}
          <CupRulesDisclosure cup={record} />
        </div>
      ) : null}
    </section>
  );
}
