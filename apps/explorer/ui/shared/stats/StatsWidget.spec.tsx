// @vitest-environment jsdom

import React from 'react';

import theme from 'toolkit/theme/theme';
import { getStatValueStyles } from 'toolkit/utils/statValue';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';

import StatsWidget from './StatsWidget';

test('sizes counts to the tile without growing the text', function sizesCountsToTheTileWithoutGrowingTheText(): void {
  const { container } = render(<StatsWidget label="Total blocks" value="1,234,567,890"/>);
  const value = container.querySelector('[data-value] > span') as HTMLElement;
  const style = window.getComputedStyle(value);

  expect(value.textContent).toBe('1,234,567,890');
  expect(window.getComputedStyle(value.parentElement!.parentElement!).containerType).toBe('inline-size');
  expect(style.fontSize).toBe('var(--chakra-font-sizes-body)');
  expect(style.maxWidth).toBe('100%');
  expect(style.minWidth).toBe('0');
  expect(style.whiteSpace).toBe('pre-wrap');
  expect(style.overflow).toBe('visible');
  expect(style.textOverflow).toBe('clip');
  expect(style.wordBreak).toBe('normal');
});

test('keeps complete counts and separator nodes at the least step', function keepsCompleteCountsAndSeparatorNodesAtTheLeastStep(): void {
  const count = '123,456,789,012,345,678,901,234,567,890';
  const { container } = render(<StatsWidget label="Total transactions" value={ count }/>);
  const value = container.querySelector('[data-value] > span') as HTMLElement;

  expect(value.textContent).toBe(count);
  expect(value.querySelectorAll('wbr')).toHaveLength(9);
  expect(Array.from(value.querySelectorAll('wbr')).every(breakPoint => breakPoint.previousSibling?.textContent?.endsWith(','))).toBe(true);
  expect(Array.from(value.querySelectorAll('wbr')).map(breakPoint => window.getComputedStyle(breakPoint).display))
    .toEqual(Array.from({ length: 9 }, () => 'inline'));
  expect(window.getComputedStyle(value).overflowWrap).toBe('anywhere');
  expect(window.getComputedStyle(value).fontSize)
    .toBe('var(--chakra-font-sizes-body)');
});

test('selects the largest fitting token for ten and thirteen characters', function selectsTheLargestFittingToken(): void {
  const sizes = { body: 14, textMd: 16, headingMd: 18, textXl: 20, statValue: 24 };
  for (const [ characters, expected ] of [ [ 10, [ 16, 20, 20, 24 ] ], [ 13, [ 14, 14, 16, 20 ] ] ] as const) {
    const styles = getStatValueStyles(characters);
    expect(styles.fontSize).toBe('body');
    expect(Object.keys(theme.css(styles)).filter(key => key.startsWith('@container'))).toEqual(Object.keys(styles).slice(1));
    expect(Object.values(styles).slice(1)).toEqual([
      { fontSize: 'textMd' }, { fontSize: 'headingMd' }, { fontSize: 'textXl' }, { fontSize: 'statValue' },
    ]);
    for (const rootSize of [ 16, 32 ]) {
      const selected = [ 102, 122, 137, 180 ].map((width) => {
        let token: keyof typeof sizes = 'body';
        for (const [ query, rule ] of Object.entries(styles).slice(1)) {
          const minimum = Number(query.match(/min-width: ([\d.]+)ch/)![1]) * 0.6 * rootSize;
          if (width * rootSize / 16 >= minimum) token = (rule as { fontSize: keyof typeof sizes }).fontSize;
        }
        return sizes[token];
      });
      expect(selected).toEqual(expected);
      expect(selected.every(size => size >= 14 && size <= 24)).toBe(true);
    }
  }
  expect(getStatValueStyles(10)).toEqual({
    fontSize: 'body',
    '@container (min-width: 10ch)': { fontSize: 'textMd' },
    '@container (min-width: 11.25ch)': { fontSize: 'headingMd' },
    '@container (min-width: 12.5ch)': { fontSize: 'textXl' },
    '@container (min-width: 15ch)': { fontSize: 'statValue' },
  });
});

test('counts the prefix and unit with the whole value', function countsThePrefixAndUnitWithTheWholeValue(): void {
  const { container } = render(<>
    <StatsWidget label="Price" value="1,234,567" valuePrefix="$" valuePostfix=" USD"/>
    <StatsWidget label="Price as text" value="$1,234,567 USD"/>
    <StatsWidget label="Price as nodes" value={ <span>1,234,567</span> } valuePrefix="$" valuePostfix=" USD"/>
  </>);
  const values = Array.from(container.querySelectorAll('[data-value] > span'));
  expect(values).toHaveLength(3);
  expect(values.map(value => value.textContent)).toEqual([ '$1,234,567 USD', '$1,234,567 USD', '$1,234,567 USD' ]);
  expect(new Set(values.map(value => value.className)).size).toBe(1);
  expect(values[0].querySelectorAll('wbr')).toHaveLength(2);
  expect(Array.from(values[0].querySelectorAll('span')).map(value => value.textContent)).toEqual([ '$', ' USD' ]);
});

test('preserves the label difference and period styles', function preservesTheLabelDifferenceAndPeriodStyles(): void {
  const { container } = render(<StatsWidget label="Total blocks" value="30,215,608" diff={ 4 } period="24h"/>);
  const value = container.querySelector('[data-value]') as HTMLElement;
  const label = container.querySelector('h2')!;
  const difference = Array.from(value.querySelectorAll('p')).find(element => element.textContent === '+4')!;

  expect(label.textContent).toBe('Total blocks');
  expect(window.getComputedStyle(label.parentElement!).fontSize).toBe('0.6875rem');
  expect(window.getComputedStyle(value).fontSize).toBe('1.5rem');
  expect(window.getComputedStyle(difference).fontSize).toBe('0.8125rem');
  expect(value.textContent).toBe('30,215,608+4(24h)(24h)');
});

test('keeps the card wrapper while its count loads', function keepsTheCardWrapperWhileItsCountLoads(): void {
  const { container, rerender } = render(<StatsWidget label="Total blocks" value="9,074,405" href={{ pathname: '/blocks' }} isLoading/>);
  const loadingCard = container.querySelector('[data-value]')!.parentElement!.parentElement!;
  const loadingWrapper = loadingCard.parentElement!;

  expect(loadingWrapper.tagName).toBe('SPAN');
  expect(loadingWrapper.hasAttribute('inert')).toBe(true);
  expect(container.querySelector('a')).toBeNull();
  const wrapperClass = loadingWrapper.className;
  const cardClass = loadingCard.className;

  rerender(<StatsWidget label="Total blocks" value="7,660,515" href={{ pathname: '/blocks' }}/>);
  const loadedCard = container.querySelector('[data-value]')!.parentElement!.parentElement!;
  const loadedWrapper = loadedCard.parentElement!;

  expect(loadedWrapper.tagName).toBe('A');
  expect(loadedWrapper.getAttribute('href')).toBe('/blocks');
  expect(loadedWrapper.hasAttribute('inert')).toBe(false);
  expect(loadedWrapper.className).toBe(wrapperClass);
  expect(loadedCard.className).toBe(cardClass);
});
