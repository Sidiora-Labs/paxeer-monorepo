import type { SystemStyleObject } from '@chakra-ui/react';
import { ChakraProvider } from '@chakra-ui/react';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { expect, test } from 'vitest';

import { TableBody, TableCell, TableRoot, TableRow } from '../chakra/table';
import { TabsCounter } from '../chakra/tabs';
import { radii } from './foundations/borders';
import colors from './foundations/colors';
import semanticTokens from './foundations/semanticTokens';
import shadows from './foundations/shadows';
import { BODY_TYPEFACE, FALLBACK_TYPEFACES, fonts, textStyles } from './foundations/typography';
import system from './theme';

type Mode = '_light' | '_dark';

function atPath(source: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (typeof node !== 'object' || node === null) {
      return undefined;
    }
    return (node as Record<string, unknown>)[key];
  }, source);
}

function colourTokens(node: unknown, path = ''): Array<[ string, Record<string, unknown> ]> {
  if (typeof node !== 'object' || node === null) {
    throw new Error(`Invalid semantic colour at ${ path }`);
  }
  const record = node as Record<string, unknown>;
  if ('value' in record) {
    expect(record.value, path).toBeTypeOf('object');
    expect(record.value, path).toHaveProperty('_light');
    expect(record.value, path).toHaveProperty('_dark');
    return [ [ path, record.value as Record<string, unknown> ] ];
  }
  return Object.entries(record).flatMap(([ key, child ]) => colourTokens(child, path ? `${ path }.${ key }` : key));
}

function resolveColour(path: string, mode: Mode, visited = new Set<string>()): string {
  if (visited.has(path)) {
    throw new Error(`Circular colour reference: ${ path }`);
  }
  const next = new Set(visited).add(path);
  const [ name, opacity ] = path.split('/');
  let node = atPath(semanticTokens, name) ?? atPath({ colors }, name);
  if (typeof node === 'object' && node !== null && !('value' in node)) {
    node = (node as Record<string, unknown>).DEFAULT;
  }
  const value = atPath(node, 'value');
  const raw = typeof value === 'string' ? value : atPath(value, mode);
  if (typeof raw !== 'string' || raw.length === 0) {
    throw new Error(`Unresolved colour ${ path } in ${ mode }`);
  }
  const reference = /^\{(.+)\}$/.exec(raw);
  const result = reference ? resolveColour(reference[1], mode, next) : raw;
  if (opacity !== undefined) {
    expect(result, path).toMatch(/^#[0-9a-f]{6}$/i);
    const rgb = [ 1, 3, 5 ].map((start) => parseInt(result.slice(start, start + 2), 16));
    return `rgba(${ rgb.join(',') },${ Number(opacity) / 100 })`;
  }
  return result;
}

function allSemanticColoursResolveInBothModes() {
  const tokens = colourTokens(semanticTokens?.colors);
  expect(tokens.length).toBeGreaterThan(200);
  for (const [ path ] of tokens) {
    for (const mode of [ '_light', '_dark' ] as const) {
      expect(resolveColour(`colors.${ path }`, mode), `${ path } ${ mode }`)
        .toMatch(/^(?:#[0-9a-f]{3,8}|rgba?\([\d\s.,%]+\)|transparent)$/i);
    }
  }
}

function semanticRolesMatchDecision2587() {
  const expected: Record<string, [ string, string ]> = {
    canvas: [ '#FFFFFF', '#0B0C10' ],
    surface: [ '#F7F8FA', '#111319' ],
    raised: [ '#FFFFFF', '#171A21' ],
    hairline: [ 'rgba(16,18,24,0.08)', 'rgba(255,255,255,0.08)' ],
    'border.strong': [ 'rgba(16,18,24,0.16)', 'rgba(255,255,255,0.16)' ],
    'text.primary': [ '#0E1016', '#F2F4F8' ],
    'text.secondary': [ '#4B5262', '#A0A8B8' ],
    'text.muted': [ '#8A90A0', '#6B7385' ],
    accent: [ '#2B4FDA', '#4F7DFF' ],
    'accent.soft': [ 'rgba(43,79,218,0.10)', 'rgba(79,125,255,0.14)' ],
    onAccent: [ '#FFFFFF', '#FFFFFF' ],
    positive: [ '#0E8F55', '#2FD27D' ],
    positiveSoft: [ 'rgba(14,143,85,0.10)', 'rgba(47,210,125,0.14)' ],
    negative: [ '#D1343B', '#FF5C5C' ],
    negativeSoft: [ 'rgba(209,52,59,0.10)', 'rgba(255,92,92,0.14)' ],
    warning: [ '#B46A00', '#F5B544' ],
    warningSoft: [ 'rgba(180,106,0,0.10)', 'rgba(245,181,68,0.14)' ],
    headerBar: [ 'rgba(255,255,255,0.85)', 'rgba(11,12,16,0.85)' ],
    focusRing: [ '#2B4FDA', '#4F7DFF' ],
    'sparkline.up': [ '#0E8F55', '#2FD27D' ],
    'sparkline.down': [ '#D1343B', '#FF5C5C' ],
    'chart.grid': [ 'rgba(16,18,24,0.08)', 'rgba(255,255,255,0.08)' ],
    'chart.area.start': [ 'rgba(43,79,218,0.30)', 'rgba(79,125,255,0.30)' ],
    'chart.area.stop': [ 'rgba(43,79,218,0)', 'rgba(79,125,255,0)' ],
  };
  for (const [ name, [ light, dark ] ] of Object.entries(expected)) {
    expect(resolveColour(`colors.${ name }`, '_light'), name).toBe(light);
    expect(resolveColour(`colors.${ name }`, '_dark'), name).toBe(dark);
  }
  expect(atPath(semanticTokens, 'blurs.headerBar.value')).toBe('12px');
}

function legacyKeysInheritTheRoles() {
  const aliases = {
    graphStart: 'chart.area.start', graphStop: 'chart.area.stop',
    success: 'positive', successSoft: 'positiveSoft', destructive: 'negative', destructiveSoft: 'negativeSoft',
    tonal: 'accent.soft', outline: 'hairline', dataTableBorder: 'hairline', selectedOption: 'accent.soft',
    'bg.primary': 'canvas', 'bg.surface': 'surface', 'bg.overlay': 'raised', 'bg.sunken': 'raised',
    'border.divider': 'hairline', heading: 'text.primary', 'icon.primary': 'text.secondary', 'icon.secondary': 'text.muted',
    'accent.strong': 'accent', 'feedback.success.fg': 'positive', 'feedback.success.bg': 'positiveSoft',
    'feedback.error.fg': 'negative', 'feedback.error.bg': 'negativeSoft',
    'feedback.warning.fg': 'warning', 'feedback.warning.bg': 'warningSoft',
    'button.solid.bg': 'accent', 'button.solid.text': 'onAccent',
    'table.row.hover': 'raised', 'card.border': 'hairline', 'header.sticky.bg': 'headerBar',
    'stat.indicator.up': 'positive', 'stat.indicator.down': 'negative',
    'badge.green.fg': 'positive', 'badge.green.bg': 'positiveSoft', 'badge.red.fg': 'negative', 'badge.red.bg': 'negativeSoft',
    'badge.blue.fg': 'accent', 'badge.blue.bg': 'accent.soft', 'badge.yellow.fg': 'warning', 'badge.yellow.bg': 'warningSoft',
  };
  for (const [ name, role ] of Object.entries(aliases)) {
    for (const mode of [ '_light', '_dark' ] as const) {
      expect(resolveColour(`colors.${ name }`, mode), `${ name } ${ mode }`).toBe(resolveColour(`colors.${ role }`, mode));
    }
  }
  for (const [ name, role ] of Object.entries({
    graphStart: 'chart.area.start', graphStop: 'chart.area.stop', success: 'positive', successSoft: 'positiveSoft',
    destructive: 'negative', destructiveSoft: 'negativeSoft', warning: 'warning', warningSoft: 'warningSoft',
    tonal: 'accent.soft', outline: 'hairline', dataTableBorder: 'hairline', selectedOption: 'accent.soft',
  })) {
    for (const mode of [ '_light', '_dark' ] as const) {
      expect(resolveColour(`colors.theme.${ name }.${ mode }`, mode)).toBe(resolveColour(`colors.${ role }`, mode));
    }
  }
}

function typeScaleAndFamiliesMatchTheBrief() {
  const scale = {
    display: [ '40px', '44px', '600' ], h1: [ '28px', '32px', '600' ], h2: [ '20px', '24px', '600' ],
    body: [ '14px', '20px', '400' ], small: [ '13px', '18px', '400' ], label: [ '11px', '16px', '600' ],
    number: [ '13px', '18px', '500' ], statValue: [ '24px', '28px', '600' ],
  };
  for (const [ name, [ fontSize, lineHeight, fontWeight ] ] of Object.entries(scale)) {
    expect(atPath(textStyles, `${ name }.value`), name).toMatchObject({ fontSize, lineHeight, fontWeight });
  }
  for (const name of [ 'display', 'h1', 'h2', 'body', 'small', 'label' ]) {
    expect(atPath(textStyles, `${ name }.value.fontFamily`)).toBe('body');
  }
  expect(atPath(textStyles, 'display.value.letterSpacing')).toBe('-0.02em');
  expect(atPath(textStyles, 'h1.value.letterSpacing')).toBe('-0.01em');
  expect(atPath(textStyles, 'label.value')).toMatchObject({ textTransform: 'uppercase', letterSpacing: '0.08em', color: 'text.muted' });
  for (const name of [ 'number', 'statValue' ]) {
    expect(atPath(textStyles, `${ name }.value`)).toMatchObject({ fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' });
  }
  expect(atPath(fonts, 'body.value')).toBe(`${ BODY_TYPEFACE }, ${ FALLBACK_TYPEFACES }`);
  expect(atPath(fonts, 'heading.value')).toBe(atPath(fonts, 'body.value'));
  expect(atPath(fonts, 'mono.value')).toBe('"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace');
}

function expectedRecipeStyles(styles: SystemStyleObject) {
  return system.layers.wrap('recipes', system.css(styles));
}

function recipesResolveToTheVisualSystem() {
  for (const variant of [ 'line', 'scan' ] as const) {
    const table = system.sva(system.getSlotRecipe('table'))({ variant, size: 'md' });
    expect(table.columnHeader).toMatchObject(expectedRecipeStyles({ textStyle: 'label', height: '36px', py: 0, borderColor: 'hairline' }));
    expect(table.row).toMatchObject(expectedRecipeStyles({ height: '44px', bg: 'canvas', _hover: { bg: 'raised' } }));
    expect(table.cell).toMatchObject(expectedRecipeStyles({ textStyle: 'small', height: '44px', borderColor: 'hairline' }));
    expect(table.cell).toMatchObject(expectedRecipeStyles({
      '&[data-is-numeric=true], &[data-numeric]': { textAlign: 'right', textStyle: 'number' },
    }));
  }
  for (const variant of [ 'solid', 'secondary', 'pill', 'segmented' ] as const) {
    const tabs = system.sva(system.getSlotRecipe('tabs'))({ variant, size: 'md' });
    expect(tabs.trigger).toMatchObject(expectedRecipeStyles({
      borderRadius: 'none', borderBottomWidth: '2px', bg: 'transparent',
      _selected: { borderColor: 'accent', color: 'accent', bg: 'transparent' },
    }));
    expect(tabs.list).toMatchObject(expectedRecipeStyles({ flexWrap: 'nowrap', overflowX: 'auto' }));
    expect(tabs.content).toMatchObject(expectedRecipeStyles({ _horizontal: { pt: '24px' } }));
  }
  for (const size of [ '2xs', 'xs', 'sm', 'md' ] as const) {
    expect(system.cva(system.getRecipe('button'))({ variant: 'solid', size })).toMatchObject(expectedRecipeStyles({ borderRadius: 'control' }));
  }
  expect(system.cva(system.getRecipe('button'))({ variant: 'solid' })).toMatchObject(expectedRecipeStyles({
    bg: 'button.solid.bg', color: 'button.solid.text',
  }));
  expect(system.cva(system.getRecipe('button'))({ variant: 'outline' })).toMatchObject(expectedRecipeStyles({ borderWidth: '1px', borderColor: 'hairline' }));
  expect(system.cva(system.getRecipe('button'))({ variant: 'solid' })).toMatchObject(expectedRecipeStyles({
    _focusVisible: { outline: '2px solid', outlineColor: 'focusRing', outlineOffset: '2px' },
  }));
  for (const variant of [ 'solid', 'subtle', 'direction' ] as const) {
    expect(system.cva(system.getRecipe('badge'))({ variant })).toMatchObject(expectedRecipeStyles({ borderRadius: 'full' }));
  }
  for (const colorPalette of [ 'gray', 'green', 'red', 'orange', 'yellow', 'blue' ] as const) {
    expect(system.cva(system.getRecipe('badge'))({ colorPalette })).toMatchObject(expectedRecipeStyles({
      bg: `badge.${ colorPalette }.bg`, color: `badge.${ colorPalette }.fg`,
    }));
    expect(system.sva(system.getSlotRecipe('tag'))({ colorPalette, variant: 'subtle' }).root).toMatchObject(expectedRecipeStyles({
      borderRadius: 'full', bgColor: `badge.${ colorPalette }.bg`, color: `badge.${ colorPalette }.fg`,
    }));
  }
  for (const size of [ 'sm', 'md', 'lg' ] as const) {
    const stat = system.sva(system.getSlotRecipe('stat'))({ variant: 'scan', size });
    expect(stat.label).toMatchObject(expectedRecipeStyles({ textStyle: 'label' }));
    expect(stat.valueText).toMatchObject(expectedRecipeStyles({ textStyle: 'statValue' }));
    expect(stat.valueText).toMatchObject(expectedRecipeStyles({
      '& [data-delta=up]': { textStyle: 'number', color: 'stat.indicator.up', fontWeight: '500' },
      '& [data-delta=down]': { textStyle: 'number', color: 'stat.indicator.down', fontWeight: '500' },
    }));
    expect(stat.root).toMatchObject(expectedRecipeStyles({ borderRadius: 'card', p: '16px', borderColor: 'hairline', boxShadow: 'card' }));
  }
  expect(atPath(radii, 'control.value')).toBe('6px');
  expect(atPath(radii, 'card.value')).toBe('8px');
  expect(atPath(radii, 'full.value')).toBe('999px');
  expect(atPath(shadows, 'card.value')).toBe('none');
  expect(atPath(shadows, 'overlay.value')).toBe('0 0 0 1px {colors.hairline}');
}

function numericCellsAndMutedCountsReachTheComponents() {
  const content = createElement('div', null,
    createElement(TableRoot, null, createElement(TableBody, null, createElement(TableRow, null,
      createElement(TableCell, { isNumeric: true }, '125.00'), createElement(TableCell, null, 'PAX'),
    ))),
    createElement(TabsCounter, { count: 0 }), createElement(TabsCounter, { count: 12 }),
  );
  const fullMarkup = renderToStaticMarkup(createElement(ChakraProvider, { value: system, children: content }));
  expect(fullMarkup).toContain('data-emotion="css-global');
  expect(fullMarkup).toContain('--chakra-colors-black-alpha-400:');
  expect(fullMarkup).toContain('--chakra-colors-white-alpha-400:');
  // Check emitted component rules separately from the provider's complete palette declarations.
  const markup = fullMarkup.replace(/<style\s[^>]*data-emotion="css-global[^"]*"[^>]*>[\s\S]*?<\/style>/g, '');
  expect(markup).toContain('data-is-numeric="true"');
  expect(markup).toContain('125.00');
  expect(markup).toContain('var(--chakra-fonts-mono)');
  expect(markup).toContain('var(--chakra-colors-text-muted)');
  expect(markup).not.toContain('black-alpha-400');
  expect(markup).not.toContain('white-alpha-400');
}

test(allSemanticColoursResolveInBothModes, allSemanticColoursResolveInBothModes);
test(semanticRolesMatchDecision2587, semanticRolesMatchDecision2587);
test(legacyKeysInheritTheRoles, legacyKeysInheritTheRoles);
test(typeScaleAndFamiliesMatchTheBrief, typeScaleAndFamiliesMatchTheBrief);
test(recipesResolveToTheVisualSystem, recipesResolveToTheVisualSystem);
test(numericCellsAndMutedCountsReachTheComponents, numericCellsAndMutedCountsReachTheComponents);
