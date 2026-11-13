import { createRequire } from 'node:module';
import { act, createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, test } from 'vitest';
import { defaultPresets, parsePresets, presetsStorageKey, useTradePresets, type PresetsState, type TradePreset } from './presets-store';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; storageQuota?: number }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;

const editedPreset: TradePreset = {
  buyAmounts: ['123.456789', '234.567891', '345.678912', '456.789123'],
  sellPercents: [15, 35, 65, 95],
  slippageBps: 725,
};

function withBrowser(options: { url: string; storageQuota?: number }, run: (browser: BrowserDom) => void) {
  const browser = new JSDOM('<!DOCTYPE html><html><body></body></html>', options);
  const values = { window: browser.window, document: browser.window.document, IS_REACT_ACT_ENVIRONMENT: true };
  const previous = Object.fromEntries(Object.keys(values).map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  for (const [key, value] of Object.entries(values)) Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
  try { run(browser); } finally {
    browser.window.close();
    for (const key of Object.keys(values)) {
      const descriptor = previous[key];
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
}

function mountPresets() {
  let current: ReturnType<typeof useTradePresets> | undefined;
  function PresetConsumer() {
    current = useTradePresets();
    return createElement('output', null, JSON.stringify({ active: current.active, presets: current.presets }));
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(createElement(PresetConsumer)));
  return {
    container,
    read() {
      if (!current) throw new Error('Preset consumer has not rendered');
      return current;
    },
    render() { act(() => root.render(createElement(PresetConsumer))); },
    close() { act(() => root.unmount()); container.remove(); },
  };
}

function expectState(consumer: ReturnType<typeof mountPresets>, expected: PresetsState) {
  expect(consumer.read().active).toBe(expected.active);
  expect(consumer.read().presets).toEqual(expected.presets);
  expect(consumer.container.textContent).toBe(JSON.stringify(expected));
}

function preset_storage_fallback_keeps_edits_when_first_write_exceeds_quota() {
  withBrowser({ url: 'http://localhost', storageQuota: 0 }, ({ window }) => {
    expect(window.localStorage.getItem(presetsStorageKey)).toBeNull();
    expect(() => window.localStorage.setItem(presetsStorageKey, '{}')).toThrow();
    const expected = defaultPresets();
    expected.active = 2;
    expected.presets[2] = editedPreset;
    const consumer = mountPresets();
    try {
      expectState(consumer, defaultPresets());
      act(() => { expect(consumer.read().updatePreset(2, editedPreset)).toBe(true); });
      act(() => consumer.read().setActive(2));
      expectState(consumer, expected);
      act(() => { expect(consumer.read().updatePreset(2, { slippageBps: 0 })).toBe(false); });
      consumer.render();
      expectState(consumer, expected);
      expect(window.localStorage.getItem(presetsStorageKey)).toBeNull();
      expect(parsePresets(window.localStorage.getItem(presetsStorageKey))).toEqual(defaultPresets());
    } finally { consumer.close(); }
    const reopened = mountPresets();
    try { expectState(reopened, expected); } finally { reopened.close(); }
  });
}

function preset_storage_fallback_preserves_baseline_and_resumes_successful_persistence() {
  const persisted = defaultPresets();
  persisted.active = 3;
  const raw = JSON.stringify(persisted);
  const expected = { ...persisted, presets: { ...persisted.presets, 3: editedPreset } };
  withBrowser({ url: 'http://localhost', storageQuota: presetsStorageKey.length + raw.length }, ({ window }) => {
    window.localStorage.setItem(presetsStorageKey, raw);
    expect(() => window.localStorage.setItem(presetsStorageKey, JSON.stringify(expected))).toThrow();
    const consumer = mountPresets();
    try {
      expectState(consumer, persisted);
      act(() => { expect(consumer.read().updatePreset(3, editedPreset)).toBe(true); });
      consumer.render();
      expectState(consumer, expected);
      expect(window.localStorage.getItem(presetsStorageKey)).toBe(raw);
    } finally { consumer.close(); }
  });
  withBrowser({ url: 'http://localhost' }, ({ window }) => {
    window.localStorage.setItem(presetsStorageKey, raw);
    const consumer = mountPresets();
    try {
      expectState(consumer, expected);
      act(() => consumer.read().setActive(2));
      expected.active = 2;
      expectState(consumer, expected);
      expect(parsePresets(window.localStorage.getItem(presetsStorageKey))).toEqual(expected);
    } finally { consumer.close(); }
    const reopened = mountPresets();
    try {
      expectState(reopened, expected);
      window.localStorage.setItem(presetsStorageKey, raw);
      act(() => { window.dispatchEvent(new window.StorageEvent('storage', { key: presetsStorageKey })); });
      expectState(reopened, persisted);
      window.localStorage.removeItem(presetsStorageKey);
      act(() => { window.dispatchEvent(new window.StorageEvent('storage', { key: presetsStorageKey })); });
      expectState(reopened, defaultPresets());
    } finally { reopened.close(); }
  });
}

function preset_storage_fallback_keeps_edits_when_storage_access_is_blocked() {
  withBrowser({ url: 'about:blank' }, ({ window }) => {
    expect(() => window.localStorage).toThrow();
    const expected = defaultPresets();
    expected.active = 1;
    expected.presets[1] = editedPreset;
    const consumer = mountPresets();
    try {
      act(() => { expect(consumer.read().updatePreset(1, editedPreset)).toBe(true); });
      consumer.render();
      expectState(consumer, expected);
    } finally { consumer.close(); }
    const reopened = mountPresets();
    try { expectState(reopened, expected); } finally { reopened.close(); }
  });
}

test(preset_storage_fallback_keeps_edits_when_first_write_exceeds_quota.name, preset_storage_fallback_keeps_edits_when_first_write_exceeds_quota);
test(preset_storage_fallback_preserves_baseline_and_resumes_successful_persistence.name, preset_storage_fallback_preserves_baseline_and_resumes_successful_persistence);
test(preset_storage_fallback_keeps_edits_when_storage_access_is_blocked.name, preset_storage_fallback_keeps_edits_when_storage_access_is_blocked);
