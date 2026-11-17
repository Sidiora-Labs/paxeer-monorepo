'use client';
import { useSyncExternalStore } from 'react';
import { DashboardBoard, useCompactHome } from '@/widgets/home/dashboard-board';
import { HomeFeed } from '@/widgets/home/home-feed';
import { Skeleton } from '@/ui/primitives';

const subscribeViewportReady = () => () => undefined;
const viewportReady = () => true;
const viewportPending = () => false;

/** Home: the column board from lg, the single callouts feed below it. */
export default function HomeModule() {
  const ready = useSyncExternalStore(subscribeViewportReady, viewportReady, viewportPending);
  const compact = useCompactHome();
  return (
    <>
      <h1 className="sr-only">Sidiora dashboard</h1>
      <div data-home-view className="min-w-0">
        {ready ? (compact ? <HomeFeed /> : <DashboardBoard />) : (
          <div className="h-[calc(100dvh-var(--shell-top))] p-4" aria-busy="true">
            <Skeleton variant="block" height="calc(100dvh - var(--shell-top) - var(--sl-space) * 8)" label="Loading dashboard" />
          </div>
        )}
      </div>
    </>
  );
}
