import { describe, expect, it } from 'vitest';

import { radii } from './borders';

const SCALE = [ 'none', 'xs', 'sm', 'base', 'md', 'lg', 'xl', '2xl', 'full', 'control', 'card', 'pill', 'indicator', 'scrollbar' ];

describe('radii', () => {
  it('declares the product corner scale in order', () => {
    expect(Object.keys(radii ?? {})).toEqual(SCALE);
  });

  it('grows from control to card to pill and keeps the compatibility steps', () => {
    const sizes = [ 'none', 'control', 'card', 'pill' ].map((key) => parseInt((radii?.[key] as unknown as { value: string }).value, 10));

    expect(SCALE.map((key) => (radii?.[key] as unknown as { value: string }).value))
      .toEqual([ '0', '6px', '6px', '6px', '8px', '8px', '8px', '8px', '999px', '6px', '8px', '999px', '0.125rem', '1.25rem' ]);

    sizes.slice(1).forEach((size, index) => {
      expect(size).toBeGreaterThan(sizes[index]);
    });
  });
});
