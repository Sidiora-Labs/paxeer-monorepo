// @vitest-environment jsdom

import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { useRouter } from 'next/router';
import QRCode from 'qrcode';
import React from 'react';

import { render } from 'ui/shared/layout/testWrapper';
import { afterEach, expect, test } from 'vitest';
import { fireEvent, waitFor } from 'vitest/lib';

import AddressQrCode from './AddressQrCode';

const FIRST_HASH = '0x1e7a5b0b0d3f4d5e6f708192a3b4c5d6e7f80912';
const SECOND_HASH = '0x2e7a5b0b0d3f4d5e6f708192a3b4c5d6e7f80913';
const THIRD_HASH = '0x3e7a5b0b0d3f4d5e6f708192a3b4c5d6e7f80914';
const QR_SELECTOR = 'svg[shape-rendering="crispEdges"]';
const routers = new Set<Router>();

function AddressQrPage() {
  const { query } = useRouter();
  if (typeof query.hash !== 'string') {
    throw new Error('An address hash is required');
  }
  return React.createElement(AddressQrCode, { hash: query.hash });
}

function createAddressRouter() {
  // In-memory Next bootstrap data lets the real router and loader run without a server.
  window.__NEXT_DATA__ = { page: '/address/[hash]', query: { hash: FIRST_HASH }, props: {}, buildId: 'qr-regression' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  window.__SSG_MANIFEST = new Set();
  const router = new Router('/address/[hash]', { hash: FIRST_HASH }, `/address/${ FIRST_HASH }`, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('qr-regression', ''),
    App,
    wrapApp: (Component) => Component,
    Component: AddressQrPage,
    subscription: async() => undefined,
    isFallback: false,
  });
  routers.add(router);
  return router;
}

afterEach(() => {
  for (const router of routers) {
    window.removeEventListener('popstate', router.onPopState);
  }
  routers.clear();
});

function addressQrCode(router: Router, hash: string) {
  router.query.hash = hash;
  return React.createElement(
    RouterContext.Provider,
    { value: router },
    React.createElement(AddressQrPage),
  );
}

function openQrCode(view: ReturnType<typeof render>) {
  const trigger = view.getByRole('button', { name: 'Show QR code' });
  trigger.focus();
  fireEvent.click(trigger);
}

function assertLoading(view: ReturnType<typeof render>, hash: string) {
  const dialog = view.getByRole('dialog');
  expect(dialog.querySelector('[data-entity-content]')?.textContent).toBe(hash);
  expect(dialog.querySelector(QR_SELECTOR)).toBeNull();
  expect(view.getByRole('status', { name: 'Generating QR code' })).toBeTruthy();
}

async function assertGeneratedDestination(view: ReturnType<typeof render>, hash: string) {
  // Compare the actual scannable SVG paths with the real encoder's output for this destination.
  const expected = document.createElement('div');
  expected.innerHTML = await QRCode.toString(hash, { type: 'svg', margin: 0 });
  const expectedSvg = expected.querySelector('svg');
  expect(expectedSvg).not.toBeNull();

  await waitFor(() => {
    const dialog = view.getByRole('dialog');
    expect(dialog.querySelector('[data-entity-content]')?.textContent).toBe(hash);
    expect(dialog.querySelector(QR_SELECTOR)?.outerHTML).toBe(expectedSvg?.outerHTML);
    expect(view.queryByRole('status', { name: 'Generating QR code' })).toBeNull();
  });
}

test('qrHashTransition', async function qrHashTransition() {
  const router = createAddressRouter();
  const view = render(addressQrCode(router, FIRST_HASH));
  openQrCode(view);
  await assertGeneratedDestination(view, FIRST_HASH);
  const firstSvg = view.getByRole('dialog').querySelector(QR_SELECTOR)?.outerHTML;

  view.rerender(addressQrCode(router, SECOND_HASH));
  // Rerender returns before the generator's asynchronous import continuation can finish.
  assertLoading(view, SECOND_HASH);
  await assertGeneratedDestination(view, SECOND_HASH);
  expect(view.getByRole('dialog').querySelector(QR_SELECTOR)?.outerHTML).not.toBe(firstSvg);

  view.rerender(addressQrCode(router, FIRST_HASH));
  assertLoading(view, FIRST_HASH);
  view.rerender(addressQrCode(router, THIRD_HASH));
  assertLoading(view, THIRD_HASH);
  await assertGeneratedDestination(view, THIRD_HASH);
});

test('qrHashTransitionOnReopen', async function qrHashTransitionOnReopen() {
  const router = createAddressRouter();
  const view = render(addressQrCode(router, FIRST_HASH));
  openQrCode(view);
  await assertGeneratedDestination(view, FIRST_HASH);
  fireEvent.click(view.getByRole('button', { name: 'Close' }));
  await waitFor(() => expect(view.queryByRole('dialog')).toBeNull());
  // Finish the real dialog's exit and focus restoration before starting another open cycle.
  await waitFor(() => {
    expect(view.queryByRole('dialog', { hidden: true })).toBeNull();
    expect(document.activeElement).toBe(view.getByRole('button', { name: 'Show QR code' }));
  });

  view.rerender(addressQrCode(router, SECOND_HASH));
  openQrCode(view);
  await waitFor(() => assertLoading(view, SECOND_HASH));
  await assertGeneratedDestination(view, SECOND_HASH);

  view.unmount();
  const reopened = render(addressQrCode(router, THIRD_HASH));
  openQrCode(reopened);
  await assertGeneratedDestination(reopened, THIRD_HASH);
});
