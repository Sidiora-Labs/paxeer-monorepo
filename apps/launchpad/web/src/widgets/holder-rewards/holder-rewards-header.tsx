'use client';

import type { ReactNode } from 'react';
import { Button } from '@/ui/primitives';

export const launchHolderRewardsHref = '/create?strategy=airdrop';

/**
 * Page hero: one surface-1 panel spanning the content width that carries the
 * title, two lines of copy, the launch CTA and (as children) the stat bar.
 * The panel separates from the canvas by tone alone.
 */
export default function HolderRewardsHeader({ children }: { children?: ReactNode }) {
  return (
    <header className="flex flex-col gap-5 rounded-lg bg-surface-1 p-4 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 basis-64 flex-col gap-2">
          <h1 className="text-xl font-bold text-text sm:text-2xl">Holder rewards coins</h1>
          <p className="max-w-[var(--sl-prose-measure)] text-base text-text-muted sm:text-md">
            Coins that pay their trading fees out to holders. Rewards are paid in USDL
          </p>
        </div>
        <Button variant="primary" size="lg" pill href={launchHolderRewardsHref} className="max-w-full shrink-0 self-start">
          Launch a holder rewards coin
        </Button>
      </div>
      {children}
    </header>
  );
}
