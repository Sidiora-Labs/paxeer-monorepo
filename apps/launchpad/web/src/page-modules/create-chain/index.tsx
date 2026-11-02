'use client';

import { Suspense } from 'react';
import { Skeleton } from '@/ui/primitives';
import ChainWizard from '@/widgets/create-chain/chain-wizard';

export function CreateChainPageFallback() {
  return (
    <div aria-busy="true" className="mx-auto flex w-full max-w-content-narrow-max-width flex-col gap-4 px-4 py-4 lg:px-6">
      <h1 className="text-md font-semibold text-text">Launch a chain</h1>
      <span className="sr-only">Loading...</span>
      <div aria-hidden="true" className="flex flex-col gap-4 rounded-lg bg-surface-1 p-4">
        <p className="text-base font-semibold text-text">Name your chain</p>
        <Skeleton variant="block" height={44} />
        <Skeleton variant="block" height={36} />
      </div>
    </div>
  );
}

export default function CreateChainModule() {
  return <Suspense fallback={<CreateChainPageFallback />}><ChainWizard /></Suspense>;
}
