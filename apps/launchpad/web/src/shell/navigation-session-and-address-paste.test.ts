// @vitest-environment jsdom
import React, { act } from 'react';
import { afterEach, beforeAll, expect, test } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { PathnameContext } from 'next/dist/shared/lib/hooks-client-context.shared-runtime';
import { routeForPath } from '@/core/app-foundation/navigation';
import { accountSession, clearAccountSession } from '@/widgets/profile/account-transport';
import MobileBottomNav, { BOTTOM_NAV_IDS, moreSheetGroups } from './mobile-bottom-nav';
import { isNavItemEligible, navItems } from './nav-config';
import { addressFromText, coinPath } from './paste-address';
import { clearNavigationSession, installNavigationSession, makeNavigationConfig } from './navigation-session.fixture';

const mounted: Array<{ root: Root; queries: QueryClient }> = [];

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  window.matchMedia = (query: string) => ({ matches: false, media: query, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false }) as MediaQueryList;
});

afterEach(() => {
  for (const { root, queries } of mounted.splice(0)) {
    act(() => root.unmount());
    queries.clear();
  }
  clearNavigationSession();
  document.body.innerHTML = '';
});

async function openNavigation() {
  const config = makeNavigationConfig();
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  mounted.push({ root, queries });
  await act(async () => {
    root.render(React.createElement(WagmiProvider, { config },
      React.createElement(QueryClientProvider, { client: queries },
        React.createElement(PathnameContext.Provider, { value: '/chat' }, React.createElement(MobileBottomNav)))));
  });
  const more = container.querySelector<HTMLButtonElement>('button[aria-label="More"]')!;
  await act(async () => { more.click(); });
  const sheet = container.querySelector<HTMLElement>('[role="dialog"][aria-label="More"]')!;
  expect(sheet).not.toBeNull();
  return { config, sheet, more };
}

function accountDestinations(sheet: HTMLElement) {
  return Array.from(sheet.querySelectorAll('section[aria-label="Your account"] a')).map(link => link.getAttribute('href'));
}

async function mobile_test_session_shape() {
  const { config, sheet, more } = await openNavigation();
  expect(more.getAttribute('aria-expanded')).toBe('true');
  expect(accountSession()).toBeNull();
  expect(accountDestinations(sheet)).toEqual([]);
  expect(more.className).not.toContain('text-accent');

  await act(async () => { await connect(config, { connector: config.connectors[0] }); });
  expect(accountSession()).toBeNull();
  expect(accountDestinations(sheet)).toEqual(['/rewards', '/watchlist', '/profile']);
  expect(more.className).not.toContain('text-accent');

  await act(async () => { installNavigationSession(); });
  expect(accountSession()?.session.wallet).toBe(config.state.connections.get(config.state.current!)?.accounts[0].toLowerCase());
  expect(accountDestinations(sheet)).toEqual(['/rewards', '/watchlist', '/profile', '/chat']);
  expect(sheet.querySelector('a[href="/chat"]')?.getAttribute('aria-current')).toBe('page');
  expect(more.className).toContain('text-accent');
}

async function mobile_session_gate() {
  const { config, sheet } = await openNavigation();
  await act(async () => { await connect(config, { connector: config.connectors[0] }); });
  await act(async () => { installNavigationSession(); });
  expect(accountDestinations(sheet)).toContain('/chat');

  await act(async () => { clearAccountSession(false); });
  expect(accountDestinations(sheet)).toEqual(['/rewards', '/watchlist', '/profile']);
  await act(async () => { installNavigationSession('0x2222222222222222222222222222222222222222'); });
  expect(accountSession()).toBeNull();
  expect(accountDestinations(sheet)).toEqual(['/rewards', '/watchlist', '/profile']);

  await act(async () => { installNavigationSession(); });
  expect(accountDestinations(sheet)).toContain('/chat');
  await act(async () => { await disconnect(config); });
  expect(accountSession()).toBeNull();
  expect(accountDestinations(sheet)).toEqual([]);
  expect(sheet.querySelector('a[href="/leaderboard"]')).not.toBeNull();

  for (const [walletReady, sessionReady] of [[false, false], [true, false], [true, true], [false, true]]) {
    const visible = moreSheetGroups(walletReady, sessionReady).flatMap(group => group.items.map(item => item.id));
    const expected = navItems.filter(item => !BOTTOM_NAV_IDS.some(id => id === item.id) && isNavItemEligible(item, walletReady, sessionReady)).map(item => item.id);
    expect(visible).toEqual(expected);
    expect(visible.includes('chat')).toBe(sessionReady);
    expect(visible.includes('profile')).toBe(walletReady);
    expect(visible).toContain('leaderboard');
  }
}

function paste_long_hex() {
  const address = '0xAbCdEf0123456789abcdef0123456789ABCDEF01';
  const expected = address.toLowerCase();
  for (const text of [address, `https://paxscan.paxeer.app/address/${address}`, `https://paxscan.paxeer.app/token/${address}?tab=holders`, `see /coin/${address} now`]) {
    expect(addressFromText(text)).toBe(expected);
    expect(coinPath(text)).toBe(`/coin/${expected}`);
    expect(routeForPath(coinPath(text))?.id).toBe('coin');
  }
  for (const digits of [41, 64]) {
    const token = `0x${'aB'.repeat(32).slice(0, digits)}`;
    for (const text of [token, `https://paxscan.paxeer.app/tx/${token}`, `see ${token} now`, `see /coin/${token}?tab=holders`]) {
      expect(addressFromText(text)).toBeNull();
      expect(() => coinPath(text)).toThrow('Invalid address');
    }
    expect(addressFromText(`${token} then ${address}`)).toBe(expected);
  }
  expect(addressFromText(`a${address}`)).toBeNull();
  expect(addressFromText('0x1234')).toBeNull();
}

test(mobile_test_session_shape.name, mobile_test_session_shape);
test(mobile_session_gate.name, mobile_session_gate);
test(paste_long_hex.name, paste_long_hex);
