// @vitest-environment jsdom

import { render as renderWithWrapper } from '@testing-library/react';
import React from 'react';

import { getExternalAssetFilePath } from 'configs/app/utils';
import { apps as appsFixture } from 'mocks/apps/apps';
import * as searchFixture from 'mocks/search/index';
import { Provider } from 'toolkit/chakra/provider';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, expect, test, vi } from 'vitest';
import { fireEvent, wrapper as TestApp } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_ENABLED: 'true',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: 'https://localhost:3000/marketplace-config.json',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import SearchBarDesktop from './SearchBarDesktop';
import SearchBarInput from './SearchBarInput';
import SearchBarMobile from './SearchBarMobile';

const MARKETPLACE_CONFIG_PATH = getExternalAssetFilePath('NEXT_PUBLIC_MARKETPLACE_CONFIG_URL');
const HERO_PLACEHOLDER = 'Search or ask: a transaction, address, block or token';
const COMPACT_PLACEHOLDER = 'Search or ask…';
const LABELS = [ 'TRANSACTIONS', 'ADDRESSES', 'BLOCKS', 'TOKENS', 'APPS' ];
const RESULTS = [ searchFixture.token2, searchFixture.block1, searchFixture.address1, searchFixture.tx1, searchFixture.token1 ];

beforeEach(() => {
  routerState.pathname = '/';
  routerState.query = {};
  window.localStorage.clear();
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    const body = (() => {
      if (request.url.includes('/api/v2/search/quick')) return RESULTS;
      if (new URL(request.url, window.location.href).pathname === MARKETPLACE_CONFIG_PATH) return appsFixture;
      return {};
    })();
    return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
  });
});

function heroAndCompactRenderPromptVariants(): void {
  const { container } = render(<><SearchBarInput isHeroBanner/><SearchBarInput/></>);
  const inputs = Array.from(container.querySelectorAll('input'));
  expect(inputs.map(input => input.placeholder)).toEqual([ HERO_PLACEHOLDER, COMPACT_PLACEHOLDER ]);
  expect(inputs.map(input => input.getAttribute('aria-label'))).toEqual([ 'Search', 'Search' ]);
  expect(window.getComputedStyle(inputs[0]).height).toBe('52px');
  expect(window.getComputedStyle(inputs[0].closest('form')!).width).toBe('640px');
  expect(window.getComputedStyle(inputs[1].closest('form')!).width).toBe('320px');
  expect(window.getComputedStyle(inputs[1]).borderRadius).toBe('999px');
  expect(window.getComputedStyle(inputs[0]).borderWidth).toBe('1px');
  expect(window.getComputedStyle(inputs[1]).borderWidth).toBe('1px');
  expect(container.textContent).toMatch(/Ctrl K|⌘K/);
}

test(heroAndCompactRenderPromptVariants.name, heroAndCompactRenderPromptVariants);

async function explicitCompactAndHeroRenderInBothModes(): Promise<void> {
  for (const mode of [ 'light', 'dark' ] as const) {
    const Wrapper = ({ children }: { children: React.ReactNode }) => (
      <Provider forcedTheme={ mode }><TestApp>{ children }</TestApp></Provider>
    );
    const { container, unmount } = renderWithWrapper(<><SearchBarInput isHeroBanner/><SearchBarInput compact/></>, { wrapper: Wrapper });
    await vi.waitFor(() => expect(document.documentElement.classList.contains(mode)).toBe(true));
    const inputs = Array.from(container.querySelectorAll('input'));
    expect(inputs.map(input => input.placeholder)).toEqual([ HERO_PLACEHOLDER, COMPACT_PLACEHOLDER ]);
    expect(inputs.map(input => input.getAttribute('aria-label'))).toEqual([ 'Search', 'Search or ask' ]);
    expect(inputs.map(input => window.getComputedStyle(input).height)).toEqual([ '52px', '36px' ]);
    expect(inputs.map(input => window.getComputedStyle(input.closest('form')!).width)).toEqual([ '640px', '320px' ]);
    expect(inputs.map(input => window.getComputedStyle(input).borderRadius)).toEqual([ '6px', '999px' ]);
    expect(inputs.map(input => window.getComputedStyle(input).borderWidth)).toEqual([ '1px', '1px' ]);
    expect(container.querySelectorAll('kbd')).toHaveLength(2);
    unmount();
  }
}

test(explicitCompactAndHeroRenderInBothModes.name, explicitCompactAndHeroRenderInBothModes);

function commandAndControlFocusNearestSearch(): void {
  routerState.pathname = '/blocks';
  const { container, unmount } = render(
    <>
      <section><button type="button">Header action</button><SearchBarDesktop compact/></section>
      <section><button type="button">Hero action</button><SearchBarDesktop isHeroBanner/></section>
    </>,
  );
  const inputs = Array.from(container.querySelectorAll('input'));
  const buttons = Array.from(container.querySelectorAll('section > button')) as Array<HTMLButtonElement>;
  buttons[1].focus();
  const command = new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(command);
  expect(document.activeElement).toBe(inputs[1]);
  expect(command.defaultPrevented).toBe(true);
  buttons[0].focus();
  const control = new KeyboardEvent('keydown', { key: 'K', ctrlKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(control);
  expect(document.activeElement).toBe(inputs[0]);
  expect(control.defaultPrevented).toBe(true);
  unmount();
  const afterUnmount = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true, cancelable: true });
  document.dispatchEvent(afterUnmount);
  expect(afterUnmount.defaultPrevented).toBe(false);
}

test(commandAndControlFocusNearestSearch.name, commandAndControlFocusNearestSearch);

function shortcutIgnoresHiddenFieldsAndPlainTyping(): void {
  const { container } = render(
    <><div hidden><SearchBarDesktop/></div><SearchBarDesktop isHeroBanner/></>,
  );
  const inputs = container.querySelectorAll('input');
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true }));
  expect(document.activeElement).toBe(inputs[1]);
  inputs[1].blur();
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', bubbles: true }));
  expect(document.activeElement).toBe(document.body);
}

test(shortcutIgnoresHiddenFieldsAndPlainTyping.name, shortcutIgnoresHiddenFieldsAndPlainTyping);

async function mobileShortcutOpensAndFocusesSearch(): Promise<void> {
  routerState.pathname = '/txs';
  render(<SearchBarMobile/>);
  fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
  await vi.waitFor(() => {
    const input = document.querySelector('input');
    expect(input).not.toBeNull();
    expect(input?.placeholder).toBe(COMPACT_PLACEHOLDER);
    expect(document.activeElement).toBe(input);
  }, { timeout: 10_000 });
}

test(mobileShortcutOpensAndFocusesSearch.name, mobileShortcutOpensAndFocusesSearch);

async function assertGroupedSuggestions(isHeroBanner: boolean): Promise<void> {
  const { container } = render(<SearchBarDesktop isHeroBanner={ isHeroBanner } compact={ !isHeroBanner }/>);
  const input = container.querySelector('input')!;
  input.focus();
  fireEvent.change(input, { target: { value: 'o' } });
  await vi.waitFor(() => {
    const headings = Array.from(document.querySelectorAll('[data-scroll-target] [role="heading"]'));
    expect(headings.map(heading => heading.textContent)).toEqual(LABELS);
    const groups = headings.map(heading => heading.parentElement!);
    expect(groups.map(group => group.querySelectorAll('a').length)).toEqual([ 1, 1, 1, 2, 2 ]);
    expect(groups[0].querySelector('a')?.getAttribute('href')).toContain(`/tx/${ searchFixture.tx1.transaction_hash }`);
    expect(groups[1].querySelector('a')?.getAttribute('href')).toContain(`/address/${ searchFixture.address1.address_hash }`);
    expect(groups[2].querySelector('a')?.getAttribute('href')).toContain('/block/');
    expect(groups[3].querySelector('a')?.textContent).toContain(searchFixture.token2.name);
    expect(groups[3].textContent).toContain(searchFixture.token1.name);
    expect(groups[4].textContent).toContain(appsFixture[0].title);
    expect(groups[4].textContent).toContain(appsFixture[0].description);
    expect(window.getComputedStyle(groups[0].querySelector('a')!).minHeight).toBe('44px');
  }, { timeout: 30_000, interval: 100 });
  const searchRequests = fetchMock.mock.calls.map(([ request ]) => String(request));
  expect(searchRequests.some(url => url.includes('/api/v2/search/quick') && url.includes('q=o'))).toBe(true);
  fireEvent.keyDown(input, { key: 'Escape' });
  expect(document.activeElement).not.toBe(input);
}

async function heroRendersGroupedResponse(): Promise<void> {
  await assertGroupedSuggestions(true);
}

test(heroRendersGroupedResponse.name, heroRendersGroupedResponse, 40_000);

async function compactOpensSameGroupedPanel(): Promise<void> {
  routerState.pathname = '/blocks';
  await assertGroupedSuggestions(false);
}

test(compactOpensSameGroupedPanel.name, compactOpensSameGroupedPanel, 40_000);

function atPath(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (typeof node !== 'object' || node === null) return undefined;
    return (node as Record<string, unknown>)[key];
  }, root);
}

function resolveColour(path: string, mode: '_light' | '_dark', visited = new Set<string>()): string {
  expect(visited.has(path)).toBe(false);
  const next = new Set(visited).add(path);
  let node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
  if (typeof node === 'object' && node !== null && !('value' in node)) node = (node as Record<string, unknown>).DEFAULT;
  const value = atPath(node, 'value');
  const raw = typeof value === 'string' ? value : atPath(value, mode);
  if (typeof raw !== 'string' || !raw.length) throw new Error(`Unresolved colour ${ path } in ${ mode }`);
  const reference = /^\{(.+)\}$/.exec(raw);
  return reference ? resolveColour(reference[1], mode, next) : raw;
}

function searchColoursResolveInBothModes(): void {
  const roles = [ 'raised', 'canvas', 'hairline', 'border.strong', 'accent', 'accent.soft', 'focusRing',
    'text.primary', 'text.secondary', 'text.muted', 'positive' ];
  for (const role of roles) {
    for (const mode of [ '_light', '_dark' ] as const) {
      expect(resolveColour(`colors.${ role }`, mode)).toMatch(/^(#[\da-f]{6}|rgba?\()/i);
    }
  }
  expect(resolveColour('colors.raised', '_light')).toBe('#FFFFFF');
  expect(resolveColour('colors.raised', '_dark')).toBe('#171A21');
  expect(resolveColour('colors.accent', '_light')).toBe('#2B4FDA');
  expect(resolveColour('colors.accent', '_dark')).toBe('#4F7DFF');
}

test(searchColoursResolveInBothModes.name, searchColoursResolveInBothModes);
