// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { IndexingStatus } from 'types/api/indexingStatus';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { resolveNavigationLayout } from 'configs/app/ui';
import { getResourceKey } from 'lib/api/useApiQuery';
import useNavItems, { getNavItemDescription, isGroupItem, isInternalItem } from 'lib/hooks/useNavItems';
import { FOOTER_LINKS } from 'mocks/config/footerLinks';
import { base as stats, indexingStatus } from 'mocks/stats';
import SideBar from 'ui/shared/layout/components/SideBar';
import Layout from 'ui/shared/layout/Layout';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import Footer from 'ui/snippets/footer/Footer';
import NavigationMobile from 'ui/snippets/navigation/mobile/NavigationMobile';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'vertical',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
    NEXT_PUBLIC_FOOTER_LINKS: 'https://localhost:3000/footer-links.json',
    NEXT_PUBLIC_HIDE_INDEXING_ALERT_INT_TXS: 'false',
  };
});

import HeaderDesktop from './HeaderDesktop';

const clients: Array<QueryClient> = [];
const routers: Array<Router> = [];
let initialData: typeof window.__NEXT_DATA__;
let middlewareMatchers: PropertyDescriptor | undefined;

beforeEach(() => {
  initialData = window.__NEXT_DATA__;
  middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  window.__NEXT_DATA__ = { props: {}, page: '/blocks', query: {}, buildId: 'explorer-shell' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });
  routerState.pathname = '/blocks';
  routerState.query = {};
});

afterEach(() => {
  clients.forEach((client) => client.clear());
  clients.length = 0;
  routers.forEach((router) => window.removeEventListener('popstate', router.onPopState));
  routers.length = 0;
  window.__NEXT_DATA__ = initialData;
  if (middlewareMatchers) {
    Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
  } else {
    Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
  }
});

function NavItemsProbe() {
  const { shellNavItems } = useNavItems();
  return <output data-testid="nav-items">{ JSON.stringify(shellNavItems) }</output>;
}

function renderShell(element: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(client);
  client.setQueryData(getResourceKey('general:stats'), stats);
  client.setQueryData<IndexingStatus>(getResourceKey('general:homepage_indexing_status'), {
    ...indexingStatus,
    finished_indexing: false,
    indexed_internal_transactions_ratio: '0.1',
  });
  client.setQueryData([ 'footer-links' ], FOOTER_LINKS);
  const router = new Router(routerState.pathname, routerState.query, routerState.pathname, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('explorer-shell', ''),
    App,
    Component: HeaderDesktop,
    wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => <WrappedApp { ...props } router={ router }/>,
    subscription: async(data, CurrentApp) => {
      renderToStaticMarkup(<CurrentApp { ...data.props } Component={ data.Component } router={ router }/>);
    },
    isFallback: false,
  });
  routers.push(router);
  return render(
    <QueryClientProvider client={ client }>
      <RouterContext.Provider value={ router }>{ element }</RouterContext.Provider>
    </QueryClientProvider>,
  );
}

function horizontalHeaderGroupsAndSearch() {
  expect(resolveNavigationLayout('horizontal')).toBe('horizontal');
  const { container } = renderShell(<HeaderDesktop/>);
  const header = container.querySelector('header') as HTMLElement;
  const navigation = within(header).getByLabelText('Main navigation');
  expect(Array.from(navigation.querySelectorAll('[data-label="nav-group"]')).map((group) => group.textContent?.trim()))
    .toEqual([ 'Explore', 'Network', 'Developers' ]);
  expect(within(header).getByPlaceholderText('Search or ask').tagName).toBe('INPUT');
  expect(header.querySelector('kbd')?.textContent).toMatch(/^(⌘K|Ctrl K)$/);
  const networkName = config.chain.name;
  if (!networkName) {
    throw new Error('The configured network must have a name');
  }
  expect(within(header).getByText(networkName)).toBeTruthy();
  expect(within(header).getByLabelText('Link to main page').getAttribute('href')).toBe('/');
  expect(within(header).getByLabelText(/^Switch to (light|dark) theme$/)).toBeTruthy();
  expect(within(header).getByLabelText('User settings')).toBeTruthy();
}

test(horizontalHeaderGroupsAndSearch, horizontalHeaderGroupsAndSearch);

function verticalSettingUsesTheSameHeader() {
  expect(window.__envs.NEXT_PUBLIC_NAVIGATION_LAYOUT).toBe('vertical');
  expect(config.UI.navigation.layout).toBe('horizontal');
  expect(resolveNavigationLayout('vertical')).toBe(resolveNavigationLayout('horizontal'));
  const { container } = renderShell(<Layout><div>Explorer page</div></Layout>);
  expect(container.querySelectorAll('[data-label="desktop-header"]')).toHaveLength(1);
  expect(container.querySelector('[data-label="utility-bar"]')).toBeNull();
  expect(container.querySelector('[data-label="content-search"]')).toBeNull();
  expect(container.querySelector('[data-label="desktop-header"] input[placeholder="Search or ask"]')).toBeTruthy();
  expect(within(container).getByRole('main').textContent).toBe('Explorer page');
  expect(SideBar()).toBeNull();
}

test(verticalSettingUsesTheSameHeader, verticalSettingUsesTheSameHeader);

async function groupPanelsUseTheHookLinks() {
  const { container } = renderShell(<><HeaderDesktop/><NavItemsProbe/></>);
  const producer = JSON.parse(screen.getByTestId('nav-items').textContent || '[]') as ReturnType<typeof useNavItems>['shellNavItems'];
  expect(producer.map((item) => item.text)).toEqual([ 'Explore', 'Network', 'Developers' ]);
  const header = container.querySelector('header') as HTMLElement;

  for (const item of producer) {
    expect(isGroupItem(item)).toBe(true);
    if (!isGroupItem(item)) {
      throw new Error('The shell requires navigation groups');
    }
    const trigger = within(header).getByLabelText(item.text);
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    fireEvent.click(trigger);
    const panel = await screen.findByRole('dialog', { name: `${ item.text } navigation` });
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    const links = within(panel).getAllByRole('link');
    expect(links).toHaveLength(item.subItems.flat().length);
    item.subItems.flat().forEach((link, index) => {
      expect(links[index].getAttribute('href')).toBe(isInternalItem(link) ? route(link.nextRoute) : link.url);
      expect(within(links[index]).getByText(link.text)).toBeTruthy();
      expect(within(links[index]).getByText(getNavItemDescription(link))).toBeTruthy();
    });
    fireEvent.click(trigger);
    await waitFor(() => expect(trigger.getAttribute('aria-expanded')).toBe('false'));
  }

  const explore = producer.find((item) => item.text === 'Explore');
  expect(explore && isGroupItem(explore) && explore.subItems.flat().slice(0, 5).map((item) => item.text))
    .toEqual([ 'Blocks', 'Transactions', 'Tokens', 'Accounts', 'Verified contracts' ]);
  const network = producer.find((item) => item.text === 'Network');
  expect(network && isGroupItem(network) && network.subItems.flat().some((item) => item.text === 'Validators'))
    .toBe(config.features.validators.isEnabled);
  expect(network && isGroupItem(network) && network.subItems.flat().some((item) => item.text === 'Epochs'))
    .toBe(config.features.celo.isEnabled);
}

test(groupPanelsUseTheHookLinks, groupPanelsUseTheHookLinks);

function searchShortcutFocusesTheRealInput() {
  const { container } = renderShell(<HeaderDesktop/>);
  const input = within(container).getByPlaceholderText('Search or ask');
  fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
  expect(document.activeElement).toBe(input);
  input.blur();
  fireEvent.keyDown(window, { key: 'k', metaKey: true });
  expect(document.activeElement).toBe(input);
}

test(searchShortcutFocusesTheRealInput, searchShortcutFocusesTheRealInput);

function mobileGroupsAreAnAccordion() {
  renderShell(<NavigationMobile/>);
  const navigation = screen.getByRole('navigation', { name: 'Mobile navigation' });
  const explore = within(navigation).getByRole('button', { name: 'Explore' });
  const developers = within(navigation).getByRole('button', { name: 'Developers' });
  expect(explore.getAttribute('aria-expanded')).toBe('false');
  fireEvent.click(explore);
  expect(explore.getAttribute('aria-expanded')).toBe('true');
  expect(within(navigation).getByRole('link', { name: /Browse the latest blocks/ }).getAttribute('href')).toBe('/blocks');
  fireEvent.click(developers);
  expect(explore.getAttribute('aria-expanded')).toBe('false');
  expect(developers.getAttribute('aria-expanded')).toBe('true');
  expect(within(navigation).getByRole('link', { name: /Publish and verify contract source/ }).getAttribute('href'))
    .toBe('/contract-verification');
}

test(mobileGroupsAreAnAccordion, mobileGroupsAreAnAccordion);

async function footerKeepsEveryLinkInThreeColumns() {
  const { container } = renderShell(<Footer/>);
  const columns = container.querySelectorAll('[data-label="footer-column"]');
  expect(columns).toHaveLength(3);
  expect(Array.from(columns).map((column) => column.querySelector('h2')?.textContent)).toEqual([ 'Explore', 'Network', 'Developers' ]);
  for (const group of FOOTER_LINKS) {
    const heading = within(container).getByRole('heading', { name: group.title });
    const section = heading.parentElement as HTMLElement;
    expect(within(section).getAllByRole('link')).toHaveLength(group.links.length);
    group.links.forEach((link) => expect(within(section).getByText(link.text).closest('a')?.getAttribute('href')).toBe(link.url));
  }
  expect(within(columns[0] as HTMLElement).getByRole('link', { name: 'Blocks' }).getAttribute('href')).toBe('/blocks');
  expect(within(columns[2] as HTMLElement).getByRole('link', { name: 'Verify contract' }).getAttribute('href')).toBe('/contract-verification');
  expect(await within(container).findByText('10%')).toBeTruthy();
}

test(footerKeepsEveryLinkInThreeColumns, footerKeepsEveryLinkInThreeColumns);
