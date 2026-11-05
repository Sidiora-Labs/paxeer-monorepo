// @vitest-environment jsdom

import React from 'react';

import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fireEvent } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import HeroBanner from './HeroBanner';

describe('HeroBanner', () => {
  beforeEach(() => {
    routerState.pathname = '/';
    routerState.query = {};
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('names the network above the search field', () => {
    const { container } = render(<HeroBanner/>);

    const title = container.querySelector('[data-label="hero-title"]') as HTMLElement;
    const search = container.querySelector('[data-label="hero-search"]') as HTMLElement;

    expect(title.textContent).toBe('Blockscout explorer');
    expect(title.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('carries the prompt search form in the band', () => {
    const { container } = render(<HeroBanner/>);

    const search = container.querySelector('[data-label="hero-search"]') as HTMLElement;

    expect(search.querySelector('form')).not.toBeNull();
    expect(search.querySelector('input')).not.toBeNull();
    expect(search.querySelector('input')?.getAttribute('enterKeyHint')).toBe('search');
  });

  it('offers the prompt and keyboard shortcut in the desktop search field', () => {
    const { container } = render(<HeroBanner/>);
    const search = container.querySelector('[data-label="hero-search-desktop"]') as HTMLElement;

    expect(search.querySelector('input')?.getAttribute('placeholder')).toBe('Search or ask: a transaction, address, block or token');
    expect(search.querySelector('kbd')?.textContent).toBe('Ctrl K');
    expect(search.querySelector('input')?.getAttribute('aria-label')).toBe('Search');
  });

  it('submits the search form from its keyboard-ready field', () => {
    const { container } = render(<HeroBanner/>);

    const form = container.querySelector('[data-label="hero-search-desktop"] form') as HTMLFormElement;
    const onSubmit = vi.fn((event: Event) => event.preventDefault());

    form.addEventListener('submit', onSubmit);
    fireEvent.submit(form);

    expect(onSubmit).toHaveBeenCalledTimes(1);
  });
});
