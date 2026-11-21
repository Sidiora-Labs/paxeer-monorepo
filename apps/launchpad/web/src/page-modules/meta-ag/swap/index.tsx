import BestRouteSwapPanel from '@/widgets/meta-ag/best-route-swap';
import Link from 'next/link';

export default function MetaAgSwapModule() {
  return (
    <main className="min-h-[calc(100dvh-var(--sl-space-8)*2)] flex flex-col items-center gap-6 px-4 pb-20 pt-6">
      <div className="w-full max-w-search-max">
        <div className="flex flex-wrap items-baseline justify-between gap-2 mb-1">
          <h1 className="text-lg font-bold text-text">Swap</h1>
          <Link prefetch={false}
            href="/swap"
            className="text-2xs text-text-faint hover:text-text transition underline-offset-2 hover:underline"
          >
            Classic swap →
          </Link>
        </div>
        <p className="text-xs text-text-faint mb-5">
          Best-route swaps are not available yet. Use classic swap to trade Sidiora coins.
        </p>
      </div>
      <BestRouteSwapPanel />
      <div className="w-full max-w-search-max space-y-1 text-2xs text-text-faint">
        <div className="flex flex-wrap justify-between gap-2">
          <span>Swap contract</span>
          <span className="font-bold text-text">Not available yet</span>
        </div>
      </div>
    </main>
  );
}
