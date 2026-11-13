// @vitest-environment jsdom
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { useAppRoute, type AppRouteState } from '../../widgets/shell/useAppRoute';
import { parseRouteQuery, parseRouteUrl, serializeRoute, type ShellRoute } from '.';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const RECIPIENT = '0xAbCdEf0123456789AbCdEf0123456789AbCdEf01';
const TOKEN = '0x1234567890AbCdEf1234567890AbCdEf12345678';
let root: Root | null = null;
let container: HTMLDivElement;
let current: AppRouteState | null = null;

function RouteConsumer() {
  const route = useAppRoute();
  useLayoutEffect(() => {
    current = route;
  });
  return createElement('output', null, route.routeState.name === 'send' ? route.routeState.to : '');
}

function appRoute(): AppRouteState {
  if (!current) throw new Error('The route consumer has not mounted');
  return current;
}

async function mountRouteConsumer() {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(RouteConsumer));
  });
}

function expectSendRoute(expected: ShellRoute) {
  expect(appRoute().routeState).toEqual(expected);
  expect(parseRouteUrl(window.location.href)).toEqual({ ok: true, value: expected });
  expect(container.textContent).toBe(RECIPIENT.toLowerCase());
  expect(window.history.state.paxportRoute).toBe('send');
}

beforeEach(() => {
  window.history.replaceState(null, '', '/wallet/');
  container = document.createElement('div');
  document.body.appendChild(container);
  current = null;
});

afterEach(async () => {
  await act(async () => root?.unmount());
  root = null;
  current = null;
  container.remove();
  window.history.replaceState(null, '', '/wallet/');
});

function sendRecipientRoundTripsThroughBothRouteParsers() {
  for (const token of [undefined, 'pax', TOKEN]) {
    const query = { name: 'send', to: RECIPIENT, ...(token ? { token } : {}) };
    const expected: ShellRoute = {
      name: 'send',
      to: RECIPIENT.toLowerCase(),
      ...(token ? { token: token.toLowerCase() } : {}),
    };
    const parsed = parseRouteQuery(query);
    expect(parsed).toEqual({ ok: true, value: expected });
    if (!parsed.ok) throw new Error('The send query must be accepted');
    const serialized = serializeRoute(parsed.value);
    expect(new URL(serialized, window.location.origin).searchParams.get('to')).toBe(expected.to);
    expect(parseRouteUrl(serialized)).toEqual(parsed);
  }
}

async function sendRecipientSurvivesInitialDeepLinkCommit() {
  window.history.replaceState(null, '', `/wallet/?screen=send&token=${TOKEN}&to=${RECIPIENT}`);
  const expected: ShellRoute = {
    name: 'send',
    token: TOKEN.toLowerCase(),
    to: RECIPIENT.toLowerCase(),
  };
  expect(parseRouteUrl(window.location.href)).toEqual({ ok: true, value: expected });
  await mountRouteConsumer();
  expectSendRoute(expected);
  expect(window.history.state.paxportDepth).toBe(0);
}

async function sendRecipientSurvivesNavigationHistoryAndReload() {
  await mountRouteConsumer();
  const destination: ShellRoute = { name: 'send', token: 'pax', to: RECIPIENT };
  const expected: ShellRoute = { ...destination, to: RECIPIENT.toLowerCase() };
  const initialHistoryLength = window.history.length;
  await act(async () => {
    window.dispatchEvent(
      new CustomEvent('paxeer:deeplink', { detail: { url: serializeRoute(destination) } })
    );
  });
  expectSendRoute(expected);
  expect(window.history.length).toBe(initialHistoryLength + 1);
  expect(window.history.state.paxportDepth).toBe(1);

  await act(async () => appRoute().setRoute('receive'));
  expect(appRoute().routeState).toEqual({ name: 'receive' });
  await act(async () => {
    const popped = new Promise<void>((resolve) => {
      window.addEventListener('popstate', () => resolve(), { once: true });
    });
    window.history.back();
    await popped;
  });
  expectSendRoute(expected);

  await act(async () => {
    const popped = new Promise<void>((resolve) => {
      window.addEventListener('popstate', () => resolve(), { once: true });
    });
    window.history.forward();
    await popped;
  });
  expect(appRoute().routeState).toEqual({ name: 'receive' });
  await act(async () => appRoute().replaceRoute(destination));
  expectSendRoute(expected);

  const savedUrl = window.location.href;
  await act(async () => root?.unmount());
  root = null;
  current = null;
  window.history.replaceState(null, '', savedUrl);
  await mountRouteConsumer();
  expectSendRoute(expected);
  expect(window.history.state.paxportDepth).toBe(0);
}

async function sendRecipientValidationRejectsInvalidDestinations() {
  await mountRouteConsumer();
  for (const to of ['', '0x1234', `${RECIPIENT}0`, 'alice.pax', 'javascript:alert(1)']) {
    expect(parseRouteQuery({ name: 'send', to }).ok).toBe(false);
    expect(parseRouteUrl(`/wallet/?screen=send&to=${encodeURIComponent(to)}`).ok).toBe(false);
  }
  await act(async () => appRoute().replaceRoute({ name: 'send', token: 'pax', to: '0x1234' }));
  expect(appRoute().routeState).toEqual({ name: 'portfolio' });
  expect(window.location.search).toBe('');
  expect(container.textContent).toBe('');
}

test(
  sendRecipientRoundTripsThroughBothRouteParsers.name,
  sendRecipientRoundTripsThroughBothRouteParsers
);
test(sendRecipientSurvivesInitialDeepLinkCommit.name, sendRecipientSurvivesInitialDeepLinkCommit);
test(
  sendRecipientSurvivesNavigationHistoryAndReload.name,
  sendRecipientSurvivesNavigationHistoryAndReload
);
test(
  sendRecipientValidationRejectsInvalidDestinations.name,
  sendRecipientValidationRejectsInvalidDestinations
);
