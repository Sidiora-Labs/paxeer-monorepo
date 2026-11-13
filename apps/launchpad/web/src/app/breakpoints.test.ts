import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { describe, expect, it } from 'vitest';
import type tailwindPostcss from '@tailwindcss/postcss';

// Compile the real globals.css (with tokens.css) through Tailwind's own compiler, exactly as
// @tailwindcss/postcss does for the app, and read the cascade order it emits.
const appDir = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const postcssPlugin = require.resolve('@tailwindcss/postcss');
type Stylesheet = Parameters<NonNullable<Extract<ReturnType<typeof tailwindPostcss>, { postcssPlugin: string }>['Once']>>[0];
const postcss = createRequire(postcssPlugin)('postcss') as { parse(css: string): Stylesheet };
const nodeEntry = createRequire(postcssPlugin).resolve('@tailwindcss/node');
const { compile } = (await import(pathToFileURL(nodeEntry).href)) as {
  compile(
    css: string,
    options: { base: string; onDependency(path: string): void }
  ): Promise<{ build(candidates: string[]): string }>;
};

async function build(candidates: string[]): Promise<string> {
  const compiler = await compile(readFileSync(`${appDir}/globals.css`, 'utf8'), {
    base: appDir,
    onDependency() {},
  });
  const css = postcss.parse(compiler.build(candidates));
  // Tailwind 4.3 lowers nested media into enclosing media blocks. Re-express
  // those same compiled predicates under their owning selectors so the shape
  // assertion below works with both compiler generations. Values and cascade
  // order come entirely from the real compiler output.
  css.walkAtRules('media', (media) => {
    const nodes = media.nodes;
    if (!nodes?.length || !nodes.every((node) => node.type === 'rule')) return;
    media.replaceWith(
      ...nodes.map((rule) =>
        rule.clone({ nodes: [media.clone({ nodes: rule.nodes.map((node) => node.clone()) })] })
      )
    );
  });
  return css.toString();
}

/** Pixel threshold of every `@media (width >= X)` block, in emitted order (rem at 16px). */
function mediaOrder(css: string): number[] {
  return [...css.matchAll(/@media \(width >= ([\d.]+)(px|rem)\)/g)].map(
    ([, value, unit]) => Number(value) * (unit === 'rem' ? 16 : 1)
  );
}

describe('breakpoint cascade order', () => {
  it('emits every breakpoint variant in ascending width, custom ones included', async () => {
    const css = await build([
      '3xl:w-30',
      '2xl:w-20',
      'xl:w-10',
      'lg:w-4',
      'mmd:w-3',
      'md:w-5',
      'sm:w-2',
      'xs:w-1',
    ]);
    const order = mediaOrder(css);
    expect(order).toEqual([375, 640, 768, 900, 1024, 1280, 1440, 1920]);
  });

  it('lets 2xl: win over xl: on the same property (xl:w-10 2xl:w-20 is 80px at 1440)', async () => {
    const css = await build(['xl:w-10', '2xl:w-20']);
    const xl = css.indexOf('.xl\\:w-10');
    const twoXl = css.indexOf('.\\32 xl\\:w-20');
    expect(xl).toBeGreaterThan(-1);
    expect(twoXl).toBeGreaterThan(xl);
    // At 1440 both media queries match, so the later rule (2xl, 20 spacing units = 80px) wins.
    const block = css.slice(twoXl);
    expect(block).toMatch(/@media \(width >= 90rem\)\s*\{\s*width: calc\(var\(--spacing\) \* 20\)/);
  });
});
