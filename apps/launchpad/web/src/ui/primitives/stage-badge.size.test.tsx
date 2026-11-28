import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { JSDOM } from 'jsdom';
import { expect, test } from 'vitest';
import { StageBadge, stageLabel, type Stage } from './badges';

const css = readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8');

function touchRule() {
  const match = css.match(/^@media \(max-width: 834px\) \{([\s\S]*?)^\}/m);
  expect(match).not.toBeNull();
  const rule = match![1];
  const selectors = rule.match(/:where\(([\s\S]*?)\)\s*\{/);
  expect(selectors).not.toBeNull();
  return { rule, entries: selectors![1].split(',').map(entry => entry.trim()) };
}

function stageBadgeCarriesTheSizeMarker() {
  const stage: Stage = 'curve';
  for (const decorative of [false, true]) {
    const markup = renderToStaticMarkup(<StageBadge stage={stage} decorative={decorative} />);
    const badge = JSDOM.fragment(markup).firstElementChild;
    expect(badge).not.toBeNull();
    expect(badge!.getAttribute('data-keeps-size')).toBe('');
    expect(badge!.getAttribute('title')).toBe(stageLabel(stage));
    if (decorative) {
      expect(badge!.getAttribute('aria-hidden')).toBe('true');
      expect(badge!.hasAttribute('tabindex')).toBe(false);
      expect(badge!.hasAttribute('role')).toBe(false);
      expect(badge!.hasAttribute('aria-label')).toBe(false);
    } else {
      expect(badge!.getAttribute('tabindex')).toBe('0');
      expect(badge!.getAttribute('role')).toBe('img');
      expect(badge!.getAttribute('aria-label')).toBe(stageLabel(stage));
    }
  }
}

function touchRuleLeavesTheMarkedElementOut() {
  const { entries } = touchRule();
  expect(entries).toContain("[tabindex='0']:not([data-keeps-size])");
  expect(entries).not.toContain("[tabindex='0']");
  expect(entries.filter(entry => entry.includes('[tabindex='))).toEqual([
    "[tabindex='0']:not([data-keeps-size])",
  ]);
}

function touchRuleKeepsItsOtherEntries() {
  const { rule, entries } = touchRule();
  expect(entries.filter(entry => !entry.includes('[tabindex='))).toEqual([
    'button',
    "[role='button']",
    'summary',
    'select',
    "input:not([type='hidden'])",
    'textarea',
    "[role='tab']",
    "[role='menuitem']",
    "[role='menuitemradio']",
    'nav a',
    "section[aria-label='Market ticker'] a",
    'a[href]:not(p a)',
  ]);
  expect(rule).toMatch(/min-block-size:\s*var\(--sl-touch-target-min\);/);
  expect(rule).toMatch(/min-inline-size:\s*var\(--sl-touch-target-min\);/);
}

for (const check of [
  stageBadgeCarriesTheSizeMarker,
  touchRuleLeavesTheMarkedElementOut,
  touchRuleKeepsItsOtherEntries,
]) test(check.name, check);
