// @vitest-environment jsdom

import React from 'react';

import LayoutDefault from 'ui/shared/layout/Layout';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import NavBar from './NavBar';

describe('NavBar', () => {
  beforeEach(() => {
    routerState.pathname = '/blocks';
    routerState.query = {};
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('includes the brand and groups in the sticky header', () => {
    const { container } = render(<NavBar/>);

    const header = container.querySelector('[data-label="desktop-header"]') as HTMLElement;

    expect(header).toBeTruthy();
    expect(header.querySelector('[data-label="header-brand"]')).toBeTruthy();
  });

  it('includes utility controls in the single sticky header', () => {
    const { container } = render(<LayoutDefault><div>page body</div></LayoutDefault>);

    const header = container.querySelector('[data-label="desktop-header"]') as HTMLElement;

    expect(header).toBeTruthy();
    expect(header.querySelector('[data-label="header-brand"]')).toBeTruthy();
    expect(header.querySelector('[data-label="utility-bar"]')).toBeNull();
    expect(header.querySelector('[data-label="header-utilities"]')).toBeTruthy();
  });
});
