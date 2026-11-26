import { ArrowLeftRight, ListOrdered } from 'lucide-react';
import { EmptyState } from '@/ui/primitives';

/** The first form of a trading capability, before it is available. */
export function CapabilityState({ kind }: { kind: 'swap' | 'orders' }) {
  return (
    <section className="w-full max-w-search-max rounded-md bg-surface-1">
      <EmptyState
        announce
        icon={kind === 'swap' ? ArrowLeftRight : ListOrdered}
        title={kind === 'swap' ? 'Swaps are not available yet' : 'Orders are not available yet'}
        body={kind === 'swap' ? 'Use classic swap to trade Sidiora coins.' : 'Open the terminal to trade while limit and stop orders are coming soon.'}
        className="p-4"
      />
    </section>
  );
}
