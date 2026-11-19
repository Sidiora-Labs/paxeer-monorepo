'use client';

import { ModalPanel } from '@/components/ui/primitives';
import { useState } from 'react';
import { useWalletState } from '@/providers/WalletProvider';
import { useWallet } from '@/wallet/WalletProvider';
import { shortenAddress } from '@/lib/format';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { getAvatarPath } from '@/lib/avatar';
import Image from 'next/image';
import Link from 'next/link';

export interface UniversalHeaderProps {
  title: string;
  showBack?: boolean;
  onBack?: () => void;
  rightAction?: React.ReactNode;
}

export function UniversalHeader({ title, showBack, onBack, rightAction }: UniversalHeaderProps) {
  const { activeAccount } = useWalletState();
  const { mode, wallet } = useWallet();
  const isEmbedded = mode === 'embedded';
  const [sheetOpen, setSheetOpen] = useState(false);
  const [addressCopied, setAddressCopied] = useState(false);

  return (
    <>
      <header className="pax-header fixed top-0 left-0 right-0 z-40 bg-pax-bg backdrop-blur-xl safe-area-pt">
        <div className="relative flex items-center justify-between gap-2 px-4 min-h-14">
          {/* Left side */}
          {showBack ? (
            <button
              type="button"
              onClick={onBack}
              aria-label="Back"
              className="justify-self-start p-2 -ml-2 press-scale z-10"
            >
              <SvgIcon name="arrow-left" className="w-5 h-5" />
            </button>
          ) : (
            <button
              type="button"
              aria-label="Open account menu"
              onClick={() => setSheetOpen(true)}
              className="justify-self-start max-w-full flex min-w-[var(--touch-target)] shrink-0 items-center gap-2 press-scale z-10 focus-visible:ring-2 focus-visible:ring-pax-accent"
            >
              <Image
                src={getAvatarPath(activeAccount?.address || '')}
                alt={activeAccount?.name || 'Account'}
                width={32}
                height={32}
                className="w-8 h-8 shrink-0 rounded-full object-cover"
              />
              <div className="pax-header-account-details min-w-0 text-left">
                <p className="text-sm font-semibold leading-tight truncate max-w-size-140">
                  {activeAccount?.name || 'Account 1'}
                </p>
                <p className="text-size-10 text-pax-muted leading-tight truncate">
                  {activeAccount ? shortenAddress(activeAccount.address) : '—'}
                </p>
              </div>
              <SvgIcon
                name="chevron-down"
                className="pax-header-account-chevron w-3.5 h-3.5 shrink-0"
              />
            </button>
          )}

          {/* Center title */}
          <h1 className="absolute inset-x-0 text-center text-sm font-bold pointer-events-none">
            {title}
          </h1>

          {/* Right side */}
          <div className="z-10 justify-self-end">{rightAction || <div className="w-9" />}</div>
        </div>
      </header>

      {sheetOpen && (
        <>
          <button
            type="button"
            aria-label="Close account menu"
            className="fixed inset-0 z-[60] bg-pax-scrim backdrop-blur-md"
            onClick={() => setSheetOpen(false)}
          />
          <ModalPanel
            onDismiss={() => setSheetOpen(false)}
            aria-label="Account"
            className="fixed inset-x-0 bottom-0 z-[70] bg-pax-bg rounded-t-3xl p-5 pb-8 animate-slide-up safe-area-pb max-w-md mx-auto"
          >
            <div className="flex flex-col items-center gap-4">
              <div className="w-10 h-1 rounded-full bg-pax-control -mt-1" />
              <Image
                src={getAvatarPath(activeAccount?.address || '')}
                alt={activeAccount?.name || 'Account'}
                width={64}
                height={64}
                className="w-16 h-16 rounded-full object-cover"
              />
              <div className="text-center">
                <p className="text-base font-bold truncate max-w-size-260">
                  {activeAccount?.name || 'Paxeer Wallet'}
                </p>
                <p className="text-size-11 text-pax-accent mt-0.5">
                  {isEmbedded
                    ? 'Paxeer Wallet · managed custody'
                    : `${wallet?.info?.name ?? 'Browser wallet'} · self-custody`}
                </p>
              </div>
              <button
                onClick={async () => {
                  if (!activeAccount?.address) return;
                  await navigator.clipboard.writeText(activeAccount.address);
                  setAddressCopied(true);
                  setTimeout(() => setAddressCopied(false), 1500);
                }}
                className="w-full flex items-center gap-2 px-4 py-3 rounded-xl bg-pax-control press-scale hover:bg-pax-overlay transition-colors"
              >
                <SvgIcon name="copy" className="w-4 h-4 shrink-0" />
                <span className="flex-1 text-left text-xs font-mono truncate">
                  {activeAccount?.address}
                </span>
                {addressCopied && <SvgIcon name="check" className="w-4 h-4 shrink-0" />}
              </button>
              <Link
                href="/account"
                onClick={() => setSheetOpen(false)}
                className="w-full rounded-xl bg-pax-control px-4 py-3 text-center text-xs font-semibold press-scale hover:bg-pax-overlay transition-colors"
              >
                Unified account
              </Link>
              <p className="text-size-11 text-pax-muted text-center leading-relaxed">
                You can manage account-level options from Settings.
              </p>
            </div>
            <button
              type="button"
              onClick={() => setSheetOpen(false)}
              className="mt-4 w-full rounded-xl bg-pax-control p-3 text-sm"
            >
              Close account menu
            </button>
          </ModalPanel>
        </>
      )}
    </>
  );
}
