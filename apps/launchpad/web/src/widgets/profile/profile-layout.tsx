'use client';

import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export interface ProfileLayoutProps {
  /** Leaderboard rail; shown from 2xl only. */
  left?: ReactNode;
  center: ReactNode;
  /** Top trades / account rail; beside the centre from lg, stacked under it below lg. */
  right?: ReactNode;
  /** Accessible name of the right rail. */
  rightLabel?: string;
  className?: string;
}

/** Flush profile columns adapt to the content width; absent rails leave no empty column. */
export default function ProfileLayout({ left, center, right, rightLabel = 'Top trades', className }: ProfileLayoutProps) {
  const hasLeft = left !== undefined && left !== null;
  const hasRight = right !== undefined && right !== null;
  const columns = hasLeft && hasRight
    ? '@market-two/profile:grid-cols-[minmax(0,1fr)_var(--sl-right-rail-width)] xl:max-2xl:@market-two/profile:grid-cols-[minmax(0,1fr)_var(--sl-right-rail-width-wide)] @market-three/profile:grid-cols-[minmax(var(--sl-stream-card-min-width),var(--sl-right-rail-width-wide))_minmax(0,var(--sl-form-max-width))_minmax(var(--sl-stream-card-min-width),var(--sl-right-rail-width-wide))]'
    : hasRight
      ? '@market-two/profile:grid-cols-[minmax(0,1fr)_var(--sl-right-rail-width)] xl:max-2xl:@market-two/profile:grid-cols-[minmax(0,1fr)_var(--sl-right-rail-width-wide)] @market-three/profile:grid-cols-[minmax(0,var(--sl-form-max-width))_minmax(var(--sl-stream-card-min-width),var(--sl-right-rail-width-wide))]'
      : hasLeft
        ? '@market-three/profile:grid-cols-[minmax(var(--sl-stream-card-min-width),var(--sl-right-rail-width-wide))_minmax(0,var(--sl-form-max-width))]'
        : '@market-two/profile:grid-cols-[minmax(0,var(--sl-form-max-width))]';
  return (
    <div className="@container/profile w-full min-w-0">
      <div data-profile-layout className={cn('grid w-full grid-cols-1 items-stretch lg:min-h-[calc(100dvh-var(--sl-shell-top-total))] @market-three/profile:justify-center', columns, className)}>
        {hasLeft ? (
          <aside aria-label="Leaderboard" className="relative hidden min-w-0 bg-surface-1 before:pointer-events-none before:absolute before:inset-y-0 before:right-0 before:w-[var(--sl-edge-width)] before:bg-surface-3 @market-three/profile:block">
            <div className="sticky top-[var(--shell-top)]">{left}</div>
          </aside>
        ) : null}
        <div data-profile-col="center" className="flex w-full min-w-0 flex-col">{center}</div>
        {hasRight ? <aside aria-label={rightLabel} className="relative flex min-w-0 flex-col gap-2 bg-surface-1 mt-2 @market-two/profile:mt-0 @market-two/profile:before:pointer-events-none @market-two/profile:before:absolute @market-two/profile:before:inset-y-0 @market-two/profile:before:left-0 @market-two/profile:before:w-[var(--sl-edge-width)] @market-two/profile:before:bg-surface-3">{right}</aside> : null}
      </div>
    </div>
  );
}
