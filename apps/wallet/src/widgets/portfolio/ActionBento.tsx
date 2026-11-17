'use client';

import { useCallback, useState } from 'react';
import { SvgIcon } from '@/components/ui/SvgIcon';
import type { AppRoute } from '@/widgets/shell/useAppRoute';

export interface ActionBentoProps {
  onNavigate: (route: AppRoute) => void;
}

export function ActionBento({ onNavigate }: ActionBentoProps) {
  const [bridgeToast, setBridgeToast] = useState(false);

  const handleBridge = useCallback(() => {
    setBridgeToast(true);
    setTimeout(() => setBridgeToast(false), 2500);
  }, []);

  // ── Standard mode: 4-button bento ─────────────────────────────────
  return (
    <>
      {bridgeToast && (
        <div className="fixed top-20 left-1/2 -translate-x-1/2 z-50 px-5 py-2.5 rounded-xl bg-pax-surface   text-sm font-semibold animate-fade-in shadow-2xl">
          Being integrated
        </div>
      )}
      <div className="col-span-2 grid aspect-[330/210] grid-cols-5 grid-rows-3 gap-2 sm:aspect-auto sm:grid-cols-4 sm:grid-rows-1">
        <button
          type="button"
          onClick={() => onNavigate('send')}
          className="col-span-2 row-span-1 sm:col-span-1 sm:row-span-1 sm:min-h-32 min-h-[var(--touch-target)] rounded-xl bg-pax-surface flex flex-col items-center justify-center gap-2 press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="send" className="w-6 h-6 text-pax-accent" />
          <span className="text-xs font-bold text-pax-subtle">Send</span>
        </button>
        <button
          type="button"
          onClick={() => onNavigate('swap')}
          className="col-span-3 row-span-2 sm:col-span-1 sm:row-span-1 sm:min-h-32 min-h-[var(--touch-target)] rounded-xl bg-pax-surface flex flex-col items-center justify-center gap-2 press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="swap" className="w-7 h-7 text-pax-accent" />
          <span className="text-sm font-bold text-pax-subtle">Swap</span>
        </button>
        <button
          type="button"
          onClick={handleBridge}
          className="col-span-2 row-span-2 sm:col-span-1 sm:row-span-1 sm:min-h-32 min-h-[var(--touch-target)] rounded-xl bg-pax-surface flex flex-col items-center justify-center gap-2 press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="bridge" className="w-6 h-6 text-pax-accent" />
          <span className="text-sm font-bold text-pax-subtle">Bridge</span>
        </button>
        <button
          type="button"
          onClick={() => onNavigate('receive')}
          className="col-span-3 row-span-1 sm:col-span-1 sm:row-span-1 sm:min-h-32 min-h-[var(--touch-target)] rounded-xl bg-pax-surface flex flex-col items-center justify-center gap-2 press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="qr-code" className="w-5 h-5 text-pax-accent" />
          <span className="text-xs font-bold text-pax-subtle">Receive</span>
        </button>
      </div>
    </>
  );
}
