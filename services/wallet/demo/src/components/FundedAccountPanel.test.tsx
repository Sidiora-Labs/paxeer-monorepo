import { readFileSync } from 'node:fs';
import { act } from 'react';
import { hydrateRoot } from 'react-dom/client';
import { renderToString } from 'react-dom/server';
import { PaxeerWallet } from '@paxeer/wallet';
import { expect, test } from 'vitest';
import { SupportedWalletPanel } from './FundedAccountPanel';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

const origin = 'http://localhost:3001';

function createWallet() {
  const wallet = new PaxeerWallet({
    apiUrl: origin,
    supabaseUrl: 'http://localhost:54321',
    supabaseAnonKey: 'demo-panel-test-anon-key',
  });
  wallet.supabase.auth.stopAutoRefresh();
  return wallet;
}

function supportedWalletPanelRendersNoCustodyHandoff() {
  const html = renderToString(<SupportedWalletPanel paxeer={createWallet()} origin={origin} />);

  expect(html).toContain('data-supported-wallet="embedded"');
  expect(html).toContain('data-wallet-accounts');
  expect(html).toContain('data-wallet-send');
  expect(html).toContain('data-wallet-typed');
  expect(html).toContain('data-kernel-plan');
  expect(html).not.toContain('data-custody-');
  expect(html).not.toContain('data-retained-custody');
  expect(html).not.toContain('Prepare custody handoff');

  const source = readFileSync('src/components/FundedAccountPanel.tsx', 'utf8');
  expect(source).not.toContain('paxeer_prepareCustody');
  expect(source).not.toContain('decodeCustodyAuthorization');
  expect(source).not.toContain('CUSTODY_TARGET');
}

async function supportedWalletPanelClearsRetainedCustodyKey() {
  const keys = [
    'paxeer-demo:custody:125:0x1111111111111111111111111111111111111111',
    'paxeer-demo:custody:125:0xABCDEFabcdefABCDEFabcdefABCDEFabcdefABCD',
  ];
  const otherChain = 'paxeer-demo:custody:126:0x1111111111111111111111111111111111111111';
  const preference = 'paxeer-demo:theme';
  const container = document.createElement('div');
  const panel = <SupportedWalletPanel paxeer={createWallet()} origin={origin} />;
  let root: ReturnType<typeof hydrateRoot> | undefined;

  document.body.append(container);
  try {
    for (const key of keys) localStorage.setItem(key, 'retired-authorization-bytes');
    localStorage.setItem(otherChain, 'other-chain-authorization');
    localStorage.setItem(preference, 'dark');
    container.innerHTML = renderToString(panel);
    for (const key of keys) expect(localStorage.getItem(key)).toBe('retired-authorization-bytes');

    await act(async () => {
      root = hydrateRoot(container, panel);
    });

    for (const key of keys) expect(localStorage.getItem(key)).toBeNull();
    expect(localStorage.getItem(otherChain)).toBe('other-chain-authorization');
    expect(localStorage.getItem(preference)).toBe('dark');
    expect(container.querySelector('[data-wallet-accounts]')).not.toBeNull();
    expect(container.querySelector('[data-custody-prepare]')).toBeNull();
    expect(container.querySelector('[role="alert"]')).toBeNull();
  } finally {
    await act(async () => {
      root?.unmount();
    });
    container.remove();
    for (const key of [...keys, otherChain, preference]) localStorage.removeItem(key);
  }
}

test(supportedWalletPanelRendersNoCustodyHandoff, supportedWalletPanelRendersNoCustodyHandoff);
test(supportedWalletPanelClearsRetainedCustodyKey, supportedWalletPanelClearsRetainedCustodyKey);
