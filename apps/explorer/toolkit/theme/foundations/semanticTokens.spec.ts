import { describe, expect, it } from 'vitest';

import semanticTokens from './semanticTokens';

function walk(path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => (node as Record<string, unknown>)[key], semanticTokens);
}

describe('semantic tokens on the data visualisation palette', () => {
  it('draws the stat indicators from the positive and negative roles', () => {
    expect(walk('colors.stat.indicator.up.value')).toEqual({
      _light: '{colors.positive}',
      _dark: '{colors.positive}',
    });
    expect(walk('colors.stat.indicator.down.value')).toEqual({
      _light: '{colors.negative}',
      _dark: '{colors.negative}',
    });
  });

  it('draws the pink and yellow badges from the reference ramps', () => {
    expect(walk('colors.badge.pink.bg.value')).toEqual({
      _light: '{colors.datavis.pink.lowest}',
      _dark: '{colors.datavis.pink.highest}',
    });
    expect(walk('colors.badge.yellow.fg.value')).toEqual({
      _light: '{colors.warning}',
      _dark: '{colors.warning}',
    });
  });

  it('greys every placeholder and field indicator with one neutral tone in both appearances', () => {
    [
      'colors.input.placeholder.DEFAULT.value',
      'colors.input.element.DEFAULT.value',
      'colors.field.placeholder.DEFAULT.value',
      'colors.select.indicator.fg.DEFAULT.value',
      'colors.select.placeholder.fg.DEFAULT.value',
    ].forEach((path) => {
      expect(walk(path)).toEqual({ _light: '{colors.gray.500}', _dark: '{colors.gray.500}' });
    });
  });
});
