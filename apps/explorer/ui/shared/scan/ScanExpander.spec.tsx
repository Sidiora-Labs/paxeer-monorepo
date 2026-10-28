// @vitest-environment jsdom

import React from 'react';

import { Provider } from 'toolkit/chakra/provider';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from 'vitest/lib';

import ScanExpander from './ScanExpander';

// jsdom implements no CSS media queries, and the Chakra provider reads window.matchMedia on mount
const createMediaQueryList = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => undefined,
  removeListener: () => undefined,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
  dispatchEvent: () => false,
});

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', { writable: true, value: createMediaQueryList });
});

// the suite runs without vitest globals, so the testing library cannot register its own teardown
afterEach(cleanup);

describe('ScanExpander', () => {
  it('starts closed behind the More Details prompt', () => {
    const { container } = render(
      <Provider>
        <ScanExpander>
          <span>the rest of the transaction</span>
        </ScanExpander>
      </Provider>,
    );

    expect(container.querySelector('[data-label]')?.textContent).toBe('More Details:');
    expect(container.querySelector('[data-toggle]')?.textContent).toBe('+ Click to show more');
    expect(container.querySelector('[data-content]')).toBeNull();
    expect(container.querySelector('[data-scan-expander]')?.getAttribute('data-open')).toBe('false');

    const toggle = screen.getByRole('button', { name: '+ Click to show more' });
    expect(toggle.tagName).toBe('BUTTON');
    expect(toggle.getAttribute('type')).toBe('button');
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(toggle.tabIndex).toBe(0);
    toggle.focus();
    expect(document.activeElement).toBe(toggle);
  });

  it('reveals the children and swaps the prompt when it is clicked', () => {
    const { container } = render(
      <Provider>
        <ScanExpander>
          <span>the rest of the transaction</span>
        </ScanExpander>
      </Provider>,
    );

    fireEvent.click(container.querySelector('[data-toggle]') as Element);

    expect(container.querySelector('[data-content]')?.textContent).toBe('the rest of the transaction');
    expect(container.querySelector('[data-toggle]')?.textContent).toBe('- Click to show less');
    expect(container.querySelector('[data-scan-expander]')?.getAttribute('data-open')).toBe('true');
    expect(screen.getByRole('button', { name: '- Click to show less' }).getAttribute('aria-expanded')).toBe('true');

    fireEvent.click(screen.getByRole('button', { name: '- Click to show less' }));

    expect(container.querySelector('[data-content]')).toBeNull();
    expect(container.querySelector('[data-scan-expander]')?.getAttribute('data-open')).toBe('false');
    expect(screen.getByRole('button', { name: '+ Click to show more' }).getAttribute('aria-expanded')).toBe('false');
  });

  it('opens on mount when the caller asks for it', () => {
    const { container } = render(
      <Provider>
        <ScanExpander defaultOpen label="Advanced">
          <span>already visible</span>
        </ScanExpander>
      </Provider>,
    );

    expect(container.querySelector('[data-label]')?.textContent).toBe('Advanced:');
    expect(screen.getByText('already visible')).toBeDefined();
    expect(screen.getByRole('button', { name: '- Click to show less' }).getAttribute('aria-expanded')).toBe('true');
  });
});
