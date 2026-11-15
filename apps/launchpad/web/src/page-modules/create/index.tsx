'use client';

import { surfaceSpacing } from '@/app/presentation-tokens';
import { Suspense } from 'react';
import { Skeleton } from '@/ui/primitives';
import CreateWizard from '@/widgets/create/create-wizard';

/**
 * Server-rendered frame shown until the wizard hydrates. It mirrors the wizard's
 * layout (H1, details card with the 2-column name/ticker row, preview column) so
 * the page never collapses to a single heading while the client bundle loads.
 */
export function CreatePageFallback() {
  return (
    <div data-create-fallback aria-busy="true" className="@container/create mx-auto w-full max-w-content-narrow-max-width px-4 py-4 lg:px-6">
      <span className="sr-only">Loading...</span>
      <div className="flex flex-col gap-4 @market-two/create:flex-row @market-two/create:items-start @market-two/create:gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-4 @market-two/create:max-w-form-max-width">
          <h1 className="text-md font-semibold text-text">Create new coin</h1>
          <section aria-hidden="true" className="flex flex-col gap-3">
            <div className="flex flex-col gap-0.5">
              <p className="text-base font-semibold text-text">Coin details</p>
              <p className="text-xs text-text-faint">Choose carefully, the name and ticker can&apos;t be changed once the coin is created</p>
            </div>
            <div className="flex flex-col gap-4 rounded-lg bg-surface-1 p-4">
              <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--sl-form-field-min-width)),1fr))] gap-4">
                {['Coin name', 'Ticker'].map(label => (
                  <div key={label} className="flex flex-col gap-1.5">
                    <p className="text-sm font-bold text-text">{label}</p>
                    <Skeleton variant="block" height={surfaceSpacing(9)} />
                  </div>
                ))}
              </div>
              <div className="flex flex-col gap-1.5">
                <p className="text-sm font-bold text-text">Description <span className="font-normal text-text-faint">(Optional)</span></p>
                <Skeleton variant="block" height={surfaceSpacing(28)} />
              </div>
              <Skeleton variant="block" height={surfaceSpacing(11)} />
              <Skeleton variant="block" height={surfaceSpacing(14)} />
            </div>
          </section>
        </div>
        <aside aria-hidden="true" className="flex shrink-0 flex-col gap-3 @market-two/create:w-56">
          <p className="text-md font-semibold text-text">Preview</p>
          <div className="min-h-40 rounded-lg bg-surface-1 lg:min-h-72" />
        </aside>
      </div>
    </div>
  );
}

export default function CreateModule() {
  return (
    <div data-create-route className="min-w-0">
      <Suspense fallback={<CreatePageFallback />}>
        <CreateWizard />
      </Suspense>
    </div>
  );
}
