// @vitest-environment jsdom

import * as React from 'react';
import { createRoot } from 'react-dom/client';
import { renderToString } from 'react-dom/server';

import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { ThemeProvider, useTheme } from './theme-provider';

function ThemeValue() {
  const { theme, resolvedTheme, accent } = useTheme();
  return <output>{ `${ theme }:${ resolvedTheme }:${ accent }` }</output>;
}

function setSystemPreference(prefersDark: boolean) {
  vi.stubGlobal('matchMedia', (media: string): MediaQueryList => {
    const events = new EventTarget();
    return {
      media,
      matches: media === '(prefers-color-scheme: dark)' && prefersDark,
      onchange: null,
      addEventListener: events.addEventListener.bind(events),
      removeEventListener: events.removeEventListener.bind(events),
      dispatchEvent: events.dispatchEvent.bind(events),
      addListener(listener) {
        if (listener) events.addEventListener('change', listener as EventListener);
      },
      removeListener(listener) {
        if (listener)
          events.removeEventListener('change', listener as EventListener);
      },
    };
  });
}

beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
});

afterEach(() => {
  window.localStorage.clear();
  document.documentElement.className = '';
  vi.unstubAllGlobals();
});

test.each([
  ["light", "ink"],
  ["dark", "dx-gradient"],
  ["system", "dx-gradient"],
  ["invalid-theme", "invalid-accent"],
])(
  "initial browser-environment markup matches SSR with stored %s / %s",
  (theme, accent) => {
    window.localStorage.setItem("dx-theme", theme);
    window.localStorage.setItem("dx-accent", accent);

    expect(
      renderToString(
        <ThemeProvider>
          <ThemeValue />
        </ThemeProvider>,
      ),
    ).toBe("<output>system:light:ink</output>");
    expect(window.localStorage.getItem("dx-theme")).toBe(theme);
    expect(window.localStorage.getItem("dx-accent")).toBe(accent);
  },
);

async function mounted_provider_keeps_the_pre_paint_theme_classes() {
  setSystemPreference(false);
  window.localStorage.setItem('dx-theme', 'dark');
  window.localStorage.setItem('dx-accent', 'dx-gradient');
  document.documentElement.className = 'dark accent-dx-gradient';

  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await React.act(async() => {
      root.render(
        <ThemeProvider>
          <ThemeValue/>
        </ThemeProvider>,
      );
    });

    expect(document.documentElement.className).toBe('dark accent-dx-gradient');
    expect(container.querySelector('output')?.textContent).toBe(
      'dark:dark:dx-gradient',
    );
  } finally {
    await React.act(async() => root.unmount());
    container.remove();
  }
}

test(
  mounted_provider_keeps_the_pre_paint_theme_classes.name,
  mounted_provider_keeps_the_pre_paint_theme_classes,
);

async function mounted_provider_applies_validated_stored_preferences() {
  setSystemPreference(false);
  for (const [ theme, accent, prePaintClasses, expectedValue ] of [
    [ 'light', 'ink', 'dark', 'light:light:ink' ],
    [
      'invalid-theme',
      'invalid-accent',
      'dark accent-dx-gradient',
      'system:light:ink',
    ],
  ]) {
    window.localStorage.setItem('dx-theme', theme);
    window.localStorage.setItem('dx-accent', accent);
    document.documentElement.className = prePaintClasses;

    const container = document.createElement('div');
    document.body.append(container);
    const root = createRoot(container);
    try {
      await React.act(async() => {
        root.render(
          <ThemeProvider>
            <ThemeValue/>
          </ThemeProvider>,
        );
      });

      expect(container.querySelector('output')?.textContent).toBe(expectedValue);
      expect(document.documentElement.classList.contains('dark')).toBe(false);
      expect(
        document.documentElement.classList.contains('accent-dx-gradient'),
      ).toBe(false);
    } finally {
      await React.act(async() => root.unmount());
      container.remove();
    }
  }
}

test(
  mounted_provider_applies_validated_stored_preferences.name,
  mounted_provider_applies_validated_stored_preferences,
);
