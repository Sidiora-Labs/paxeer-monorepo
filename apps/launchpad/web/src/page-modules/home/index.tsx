'use client';
import './home.css';
import { useSyncExternalStore } from 'react';
import { DashboardBoard, useCompactHome } from '@/widgets/home/dashboard-board';
import { HomeFeed } from '@/widgets/home/home-feed';

const subscribeViewportReady = () => () => undefined;
const viewportReady = () => true;
const viewportPending = () => false;

/** Home: the column board from lg, coin and callout tabs below it. */
export default function HomeModule() {
  const ready = useSyncExternalStore(subscribeViewportReady, viewportReady, viewportPending);
  const compact = useCompactHome();
  return (
    <>
      <h1 className="sr-only">Sidiora dashboard</h1>
      <div data-home-view className="flex min-h-0 min-w-0 flex-1 flex-col">
        {!ready || compact ? (
          <div className="flex shrink-0 flex-col lg:hidden">
            <HomeFeed />
          </div>
        ) : null}
        {!ready || !compact ? (
          <div className="hidden min-h-0 flex-1 flex-col lg:flex">
            <DashboardBoard />
          </div>
        ) : null}
      </div>
    </>
  );
}
