// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join } from 'node:path';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest';
import { chromium } from 'playwright';
import { toast } from 'sonner';
import { createConfig, http, WagmiProvider } from 'wagmi';
import { injected } from 'wagmi/connectors';
import { mainnet } from 'wagmi/chains';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { applyBrowserSurface } from '@/core/app-foundation/browser-surface.test-support';
import { useWalletToasts } from '@/hooks/ui/use-wallet-toasts';
import ToastProvider, { PLAIN_NOTICE_DURATION_MS } from './toast-provider';

let restoreBrowser: () => Promise<void>;
let root: Root;
let container: HTMLDivElement;

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  restoreBrowser = applyBrowserSurface();
});

beforeEach(() => {
  vi.useFakeTimers({
    toFake: ['Date', 'setTimeout', 'clearTimeout', 'requestAnimationFrame', 'cancelAnimationFrame'],
  });
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root.render(<ToastProvider />));
});

async function advance(milliseconds: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(milliseconds);
  });
}

async function showNotice(publish: () => unknown) {
  await act(async () => {
    publish();
    await vi.advanceTimersByTimeAsync(1);
  });
}

function activeNotices() {
  return Array.from(
    container.querySelectorAll<HTMLElement>('[data-sonner-toast]:not([data-removed="true"])')
  );
}

afterEach(async () => {
  await act(async () => {
    toast.dismiss();
    await vi.advanceTimersByTimeAsync(1000);
  });
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
});

afterAll(async () => {
  await restoreBrowser();
});

async function plainNoticeEndsAfterSixSeconds() {
  expect(PLAIN_NOTICE_DURATION_MS).toBe(6000);
  await showNotice(() => toast.success('Link copied'));
  expect(activeNotices()).toHaveLength(1);
  await advance(PLAIN_NOTICE_DURATION_MS - 1);
  expect(activeNotices()).toHaveLength(1);
  await advance(1);
  await advance(32);
  expect(activeNotices()).toHaveLength(0);
}
test(plainNoticeEndsAfterSixSeconds.name, plainNoticeEndsAfterSixSeconds);

async function plainNoticeKeepsRemainingTimeWhileHovered() {
  await showNotice(() => toast.success('Profile link copied'));
  await advance(2000);
  const notice = activeNotices()[0];
  act(() => notice.dispatchEvent(new MouseEvent('pointerover', { bubbles: true })));
  await advance(PLAIN_NOTICE_DURATION_MS * 2);
  expect(activeNotices()).toHaveLength(1);
  act(() => notice.dispatchEvent(
    new MouseEvent('pointerout', { bubbles: true, relatedTarget: document.body })
  ));
  await advance(PLAIN_NOTICE_DURATION_MS - 2000 - 1);
  expect(activeNotices()).toHaveLength(1);
  await advance(1);
  await advance(32);
  expect(activeNotices()).toHaveLength(0);
}
test(plainNoticeKeepsRemainingTimeWhileHovered.name, plainNoticeKeepsRemainingTimeWhileHovered);

async function focusInNoticeLayerHoldsAllPlainNotices() {
  const origin = document.createElement('button');
  document.body.append(origin);
  origin.focus();
  try {
    await showNotice(() => toast('Wallet disconnected'));
    await advance(1000);
    const close = activeNotices()[0].querySelector<HTMLButtonElement>('[data-close-button]')!;
    act(() => close.focus());
    await showNotice(() => toast.info('Connect again to trade'));
    await advance(PLAIN_NOTICE_DURATION_MS * 2);
    expect(activeNotices()).toHaveLength(2);
    act(() => origin.focus());
    await advance(PLAIN_NOTICE_DURATION_MS - 1000);
    await advance(32);
    expect(activeNotices().map((notice) => notice.textContent)).toEqual(['Connect again to trade']);
    await advance(1000);
    await advance(32);
    expect(activeNotices()).toHaveLength(0);
  } finally {
    origin.remove();
  }
}
test(focusInNoticeLayerHoldsAllPlainNotices.name, focusInNoticeLayerHoldsAllPlainNotices);

async function errorsWarningsAndActionsStayUntilClosed() {
  let actions = 0;
  await showNotice(() => toast.error("Couldn't copy link"));
  await showNotice(() => toast.warning('Review this trade'));
  await showNotice(() => toast.success('Trade ready', {
    action: { label: 'Review', onClick: () => { actions += 1; } },
  }));
  await advance(PLAIN_NOTICE_DURATION_MS * 3);
  expect(activeNotices()).toHaveLength(3);
  expect(actions).toBe(0);
  const actionNotice = activeNotices().find((notice) => notice.textContent?.includes('Trade ready'))!;
  act(() => actionNotice.querySelector<HTMLButtonElement>('[data-close-button]')!.click());
  await advance(1000);
  expect(activeNotices()).toHaveLength(2);
  expect(actions).toBe(0);
}
test(errorsWarningsAndActionsStayUntilClosed.name, errorsWarningsAndActionsStayUntilClosed);

async function modalSuppressionAndCloseFocusReturnRemainAvailable() {
  const origin = document.createElement('button');
  const modal = document.createElement('dialog');
  document.body.append(origin, modal);
  origin.focus();
  try {
    await showNotice(() => toast.error('Unable to share'));
    const layer = container.querySelector<HTMLElement>('.sl-notifications')!;
    expect(layer.querySelector('[aria-live]')).not.toBeNull();
    await act(async () => { modal.setAttribute('open', ''); });
    expect(layer.dataset.suppressed).toBe('true');
    expect(layer.getAttribute('aria-hidden')).toBe('true');
    await act(async () => { modal.removeAttribute('open'); });
    expect(layer.dataset.suppressed).toBe('false');
    const close = activeNotices()[0].querySelector<HTMLButtonElement>('[data-close-button]')!;
    act(() => close.focus());
    act(() => close.click());
    await advance(1000);
    expect(document.activeElement).toBe(origin);
  } finally {
    origin.remove();
    modal.remove();
  }
}
test(
  modalSuppressionAndCloseFocusReturnRemainAvailable.name,
  modalSuppressionAndCloseFocusReturnRemainAvailable
);

function WalletNotices() {
  useWalletToasts();
  return null;
}

async function walletConnectionAndDisconnectionArePlainNotices() {
  const config = createConfig({
    chains: [mainnet],
    connectors: [injected()],
    transports: { [mainnet.id]: http() },
    storage: null,
  });
  act(() => root.render(
    <WagmiProvider config={config} reconnectOnMount={false}>
      <ToastProvider />
      <WalletNotices />
    </WagmiProvider>
  ));
  const account = privateKeyToAccount(generatePrivateKey());
  const connector = config.connectors[0];
  await act(async () => {
    config.setState((state) => ({
      ...state,
      status: 'connected',
      current: connector.uid,
      connections: new Map([
        [connector.uid, { accounts: [account.address], chainId: mainnet.id, connector }],
      ]),
    }));
  });
  await advance(1);
  expect(activeNotices()[0].dataset.type).toBe('success');
  expect(activeNotices()[0].textContent).toContain('Wallet connected:');
  expect(activeNotices()[0].querySelector('[data-button]')).toBeNull();
  await advance(PLAIN_NOTICE_DURATION_MS);
  await advance(32);
  expect(activeNotices()).toHaveLength(0);
  await act(async () => {
    config.setState((state) => ({
      ...state,
      status: 'disconnected',
      current: null,
      connections: new Map(),
    }));
  });
  await advance(1);
  expect(activeNotices()[0].textContent).toContain('Wallet disconnected');
  await advance(PLAIN_NOTICE_DURATION_MS);
  await advance(32);
  expect(activeNotices()).toHaveLength(0);
}
test(walletConnectionAndDisconnectionArePlainNotices.name, walletConnectionAndDisconnectionArePlainNotices);

async function noticeOffsetClearsCoinControlsAndNavigationAtEachBreakpoint() {
  await showNotice(() => toast.success('Address copied'));
  const require = createRequire(join(process.cwd(), 'package.json'));
  const css = [
    readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8'),
    readFileSync(join(process.cwd(), 'src/app/globals.css'), 'utf8'),
    readFileSync(join(require.resolve('sonner'), '../styles.css'), 'utf8'),
  ].join('\n');
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.setContent(`<style>${css}</style>${container.innerHTML}<div class="foundation-shell" data-shell-route-id="coin"></div>`);
    for (const route of ['coin', 'explore']) {
      await page.locator('.foundation-shell').evaluate((shell, id) => {
        shell.setAttribute('data-shell-route-id', id);
      }, route);
      for (const width of [390, 600, 601, 767, 768, 900, 1023, 1024, 1440]) {
        await page.setViewportSize({ width, height: 900 });
        const result = await page.evaluate(({ route, width }) => {
          const layer = document.querySelector<HTMLElement>('.sl-notifications')!;
          const toaster = document.querySelector<HTMLElement>('[data-sonner-toaster]')!;
          const measure = document.createElement('div');
          const token = width >= 1024
            ? 'var(--sl-space-6)'
            : route === 'coin'
              ? 'calc(var(--sl-trade-cta-clearance) + env(safe-area-inset-bottom, 0rem))'
              : width < 768
                ? 'calc(var(--sl-bottom-nav-height) + var(--sl-space-8) + env(safe-area-inset-bottom, 0rem))'
                : 'calc(var(--sl-space-6) + env(safe-area-inset-bottom, 0rem))';
          measure.style.height = token;
          layer.append(measure);
          const expected = getComputedStyle(measure).height;
          measure.remove();
          return { actual: getComputedStyle(toaster).bottom, expected, position: toaster.dataset.yPosition };
        }, { route, width });
        expect(result, `${route} at ${width}px`).toEqual({
          actual: result.expected,
          expected: result.expected,
          position: 'bottom',
        });
      }
    }
  } finally {
    await browser.close();
  }
}
test(
  noticeOffsetClearsCoinControlsAndNavigationAtEachBreakpoint.name,
  noticeOffsetClearsCoinControlsAndNavigationAtEachBreakpoint
);
