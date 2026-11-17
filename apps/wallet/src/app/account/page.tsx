'use client';

import Link from 'next/link';
import { AccountView } from '@/account/AccountView';
import { AssetList } from '@/account/AssetList';
import { useWallet } from '@/wallet/WalletProvider';

const LINKS = [
  { href: '/account/history', label: 'History' },
  { href: '/account/plan', label: 'Move value' },
] as const;

export default function AccountPage() {
  const wallet = useWallet();
  if (wallet.status !== 'ready' || !wallet.address) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-pax-muted">Sign in to see the unified account.</p>
        <Link href="/" className="rounded-xl bg-pax-control px-4 py-3 text-sm font-semibold">
          Sign in to wallet
        </Link>
      </div>
    );
  }
  return (
    <>
      <AccountView account={wallet.address} />
      <AssetList account={wallet.address} />
      <nav aria-label="Account" className="grid grid-cols-2 gap-2">
        {LINKS.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            className="rounded-xl bg-pax-control px-3 py-3 text-center text-xs font-semibold"
          >
            {link.label}
          </Link>
        ))}
      </nav>
    </>
  );
}
