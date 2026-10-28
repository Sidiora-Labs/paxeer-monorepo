// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import React from 'react';
import type { Root } from 'react-dom/client';
import { createRoot } from 'react-dom/client';

import theme from 'toolkit/theme/theme';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';

import ScanExpander from './ScanExpander';

let container: HTMLDivElement;
let root: Root;

beforeAll(() => {
  Object.defineProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT', { configurable: true, value: true });
});

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async() => {
  await React.act(async() => root.unmount());
  container.remove();
});

describe('ScanExpander native control', () => {
  it('renders a focusable, non-submitting button with the collapsed state', async() => {
    await React.act(async() => {
      root.render(
        <ChakraProvider value={ theme }>
          <form>
            <ScanExpander><span>Gas usage and fee per gas</span></ScanExpander>
          </form>
        </ChakraProvider>,
      );
    });

    const toggle = container.querySelector('[data-toggle]') as HTMLButtonElement;
    expect(toggle).toBeInstanceOf(HTMLButtonElement);
    expect(toggle.type).toBe('button');
    expect(toggle.tabIndex).toBe(0);
    expect(toggle.textContent).toBe('+ Click to show more');
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(container.querySelector('[data-scan-expander]')?.getAttribute('data-open')).toBe('false');
    expect(container.querySelector('[data-content]')).toBeNull();

    toggle.focus();
    expect(document.activeElement).toBe(toggle);

    let submissions = 0;
    container.querySelector('form')?.addEventListener('submit', (event) => {
      event.preventDefault();
      submissions += 1;
    });
    await React.act(async() => toggle.click());

    expect(submissions).toBe(0);
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(toggle.textContent).toBe('- Click to show less');
    expect(container.querySelector('[data-content]')?.textContent).toBe('Gas usage and fee per gas');
    expect(document.activeElement).toBe(toggle);

    await React.act(async() => toggle.click());

    expect(submissions).toBe(0);
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(toggle.textContent).toBe('+ Click to show more');
    expect(container.querySelector('[data-content]')).toBeNull();
    expect(document.activeElement).toBe(toggle);
  });

  it('preserves defaultOpen, custom prompts and independent expanded states', async() => {
    await React.act(async() => {
      root.render(
        <ChakraProvider value={ theme }>
          <ScanExpander defaultOpen label="Advanced" showText="Expand details" hideText="Collapse details">
            <span>Already visible</span>
          </ScanExpander>
          <ScanExpander><span>Other details</span></ScanExpander>
        </ChakraProvider>,
      );
    });

    const expanders = container.querySelectorAll('[data-scan-expander]');
    const toggle = expanders[0].querySelector('[data-toggle]') as HTMLButtonElement;
    const otherToggle = expanders[1].querySelector('[data-toggle]') as HTMLButtonElement;

    expect(expanders[0].querySelector('[data-label]')?.textContent).toBe('Advanced:');
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(toggle.textContent).toBe('Collapse details');
    expect(expanders[0].querySelector('[data-content]')?.textContent).toBe('Already visible');
    expect(otherToggle.getAttribute('aria-expanded')).toBe('false');

    await React.act(async() => toggle.click());

    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(toggle.textContent).toBe('Expand details');
    expect(expanders[0].querySelector('[data-content]')).toBeNull();
    expect(otherToggle.getAttribute('aria-expanded')).toBe('false');
    expect(expanders[1].querySelector('[data-content]')).toBeNull();

    await React.act(async() => toggle.click());

    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(toggle.textContent).toBe('Collapse details');
    expect(expanders[0].querySelector('[data-content]')?.textContent).toBe('Already visible');
    expect(otherToggle.getAttribute('aria-expanded')).toBe('false');
  });
});
