// @vitest-environment jsdom
// @vitest-environment-options {"url":"https://theme.test/","storageQuota":128}

import * as React from 'react';
import { createRoot, type Root } from 'react-dom/client';

import { afterEach, beforeEach, expect, test } from 'vitest';

import { ThemeProvider, useTheme } from './theme-provider';

declare const jsdom: { reconfigure(options: { url: string }): void };

const roots = new Set<Root>();
const descriptors = new Map<string, PropertyDescriptor | undefined>();

function lightSystemPreference(media: string): MediaQueryList {
  const events = new EventTarget();
  return {
    media,
    matches: false,
    onchange: null,
    addEventListener: events.addEventListener.bind(events),
    removeEventListener: events.removeEventListener.bind(events),
    dispatchEvent: events.dispatchEvent.bind(events),
    addListener(listener) {
      if (listener) events.addEventListener('change', listener as EventListener);
    },
    removeListener(listener) {
      if (listener) events.removeEventListener('change', listener as EventListener);
    },
  };
}

beforeEach(() => {
  jsdom.reconfigure({ url: 'https://theme.test/' });
  window.localStorage.clear();
  descriptors.set('matchMedia', Object.getOwnPropertyDescriptor(window, 'matchMedia'));
  descriptors.set('IS_REACT_ACT_ENVIRONMENT', Object.getOwnPropertyDescriptor(globalThis, 'IS_REACT_ACT_ENVIRONMENT'));
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: lightSystemPreference,
  });
  Object.defineProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT', {
    configurable: true,
    value: true,
  });
});

afterEach(async() => {
  await React.act(async() => {
    for (const root of roots) root.unmount();
    roots.clear();
  });
  document.body.replaceChildren();
  document.documentElement.className = '';
  jsdom.reconfigure({ url: 'https://theme.test/' });
  window.localStorage.clear();
  for (const [ key, descriptor ] of descriptors) {
    const target = key === 'matchMedia' ? window : globalThis;
    if (descriptor) Object.defineProperty(target, key, descriptor);
    else Reflect.deleteProperty(target, key);
  }
  descriptors.clear();
});

async function mountPreferences() {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  roots.add(root);
  function Controls() {
    const { theme, resolvedTheme, accent, setTheme, setAccent, toggleTheme } = useTheme();
    return React.createElement(React.Fragment, null,
      React.createElement('output', null, `${ theme }:${ resolvedTheme }:${ accent }`),
      React.createElement('button', { onClick: () => setTheme('dark'), 'data-action': 'dark' }, 'Dark'),
      React.createElement('button', { onClick: () => setAccent('dx-gradient'), 'data-action': 'gradient' }, 'Gradient'),
      React.createElement('button', { onClick: toggleTheme, 'data-action': 'toggle' }, 'Toggle'),
      React.createElement('button', { onClick: () => setAccent('ink'), 'data-action': 'ink' }, 'Ink'),
    );
  }
  await React.act(async() => {
    root.render(React.createElement(ThemeProvider, null, React.createElement(Controls)));
  });
  return {
    value() {
      return container.querySelector('output')?.textContent;
    },
    async click(action: 'dark' | 'gradient' | 'toggle' | 'ink') {
      const button = container.querySelector<HTMLButtonElement>(`[data-action="${ action }"]`);
      expect(button).not.toBeNull();
      await React.act(async() => button!.click());
    },
    async unmount() {
      await React.act(async() => root.unmount());
      roots.delete(root);
      container.remove();
    },
  };
}

function expectPreferences(
  controls: Awaited<ReturnType<typeof mountPreferences>>,
  value: string,
  dark: boolean,
  gradient: boolean,
) {
  expect(controls.value()).toBe(value);
  expect(document.documentElement.classList.contains('dark')).toBe(dark);
  expect(document.documentElement.classList.contains('accent-dx-gradient')).toBe(gradient);
}

async function exerciseDeniedWrites(controls: Awaited<ReturnType<typeof mountPreferences>>) {
  await controls.click('dark');
  expectPreferences(controls, 'dark:dark:ink', true, false);
  await controls.click('gradient');
  expectPreferences(controls, 'dark:dark:dx-gradient', true, true);
  await controls.click('toggle');
  expectPreferences(controls, 'light:light:dx-gradient', false, true);
  await controls.click('ink');
  expectPreferences(controls, 'light:light:ink', false, false);
}

function nextStorageEvent(target: Window, key: string, newValue: string) {
  return new Promise<StorageEvent>((resolve) => {
    function onStorage(event: StorageEvent) {
      if (event.key !== key || event.newValue !== newValue) return;
      target.removeEventListener('storage', onStorage);
      resolve(event);
    }
    target.addEventListener('storage', onStorage);
  });
}

async function theme_storage_write_handles_denied_reads_and_writes() {
  jsdom.reconfigure({ url: 'file:///blocked-storage' });
  expect(() => window.localStorage).toThrow(expect.objectContaining({ name: 'SecurityError' }));
  const controls = await mountPreferences();
  expectPreferences(controls, 'system:light:ink', false, false);
  await exerciseDeniedWrites(controls);
}

async function theme_storage_write_handles_storage_denied_after_mount() {
  const controls = await mountPreferences();
  jsdom.reconfigure({ url: 'file:///blocked-storage' });
  expect(() => window.localStorage).toThrow(expect.objectContaining({ name: 'SecurityError' }));
  await exerciseDeniedWrites(controls);
}

async function theme_storage_write_handles_quota_and_recovers_persistence() {
  window.localStorage.setItem('fill', 'x'.repeat(124));
  expect(() => window.localStorage.setItem('dx-theme', 'dark')).toThrow(
    expect.objectContaining({ name: 'QuotaExceededError' }),
  );
  const controls = await mountPreferences();
  await exerciseDeniedWrites(controls);
  expect(window.localStorage.getItem('dx-theme')).toBeNull();
  expect(window.localStorage.getItem('dx-accent')).toBeNull();

  await controls.unmount();
  const reopened = await mountPreferences();
  expectPreferences(reopened, 'system:light:ink', false, false);
  window.localStorage.removeItem('fill');
  await reopened.click('dark');
  await reopened.click('gradient');
  expect(window.localStorage.getItem('dx-theme')).toBe('dark');
  expect(window.localStorage.getItem('dx-accent')).toBe('dx-gradient');
  await reopened.unmount();

  const recovered = await mountPreferences();
  expectPreferences(recovered, 'dark:dark:dx-gradient', true, true);
}

async function theme_storage_write_preserves_real_cross_document_synchronization() {
  const controls = await mountPreferences();
  const frame = document.createElement('iframe');
  document.body.append(frame);
  const peer = frame.contentWindow;
  expect(peer).not.toBeNull();
  if (!peer) throw new Error('Same-origin storage document did not open');

  const sent = nextStorageEvent(peer, 'dx-theme', 'dark');
  await controls.click('dark');
  const written = await sent;
  expect(written.key).toBe('dx-theme');
  expect(written.newValue).toBe('dark');
  expect(peer.localStorage.getItem('dx-theme')).toBe('dark');

  const accentSent = nextStorageEvent(peer, 'dx-accent', 'dx-gradient');
  await controls.click('gradient');
  const accentWritten = await accentSent;
  expect(accentWritten.key).toBe('dx-accent');
  expect(accentWritten.newValue).toBe('dx-gradient');
  expect(peer.localStorage.getItem('dx-accent')).toBe('dx-gradient');

  const receivePreference = async(key: string, value: string) => {
    const received = nextStorageEvent(window, key, value);
    await React.act(async() => {
      peer.localStorage.setItem(key, value);
      const event = await received;
      expect(event.key).toBe(key);
      expect(event.newValue).toBe(value);
    });
  };

  await receivePreference('dx-theme', 'light');
  expectPreferences(controls, 'light:light:dx-gradient', false, true);
  await receivePreference('dx-accent', 'ink');
  expectPreferences(controls, 'light:light:ink', false, false);
  await receivePreference('dx-theme', 'invalid-theme');
  await receivePreference('dx-accent', 'invalid-accent');
  expectPreferences(controls, 'light:light:ink', false, false);

  await controls.click('toggle');
  await controls.click('ink');
  await controls.unmount();
  const recovered = await mountPreferences();
  expectPreferences(recovered, 'dark:dark:ink', true, false);
}

test(theme_storage_write_handles_denied_reads_and_writes.name, theme_storage_write_handles_denied_reads_and_writes);
test(theme_storage_write_handles_storage_denied_after_mount.name, theme_storage_write_handles_storage_denied_after_mount);
test(theme_storage_write_handles_quota_and_recovers_persistence.name, theme_storage_write_handles_quota_and_recovers_persistence);
test(theme_storage_write_preserves_real_cross_document_synchronization.name, theme_storage_write_preserves_real_cross_document_synchronization);
