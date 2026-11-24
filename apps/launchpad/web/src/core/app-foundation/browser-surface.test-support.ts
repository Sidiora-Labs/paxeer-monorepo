import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { Worker } from 'node:worker_threads';
import { dirname, join } from 'node:path';
import { pathToFileURL } from 'node:url';
import type { BrowserType } from 'playwright';

/** Supply native browser media queries and resolved application tokens to DOM render cases. */
export function applyBrowserSurface() {
  const require = createRequire(join(process.cwd(), 'package.json'));
  const { chromium } = require('playwright') as { chromium: BrowserType };
  if (!existsSync(chromium.executablePath())) {
    execFileSync(process.execPath, [join(dirname(require.resolve('playwright/package.json')), 'cli.js'), 'install', 'chromium'], { stdio: 'inherit' });
  }
  const signal = new Int32Array(new SharedArrayBuffer(8));
  const bytes = new Uint8Array(new SharedArrayBuffer(256 * 1024));
  const worker = new Worker(pathToFileURL(join(__dirname, 'browser-surface-worker.mjs')), {
    workerData: { signal: signal.buffer, bytes: bytes.buffer },
  });
  function call<T>(operation: string, args?: unknown): T {
    Atomics.store(signal, 0, 0);
    worker.postMessage({ operation, args });
    if (Atomics.wait(signal, 0, 0, 30_000) === 'timed-out') {
      throw new Error(`Browser surface ${operation} did not answer`);
    }
    const response = JSON.parse(new TextDecoder().decode(bytes.subarray(0, Atomics.load(signal, 1)))) as { value: T; error?: string };
    if (response.error) throw new Error(response.error);
    return response.value;
  }
  const viewport = () => ({ width: window.innerWidth, height: window.innerHeight });
  const style = document.createElement('style');
  try {
    style.textContent = call<string>('open', {
      viewport: viewport(),
      css: readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8'),
    });
  } catch (error) {
    void worker.terminate();
    throw error;
  }
  document.head.append(style);
  const originalMedia = window.matchMedia;
  const lists = new Map<number, MediaQueryList>();
  window.matchMedia = query => {
    const id = lists.size;
    const result = call<{ media: string; matches: boolean }>('media', { id, query });
    const list = new window.EventTarget() as MediaQueryList;
    Object.defineProperties(list, {
      media: { value: result.media },
      matches: { get: () => call<boolean>('matches', id) },
    });
    list.onchange = null;
    list.addListener = listener => { if (listener) list.addEventListener('change', listener); };
    list.removeListener = listener => { if (listener) list.removeEventListener('change', listener); };
    lists.set(id, list);
    return list;
  };
  worker.on('message', ({ id, matches }: { id: number; matches: boolean }) => {
    const list = lists.get(id);
    if (!list) throw new Error('Unknown browser media query');
    const event = new window.Event('change') as MediaQueryListEvent;
    Object.defineProperties(event, { matches: { value: matches }, media: { value: list.media } });
    list.dispatchEvent(event);
    list.onchange?.call(list, event);
  });
  const resize = () => call<void>('viewport', viewport());
  window.addEventListener('resize', resize);
  return async () => {
    window.removeEventListener('resize', resize);
    window.matchMedia = originalMedia;
    style.remove();
    try { call<null>('close'); } finally { await worker.terminate(); }
  };
}
