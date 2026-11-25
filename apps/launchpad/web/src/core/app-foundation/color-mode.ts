import { launchpadCanvasColors } from '@/app/presentation-tokens';

export type ColorMode = 'dark' | 'light' | 'device';
export type ColorScheme = 'dark' | 'light';
export const COLOR_MODE_STORAGE_KEY = 'sidiora_color_mode';

export function isColorMode(value: unknown): value is ColorMode {
  return value === 'dark' || value === 'light' || value === 'device';
}

export function deploymentColorMode(): ColorMode {
  const value = process.env.NEXT_PUBLIC_COLOR_MODE_DEFAULT;
  return isColorMode(value) ? value : 'dark';
}

/** Apply the stored choice before content paints; storage may be unavailable on this device. */
export function applyInitialColorMode(
  defaultMode: ColorMode,
  storageKey: string,
  canvas: Record<ColorScheme, string>
): ColorMode {
  let mode = defaultMode;
  try {
    const stored = window.localStorage.getItem(storageKey);
    if (stored === 'dark' || stored === 'light' || stored === 'device') mode = stored;
  } catch {
    // A browser that refuses storage uses the deployment's default.
  }
  const scheme = mode === 'device'
    ? window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    : mode;
  document.documentElement.setAttribute('data-color-mode', mode);
  if (mode === 'device') document.documentElement.style.removeProperty('color-scheme');
  else document.documentElement.style.colorScheme = scheme;
  let frame = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
  if (!frame) {
    frame = document.createElement('meta');
    frame.name = 'theme-color';
    document.head.append(frame);
  }
  frame.content = canvas[scheme];
  return mode;
}

export function colorModeScript(defaultMode: ColorMode): string {
  return `(${applyInitialColorMode.toString()})(${JSON.stringify(defaultMode)},${JSON.stringify(COLOR_MODE_STORAGE_KEY)},${JSON.stringify(launchpadCanvasColors)})`;
}

function documentDefault(): ColorMode {
  const value = document.documentElement.getAttribute('data-color-mode-default');
  return isColorMode(value) ? value : deploymentColorMode();
}

export function getColorMode(): ColorMode {
  if (typeof document === 'undefined') return deploymentColorMode();
  const value = document.documentElement.getAttribute('data-color-mode');
  return isColorMode(value) ? value : documentDefault();
}

export function getColorScheme(): ColorScheme {
  const mode = getColorMode();
  if (mode !== 'device') return mode;
  if (typeof window === 'undefined') return 'light';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

const readers = new Set<() => void>();
let stopListening: (() => void) | undefined;
let stopDeviceListening: (() => void) | undefined;
let lastSnapshot = '';

function publishColorMode() {
  const mode = getColorMode();
  if (mode === 'device' && readers.size > 0) {
    if (!stopDeviceListening) {
      const media = window.matchMedia('(prefers-color-scheme: dark)');
      media.addEventListener('change', publishColorMode);
      stopDeviceListening = () => media.removeEventListener('change', publishColorMode);
    }
  } else {
    stopDeviceListening?.();
    stopDeviceListening = undefined;
  }
  const scheme = getColorScheme();
  if (mode === 'device') document.documentElement.style.removeProperty('color-scheme');
  else document.documentElement.style.colorScheme = scheme;
  for (const frame of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    frame.removeAttribute('media');
    frame.content = launchpadCanvasColors[scheme];
  }
  const snapshot = `${mode}:${scheme}`;
  if (snapshot === lastSnapshot) return;
  lastSnapshot = snapshot;
  for (const reader of readers) reader();
}

export function subscribeColorMode(reader: () => void): () => void {
  readers.add(reader);
  if (!stopListening) {
    const root = document.documentElement;
    if (!isColorMode(root.getAttribute('data-color-mode'))) {
      applyInitialColorMode(documentDefault(), COLOR_MODE_STORAGE_KEY, launchpadCanvasColors);
    }
    const storageChanged = (event: StorageEvent) => {
      if (event.key !== null && event.key !== COLOR_MODE_STORAGE_KEY) return;
      applyInitialColorMode(documentDefault(), COLOR_MODE_STORAGE_KEY, launchpadCanvasColors);
      publishColorMode();
    };
    window.addEventListener('storage', storageChanged);
    const observer = new MutationObserver(publishColorMode);
    observer.observe(root, { attributes: true, attributeFilter: ['data-color-mode'] });
    stopListening = () => {
      stopDeviceListening?.();
      stopDeviceListening = undefined;
      window.removeEventListener('storage', storageChanged);
      observer.disconnect();
    };
  }
  publishColorMode();
  return () => {
    readers.delete(reader);
    if (readers.size === 0) {
      stopListening?.();
      stopListening = undefined;
      lastSnapshot = '';
    }
  };
}

export function setColorMode(mode: ColorMode): void {
  document.documentElement.setAttribute('data-color-mode', mode);
  try {
    window.localStorage.setItem(COLOR_MODE_STORAGE_KEY, mode);
  } catch {
    // The choice still applies to the current page when storage is unavailable.
  }
  publishColorMode();
}
