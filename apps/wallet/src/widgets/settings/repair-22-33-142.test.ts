// @vitest-environment jsdom
import { createElement } from 'react';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { mount, type Mounted } from '@/account/test/render';
import { getMessages } from '@/lib/locale/messages';
import { currencyRatesRepository, preferencesRepository } from '@/platform/storage/repositories';
import { LocaleProvider } from '@/providers/LocaleProvider';
import { PWAProvider } from '@/providers/PWAProvider';
import { ThemeProvider } from '@/theme/ThemeProvider';
import { WalletProvider } from '@/wallet/WalletProvider';
import { SettingsWidget } from './SettingsWidget';

// Supply jsdom's missing browser media-query API using CSS evaluation.
class CssMediaQueryList extends EventTarget implements MediaQueryList {
  onchange: MediaQueryList['onchange'] = null;

  constructor(readonly media: string) {
    super();
  }

  get matches(): boolean {
    const style = document.createElement('style');
    const probe = document.createElement('div');
    probe.id = 'settings-media-query-probe';
    style.textContent = `#${probe.id} { position: static; } @media ${this.media} { #${probe.id} { position: fixed; } }`;
    document.head.appendChild(style);
    document.body.appendChild(probe);
    try {
      return window.getComputedStyle(probe).position === 'fixed';
    } finally {
      probe.remove();
      style.remove();
    }
  }

  addListener(listener: (this: MediaQueryList, event: MediaQueryListEvent) => void): void {
    this.addEventListener('change', listener as EventListener);
  }

  removeListener(listener: (this: MediaQueryList, event: MediaQueryListEvent) => void): void {
    this.removeEventListener('change', listener as EventListener);
  }
}

const originalMatchMedia = Object.getOwnPropertyDescriptor(window, 'matchMedia');

beforeEach(() => {
  localStorage.clear();
  currencyRatesRepository.write({ base: 'USD', rates: { USD: 1 }, fetchedAt: Date.now() });
  preferencesRepository.update((current) => ({ ...current, developerMode: true }));
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => new CssMediaQueryList(query),
  });
});

afterEach(() => {
  localStorage.clear();
  if (originalMatchMedia) Object.defineProperty(window, 'matchMedia', originalMatchMedia);
  else Reflect.deleteProperty(window, 'matchMedia');
});

const messages = getMessages('en');
const nonceInput = `input[placeholder="${messages.settings.customNoncePlaceholder}"]`;

async function clickSettingsButton(view: Mounted, label: string) {
  const button = view
    .all<HTMLButtonElement>('button')
    .find(
      (entry) =>
        entry.textContent?.trim() === label ||
        Array.from(entry.querySelectorAll('span')).some((span) => span.textContent === label)
    );
  expect(button, `settings button ${label}`).toBeDefined();
  button!.setAttribute('data-repair-action', 'target');
  try {
    await view.click('[data-repair-action="target"]');
  } finally {
    button!.removeAttribute('data-repair-action');
  }
}

async function openAdvancedSettings(): Promise<Mounted> {
  const view = await mount(
    createElement(WalletProvider, {
      config: null,
      identity: null,
      children: createElement(LocaleProvider, {
        children: createElement(ThemeProvider, {
          children: createElement(PWAProvider, { children: createElement(SettingsWidget) }),
        }),
      }),
    })
  );
  try {
    await clickSettingsButton(view, messages.settings.advanced);
    expect(view.text('h2')).toBe(messages.settings.advanced);
    return view;
  } catch (error) {
    await view.unmount();
    throw error;
  }
}

function expectSelectedNonce(view: Mounted, displayed: string, saved: number | null) {
  expect(view.query<HTMLInputElement>(nonceInput)?.value).toBe(displayed);
  expect(preferencesRepository.read().customNonce).toBe(saved);
  const envelope = JSON.parse(localStorage.getItem(preferencesRepository.registration.key)!);
  expect(envelope.value.customNonce).toBe(saved);
}

async function settings_nonce_parser_preserves_exact_safe_decimal_values() {
  let view = await openAdvancedSettings();
  try {
    await clickSettingsButton(view, 'Manual');
    for (const value of ['0', '42', '00042', String(Number.MAX_SAFE_INTEGER)]) {
      await view.type(nonceInput, value);
      expectSelectedNonce(view, value, Number(value));
      expect(view.query(nonceInput)?.getAttribute('aria-invalid')).toBe('false');
      expect(view.query('[role="alert"]')).toBeNull();
    }
    await view.unmount();
    view = await openAdvancedSettings();
    expectSelectedNonce(view, String(Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER);
  } finally {
    await view.unmount();
  }
}

async function settings_nonce_parser_clears_stale_selection_for_every_invalid_edit() {
  const view = await openAdvancedSettings();
  try {
    await clickSettingsButton(view, 'Manual');
    for (const invalid of [
      '1.5',
      '1e3',
      '1E3',
      '-1',
      '+1',
      '1abc',
      '0x10',
      ' 1',
      '1 ',
      'NaN',
      'Infinity',
      '9007199254740992',
      '9007199254740993',
      '9'.repeat(400),
    ]) {
      await view.type(nonceInput, '17');
      expectSelectedNonce(view, '17', 17);
      await view.type(nonceInput, invalid);
      expectSelectedNonce(view, invalid, null);
      expect(view.query(nonceInput)?.getAttribute('aria-invalid')).toBe('true');
      const errorId = view.query(nonceInput)?.getAttribute('aria-describedby');
      expect(errorId).toBeTruthy();
      expect(view.query(`#${errorId}`)?.getAttribute('role')).toBe('alert');
      expect(view.text(`#${errorId}`)).toContain('9007199254740991');
      expect(view.text(`#${errorId}`)).toContain('Automatic nonce selection');
    }
    await view.type(nonceInput, '23');
    expectSelectedNonce(view, '23', 23);
    expect(view.query(nonceInput)?.getAttribute('aria-invalid')).toBe('false');
    expect(view.query('[role="alert"]')).toBeNull();
  } finally {
    await view.unmount();
  }
}

async function settings_nonce_parser_does_not_restore_invalid_selection_after_reopen() {
  let view = await openAdvancedSettings();
  try {
    await clickSettingsButton(view, 'Manual');
    await view.type(nonceInput, '17');
    await view.type(nonceInput, '-1');
    expectSelectedNonce(view, '-1', null);
    await view.unmount();
    view = await openAdvancedSettings();
    expect(preferencesRepository.read().customNonce).toBeNull();
    expect(view.query(nonceInput)).toBeNull();
    await clickSettingsButton(view, 'Manual');
    expectSelectedNonce(view, '', null);
    await view.type(nonceInput, '0');
    expectSelectedNonce(view, '0', 0);
  } finally {
    await view.unmount();
  }
}

async function settings_nonce_parser_clears_selection_on_empty_input_and_auto_mode() {
  const view = await openAdvancedSettings();
  try {
    await clickSettingsButton(view, 'Manual');
    await view.type(nonceInput, '17');
    await view.type(nonceInput, '');
    expectSelectedNonce(view, '', null);
    expect(view.query('[role="alert"]')).toBeNull();
    await view.type(nonceInput, '23');
    await clickSettingsButton(view, 'Auto');
    expect(preferencesRepository.read().customNonce).toBeNull();
    expect(view.query(nonceInput)).toBeNull();
    await clickSettingsButton(view, 'Manual');
    expectSelectedNonce(view, '', null);
    await view.type(nonceInput, '1.5');
    await clickSettingsButton(view, 'Auto');
    await clickSettingsButton(view, 'Manual');
    expectSelectedNonce(view, '', null);
    expect(view.query('[role="alert"]')).toBeNull();
  } finally {
    await view.unmount();
  }
}

async function settings_nonce_parser_clears_unsafe_previously_saved_values() {
  preferencesRepository.update((current) => ({ ...current, customNonce: 9007199254740992 }));
  const view = await openAdvancedSettings();
  try {
    expectSelectedNonce(view, '9007199254740992', null);
    expect(view.query(nonceInput)?.getAttribute('aria-invalid')).toBe('true');
    expect(view.query('[role="alert"]')).not.toBeNull();
    await view.type(nonceInput, '42');
    expectSelectedNonce(view, '42', 42);
    expect(view.query('[role="alert"]')).toBeNull();
  } finally {
    await view.unmount();
  }
}

test(
  settings_nonce_parser_preserves_exact_safe_decimal_values.name,
  settings_nonce_parser_preserves_exact_safe_decimal_values
);
test(
  settings_nonce_parser_clears_stale_selection_for_every_invalid_edit.name,
  settings_nonce_parser_clears_stale_selection_for_every_invalid_edit
);
test(
  settings_nonce_parser_does_not_restore_invalid_selection_after_reopen.name,
  settings_nonce_parser_does_not_restore_invalid_selection_after_reopen
);
test(
  settings_nonce_parser_clears_selection_on_empty_input_and_auto_mode.name,
  settings_nonce_parser_clears_selection_on_empty_input_and_auto_mode
);
test(
  settings_nonce_parser_clears_unsafe_previously_saved_values.name,
  settings_nonce_parser_clears_unsafe_previously_saved_values
);
