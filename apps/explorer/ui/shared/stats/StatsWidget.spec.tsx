// @vitest-environment jsdom

import React from 'react';

import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';

import StatsWidget from './StatsWidget';

test('sizes counts to the tile without growing the text', function sizesCountsToTheTileWithoutGrowingTheText(): void {
  const { container } = render(<StatsWidget label="Total blocks" value="1,234,567,890"/>);
  const value = container.querySelector('[data-value] > span') as HTMLElement;
  const style = window.getComputedStyle(value);

  expect(value.textContent).toBe('1,234,567,890');
  expect(window.getComputedStyle(value.parentElement!.parentElement!).containerType).toBe('inline-size');
  expect(style.fontSize).toBe('var(--chakra-font-sizes-stat-value)');
  expect(style.maxWidth).toBe('100%');
  expect(style.minWidth).toBe('0');
  expect(style.whiteSpace).toBe('nowrap');
  expect(style.overflow).toBe('hidden');
  expect(style.textOverflow).toBe('ellipsis');
  expect(style.wordBreak).toBe('normal');
});

test('keeps complete counts and separator nodes at the fixed size', function keepsCompleteCountsAndSeparatorNodesAtTheFixedSize(): void {
  const count = '123,456,789,012,345,678,901,234,567,890';
  const { container } = render(<StatsWidget label="Total transactions" value={ count }/>);
  const value = container.querySelector('[data-value] > span') as HTMLElement;

  expect(value.textContent).toBe(count);
  expect(value.querySelectorAll('wbr')).toHaveLength(9);
  expect(Array.from(value.querySelectorAll('wbr')).every(breakPoint => breakPoint.previousSibling?.textContent?.endsWith(','))).toBe(true);
  expect(Array.from(value.querySelectorAll('wbr')).map(breakPoint => window.getComputedStyle(breakPoint).display)).toEqual(Array.from({ length: 9 }, () => 'contents'));
  expect(window.getComputedStyle(value).overflowWrap).toBe('anywhere');
  expect(window.getComputedStyle(value).fontSize)
    .toBe('var(--chakra-font-sizes-stat-value)');
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
