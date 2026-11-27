import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from 'vitest';

const css = readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8');
const shellCss = readFileSync(join(process.cwd(), 'src/shell/shell.css'), 'utf8');
const supportedFloating = css.match(
  /@supports\s*\(backdrop-filter:\s*blur\(1rem\)\)\s*\{\s*:root\s*\{[^}]*\}\s*\.sl-floating,\s*\.sl-native-dialog\.sl-floating\s*\{[^}]*\}\s*\}/
);

function edgeLayerRuleIsOpaque() {
  const opaqueRules = [...css.matchAll(/\.sl-floating-opaque\s*\{([^}]*)\}/g)];
  expect(opaqueRules).toHaveLength(1);
  const opaque = opaqueRules[0];
  expect(opaque[1]).toMatch(/\bbackground:\s*var\(--sl-surface-2\)\s*;/);
  expect(opaque[1]).toMatch(/\bbackdrop-filter:\s*none\s*;/);
  expect(supportedFloating).not.toBeNull();
  expect(opaque.index).toBeGreaterThanOrEqual(
    supportedFloating!.index! + supportedFloating![0].length
  );
}

function barOfDestinationsIsOpaque() {
  const rules = [...shellCss.matchAll(/\.sl-mobile-destinations\s*\{([^}]*)\}/g)];
  expect(rules).toHaveLength(1);
  expect(rules[0][1]).toMatch(/\bbackground:\s*var\(--sl-surface-1\)\s*;/);
  expect(shellCss).not.toMatch(/backdrop-filter|color-mix/);
}

function edgeLayersCarryBothClasses() {
  const layers = [
    ['ui/shared/cookie-banner.tsx', 'data-cookie-banner'],
    ['widgets/trading/mobile-trade-drawer.tsx', 'data-mobile-trade-cta'],
    ['widgets/create/create-wizard.tsx', 'data-create-action'],
    ['widgets/create-chain/chain-wizard.tsx', 'data-chain-action'],
  ];
  for (const [file, attribute] of layers) {
    const source = readFileSync(join(process.cwd(), 'src', file), 'utf8');
    const openingTags = [
      ...source.matchAll(new RegExp(`<div\\b[^>]*\\b${attribute}(?=[\\s=>])[^>]*>`, 'g')),
    ];
    expect(openingTags, file).toHaveLength(1);
    const className = openingTags[0][0].match(/className=(?:"([^"]*)"|\{`([^`]*)`\})/);
    expect(className, file).not.toBeNull();
    const classes = (className![1] ?? className![2]).split(/\s+/);
    expect(classes, file).toEqual(expect.arrayContaining(['sl-floating', 'sl-floating-opaque']));
  }
}

function otherFloatingLayersStayAsTheyAre() {
  expect(css).toMatch(
    /--sl-floating-background:\s*color-mix\(in srgb, var\(--sl-surface-2\) 82%, transparent\)\s*;/
  );
  expect(supportedFloating).not.toBeNull();
  expect(supportedFloating![0]).toMatch(
    /--sl-floating-layer-background:\s*var\(--sl-floating-background\)\s*;/
  );
  expect(supportedFloating![0]).toMatch(
    /\.sl-floating,\s*\.sl-native-dialog\.sl-floating\s*\{\s*background:\s*var\(--sl-floating-background\)\s*;\s*backdrop-filter:\s*blur\(var\(--sl-floating-blur\)\)\s*;/
  );
  expect(css).toMatch(
    /\.sl-floating,\s*\.sl-native-dialog\.sl-floating\s*\{\s*background:\s*var\(--sl-surface-2\)\s*;\s*border:\s*var\(--sl-edge-width\) solid var\(--sl-surface-3\)\s*;\s*box-shadow:\s*var\(--sl-shadow-pop\)\s*;/
  );
  const source = readFileSync(join(process.cwd(), 'src/shell/mobile-bottom-nav.tsx'), 'utf8');
  const sheet = source.match(/<ModalDialog\b[^>]*aria-label="More"[^>]*>/);
  expect(sheet).not.toBeNull();
  const className = sheet![0].match(/className="([^"]*)"/);
  expect(className).not.toBeNull();
  const classes = className![1].split(/\s+/);
  expect(classes).toContain('sl-floating');
  expect(classes).not.toContain('sl-floating-opaque');
}

test(edgeLayerRuleIsOpaque.name, edgeLayerRuleIsOpaque);
test(barOfDestinationsIsOpaque.name, barOfDestinationsIsOpaque);
test(edgeLayersCarryBothClasses.name, edgeLayersCarryBothClasses);
test(otherFloatingLayersStayAsTheyAre.name, otherFloatingLayersStayAsTheyAre);
