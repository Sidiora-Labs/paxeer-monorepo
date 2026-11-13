// @vitest-environment jsdom
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { useAppRoute, type AppRouteState } from '../../widgets/shell/useAppRoute';
import { parseRouteQuery, parseRouteUrl, serializeRoute, type ShellRoute } from '.';
import { ROUTES } from './navigation.test';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const RECIPIENT = '0x2222222222222222222222222222222222222222';
const TOKEN = '0x1111111111111111111111111111111111111111';
let root: Root | null = null;
let container: HTMLDivElement;
let current: AppRouteState | null = null;

function RecipientRouteConsumer() {
  const state = useAppRoute();
  useLayoutEffect(() => {
    current = state;
  });
  return createElement('output', null, state.routeState.name === 'send' ? state.routeState.to : '');
}

function appRoute(): AppRouteState {
  if (!current) throw new Error('The recipient route consumer has not mounted');
  return current;
}

async function mountConsumer() {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(RecipientRouteConsumer));
  });
}

function expectRecipient(expected: ShellRoute) {
  expect(appRoute().routeState).toEqual(expected);
  expect(container.textContent).toBe(RECIPIENT);
  expect(parseRouteUrl(window.location.href)).toEqual({ ok: true, value: expected });
  expect(new URL(window.location.href).searchParams.get('to')).toBe(RECIPIENT);
  expect(window.history.state.paxportRoute).toBe('send');
}

async function proveRecipientHistoryAndReload(token?: string) {
  const expected: ShellRoute = {
    name: 'send',
    ...(token ? { token } : {}),
    to: RECIPIENT,
  };
  const fixture = ROUTES.find(
    (route) => route.name === 'send' && route.to === RECIPIENT && route.token === token
  );
  expect(fixture).toEqual(expected);
  const parsed = parseRouteQuery(fixture);
  expect(parsed).toEqual({ ok: true, value: expected });
  if (!parsed.ok) throw new Error('The recipient-bearing navigation fixture must parse');

  await mountConsumer();
  expect(appRoute().routeState).toEqual({ name: 'portfolio' });
  const historyLength = window.history.length;
  await act(async () => {
    window.dispatchEvent(
      new CustomEvent('paxeer:deeplink', { detail: { url: serializeRoute(parsed.value) } })
    );
  });
  expectRecipient(expected);
  expect(window.history.length).toBe(historyLength + 1);
  expect(window.history.state.paxportDepth).toBe(1);

  await act(async () => appRoute().setRoute('settings'));
  expect(appRoute().routeState).toEqual({ name: 'settings' });
  expect(container.textContent).toBe('');
  await act(async () => {
    const popped = new Promise<void>((resolve) => {
      window.addEventListener('popstate', () => resolve(), { once: true });
    });
    window.history.back();
    await popped;
  });
  expectRecipient(expected);
  const savedUrl = window.location.href;

  await act(async () => root?.unmount());
  root = null;
  current = null;
  expect(container.textContent).toBe('');
  await mountConsumer();
  expectRecipient(expected);
  expect(window.location.href).toBe(savedUrl);
  expect(window.history.state.paxportDepth).toBe(0);

  await act(async () => {
    const popped = new Promise<void>((resolve) => {
      window.addEventListener('popstate', () => resolve(), { once: true });
    });
    window.history.forward();
    await popped;
  });
  expect(appRoute().routeState).toEqual({ name: 'settings' });
  expect(container.textContent).toBe('');
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

async function recipientOnlyRouteSurvivesHistoryCommitAndReload() {
  await proveRecipientHistoryAndReload();
}

async function nativeTokenRecipientRouteSurvivesHistoryCommitAndReload() {
  await proveRecipientHistoryAndReload('pax');
}

async function erc20RecipientRouteSurvivesHistoryCommitAndReload() {
  await proveRecipientHistoryAndReload(TOKEN);
}

test(
  recipientOnlyRouteSurvivesHistoryCommitAndReload.name,
  recipientOnlyRouteSurvivesHistoryCommitAndReload
);
test(
  nativeTokenRecipientRouteSurvivesHistoryCommitAndReload.name,
  nativeTokenRecipientRouteSurvivesHistoryCommitAndReload
);
test(
  erc20RecipientRouteSurvivesHistoryCommitAndReload.name,
  erc20RecipientRouteSurvivesHistoryCommitAndReload
);
