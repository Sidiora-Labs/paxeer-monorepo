import SwapPanel from '@/widgets/swap/swap-panel';
export default function SwapModule() {
  return (
    <main className="min-h-[calc(100dvh-var(--sl-space-8)*2)] flex items-start justify-center px-4 pt-8 pb-20">
      <SwapPanel />
    </main>
  );
}
