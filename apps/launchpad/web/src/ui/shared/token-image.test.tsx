// @vitest-environment jsdom
import React, { act } from 'react';
import { beforeAll, describe, it, expect, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import TokenImage from './token-image';

beforeAll(() => { (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true; });

// vitest.config.ts sets `deps.interopDefault: false` (required by @kindle/sdk),
// so the default import of CJS `next/image` arrives as its whole `exports`
// object. Re-expose the real component as the default, as Next's bundler does.
vi.mock('next/image', async (importOriginal) => {
  const mod = await importOriginal<{ default: unknown }>();
  const cjs = mod.default as { default?: unknown } | undefined;
  return { ...mod, default: cjs && typeof cjs === 'object' && 'default' in cjs ? cjs.default : mod.default };
});

describe('token image raster policy', () => {
  it('uses local fallback for remote SVG even with query or uppercase extension', () => {
    for (const src of ['https://ipfs.io/ipfs/token.svg', 'https://metadata.kindlelaunch.com/logo/token.SVG?v=1']) {
      const html = renderToStaticMarkup(<TokenImage src={src} alt="Token" width={32} height={32} />);
      expect(html).toContain('shadcn.png');
      expect(html).not.toContain('ipfs.io');
      expect(html).not.toContain('metadata.kindlelaunch.com');
    }
  });
  it('keeps authored local SVG icons and optimizes raster originals', () => {
    const local = renderToStaticMarkup(<TokenImage src="/icons/token-fallback.svg" alt="Token" width={32} height={32} />);
    expect(local).toContain('src="/icons/token-fallback.svg"');
    const raster = renderToStaticMarkup(<TokenImage src="https://metadata.kindlelaunch.com/logo/token.png" alt="Token" width={32} height={32} />);
    expect(raster).toContain('/_next/image?');
    expect(raster).toContain('token.png');
  });
});

function tokenImageDoubleFailure() {
  for (const layout of [{ width: 32, height: 32 }, { fill: true as const, sizes: '32px' }]) {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);
    const primary = 'https://metadata.kindlelaunch.com/logo/primary.png';
    const fallback = 'https://metadata.kindlelaunch.com/logo/fallback.png';
    const replacement = 'https://metadata.kindlelaunch.com/logo/replacement.png';
    const image = () => {
      const element = container.querySelector('img');
      if (!element) throw new Error('TokenImage did not render an image');
      return element;
    };
    const source = () => new URL(image().src).searchParams.get('url') ?? image().getAttribute('src');
    const fail = () => act(() => { image().dispatchEvent(new Event('error')); });

    try {
      act(() => root.render(<TokenImage {...layout} src={primary} fallbackSrc={fallback} alt="Token" />));
      expect(source()).toBe(primary);
      fail();
      expect(source()).toBe(fallback);
      fail();
      expect(source()).toBe('/shadcn.png');
      fail();
      expect(source()).toBe('/shadcn.png');
      expect(image().alt).toBe('Token');

      act(() => root.render(<TokenImage {...layout} src={replacement} fallbackSrc={fallback} alt="Replacement" />));
      expect(source()).toBe(replacement);
      fail();
      expect(source()).toBe(fallback);
      fail();
      expect(source()).toBe('/shadcn.png');

      act(() => root.render(<TokenImage {...layout} src={replacement} fallbackSrc="/alternate.png" alt="Replacement" />));
      expect(source()).toBe(replacement);
      fail();
      expect(source()).toBe('/alternate.png');
      fail();
      expect(source()).toBe('/shadcn.png');
    } finally {
      act(() => root.unmount());
      container.remove();
    }
  }
}

function tokenImageSkipsDuplicateAttempts() {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const image = () => {
    const element = container.querySelector('img');
    if (!element) throw new Error('TokenImage did not render an image');
    return element;
  };
  const source = () => new URL(image().src).searchParams.get('url') ?? image().getAttribute('src');
  const fail = () => act(() => { image().dispatchEvent(new Event('error')); });

  try {
    act(() => root.render(<TokenImage src="/primary.png" fallbackSrc="/primary.png" alt="Token" width={32} height={32} />));
    expect(source()).toBe('/primary.png');
    fail();
    expect(source()).toBe('/shadcn.png');
    fail();
    expect(source()).toBe('/shadcn.png');

    act(() => root.render(<TokenImage src={null} fallbackSrc="/fallback.png" alt="Token" width={32} height={32} />));
    expect(source()).toBe('/fallback.png');
    fail();
    expect(source()).toBe('/shadcn.png');

    act(() => root.render(<TokenImage src={undefined} alt="Token" width={32} height={32} />));
    expect(source()).toBe('/shadcn.png');
  } finally {
    act(() => root.unmount());
    container.remove();
  }
}

it(tokenImageDoubleFailure.name, tokenImageDoubleFailure);
it(tokenImageSkipsDuplicateAttempts.name, tokenImageSkipsDuplicateAttempts);
