// @vitest-environment jsdom
import React, { act } from 'react';
import { readFileSync } from 'node:fs';
import { createRoot, type Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type { V2ContractTypes } from '@kindle/sdk';
import { applyBrowserSurface } from '@/core/app-foundation/browser-surface.test-support';

const joined = JSON.parse(readFileSync(process.env.JOINED_CONFIG_FILE!, 'utf8')) as {
  stream_application_origin: string;
  api_origin: string;
  rpc_url: string;
  processes: Array<{ role: string; ready_url: string }>;
};
const gateway = joined.processes.find(process => process.role === 'gateway');
if (!gateway) throw new Error('Account service is required');
const gatewayOrigin = new URL(gateway.ready_url).origin;
const previousEnvironment = { ...process.env };
process.env.NEXT_PUBLIC_GATEWAY_URL = gatewayOrigin;
process.env.NEXT_PUBLIC_METADATA_API_URL = gatewayOrigin;
process.env.NEXT_PUBLIC_DATA_API_URL = joined.api_origin;
process.env.NEXT_PUBLIC_PNL_API_URL = joined.api_origin;
process.env.NEXT_PUBLIC_RPC_URL = joined.rpc_url;

const { default: MobileTradeDrawer } = await import('./mobile-trade-drawer');
const { default: AppProviders } = await import('@/ui/providers/app-providers');
const { default: CookieBanner } = await import('@/ui/shared/cookie-banner');
const { RULES_AGREED_KEY } = await import('@/ui/shared/platform-rules-modal');
const { CreateBountyFab, CreateBountyNotice, useCreateBountyGate } = await import(
  '@/widgets/bounties/bounties-header'
);
const { accountEpoch, clearAccountSession, installAccountSession } = await import(
  '@/widgets/profile/account-transport'
);

const pool = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8')).pool as string;
let restoreBrowser: () => Promise<void>;
const mounted: Array<{ root: Root; container: HTMLDivElement }> = [];

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  restoreBrowser = applyBrowserSurface();
});

beforeEach(() => {
  clearAccountSession(false);
  window.localStorage.clear();
  window.localStorage.setItem(RULES_AGREED_KEY, Date.now().toString());
});

afterEach(() => {
  for (const { root, container } of mounted.splice(0)) {
    act(() => root.unmount());
    container.remove();
  }
  clearAccountSession(false);
  window.localStorage.clear();
});

afterAll(async () => {
  await restoreBrowser();
  for (const key of [
    'NEXT_PUBLIC_GATEWAY_URL',
    'NEXT_PUBLIC_METADATA_API_URL',
    'NEXT_PUBLIC_DATA_API_URL',
    'NEXT_PUBLIC_PNL_API_URL',
    'NEXT_PUBLIC_RPC_URL',
  ]) {
    const value = previousEnvironment[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

function mount(node: React.ReactNode) {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ root, container });
  act(() => root.render(
    <AppProviders>
      <CookieBanner />
      {node}
    </AppProviders>
  ));
  expect(container.querySelector('[data-cookie-banner]')).not.toBeNull();
  return container;
}

async function signIn() {
  const owner = privateKeyToAccount(generatePrivateKey());
  async function post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(gatewayOrigin + path, {
      method: 'POST',
      headers: {
        Origin: joined.stream_application_origin,
        'Content-Type': 'application/json',
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify(body),
    });
    expect(response.status).toBeLessThan(300);
    return response.json();
  }
  const { data: { challenge } } = await post<{
    data: { challenge: V2ContractTypes['AuthChallenge'] };
  }>('/v2/auth/challenges', {
    wallet: owner.address,
    origin: joined.stream_application_origin,
  });
  const signature = await owner.signMessage({ message: challenge.message });
  const { data } = await post<{
    data: { session: V2ContractTypes['Session']; accessToken: string };
  }>('/v2/auth/sessions', {
    challengeId: challenge.challengeId,
    proof: { wallet: owner.address, signature, kind: 'eoa_eip191' },
    deviceLabel: 'Web browser',
  });
  installAccountSession(data.session, data.accessToken, accountEpoch());
}

function expectTradeOffsets(cta: HTMLElement) {
  expect(cta.classList).toContain('bottom-[calc(var(--sl-bottom-nav-height)+var(--sl-cookie-banner-height,0px))]');
  expect(cta.classList).toContain('md:bottom-[calc(var(--sl-space-4)+var(--sl-cookie-banner-height,0px))]');
  expect(cta.style.paddingBottom).toBe('env(safe-area-inset-bottom, var(--sl-space-1))');
  expect(cta.classList).toContain('lg:hidden');
}

function signedOutTradeButtonClearsCookieNotice() {
  const container = mount(<MobileTradeDrawer poolAddress={pool} />);
  const cta = container.querySelector<HTMLElement>('[data-mobile-trade-cta="sign-in"]')!;
  expect(cta.textContent).toBe('Sign in to trade');
  expectTradeOffsets(cta);
}
test(signedOutTradeButtonClearsCookieNotice.name, signedOutTradeButtonClearsCookieNotice);

async function signedInTradeButtonClearsCookieNotice() {
  await signIn();
  const container = mount(<MobileTradeDrawer poolAddress={pool} />);
  const cta = container.querySelector<HTMLElement>('[data-mobile-trade-cta="trade"]')!;
  expect(cta.textContent).toBe('Trade');
  expectTradeOffsets(cta);
}
test(signedInTradeButtonClearsCookieNotice.name, signedInTradeButtonClearsCookieNotice);

async function openTradeDrawerScrollsAboveCookieNoticeAndPhoneNavigation() {
  await signIn();
  const container = mount(<MobileTradeDrawer poolAddress={pool} />);
  act(() => container.querySelector<HTMLButtonElement>('[data-mobile-trade-cta] button')!.click());
  const drawer = container.querySelector<HTMLElement>('[role="dialog"][aria-label="Trade"]')!;
  expect(drawer.hidden).toBe(false);
  expect(drawer.querySelector('form[aria-label="Trade"]')).not.toBeNull();
  const content = drawer.firstElementChild!;
  expect(content.classList).toContain('overflow-y-auto');
  expect(content.classList).toContain('max-h-[var(--sl-drawer-max-height)]');
  expect(content.classList).toContain('pb-[calc(var(--sl-space-6)+env(safe-area-inset-bottom,0px)+var(--sl-cookie-banner-height,0px))]');
  expect(content.classList).toContain('[body:has([data-cookie-banner])_&]:max-md:pb-[calc(var(--sl-space-6)+env(safe-area-inset-bottom,0px)+var(--sl-cookie-banner-height,0px)+var(--sl-bottom-nav-height))]');
  act(() => drawer.querySelector<HTMLButtonElement>('[aria-label="Close trade drawer"]')!.click());
  expect(drawer.hidden).toBe(true);
}
test(openTradeDrawerScrollsAboveCookieNoticeAndPhoneNavigation.name, openTradeDrawerScrollsAboveCookieNoticeAndPhoneNavigation);

function BountyActions() {
  const gate = useCreateBountyGate({ state: 'loading' }, false);
  return (
    <>
      <CreateBountyFab gate={gate} />
      <CreateBountyNotice gate={gate} />
    </>
  );
}

function bountyActionAndWarningClearCookieNotice() {
  const container = mount(<BountyActions />);
  const fab = container.querySelector<HTMLElement>('[data-testid="post-bounty-fab"]')!;
  expect(fab.classList).toContain('bottom-[calc(var(--sl-bottom-nav-height)+var(--sl-space-4)+env(safe-area-inset-bottom,0px)+var(--sl-cookie-banner-height,0px))]');
  act(() => fab.querySelector<HTMLButtonElement>('button')!.click());
  const notice = container.querySelector<HTMLElement>('[role="status"]')!;
  expect(notice.textContent).toContain('Still checking, try again in a moment');
  expect(notice.classList).toContain('bottom-[calc(var(--sl-bounty-notice-bottom)+env(safe-area-inset-bottom,0px)+var(--sl-cookie-banner-height,0px))]');
}
test(bountyActionAndWarningClearCookieNotice.name, bountyActionAndWarningClearCookieNotice);
