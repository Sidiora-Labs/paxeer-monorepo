import BestRouteSwapPanel from '@/widgets/meta-ag/best-route-swap';
import Link from 'next/link';

export default function MetaAgSwapModule() {
  return (
    <main className="min-h-dvh flex flex-col items-center gap-6 px-4 pb-20 pt-6">
      <div className="w-full max-w-lg">
        <div className="flex flex-wrap items-baseline justify-between gap-2 mb-1">
          <h1 className="text-lg font-bold text-text">Swap</h1>
          <Link
            href="/swap"
            className="text-2xs text-text-faint hover:text-text transition underline-offset-2 hover:underline"
          >
            Classic swap →
          </Link>
        </div>
        <p className="text-xs text-text-faint mb-5">
          Finds the best price across Sidiora coins and the vault
        </p>
      </div>
      <BestRouteSwapPanel />
      <div className="w-full max-w-lg space-y-1 text-2xs text-text-faint">
        <div className="flex flex-wrap justify-between gap-2">
          <span>Swap contract</span>
          <span className="font-bold text-text">0x732A…6449</span>
        </div>
      </div>
    </main>
  );
}
