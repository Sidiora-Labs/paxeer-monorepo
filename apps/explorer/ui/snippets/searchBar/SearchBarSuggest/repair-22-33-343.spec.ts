// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, waitFor } from '@testing-library/react';
import 'css.escape';
import React from 'react';

import type { QuickSearchResultItem } from 'types/client/search';

import useApiQuery, { getResourceKey } from 'lib/api/useApiQuery';
import * as searchFixtures from 'mocks/search/index';
import { render } from 'ui/shared/layout/testWrapper';
import type { Category } from 'ui/shared/search/utils';
import { searchCategories } from 'ui/shared/search/utils';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import SearchBarSuggest from './SearchBarSuggest';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_ENABLED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
    NEXT_PUBLIC_HAS_USER_OPS: 'false',
  };
});

const QUERY_KEY = getResourceKey('general:quick_search');
const RESULTS = [ searchFixtures.address1, searchFixtures.tx1, searchFixtures.block1, searchFixtures.userOp1 ];
const clients: Array<QueryClient> = [];
const errors: Array<string> = [];

function recordError(event: ErrorEvent) {
  errors.push(event.message);
}

beforeEach(() => {
  errors.length = 0;
  window.addEventListener('error', recordError);
});

afterEach(() => {
  window.removeEventListener('error', recordError);
  clients.splice(0).forEach(client => client.clear());
  expect(errors).toEqual([]);
});

function Harness({ category }: { category?: Category }) {
  const query = useApiQuery('general:quick_search', { queryOptions: { enabled: false } });
  const zetaChainCCTXQuery = useApiQuery('zetachain:transactions', { queryOptions: { enabled: false } });

  return React.createElement(SearchBarSuggest, {
    query,
    zetaChainCCTXQuery,
    category,
    externalSearchItem: null,
    searchTerm: '0xb64a',
    onItemClick: event => event.preventDefault(),
  });
}

function renderSearch(items: Array<QuickSearchResultItem>, category?: Category) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } } });
  clients.push(client);
  client.setQueryData(QUERY_KEY, items);
  client.setQueryData([ 'marketplace-dapps' ], []);
  const element = (selected?: Category) => React.createElement(
    QueryClientProvider,
    { client },
    React.createElement(Harness, { category: selected }),
  );
  const view = render(element(category));

  return {
    ...view,
    setCategory: (selected?: Category) => view.rerender(element(selected)),
    setItems: async(nextItems: Array<QuickSearchResultItem>) => {
      await act(async() => {
        client.setQueryData(QUERY_KEY, nextItems);
      });
    },
  };
}

function shownCategories(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-scroll-target] > [data-id]')).map(el => el.getAttribute('data-id'));
}

function scrollContainer(container: HTMLElement): HTMLElement {
  const element = container.querySelector('[data-scroll-target]')?.parentElement;
  if (!element) {
    throw new Error('Expected mounted search result sections');
  }
  return element;
}

function tab(container: HTMLElement, category: Category): HTMLButtonElement {
  const element = container.querySelector<HTMLButtonElement>(`[role="tab"][data-value="${ category }"]`);
  if (!element) {
    throw new Error(`Expected tab for ${ category }`);
  }
  return element;
}

async function scroll(element: HTMLElement, top = 0) {
  await act(async() => {
    element.scrollTop = top;
    fireEvent.scroll(element);
    await new Promise(resolve => setTimeout(resolve, 350));
  });
  expect(errors).toEqual([]);
}

// Committed viewport coordinates supply the browser layout that jsdom does not perform.
// Scrolling changes the real element's scrollTop; the production handler measures these DOM elements.
function attachViewport(container: HTMLElement) {
  const viewport = scrollContainer(container);
  const requests: Array<ScrollToOptions> = [];
  const positions: Partial<Record<Category, number>> = { token: 20, address: 200, transaction: 400, block: 600 };
  viewport.getBoundingClientRect = () => new DOMRect(0, 100, 800, 400);
  container.querySelectorAll<HTMLElement>('[data-scroll-target]').forEach(section => {
    const heading = section.querySelector<HTMLParagraphElement>('[data-id]');
    const id = heading?.getAttribute('data-id') as Category;
    const position = positions[id];
    if (!heading || position === undefined) {
      throw new Error(`Missing viewport coordinates for ${ id }`);
    }
    const rectangle = () => new DOMRect(0, 100 + position - viewport.scrollTop, 800, 20);
    section.getBoundingClientRect = rectangle;
    heading.getBoundingClientRect = rectangle;
  });
  Object.defineProperty(viewport, 'scrollTo', {
    configurable: true,
    value: (options: ScrollToOptions) => {
      requests.push(options);
      viewport.scrollTop = options.top ?? viewport.scrollTop;
      fireEvent.scroll(viewport);
    },
  });
  return { viewport, requests };
}

async function searchCategoryRefCrashHiddenKinds() {
  expect(searchCategories.some(cat => cat.id === 'user_operation')).toBe(false);
  const view = renderSearch(RESULTS);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'address', 'transaction', 'block' ]));
  const { viewport } = attachViewport(view.container);

  await scroll(viewport, 350);
  expect(tab(view.container, 'transaction').getAttribute('aria-selected')).toBe('true');
  await scroll(viewport, 550);
  expect(tab(view.container, 'block').getAttribute('aria-selected')).toBe('true');
  await scroll(viewport, 150);
  expect(tab(view.container, 'address').getAttribute('aria-selected')).toBe('true');
}

async function searchCategoryRefCrashCategoryChanges() {
  const view = renderSearch(RESULTS);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'address', 'transaction', 'block' ]));

  view.setCategory('block');
  expect(shownCategories(view.container)).toEqual([ 'block' ]);
  expect(view.container.querySelector('[role="tablist"]')).toBeNull();
  await scroll(scrollContainer(view.container));

  view.setCategory('address');
  expect(shownCategories(view.container)).toEqual([ 'address' ]);
  await scroll(scrollContainer(view.container));

  view.setCategory();
  await view.setItems([ searchFixtures.token2, searchFixtures.block1 ]);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'token', 'block' ]));
  const { viewport } = attachViewport(view.container);
  await scroll(viewport, 550);
  expect(tab(view.container, 'block').getAttribute('aria-selected')).toBe('true');
  await scroll(viewport, 0);
  expect(tab(view.container, 'token').getAttribute('aria-selected')).toBe('true');
}

async function searchCategoryRefCrashEmptyResults() {
  const view = renderSearch([]);
  await waitFor(() => expect(view.container.textContent).toContain('No results found.'));
  expect(shownCategories(view.container)).toEqual([]);

  await view.setItems(RESULTS);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'address', 'transaction', 'block' ]));
  const previousViewport = scrollContainer(view.container);
  fireEvent.scroll(previousViewport);
  await view.setItems([]);
  await waitFor(() => expect(view.container.textContent).toContain('No results found.'));
  expect(previousViewport.isConnected).toBe(false);
  await scroll(previousViewport);
  expect(shownCategories(view.container)).toEqual([]);

  await view.setItems([ searchFixtures.token2, searchFixtures.tx1 ]);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'token', 'transaction' ]));
  const { viewport } = attachViewport(view.container);
  await scroll(viewport, 350);
  expect(tab(view.container, 'transaction').getAttribute('aria-selected')).toBe('true');

  fireEvent.scroll(viewport);
  view.unmount();
  await scroll(viewport);
  expect(view.container.childElementCount).toBe(0);
}

async function searchCategoryRefCrashTabsAndKeyboard() {
  expect(CSS.escape(':tabs:')).toBe('\\:tabs\\:');
  const view = renderSearch([ searchFixtures.token2, searchFixtures.address1, searchFixtures.tx1, searchFixtures.block1 ]);
  await waitFor(() => expect(shownCategories(view.container)).toEqual([ 'token', 'address', 'transaction', 'block' ]));
  const { viewport, requests } = attachViewport(view.container);

  fireEvent.click(tab(view.container, 'transaction'));
  await waitFor(() => expect(tab(view.container, 'transaction').getAttribute('aria-selected')).toBe('true'));
  expect(requests).toEqual([ { top: 400, behavior: 'smooth' } ]);

  await act(async() => tab(view.container, 'transaction').focus());
  fireEvent.keyDown(view.getByRole('tablist'), { key: 'ArrowRight', code: 'ArrowRight' });
  await waitFor(() => expect(document.activeElement).toBe(tab(view.container, 'block')));
  expect(await view.findByRole('tab', { name: 'Blocks', selected: true })).toBe(tab(view.container, 'block'));
  expect(requests[requests.length - 1]).toEqual({ top: 600, behavior: 'smooth' });

  fireEvent.keyDown(view.getByRole('tablist'), { key: 'ArrowLeft', code: 'ArrowLeft' });
  await waitFor(() => expect(document.activeElement).toBe(tab(view.container, 'transaction')));
  expect(await view.findByRole('tab', { name: 'Transactions', selected: true })).toBe(tab(view.container, 'transaction'));
  expect(requests[requests.length - 1]).toEqual({ top: 400, behavior: 'smooth' });

  await scroll(viewport, 0);
  expect(tab(view.container, 'token').getAttribute('aria-selected')).toBe('true');
}

test('searchCategoryRefCrashHiddenKinds', async() => {
  expect.hasAssertions();
  await searchCategoryRefCrashHiddenKinds();
});
test('searchCategoryRefCrashCategoryChanges', async() => {
  expect.hasAssertions();
  await searchCategoryRefCrashCategoryChanges();
});
test('searchCategoryRefCrashEmptyResults', async() => {
  expect.hasAssertions();
  await searchCategoryRefCrashEmptyResults();
});
test('searchCategoryRefCrashTabsAndKeyboard', async() => {
  expect.hasAssertions();
  await searchCategoryRefCrashTabsAndKeyboard();
});
