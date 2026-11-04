// @vitest-environment jsdom
// @vitest-environment-options {"url":"https://localhost/explorer"}

import React from 'react';

import { render } from 'ui/shared/layout/testWrapper';
import { expect, test, beforeEach } from 'vitest';
import { fireEvent, screen, waitFor } from 'vitest/lib';

import * as cookies from 'lib/cookies';
import { COLOR_THEMES, getThemeHexWithOverrides } from 'lib/settings/colorTheme';

import ColorModeToggle from '../ColorModeToggle';
import Settings from './Settings';

// The committed layout fixture supplies jsdom's viewport/observer support. Both
// controls, the Chakra/next-themes provider, cookies and localStorage are real.
function renderControls() {
  return render(React.createElement(React.Fragment, null,
    React.createElement(ColorModeToggle),
    React.createElement(Settings),
  ));
}

function selectSwatch(id: string) {
  const swatch = document.querySelector(`[data-value="${ id }"]`);

  expect(swatch).toBeInstanceOf(HTMLElement);
  fireEvent.click(swatch as HTMLElement);
}

async function assertTheme(id: typeof COLOR_THEMES[number]['id']) {
  const theme = COLOR_THEMES.find((item) => item.id === id)!;
  const mode = theme.colorMode;
  const hex = getThemeHexWithOverrides(id);

  await waitFor(() => {
    expect(screen.getByText(theme.label)).toBeTruthy();
    expect(document.querySelector('[data-value][aria-current="true"]')?.getAttribute('data-value')).toBe(id);
    expect(document.querySelectorAll('[data-value][aria-current="true"]')).toHaveLength(2);
    // The other selected sample is the address identicon, not a second theme.
    expect(document.querySelectorAll('[data-value="light"][aria-current="true"], [data-value="dim"][aria-current="true"], ' +
      '[data-value="midnight"][aria-current="true"], [data-value="dark"][aria-current="true"]')).toHaveLength(1);
    expect(document.documentElement.classList.contains(mode)).toBe(true);
    expect(document.documentElement.style.getPropertyValue(mode === 'light' ? '--chakra-colors-white' : '--chakra-colors-black')).toBe(hex);
    expect(document.documentElement.style.getPropertyValue(`--chakra-colors-theme-bg-primary-_${ mode }`)).toBe(hex);
    expect(cookies.get(cookies.NAMES.COLOR_MODE)).toBe(mode);
    expect(cookies.get(cookies.NAMES.COLOR_THEME)).toBe(id);
    expect(localStorage.getItem(cookies.NAMES.COLOR_MODE)).toBe(mode);
  });
}

beforeEach(() => {
  localStorage.clear();
  cookies.remove(cookies.NAMES.COLOR_MODE);
  cookies.remove(cookies.NAMES.COLOR_THEME);
  document.documentElement.removeAttribute('style');
  document.documentElement.classList.remove('dark', 'light');
});

async function themeSettingsStaleTracksTopBarAndReload() {
  const controls = renderControls();

  fireEvent.click(screen.getByLabelText('User settings'));
  await assertTheme('light');

  selectSwatch('dim');
  await assertTheme('dim');

  // Selecting a theme closes the popover but leaves its settings consumer mounted.
  fireEvent.click(screen.getByLabelText('Switch to light theme'));
  fireEvent.click(screen.getByLabelText('User settings'));
  await assertTheme('light');

  fireEvent.click(screen.getByLabelText('User settings'));
  fireEvent.click(screen.getByLabelText('Switch to dark theme'));
  fireEvent.click(screen.getByLabelText('User settings'));
  await assertTheme('dark');

  // Restart the provider and settings, preserving only browser persistence.
  controls.unmount();
  document.documentElement.removeAttribute('style');
  document.documentElement.classList.remove('dark', 'light');
  const reopened = renderControls();

  fireEvent.click(screen.getByLabelText('User settings'));
  await assertTheme('dark');

  // A nondefault swatch in the same mode must also survive a provider restart.
  selectSwatch('midnight');
  await assertTheme('midnight');
  reopened.unmount();
  document.documentElement.removeAttribute('style');
  document.documentElement.classList.remove('dark', 'light');
  renderControls();
  fireEvent.click(screen.getByLabelText('User settings'));
  await assertTheme('midnight');
}

test(themeSettingsStaleTracksTopBarAndReload);
