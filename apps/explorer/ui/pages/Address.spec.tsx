// @vitest-environment jsdom

import React from 'react';

import * as addressMock from 'mocks/address/address';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The address lists render fifty placeholder rows of real table items, which jsdom lays out well past the
// default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import AddressPageContent from './Address';
import { addressDetailFixtures } from './repair-22-33-314.fixtures';

const HASH = addressMock.hash;

const tabTitles = (container: HTMLElement) =>
  Array.from(container.querySelectorAll('[role="tab"]')).map((item) => item.textContent ?? '');

// A responsive style prop reaches the document as one rule per breakpoint, each of them inserted
// into a style element of its own, and jsdom performs no layout, so the widths a declaration belongs
// to are read from the rules that name the element's own class.
const ruleTexts = () => {
  const inline = Array.from(document.querySelectorAll('style')).map((node) => node.textContent ?? '');
  const parsed = Array.from(document.styleSheets).flatMap((sheet) => {
    try {
      return Array.from(sheet.cssRules).map((rule) => rule.cssText);
    } catch {
      return [];
    }
  });

  return [ ...inline, ...parsed ].filter(Boolean);
};

const declarationsOf = (element: Element, property: string) => ruleTexts()
  .filter((text) => Array.from(element.classList).some((name) => new RegExp(`\\.${ name }(?![\\w-])`).test(text)))
  .map((text) => ({ text, value: new RegExp(`(?:^|[;{\\s])${ property }\\s*:\\s*([^;}]+)`).exec(text)?.[1]?.trim() }))
  .filter((rule): rule is { text: string; value: string } => rule.value !== undefined);

// Each rule is inserted on its own, so a declaration belongs to the wide layout when the rule that
// carries it is a breakpoint rule and not the base rule, which declares its own minimum width.
const isWide = (text: string) => /^\s*@media[^{]*\(min-width/.test(text);

const LEFT_INSET = 'margin-(?:left|inline-start)';

// The scale resolves a zero inset either to the plain length or to the token of the same name.
const isSpace = (value: string) => !/^0[a-z%]*$/.test(value) && !/spacing-0\b/.test(value);

async function renderLoadedAddress() {
  const result = render(<AddressPageContent/>);
  await waitFor(() => {
    expect(result.container.querySelector(`a[href="/tx/${ addressMock.contract.creation_transaction_hash }"]`)).not.toBeNull();
    expect(result.container.querySelector('[data-address-card="More info"]')?.textContent).toContain('5,462');
  });
  return result;
}

const unexpectedRequests: Array<string> = [];

describe('AddressPageContent', () => {
  beforeEach(() => {
    routerState.pathname = '/address/[hash]';
    routerState.query = { hash: HASH };
    fetchMock.resetMocks();
    unexpectedRequests.length = 0;
    fetchMock.mockResponse((request) => {
      const path = new URL(request.url).pathname;
      if (!addressDetailFixtures.has(path)) {
        unexpectedRequests.push(path);
        return Promise.reject(new Error(`Unexpected address request: ${ path }`));
      }
      return Promise.resolve({ body: JSON.stringify(addressDetailFixtures.get(path)), headers: { 'Content-Type': 'application/json' } });
    });
  });

  afterEach(() => {
    expect(unexpectedRequests).toEqual([]);
  });

  it('heads the page with the address label rather than a details heading', async() => {
    const { container } = await renderLoadedAddress();

    const heading = container.querySelector('h1')?.textContent ?? '';

    expect([ 'Address', 'Contract' ]).toContain(heading);
    expect(heading.endsWith('details')).toBe(false);
  });

  it('puts the icon action row beneath the title', async() => {
    const { container } = await renderLoadedAddress();

    expect(container.querySelector('[data-address-actions]')).toBeTruthy();
  });

  it('renders the three detail cards above the tab strip', async() => {
    const { container } = await renderLoadedAddress();

    const details = container.querySelector('[data-address-details]') as Element;
    const tabList = container.querySelector('[role="tablist"]') as Element;

    expect(details).toBeTruthy();
    expect(tabList).toBeTruthy();
    expect(details.querySelectorAll('[data-address-card]')).toHaveLength(3);
    expect(details.compareDocumentPosition(tabList) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('opens the tab strip on the transaction lists and keeps no separate details tab', async() => {
    const { container } = await renderLoadedAddress();

    const titles = tabTitles(container);

    expect(titles[0].startsWith('Transactions')).toBe(true);
    expect(titles[1].startsWith('Internal transactions')).toBe(true);
    expect(titles[2].startsWith('Token transfers')).toBe(true);
    expect(titles[3].startsWith('Tokens')).toBe(true);
    expect(titles.some((title) => title.startsWith('Details'))).toBe(false);
  });

  it('keeps the coin balance history and the advanced filter on the tab row', async() => {
    const { container } = await renderLoadedAddress();

    expect(tabTitles(container).some((title) => title.startsWith('Coin balance history'))).toBe(true);
    expect(container.querySelector('a[href^="/advanced-filter"]')).toBeTruthy();
  });

  it('lets the identifier share the title row and keeps its inset for the wide layout only', async() => {
    const { container } = await renderLoadedAddress();

    const row = container.querySelector('[data-title-after]')?.firstElementChild as HTMLElement;

    expect(row.querySelector('[data-entity-kind="address"]')).toBeTruthy();

    const insets = declarationsOf(row, LEFT_INSET);
    const spaced = insets.filter((rule) => isSpace(rule.value));

    expect(insets.length).toBeGreaterThan(0);
    expect(spaced.length).toBeGreaterThan(0);
    expect(spaced.every((rule) => isWide(rule.text))).toBe(true);
  });
});
