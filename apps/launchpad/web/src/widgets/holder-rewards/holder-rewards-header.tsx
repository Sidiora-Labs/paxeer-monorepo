'use client';

import type { ReactNode } from 'react';
import { Button } from '@/ui/primitives';

export const launchHolderRewardsHref = '/create?strategy=airdrop';

/**
 * Page header spanning the content width that carries the
 * title, two lines of copy, the launch CTA and (as children) the stat bar.
 * Totals sit above the ranked coins on the canvas.
 */
export default function HolderRewardsHeader({ children }: { children?: ReactNode }) {
  return (
    <header className="flex min-w-0 flex-col gap-5 p-4 sm:p-6">
      <div className="flex min-w-0 flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex min-w-0 flex-col gap-2">
          <h1 className="text-lg font-bold text-text">
            Holder rewards coins
          </h1>
          <p className="max-w-[var(--sl-prose-measure)] text-base leading-[calc(var(--sl-leading-base)/var(--sl-text-base))] text-text-muted sm:text-md sm:leading-[calc(var(--sl-leading-md)/var(--sl-text-md))]">
            Coins that pay their trading fees out to holders. Rewards are paid in USDL
          </p>
        </div>
        <Button
          variant="primary"
          size="lg"
          pill
          href={launchHolderRewardsHref}
          className="max-w-full shrink-0 self-start"
        >
          Launch a holder rewards coin
        </Button>
      </div>
      {children}
    </header>
  );
}
