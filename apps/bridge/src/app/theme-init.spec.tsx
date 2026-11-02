// @vitest-environment jsdom

import * as React from 'react';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { expect, test } from 'vitest';

import { ThemeProvider } from '../components/theme-provider';
import { THEME_INIT } from './theme-init';

async function theme_init_and_provider_agree_on_an_invalid_stored_theme() {
  const matchMediaDescriptor = Object.getOwnPropertyDescriptor(window, 'matchMedia');
  const actEnvironmentDescriptor = Object.getOwnPropertyDescriptor(
    globalThis,
    'IS_REACT_ACT_ENVIRONMENT',
  );
  const container = document.createElement('div');
  document.body.appendChild(container);
  let root: Root | undefined;

  Object.defineProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT', {
    configurable: true,
    value: true,
  });
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value(query: string): MediaQueryList {
      const events = new EventTarget();
      return {
        matches: query === '(prefers-color-scheme: dark)',
        media: query,
        onchange: null,
        addListener(listener) {
          events.addEventListener('change', listener as EventListener | null);
        },
        removeListener(listener) {
          events.removeEventListener('change', listener as EventListener | null);
        },
        addEventListener: events.addEventListener.bind(events),
        removeEventListener: events.removeEventListener.bind(events),
        dispatchEvent: events.dispatchEvent.bind(events),
      };
    },
  });

  try {
    window.localStorage.setItem('dx-theme', 'invalid-theme');
    document.documentElement.classList.remove('dark');

    window.eval(THEME_INIT);
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    await act(async() => {
      const mountedRoot = createRoot(container);
      root = mountedRoot;
      mountedRoot.render(
        <ThemeProvider>
          <span>Bridge</span>
        </ThemeProvider>,
      );
    });
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  } finally {
    await act(async() => {
      root?.unmount();
    });
    container.remove();
    window.localStorage.clear();
    document.documentElement.className = '';
    if (matchMediaDescriptor) {
      Object.defineProperty(window, 'matchMedia', matchMediaDescriptor);
    } else {
      Reflect.deleteProperty(window, 'matchMedia');
    }
    if (actEnvironmentDescriptor) {
      Object.defineProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT', actEnvironmentDescriptor);
    } else {
      Reflect.deleteProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT');
    }
  }
}

test(theme_init_and_provider_agree_on_an_invalid_stored_theme);
