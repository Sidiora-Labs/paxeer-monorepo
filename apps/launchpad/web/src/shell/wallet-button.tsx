'use client';

import { useEffect, useState } from 'react';
import type { Address } from 'viem';
import { usePathname } from 'next/navigation';
import { TriangleAlert, UserRound } from 'lucide-react';
import { useAccount } from 'wagmi';
import { Avatar, Button, ColorModeControl, IconButton, Sheet, Skeleton } from '@/ui/primitives';
import { formatUsdl, shortAddress } from '@/utils/format';
import { useProfile } from '@/core/market/use-profile';
import { signOutAccount } from '@/widgets/profile/account-transport';
import { useAccountSession } from '@/widgets/profile/use-account-session';
import { USDL_ADDRESS } from '@/core/network/contracts';
import { getPaxeerRpcUrl } from '@/core/rpc';
import { homeBalanceReader } from '@/widgets/trade/home-chain';
import { useAuthModal } from './auth-modal-store';
import SignInModal from './sign-in-modal';
import WalletModal from './wallet-modal';
import { closeAccountMenu, openAccountMenu, useAccountMenu } from './account-menu-store';
import { WalletMark } from './sign-in-wallet-row';

function useHeaderUsdlBalance(account: Address | null, epoch: number | undefined) {
  const key = account ? `${account.toLowerCase()}|${epoch}` : null;
  const [state, setState] = useState<{
    key: string;
    balance: bigint | null;
    failed: boolean;
  } | null>(null);

  useEffect(() => {
    if (!account || !key) {
      setState(null);
      return;
    }
    const reader = homeBalanceReader({
      registryNumber: 0,
      chainId: 125,
      rpcUrl: getPaxeerRpcUrl(),
      walletNetwork: null,
    });
    let live = true;
    let pending = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;
    const visible = () => document.visibilityState !== 'hidden';
    const poll = async () => {
      if (!live || pending || !visible()) return;
      clearTimeout(timer);
      pending = true;
      controller = new AbortController();
      try {
        const balance = await reader.readBalance(USDL_ADDRESS, account, undefined, controller.signal);
        if (live) setState({ key, balance, failed: false });
      } catch {
        if (live) setState({ key, balance: null, failed: true });
      } finally {
        pending = false;
        controller = undefined;
        if (live && visible()) timer = setTimeout(() => void poll(), 15_000);
      }
    };
    const onVisibility = () => {
      if (visible()) void poll();
      else clearTimeout(timer);
    };
    document.addEventListener('visibilitychange', onVisibility);
    void poll();
    return () => {
      live = false;
      clearTimeout(timer);
      controller?.abort();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [account, key]);

  return key !== null && state?.key === key ? state : null;
}

/**
 * Header account cluster. Guest: Sign in (opens the shared sign-in dialog).
 * Signed in: Deposit, USDL balance pill read from chain, avatar with
 * the connector mark; each opens the account menu.
 */
export default function WalletButton() {
  const { session } = useAccountSession();
  const { address, isConnected, chainId, connector } = useAccount();
  const { openModal } = useAuthModal();
  const pathname = usePathname();
  const menu = useAccountMenu();
  useEffect(() => {
    closeAccountMenu();
  }, [address, chainId, pathname]);

  const wallet = session?.session.wallet ?? null;
  const profile = useProfile(wallet);
  const balance = useHeaderUsdlBalance(session && address ? address : null, session?.epoch);

  /** Ends the application session only; the wallet connection stays so the user can sign in again or keep trading. */
  const logOut = async () => {
    await signOutAccount();
  };

  if (!session || !wallet) {
    return (
      <div className="inline-flex max-w-full flex-wrap items-center gap-2">
        {isConnected && chainId !== 125 ? (
          <Button
            variant="secondary"
            size="sm"
            className="py-0"
            icon={<TriangleAlert className="text-warning" />}
            onClick={() =>
              openModal('Your wallet is on another network. Switch to Paxeer to continue')
            }
          >
            Wrong network
          </Button>
        ) : null}
        <Button variant="inverse" size="sm" className="py-0" onClick={() => openModal()}>
          Sign in
        </Button>
        <IconButton
          label="Open account menu"
          icon={<UserRound />}
          variant="ghost"
          size="sm"
          aria-haspopup="dialog"
          aria-expanded={menu !== null}
          onClick={() => openAccountMenu('account')}
        />
        <Sheet
          open={menu !== null}
          onClose={closeAccountMenu}
          title="Account"
          size="sm"
          closeLabel="Close account menu"
        >
          <div className="flex flex-col gap-4">
            <ColorModeControl />
            <Button
              variant="inverse"
              onClick={() => {
                closeAccountMenu();
                openModal();
              }}
            >
              Sign in
            </Button>
          </div>
        </Sheet>
        <SignInModal />
      </div>
    );
  }

  const balanceText = balance?.balance !== null && balance?.balance !== undefined
    ? formatUsdl(balance.balance, { compact: true })
    : null;
  const balanceTitle = balanceText !== null
    ? `USDL balance ${balanceText}`
    : balance?.failed
      ? "Can't load your balance right now"
      : 'Loading USDL balance';
  const displayName = profile.data?.displayName ?? null;

  return (
    <div className="inline-flex max-w-full flex-wrap items-center gap-2">
      <Button
        variant="inverse"
        size="sm"
        className="py-0"
        onClick={() => openAccountMenu('receive')}
      >
        Deposit
      </Button>
      <button
        type="button"
        onClick={() => openAccountMenu('account')}
        title={balanceTitle}
        aria-label={`USDL balance ${balanceText ?? 'not available yet'}, open account menu`}
        className="flex min-h-control-height-sm items-center py-0 gap-1.5 rounded-pill bg-surface-2 px-3 text-num-sm text-text transition-colors duration-fast hover:bg-surface-hover active:bg-surface-active"
        data-numeric
      >
        {!balance ? (
          <Skeleton variant="text" width="var(--sl-chart-mini-width)" label="Loading balance" />
        ) : balanceText !== null ? (
          balanceText
        ) : (
          <span className="text-text-faint">—</span>
        )}
      </button>
      <button
        type="button"
        onClick={() => openAccountMenu('account')}
        aria-label={`Account ${displayName ?? shortAddress(wallet)}; open account menu`}
        className="inline-flex size-8 items-center justify-center rounded-pill transition-opacity duration-fast hover:opacity-90"
      >
        <Avatar
          src={profile.data?.avatarUrl ?? null}
          name={displayName ?? shortAddress(wallet)}
          size="md"
          decorative
          badge={
            <WalletMark
              icon={connector?.icon}
              name={connector?.name ?? 'Wallet'}
              size="sm"
              className="size-4 rounded-pill bg-surface-1"
            />
          }
        />
      </button>
      <WalletModal
        open={menu !== null}
        tab={menu ?? 'account'}
        onTabChange={openAccountMenu}
        onClose={closeAccountMenu}
        address={wallet}
        displayName={displayName}
        avatarUrl={profile.data?.avatarUrl ?? null}
        connectorName={connector?.name ?? null}
        connectorIcon={connector?.icon}
        onLogOut={logOut}
      />
      <SignInModal />
    </div>
  );
}
