// @vitest-environment jsdom

import React from 'react';

import { Provider } from 'toolkit/chakra/provider';
import theme from 'toolkit/theme/theme';
import { getStatValueStyles } from 'toolkit/utils/statValue';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render } from 'vitest/lib';

import ScanStatCard from './ScanStatCard';

// jsdom implements no CSS media queries, and the Chakra provider reads window.matchMedia on mount
const createMediaQueryList = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => undefined,
  removeListener: () => undefined,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
  dispatchEvent: () => false,
});

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', { writable: true, value: createMediaQueryList });
});

// the suite runs without vitest globals, so the testing library cannot register its own teardown
afterEach(cleanup);

const declarationsOf = (element: Element): string => {
  const css = [
    ...Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText)),
    ...Array.from(document.querySelectorAll('style')).map((style) => style.textContent),
  ]
    .join('').replace(/\s+/g, '');

  return Array.from(element.classList)
    .flatMap((name) => Array.from(css.matchAll(new RegExp(`\\.${ name }\\{([^}]*)\\}`, 'g'))).map((match) => match[1]))
    .join(';');
};

describe('ScanStatCard', () => {
  it('lets label text wrap within its tile', function statLabelWraps() {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Average block time" value="1.0s"/>
      </Provider>,
    );
    const label = container.querySelector('[data-label]') as HTMLElement;

    expect(declarationsOf(label)).toContain('white-space:normal');
    expect(declarationsOf(label)).not.toContain('white-space:nowrap');
    expect(declarationsOf(label)).toContain('min-width:0');
    expect(declarationsOf(label)).toContain('overflow-wrap:anywhere');
  });

  it('limits label text to two lines while keeping the hint on the first line', function statLabelClampsToTwoLines() {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Transactions over the last day" value="1,234" hint="Daily transactions" icon={ <svg aria-label="transactions"/> }/>
      </Provider>,
    );
    const label = container.querySelector('[data-label]') as HTMLElement;
    const labelRow = label.parentElement as HTMLElement;
    const hint = container.querySelector('button[aria-label="hint"]') as HTMLElement;

    expect(declarationsOf(label)).toContain('display:-webkit-box');
    expect(declarationsOf(label)).toContain('-webkit-line-clamp:2');
    expect(declarationsOf(label)).toContain('-webkit-box-orient:vertical');
    expect(declarationsOf(label)).toContain('overflow:hidden');
    expect(declarationsOf(labelRow)).toContain('min-height:fit-content');
    expect(declarationsOf(labelRow)).toContain('align-items:flex-start');
    expect(declarationsOf(hint.parentElement as HTMLElement)).toContain('height:var(--chakra-sizes-4)');
    expect(declarationsOf(hint.parentElement as HTMLElement)).toContain('flex-shrink:0');
    expect(labelRow.querySelector('svg[aria-label="transactions"]')).not.toBeNull();
  });

  it('keeps the full label in its title', function statLabelHasFullTitle() {
    const label = 'Transactions per second over the last twenty four hours';
    const { container } = render(
      <Provider>
        <ScanStatCard label={ label } value="12"/>
      </Provider>,
    );
    const text = container.querySelector('[data-label]');

    expect(text?.getAttribute('title')).toBe(label);
    expect(text?.textContent).toBe(label);
  });

  it('places the value at the end of the tile', function statValueAtTileEnd() {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Transactions per second" value="12"/>
        <ScanStatCard label="Blocks" value="42"/>
      </Provider>,
    );
    const cards = container.querySelectorAll('[data-scan-stat]');

    expect(cards).toHaveLength(2);
    for (const card of cards) {
      expect(declarationsOf(card)).toContain('height:100%');
      expect(declarationsOf(card.querySelector('[data-value]')?.parentElement as HTMLElement)).toContain('margin-top:auto');
    }
  });

  it.each([ false, true ])('keeps labels and values aligned with and without a hint when loading is %s', function statLabelsShareLineHeight(isLoading) {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Transactions over the last day" value="1,234" hint="Daily transactions" isLoading={ isLoading } secondary="12 TPS"/>
        <ScanStatCard label="Total transactions" value="5,678" isLoading={ isLoading }/>
      </Provider>,
    );

    const labels = Array.from(container.querySelectorAll('[data-label]')).map((label) => label.parentElement as HTMLElement);
    const heightSettings = labels.map((label) => declarationsOf(label).match(/(?:^|;)height:([^;]+)/)?.[1]);

    expect(labels).toHaveLength(2);
    expect(heightSettings).toEqual([ 'var(--chakra-sizes-4)', 'var(--chakra-sizes-4)' ]);
    for (const label of labels) {
      expect(declarationsOf(label)).toContain('flex-wrap:nowrap');
      expect(declarationsOf(label)).toContain('white-space:nowrap');
      expect(declarationsOf(label)).toContain('flex-shrink:0');
      expect(declarationsOf(label.querySelector('[data-label]') as Element)).toContain('text-overflow:ellipsis');
    }

    const hint = container.querySelector('button[aria-label="hint"]') as HTMLButtonElement;

    expect(declarationsOf(hint)).toContain('position:absolute');
    expect(declarationsOf(hint)).toContain('top:50%');
    expect(declarationsOf(hint)).toContain('transform:translateY(-50%)');
    expect(hint.hasAttribute('data-loading-skeleton')).toBe(isLoading);
    for (const value of container.querySelectorAll('[data-value]')) {
      expect(declarationsOf(value)).toContain('white-space:pre-wrap');
    }
    expect(container.querySelector('[data-secondary]')?.textContent).toBe('(12 TPS)');
    expect(declarationsOf(container.querySelector('[data-secondary]')?.parentElement as Element)).toContain('flex-wrap:wrap');
  });

  it.each([ false, true ])('keeps long figures within the card when loading is %s', function longStatValuesRetainTheirFullFigure(isLoading) {
    const figure = '9,074,405 PAX';
    const { container } = render(
      <Provider>
        <ScanStatCard label="Total transaction fee" value={ figure } isLoading={ isLoading }/>
      </Provider>,
    );
    const value = container.querySelector('[data-value]') as HTMLElement;

    expect(value.textContent).toBe(figure);
    expect(value.title).toBe(figure);
    expect(declarationsOf(value)).toContain('text-overflow:clip');
    expect(declarationsOf(value)).not.toContain('text-overflow:ellipsis');
    expect(declarationsOf(value)).toContain('white-space:pre-wrap');
    expect(declarationsOf(value)).toContain('overflow:visible');
    expect(declarationsOf(value)).not.toContain('overflow:hidden');
    expect(declarationsOf(value)).toContain('overflow-wrap:anywhere');
    expect(declarationsOf(value)).toContain('word-break:normal');
    expect(declarationsOf(value)).toContain('font-size:var(--chakra-font-sizes-body)');
    expect(declarationsOf(value)).toContain('max-width:100%');
    expect(declarationsOf(value)).toContain('min-width:0');
    expect(declarationsOf(value.parentElement as HTMLElement)).toContain('max-width:100%');
    expect(declarationsOf(value.parentElement as HTMLElement)).toContain('min-width:0');
    expect(declarationsOf(value.parentElement as HTMLElement)).toContain('container-type:inline-size');
    expect(value.querySelectorAll('wbr')).toHaveLength(2);
    expect(Array.from(value.querySelectorAll('wbr')).every(point => point.previousSibling?.textContent?.endsWith(','))).toBe(true);
    expect(Array.from(value.querySelectorAll('wbr')).map(point => window.getComputedStyle(point).display)).toEqual([ 'inline', 'inline' ]);
  });

  it('selects fitting steps for ten and twelve characters with the fixed size as its upper bound', function statValuesSelectFittingSteps() {
    const sizes = { body: 14, textMd: 16, headingMd: 18, textXl: 20, statValue: 24 };
    const widths = [ 120, 141, 180 ];
    for (const [ text, compact, regular ] of [
      [ '12,345,678', [ 24, 24, 24 ], [ 18, 20, 24 ] ],
      [ '1,234.56 PAX', [ 20, 20, 24 ], [ 16, 18, 24 ] ],
    ] as const) {
      const characters = Array.from(text).length;
      const styles = getStatValueStyles(characters);
      const { container } = render(<Provider><ScanStatCard label="Count" value={ text }/></Provider>);
      const value = container.querySelector('[data-value]') as HTMLElement;
      const css = Array.from(document.querySelectorAll('style')).map(style => style.textContent).join('');

      expect(characters).toBe(text === '12,345,678' ? 10 : 12);
      expect(value.textContent).toBe(text);
      expect(Object.keys(theme.css(styles)).filter(key => key.startsWith('@container'))).toEqual(Object.keys(styles).slice(1));
      expect(Object.values(styles).slice(1)).toEqual([
        { fontSize: 'textMd' }, { fontSize: 'headingMd' }, { fontSize: 'textXl' }, { fontSize: 'statValue' },
      ]);
      for (const query of Object.keys(styles).slice(1)) expect(css).toContain(query);
      for (const [ advance, expected ] of [ [ 8, compact ], [ 9.6328125, regular ] ] as const) {
        for (const rootScale of [ 1, 2 ]) {
          const selected = widths.map(width => {
            let token: keyof typeof sizes = 'body';
            for (const [ query, rule ] of Object.entries(styles).slice(1)) {
              const minimum = Number(query.match(/min-width: ([\d.]+)ch/)![1]) * advance * rootScale;
              if (width * rootScale >= minimum) token = (rule as { fontSize: keyof typeof sizes }).fontSize;
            }
            return sizes[token];
          });
          expect(selected).toEqual(expected);
          expect(selected.every(size => size >= 14 && size <= 24)).toBe(true);
        }
      }
    }
  });

  it('counts units in nodes and retains their text when the value changes', function statNodeValuesIncludeUnits() {
    const { container, rerender } = render(
      <Provider><ScanStatCard label="Fee" value={ <span data-delta="up">1,234.56 PAX</span> }/></Provider>,
    );
    const value = container.querySelector('[data-value]') as HTMLElement;
    const css = () => Array.from(document.querySelectorAll('style')).map(style => style.textContent).join('');

    expect(value.textContent).toBe('1,234.56 PAX');
    expect(css()).toContain('@container (min-width: 18ch)');
    expect(css()).toContain('font-size:inherit');
    rerender(<Provider><ScanStatCard label="Fee" value={ <span>1 PAX</span> }/></Provider>);
    expect(value.textContent).toBe('1 PAX');
    expect(css()).toContain('@container (min-width: 7.5ch)');
  });

  it('puts the label above the value', () => {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Transactions" value="1,234,567"/>
      </Provider>,
    );

    const card = container.querySelector('[data-scan-stat="Transactions"]');

    expect(card).not.toBeNull();
    expect(card?.querySelector('[data-label]')?.textContent).toBe('Transactions');
    expect(card?.querySelector('[data-value]')?.textContent).toBe('1,234,567');
  });

  it('shows the secondary figure and the delta in parentheses after the value', () => {
    const { container } = render(
      <Provider>
        <ScanStatCard
          label="Transactions"
          value="1,234,567"
          secondary="12.5 TPS"
          delta={{ value: '3.2%', direction: 'up' }}
        />
      </Provider>,
    );

    expect(container.querySelector('[data-secondary]')?.textContent).toBe('(12.5 TPS)');
    expect(container.querySelector('[data-delta="up"]')?.textContent).toBe('(3.2%)');
  });

  it('marks a falling delta apart from a rising one', () => {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Median gas price" value="0.1 PAX" delta={{ value: '1.4%', direction: 'down' }}/>
      </Provider>,
    );

    expect(container.querySelector('[data-delta="down"]')).not.toBeNull();
    expect(container.querySelector('[data-delta="up"]')).toBeNull();
  });

  it('leaves out the secondary text and the delta when neither is given', () => {
    const { container } = render(
      <Provider>
        <ScanStatCard label="Total blocks" value="42"/>
      </Provider>,
    );

    expect(container.querySelector('[data-secondary]')).toBeNull();
    expect(container.querySelector('[data-delta]')).toBeNull();
  });
});
