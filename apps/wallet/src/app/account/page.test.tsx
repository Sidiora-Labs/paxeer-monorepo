// @vitest-environment jsdom
import { act, useLayoutEffect } from 'react';
import { expect, test } from 'vitest';
import { AccountProvider } from '@/account/AccountProvider';
import { startAccountEndpoint } from '@/account/test/endpoint';
import { mount, type Mounted } from '@/account/test/render';
import { IdentitySession } from '@/wallet/identity';
import { WalletProvider, useWallet, type WalletContextValue } from '@/wallet/WalletProvider';
import { EMAIL, EMAIL_CODE, startGateway } from '@/wallet/test/gateway';
import AccountPage from './page';

async function account_page_offers_no_deposit_link() {
  const endpoint = await startAccountEndpoint();
  let view: Mounted | null = null;
  try {
    const gateway = await startGateway();
    try {
      window.localStorage.clear();
      let wallet: WalletContextValue | null = null;
      function Probe() {
        const value = useWallet();
        useLayoutEffect(() => {
          wallet = value;
        });
        return null;
      }
      function currentWallet(): WalletContextValue {
        if (!wallet) throw new Error('The wallet context did not render');
        return wallet;
      }
      const identity = IdentitySession.create(gateway.config, {
        persistSession: false,
        detectSessionInUrl: false,
      });
      view = await mount(
        <WalletProvider config={gateway.config} identity={identity} eventTarget={new EventTarget()}>
          <Probe />
          <AccountProvider config={endpoint.config('deposit')}>
            <AccountPage />
          </AccountProvider>
        </WalletProvider>
      );
      await view.until(() => currentWallet().status === 'signed-out', 'wallet signed out');
      expect(view.container.textContent).toContain('Sign in to see the unified account.');
      expect(view.query('a[href="/account/deposit"]')).toBeNull();

      await act(async () => {
        await currentWallet().verifyEmailCode(EMAIL, EMAIL_CODE);
      });
      await view.until(() => currentWallet().status === 'ready', 'wallet ready');
      expect(view.query('nav[aria-label="Account"]')).not.toBeNull();
      expect(
        view.all('nav[aria-label="Account"] a').map((link) => ({
          href: link.getAttribute('href'),
          label: link.textContent,
        }))
      ).toEqual([
        { href: '/account/history', label: 'History' },
        { href: '/account/plan', label: 'Move value' },
      ]);
      expect(view.query('a[href="/account/deposit"]')).toBeNull();
    } finally {
      await view?.unmount();
      await gateway.close();
      window.localStorage.clear();
    }
  } finally {
    await endpoint.close();
  }
}

test(account_page_offers_no_deposit_link);
