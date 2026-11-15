'use client';
import WalletGate from '@/ui/shared/wallet-gate';
import RewardsView from '@/widgets/rewards/rewards-view';
export default function RewardsModule() {
  return (
    <div data-rewards-route className="min-w-0 px-4 py-4 md:px-6">
      <WalletGate message="Connect wallet to see your rewards">
        <RewardsView />
      </WalletGate>
    </div>
  );
}
