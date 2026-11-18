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
import { fireEvent, screen, wrapper as TestApp } from 'vitest/lib';

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

test('hero and compact render prompt variants', function heroAndCompactRenderPromptVariants(): void {
  const { container } = render(<><SearchBarInput isHeroBanner/><SearchBarInput/></>);
  const inputs = Array.from(container.querySelectorAll('input'));
  expect(inputs.map(input => input.placeholder)).toEqual([ HERO_PLACEHOLDER, COMPACT_PLACEHOLDER ]);
  expect(inputs.map(input => input.getAttribute('aria-label'))).toEqual([ 'Search', 'Search' ]);
  expect(window.getComputedStyle(inputs[0]).height).toBe('auto');
  expect(window.getComputedStyle(inputs[0]).minHeight).toBe('calc(var(--chakra-spacing-12) + var(--chakra-spacing-1))');
  expect(window.getComputedStyle(inputs[0].closest('form')!).width).toBe('calc(var(--chakra-spacing-80) * 2)');
  expect(window.getComputedStyle(inputs[1].closest('form')!).width).toBe('var(--chakra-spacing-80)');
  expect(window.getComputedStyle(inputs[1]).borderRadius).toBe('var(--chakra-radii-pill)');
  expect(window.getComputedStyle(inputs[0]).borderWidth).toBe('var(--chakra-border-widths-1)');
  expect(window.getComputedStyle(inputs[1]).borderWidth).toBe('var(--chakra-border-widths-1)');
  expect(container.textContent).toMatch(/Ctrl K|⌘K/);
});

test('explicit compact and hero render in both modes', async function explicitCompactAndHeroRenderInBothModes(): Promise<void> {
  for (const mode of [ 'light', 'dark' ] as const) {
    const Wrapper = ({ children }: { children: React.ReactNode }) => (
      <Provider forcedTheme={ mode }><TestApp>{ children }</TestApp></Provider>
    );
    const { container, unmount } = renderWithWrapper(<><SearchBarInput isHeroBanner/><SearchBarInput compact/></>, { wrapper: Wrapper });
    await vi.waitFor(() => expect(document.documentElement.classList.contains(mode)).toBe(true));
    const inputs = Array.from(container.querySelectorAll('input'));
    expect(inputs.map(input => input.placeholder)).toEqual([ HERO_PLACEHOLDER, COMPACT_PLACEHOLDER ]);
    expect(inputs.map(input => input.getAttribute('aria-label'))).toEqual([ 'Search', 'Search or ask' ]);
    expect(inputs.map(input => window.getComputedStyle(input).height)).toEqual([ 'auto', 'auto' ]);
    expect(inputs.map(input => window.getComputedStyle(input).minHeight))
      .toEqual([ 'calc(var(--chakra-spacing-12) + var(--chakra-spacing-1))', 'var(--chakra-sizes-touch-target)' ]);
    expect(inputs.map(input => window.getComputedStyle(input.closest('form')!).width))
      .toEqual([ 'calc(var(--chakra-spacing-80) * 2)', 'var(--chakra-spacing-80)' ]);
    expect(inputs.map(input => window.getComputedStyle(input).borderRadius)).toEqual([ 'var(--chakra-radii-control)', 'var(--chakra-radii-pill)' ]);
    expect(inputs.map(input => window.getComputedStyle(input).borderWidth)).toEqual([ 'var(--chakra-border-widths-1)', 'var(--chakra-border-widths-1)' ]);
    expect(container.querySelectorAll('kbd')).toHaveLength(2);
    unmount();
  }
});

test('command and control focus nearest search', function commandAndControlFocusNearestSearch(): void {
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
});

test('shortcut ignores hidden fields and plain typing', function shortcutIgnoresHiddenFieldsAndPlainTyping(): void {
  const { container } = render(
    <><div hidden><SearchBarDesktop/></div><SearchBarDesktop isHeroBanner/></>,
  );
  const inputs = container.querySelectorAll('input');
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true }));
  expect(document.activeElement).toBe(inputs[1]);
  inputs[1].blur();
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', bubbles: true }));
  expect(document.activeElement).toBe(document.body);
});

test('mobile shortcut opens and focuses search', async function mobileShortcutOpensAndFocusesSearch(): Promise<void> {
  routerState.pathname = '/txs';
  render(<SearchBarMobile/>);
  fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
  await vi.waitFor(() => {
    const input = document.querySelector('input');
    expect(input).not.toBeNull();
    expect(input?.placeholder).toBe(COMPACT_PLACEHOLDER);
    expect(document.activeElement).toBe(input);
  }, { timeout: 10_000 });
});

async function assertGroupedSuggestions(isHeroBanner: boolean): Promise<Array<string | null>> {
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
    expect(window.getComputedStyle(groups[0].querySelector('a')!).minHeight).toBe('var(--chakra-sizes-touch-target)');
  }, { timeout: 30_000, interval: 100 });
  const searchRequests = fetchMock.mock.calls.map(([ request ]) => String(request));
  expect(searchRequests.some(url => url.includes('/api/v2/search/quick') && url.includes('q=o'))).toBe(true);
  const headings = Array.from(document.querySelectorAll('[data-scroll-target] [role="heading"]')).map(heading => heading.textContent);
  fireEvent.keyDown(input, { key: 'Escape' });
  expect(document.activeElement).not.toBe(input);
  return headings;
}

test('hero renders grouped response', async function heroRendersGroupedResponse(): Promise<void> {
  expect(await assertGroupedSuggestions(true)).toEqual(LABELS);
}, 40_000);

test('compact opens same grouped panel', async function compactOpensSameGroupedPanel(): Promise<void> {
  routerState.pathname = '/blocks';
  expect(await assertGroupedSuggestions(false)).toEqual(LABELS);
}, 40_000);

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

test('search colours resolve in both modes', function searchColoursResolveInBothModes(): void {
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
});

test('mobile hero uses an accessible search trigger', async function mobileHeroUsesAnAccessibleSearchTrigger(): Promise<void> {
  const { container } = render(<SearchBarMobile isHeroBanner/>);
  const trigger = screen.getByRole('button', { name: 'Search' });
  expect(container.querySelector('form')).toBeNull();
  trigger.focus();
  expect(document.activeElement).toBe(trigger);
  fireEvent.click(trigger);
  await vi.waitFor(() => {
    const input = document.querySelector('input');
    expect(input?.name).toBe('q');
    expect(document.activeElement).toBe(input);
  });
});

test('mobile hero keeps the search and ask prompt on one line', function mobileHeroKeepsTheSearchAndAskPromptOnOneLine(): void {
  render(<SearchBarMobile isHeroBanner/>);
  const trigger = screen.getByRole('button', { name: 'Search' });
  const style = window.getComputedStyle(trigger);

  expect(trigger.textContent).toBe(COMPACT_PLACEHOLDER);
  expect(style.whiteSpace).toBe('nowrap');
  expect(style.overflow).toBe('visible');
  expect(style.textOverflow).not.toBe('ellipsis');
  expect(style.width).toBe('100%');
  expect(style.minHeight).toBe('calc(var(--chakra-spacing-12) + var(--chakra-spacing-1))');
  expect(style.minWidth).toBe('var(--chakra-sizes-touch-target)');
  expect(window.getComputedStyle(trigger.querySelector('span')!).fontSize).toBe('0.8125rem');
});
