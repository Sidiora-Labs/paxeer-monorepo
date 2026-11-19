// @vitest-environment jsdom

import React from 'react';

import * as searchMock from 'mocks/search/index';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fireEvent } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
    NEXT_PUBLIC_ASSISTANT_ENABLED: 'true',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import SearchBarDesktop from './SearchBarDesktop';

const TERM = '0xb64a';
const RESULTS = [ searchMock.address1, searchMock.tx1, searchMock.block1 ];

const shownCategories = (): Array<string | null> => Array.from(document.body.querySelectorAll('[data-scroll-target] [data-id]'))
  .map((item) => item.getAttribute('data-id'));

function typeTerm(container: HTMLElement): void {
  const input = container.querySelector('input') as HTMLInputElement;

  fireEvent.focus(input);
  fireEvent.change(input, { target: { value: TERM } });
}

vi.setConfig({ testTimeout: 60_000 });

describe('SearchBarDesktop', () => {
  beforeEach(() => {
    routerState.pathname = '/';
    routerState.query = {};
    window.localStorage.clear();
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      const body = request.url.includes('/api/v2/search/quick') ? RESULTS : {};

      return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('suggests every category the results fall in when no category is chosen', async() => {
    const { container } = render(<SearchBarDesktop isHeroBanner/>);

    typeTerm(container);

    await vi.waitFor(() => {
      expect(shownCategories()).toEqual([ 'address', 'transaction', 'block' ]);
    }, { timeout: 30_000, interval: 100 });
  });

  it('passes the chosen category to the suggestions so only that category is listed', async() => {
    const { container } = render(<SearchBarDesktop isHeroBanner category="transaction"/>);

    typeTerm(container);

    await vi.waitFor(() => {
      expect(shownCategories()).toEqual([ 'transaction' ]);
    }, { timeout: 30_000, interval: 100 });
  });

  it('widens the assistant panel while preserving the ordinary panel and search field widths', async function assistantPanelUsesMoreSpace() {
    for (const variant of [ {}, { isHeroBanner: true }, { isHeroBanner: true, compact: true } ]) {
      const { container, unmount } = render(<SearchBarDesktop { ...variant }/>);
      typeTerm(container);
      await vi.waitFor(() => expect(shownCategories()).toEqual([ 'address', 'transaction', 'block' ]));
      const panel = document.querySelector<HTMLElement>('[aria-label="Search suggestions"]')!;
      const form = container.querySelector('form')!;
      const originalWidth = window.getComputedStyle(form).width;
      const ordinaryWidth = variant.isHeroBanner && !variant.compact ? 'calc(var(--chakra-spacing-80) * 2)' : 'var(--chakra-spacing-80)';

      expect(window.getComputedStyle(panel).width).toBe(ordinaryWidth);
      expect(window.getComputedStyle(panel).minWidth).toBe('');

      fireEvent.change(container.querySelector('input')!, { target: { value: 'What does the kernel do?' } });
      await vi.waitFor(() => expect(panel.textContent).toContain('Ask: What does the kernel do?'));

      expect(window.getComputedStyle(panel).minWidth)
        .toBe('min(calc(var(--chakra-spacing-80) * 2), calc(100dvw - var(--chakra-spacing-8)))');
      expect(window.getComputedStyle(panel).maxWidth).toBe('calc(100dvw - var(--chakra-spacing-8))');
      expect(window.getComputedStyle(form).width).toBe(originalWidth);

      typeTerm(container);
      await vi.waitFor(() => expect(panel.textContent).not.toContain('Ask:'));
      expect(window.getComputedStyle(panel).minWidth).toBe('');
      expect(window.getComputedStyle(panel).width).toBe(ordinaryWidth);
      expect(window.getComputedStyle(form).width).toBe(originalWidth);
      unmount();
    }
  });
});
