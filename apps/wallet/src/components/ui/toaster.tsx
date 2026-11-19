'use client';

import { AnimatePresence } from 'framer-motion';
import { useToast } from './use-toast';
import { ToastItem } from './toast';

/**
 * Global toast viewport. Mount once at the root layout under <body>.
 * Stacks up to TOAST_LIMIT toasts, auto-dismissing after TOAST_REMOVE_DELAY.
 *
 * Positioned:
 *   - Safe-area aware (respects iOS notch + Android status bar)
 *   - Top-center on narrow screens (mobile-first, matches wallet's existing
 *     inline toast positioning like RampPage)
 *   - Top-right above `md` so trade confirm drawers / sheets don't collide
 */
export function Toaster() {
  const { toasts, dismiss } = useToast();

  return (
    <div
      className="pointer-events-none fixed inset-x-3 top-3 z-[120] flex max-h-[calc(100dvh-var(--space-8)-var(--safe-area-top))] flex-col-reverse gap-2 overflow-y-auto overscroll-contain md:inset-x-auto md:right-4 md:top-4 md:w-toast"
      style={{ paddingTop: 'var(--safe-area-top)' }}
    >
      <AnimatePresence initial={false}>
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onClose={() => dismiss(t.id)} />
        ))}
      </AnimatePresence>
    </div>
  );
}
