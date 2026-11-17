'use client';

import Link from 'next/link';

import { KernelPlan } from '@/account/KernelPlan';
import { useWallet } from '@/wallet/WalletProvider';

export default function AccountPlanPage() {
  const wallet = useWallet();
  if (wallet.status !== 'ready' || !wallet.address) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-pax-muted">Sign in to move value across the network.</p>
        <Link
          href="/"
          className="inline-flex items-center rounded-xl bg-pax-control px-4 py-3 text-sm font-semibold"
        >
          Sign in to wallet
        </Link>
      </div>
    );
  }
  return <KernelPlan account={wallet.address} address={wallet.address} />;
}
