'use client';

import { usePathname } from 'next/navigation';
import { ExplorePageLoading } from '@/widgets/explore/explore-loading';

export default function Loading() {
  const pathname = usePathname();
  if (pathname === '/new' || pathname === '/explore') return <ExplorePageLoading />;

  const reservesViewport =
    pathname === '/live' || pathname.startsWith('/live/') || pathname.startsWith('/profile/');
  const indicator = (
    <div className="flex min-h-[var(--sl-page-min-height)] items-center justify-center">
      <div role="status" className="flex flex-col items-center gap-4">
        <div className="relative h-10 w-10 animate-spin rounded-full bg-surface-2">
          <div className="absolute left-1/2 top-0 h-3 w-3 -translate-x-1/2 rounded-full bg-accent" />
        </div>
        <p className="text-xs text-text-faint animate-pulse">Loading...</p>
      </div>
    </div>
  );
  return reservesViewport ? (
    <div className="min-h-[calc(100dvh-var(--shell-top))]">{indicator}</div>
  ) : indicator;
}
