import { describe, expect, it } from 'vitest';

import { recipe } from './button.recipe';

const control = recipe.variants?.variant.scan_control;

describe('button recipe, scan_control variant', () => {
  it('draws an outlined control on the surface', () => {
    expect(control).toMatchObject({
      borderWidth: 'var(--chakra-border-widths-1)',
      bg: 'bg.surface',
      color: 'text.secondary',
      borderColor: 'border.divider',
    });
  });

  it('fills the selected control with the accent and mutes the disabled one', () => {
    expect(control?._selected).toMatchObject({
      bg: 'selected.control.bg',
      color: 'selected.control.text',
      borderColor: 'transparent',
    });
    expect(control?._disabled).toMatchObject({ color: 'text.muted', borderColor: 'border.divider' });
  });

  it('draws outlined variants with contrasting borders and the product padding', () => {
    expect(recipe.base?.fontWeight).toBe('medium');
    expect(recipe.variants?.variant.outline).toMatchObject({ borderWidth: 'var(--chakra-border-widths-1)', borderColor: 'border.strong' });
    expect(recipe.variants?.variant.dropdown).toMatchObject({ borderWidth: 'var(--chakra-border-widths-1)' });
    expect(recipe.variants?.variant.pagination).toMatchObject({ borderWidth: 'var(--chakra-border-widths-1)' });
    expect(recipe.variants?.size.md).toMatchObject({ px: 6, h: 10, borderRadius: 'full' });
    expect(recipe.variants?.size.sm).toMatchObject({ px: 4, h: 8 });
  });

  it('keeps the plain variant and the default the rest of the app uses', () => {
    expect(recipe.variants?.variant.plain).toBeDefined();
    expect(recipe.defaultVariants?.variant).toBe('solid');
  });
});
