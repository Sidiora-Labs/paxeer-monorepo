'use client';
import BountiesHome from '@/widgets/bounties/bounties-home';

export default function BountiesPage({ requestedTab }: { requestedTab: string | null }) {
  return (
    <div data-bounties-route className="min-w-0">
      <BountiesHome requestedTab={requestedTab} />
    </div>
  );
}
