'use client';
import { useSyncExternalStore } from 'react';
import { DashboardBoard, useCompactHome } from '@/widgets/home/dashboard-board';
import { HomeFeed } from '@/widgets/home/home-feed';

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
        {!ready || compact ? <div className="lg:hidden"><HomeFeed /></div> : null}
        {!ready || !compact ? <div className="hidden lg:block"><DashboardBoard /></div> : null}
      </div>
    </>
  );
}
