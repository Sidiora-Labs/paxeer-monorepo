'use client';

import Link from 'next/link';

import { HistoryView } from '@/account/HistoryView';
import { useWallet } from '@/wallet/WalletProvider';

export default function AccountHistoryPage() {
  const wallet = useWallet();
  if (wallet.status !== 'ready' || !wallet.address) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-pax-muted">Sign in to see your chain history.</p>
        <Link href="/" className="rounded-xl bg-pax-control px-4 py-3 text-sm font-semibold">
          Sign in to wallet
        </Link>
      </div>
    );
  }
  return <HistoryView account={wallet.address} />;
}
