'use client';
import { Suspense } from 'react';
import BountiesHome, { BountiesHomeSkeleton } from '@/widgets/bounties/bounties-home';

export default function BountiesPage() {
  return (
    <div data-bounties-route className="min-w-0">
      <Suspense fallback={<BountiesHomeSkeleton />}>
        <BountiesHome />
      </Suspense>
    </div>
  );
}
