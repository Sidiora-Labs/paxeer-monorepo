// @vitest-environment jsdom
// @vitest-environment-options {"url":"https://localhost/explorer"}

import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import React from 'react';

import * as cookies from 'lib/cookies';
import { getThemeHexWithOverrides } from 'lib/settings/colorTheme';
import * as addressMock from 'mocks/address/address';
import { forContract } from 'mocks/address/counters';
import { base as addressTabCounters } from 'mocks/address/tabCounters';
import { base as block } from 'mocks/blocks/block';
import { base as dailyTxs } from 'mocks/stats/daily_txs';
import { base as stats } from 'mocks/stats/index';
import { tokenInfoERC20b } from 'mocks/tokens/tokenInfo';
import { base as transaction } from 'mocks/txs/tx';
import { useColorMode } from 'toolkit/chakra/color-mode';
import colors, { LIGHT, DARK } from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import Address from 'ui/pages/Address';
import Home from 'ui/pages/Home';
import Transaction from 'ui/pages/Transaction';
import Layout from 'ui/shared/layout/Layout';
import LayoutHome from 'ui/shared/layout/LayoutHome';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import ColorModeToggle from 'ui/snippets/topBar/ColorModeToggle';
import SettingsColorTheme from 'ui/snippets/topBar/settings/SettingsColorTheme';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { fireEvent, screen, waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'vertical',
    NEXT_PUBLIC_STATS_API_HOST: '',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());
vi.setConfig({ testTimeout: 60_000 });

// jsdom supplies no SVG geometry; the real chart animation asks for its path length.
if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

const appRoot = path.resolve(__dirname, '..');
const widthMatchMedia = window.matchMedia;
let systemDark = false;

beforeEach(() => {
  localStorage.clear();
  cookies.remove(cookies.NAMES.COLOR_MODE);
  cookies.remove(cookies.NAMES.COLOR_THEME);
  document.documentElement.removeAttribute('style');
  document.documentElement.classList.remove('dark', 'light');
  systemDark = false;
  window.matchMedia = (query) => query.includes('prefers-color-scheme') ? {
    ...widthMatchMedia(query),
    matches: systemDark,
  } : widthMatchMedia(query);
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    const url = new URL(request.url);
    const pathname = url.pathname;
    let body: unknown;
    if (pathname === '/api/v2/tokens') body = { items: [ tokenInfoERC20b ], next_page_params: null };
    else if (pathname.includes('/stats/charts/transactions')) body = dailyTxs;
    else if (pathname === '/api/v2/stats') body = stats;
    else if (pathname === '/api/v2/main-page/blocks') body = [ block ];
    else if (pathname === '/api/v2/main-page/transactions') body = [ transaction ];
    else if (pathname === `/api/v2/transactions/${ transaction.hash }`) body = transaction;
    else if (pathname === `/api/v2/addresses/${ addressMock.hash }`) body = addressMock.contract;
    else if (pathname.endsWith('/tabs-counters')) body = addressTabCounters;
    else if (pathname.includes('/addresses/') && pathname.endsWith('/counters')) body = forContract;
    else if (pathname.endsWith('/token-balances')) body = [];
    else body = { items: [], next_page_params: null, addresses: {}, days: 0 };
    return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
  });
});

afterEach(() => {
  window.matchMedia = widthMatchMedia;
});

function sourceFiles(directory: string): Array<string> {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) return sourceFiles(filename);
    return /\.(?:[cm]?[jt]sx?|css)$/.test(filename) ? [ filename ] : [];
  });
}

function componentsUseSemanticColours() {
  // Tests and API payload fixtures deliberately exercise arbitrary caller-provided colours.
  const allowedFiles = /(?:\.(?:spec|fixtures)\.[jt]sx?$|\/mocks(?:\/|\.[jt]sx?$))/;
  const literal = /(?<![\w&])#[\da-f]{3,8}\b(?!;)|\brgba?\(\s*\d/gi;
  const violations = [ 'ui', 'toolkit/components' ].flatMap((directory) => sourceFiles(path.join(appRoot, directory)))
    .filter((filename) => !allowedFiles.test(filename))
    .flatMap((filename) => readFileSync(filename, 'utf8').split('\n').flatMap((line, index) => {
      const values = line.match(literal);
      return values ? [ `${ path.relative(appRoot, filename) }:${ index + 1 }: ${ values.join(', ') }` ] : [];
    }));
  expect(violations, `Literal colours outside foundations:\n${ violations.join('\n') }`).toEqual([]);
}

function nestedValue(source: unknown, key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => (
    typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[part] : undefined
  ), source);
}

function resolveColour(key: string, mode: 'light' | 'dark', visited = new Set<string>()): string {
  expect(visited.has(key), `Cyclic token ${ key }`).toBe(false);
  const node = nestedValue(semanticTokens, key) ?? nestedValue({ colors }, key);
  const normalized = nestedValue(node, 'DEFAULT') ?? node;
  const value = nestedValue(normalized, 'value');
  const raw = typeof value === 'string' ? value : nestedValue(value, `_${ mode }`);
  expect(raw, `Missing token ${ key } in ${ mode }`).toBeTypeOf('string');
  const reference = /^\{(.+)\}$/.exec(String(raw));
  return reference ? resolveColour(reference[1], mode, new Set(visited).add(key)) : String(raw);
}

function cssRules(rules: CSSRuleList): Array<CSSStyleRule> {
  return Array.from(rules).flatMap((rule) => {
    const ownRule = 'selectorText' in rule ? [ rule as CSSStyleRule ] : [];
    return 'cssRules' in rule ? [ ...ownRule, ...cssRules((rule as CSSGroupingRule).cssRules) ] : ownRule;
  });
}

function ModeState() {
  const { colorMode } = useColorMode();
  return React.createElement('output', { 'data-testid': 'mode' }, colorMode);
}

async function pageBodyBackground(Page: typeof Home | typeof Address | typeof Transaction, mode: 'light' | 'dark') {
  localStorage.setItem(cookies.NAMES.COLOR_MODE, mode);
  // An opposite OS preference proves the saved mode wins during mounting.
  systemDark = mode === 'light';
  const rendered = render(React.createElement(React.Fragment, null, React.createElement(ModeState), React.createElement(Page)));
  await waitFor(() => {
    expect(screen.getByTestId('mode').textContent).toBe(mode);
    expect(document.documentElement.classList.contains(mode)).toBe(true);
  });
  if (Page === Home) {
    await screen.findByText(tokenInfoERC20b.symbol!);
    await waitFor(() => expect(rendered.container.querySelector('[data-latest-tx]')?.getAttribute('data-latest-tx')).toBe(transaction.hash));
  } else if (Page === Transaction) {
    await waitFor(() => expect(rendered.container.querySelector('[data-section-group="SUMMARY"]')).not.toBeNull());
    expect(rendered.container.querySelector('h1')?.textContent).toBe(transaction.hash);
  } else {
    await waitFor(() => expect(rendered.container.querySelector('[data-entity-kind-label]')?.textContent).toBe('CONTRACT'));
    expect(rendered.container.querySelector('[data-section-group="ADDRESS INFO"]')).not.toBeNull();
  }
  const rules = Array.from(document.styleSheets).flatMap((sheet) => cssRules(sheet.cssRules));
  // jsdom does not resolve CSS custom properties. Assert the actual injected body rule,
  // then resolve that rule's semantic token through the same foundations used by Chakra.
  expect(rules.some((rule) => rule.selectorText === 'body' && rule.style.getPropertyValue('background') === 'var(--chakra-colors-global-body-bg)')).toBe(true);
  expect(rules.some((rule) => rule.style.getPropertyValue('--chakra-colors-global-body-bg') === 'var(--chakra-colors-bg-primary)')).toBe(true);
  const background = resolveColour('colors.global.body.bg', mode);
  expect(background).toBe(mode === 'light' ? LIGHT.canvas : DARK.canvas);
  rendered.unmount();
  return background;
}

async function homeRendersInBothColourModes() {
  routerState.pathname = '/';
  routerState.query = {};
  const light = await pageBodyBackground(Home, 'light');
  const dark = await pageBodyBackground(Home, 'dark');
  expect(light, '/ body background').not.toBe(dark);
}

async function transactionRendersInBothColourModes() {
  routerState.pathname = '/tx/[hash]';
  routerState.query = { hash: transaction.hash };
  const light = await pageBodyBackground(Transaction, 'light');
  const dark = await pageBodyBackground(Transaction, 'dark');
  expect(light, '/tx/[hash] body background').not.toBe(dark);
}

async function addressRendersInBothColourModes() {
  routerState.pathname = '/address/[hash]';
  routerState.query = { hash: addressMock.hash };
  const light = await pageBodyBackground(Address, 'light');
  const dark = await pageBodyBackground(Address, 'dark');
  expect(light, '/address/[hash] body background').not.toBe(dark);
}

async function storedPreferenceToggleAndSettingsSurviveRemount() {
  systemDark = true;
  localStorage.setItem(cookies.NAMES.COLOR_MODE, 'light');
  const controls = () => React.createElement(React.Fragment, null,
    React.createElement(ModeState), React.createElement(ColorModeToggle), React.createElement(SettingsColorTheme),
  );
  const first = render(controls());
  await waitFor(() => expect(screen.getByTestId('mode').textContent).toBe('light'));
  fireEvent.click(screen.getByLabelText('Switch to dark theme'));
  await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(true));
  expect(localStorage.getItem(cookies.NAMES.COLOR_MODE)).toBe('dark');
  expect(cookies.get(cookies.NAMES.COLOR_THEME)).toBe('dark');
  fireEvent.click(first.container.querySelector('[data-value="midnight"]')!);
  await waitFor(() => expect(first.container.querySelector('[data-value="midnight"]')?.getAttribute('aria-current')).toBe('true'));
  expect(cookies.get(cookies.NAMES.COLOR_THEME)).toBe('midnight');
  first.unmount();
  document.documentElement.removeAttribute('style');
  systemDark = false;
  const restarted = render(controls());
  await waitFor(() => {
    expect(screen.getByTestId('mode').textContent).toBe('dark');
    expect(restarted.container.querySelector('[data-value="midnight"]')?.getAttribute('aria-current')).toBe('true');
    expect(document.documentElement.style.getPropertyValue('--chakra-colors-theme-bg-primary-_dark')).toBe(getThemeHexWithOverrides('midnight'));
  });
  fireEvent.click(screen.getByLabelText('Switch to light theme'));
  await waitFor(() => expect(restarted.container.querySelector('[data-value="light"]')?.getAttribute('aria-current')).toBe('true'));
  expect(localStorage.getItem(cookies.NAMES.COLOR_MODE)).toBe('light');
}

function integrationTokensResolveInBothModes() {
  const roles = [
    'indicator.transactions', 'indicator.addresses', 'indicator.locked', 'brand.uniswap',
    'securitySeverity.critical', 'securitySeverity.high', 'securitySeverity.medium',
    'securitySeverity.low', 'securitySeverity.informational', 'securitySeverity.gas',
    'rewards.badge.first', 'rewards.badge.second', 'rewards.badge.third',
    'rewards.pass.bg', 'rewards.pass.badge', 'rewards.pass.fg', 'rewards.pass.icon',
    'rewards.icon.shadow', 'rewards.banner.start', 'rewards.banner.stop', 'identicon.overlay',
    'description.fade', 'promo.start', 'promo.stop', 'editor.search.shadow', 'progress.stripe',
  ];
  for (const role of roles) {
    for (const mode of [ 'light', 'dark' ] as const) {
      expect(resolveColour(`colors.${ role }`, mode), `${ role } in ${ mode }`)
        .toMatch(/^(?:#[\da-f]{3,8}|rgba?\([\d\s.,%]+\)|transparent)$/i);
    }
  }
}

async function savedCookieWinsOverSystemPreference() {
  systemDark = true;
  cookies.set(cookies.NAMES.COLOR_MODE, 'light');
  render(React.createElement(ModeState));
  await waitFor(() => {
    expect(screen.getByTestId('mode').textContent).toBe('light');
    expect(document.documentElement.classList.contains('light')).toBe(true);
  });
}

async function systemPreferenceAppliesWithoutSavedMode() {
  systemDark = true;
  render(React.createElement(ModeState));
  await waitFor(() => {
    expect(screen.getByTestId('mode').textContent).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
}

function verticalLayoutsRenderNoSidebar() {
  expect(window.__envs.NEXT_PUBLIC_NAVIGATION_LAYOUT).toBe('vertical');
  for (const Shell of [ Layout, LayoutHome ]) {
    const view = render(React.createElement(Shell, null, React.createElement('p', null, 'Page content')));
    expect(view.container.querySelector('[data-label="desktop-header"]')).not.toBeNull();
    expect(view.container.querySelector('aside, [data-label="sidebar"], [data-label="navigation-vertical"]')).toBeNull();
    expect(view.getByText('Page content')).toBeTruthy();
    view.unmount();
  }
}

test(componentsUseSemanticColours.name, componentsUseSemanticColours);
test(homeRendersInBothColourModes.name, homeRendersInBothColourModes, 180000);
test(transactionRendersInBothColourModes.name, transactionRendersInBothColourModes, 180000);
test(addressRendersInBothColourModes.name, addressRendersInBothColourModes, 180000);
test(storedPreferenceToggleAndSettingsSurviveRemount.name, storedPreferenceToggleAndSettingsSurviveRemount);
test(systemPreferenceAppliesWithoutSavedMode.name, systemPreferenceAppliesWithoutSavedMode);
test(verticalLayoutsRenderNoSidebar.name, verticalLayoutsRenderNoSidebar);

test(integrationTokensResolveInBothModes.name, integrationTokensResolveInBothModes);
test(savedCookieWinsOverSystemPreference.name, savedCookieWinsOverSystemPreference);
