// @vitest-environment jsdom

import React from 'react';

import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TopBar from './TopBar';

describe('TopBar', () => {
  beforeEach(() => {
    routerState.pathname = '/';
    routerState.query = {};
  });

  it('keeps the utilities together inside the header', () => {
    routerState.pathname = '/blocks';

    const { container } = render(<TopBar/>);

    const bar = container.querySelector('[data-label="header-utilities"]');

    expect(bar).toBeTruthy();
    expect(Array.from(bar?.querySelectorAll('[data-label]') ?? []).map((element) => element.getAttribute('data-label')))
      .toEqual([]);
    expect(bar?.querySelector('button[aria-label="User settings"]')).toBeTruthy();
  });

  it('carries the settings, the theme toggle and the network controls', () => {
    routerState.pathname = '/blocks';

    render(<TopBar/>);

    expect(screen.getByLabelText('User settings')).toBeTruthy();
    expect(screen.getByLabelText(/^Switch to (dark|light) theme$/)).toBeTruthy();
  });

  it('keeps the same utilities available on the home page', () => {
    const { container } = render(<TopBar/>);

    expect(container.querySelector('[data-label="header-utilities"]')).toBeTruthy();
    expect(container.querySelector('[data-label="utility-bar-search"]')).toBeNull();
    expect(screen.getByLabelText('User settings')).toBeTruthy();
  });
});
