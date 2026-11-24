// @vitest-environment jsdom

import React from 'react';

import { render } from 'ui/shared/layout/testWrapper';
import { STATS_INTERVALS } from 'ui/stats/constants';
import { describe, expect, it, test, vi } from 'vitest';
import { screen } from 'vitest/lib';

import ChartIntervalSelect from './ChartIntervalSelect';

const shortTitles = Object.values(STATS_INTERVALS).map((entry) => entry.shortTitle);

test('chartIntervalControlFollowsTheDisplayedInterval', function chartIntervalControlFollowsTheDisplayedInterval() {
  const onIntervalChange = vi.fn();
  const view = render(<ChartIntervalSelect interval="oneMonth" onIntervalChange={ onIntervalChange }/>);
  const select = screen.getByRole('combobox', { name: 'Chart interval' });
  expect(select.textContent).toContain(STATS_INTERVALS.oneMonth.title);
  expect(screen.getByRole('button', { name: STATS_INTERVALS.oneMonth.shortTitle, pressed: true }).tagName).toBe('BUTTON');

  view.rerender(<ChartIntervalSelect interval="oneYear" onIntervalChange={ onIntervalChange }/>);
  expect(screen.getByRole('combobox', { name: 'Chart interval' }).textContent).toContain(STATS_INTERVALS.oneYear.title);
  expect(screen.getByRole('button', { name: STATS_INTERVALS.oneYear.shortTitle, pressed: true })).toBeDefined();
  expect(onIntervalChange).not.toHaveBeenCalled();
});

describe('ChartIntervalSelect', () => {
  it('offers every interval the statistics constants define, in their order', () => {
    const { container } = render(
      <ChartIntervalSelect interval="oneMonth" onIntervalChange={ vi.fn() }/>,
    );

    const tags = Array.from(container.querySelectorAll('[data-chart-interval-select] [data-id]'));

    expect(tags.map((tag) => tag.getAttribute('data-id'))).toEqual(Object.keys(STATS_INTERVALS));
    expect(tags.map((tag) => tag.textContent)).toEqual(shortTitles);
  });

  it('carries the interval it is showing', () => {
    const { container } = render(
      <ChartIntervalSelect interval="sixMonths" onIntervalChange={ vi.fn() }/>,
    );

    expect(container.querySelector('[data-chart-interval-select="sixMonths"]')).not.toBeNull();
  });

  it('hands the chosen interval back', () => {
    const onIntervalChange = vi.fn();
    const { container } = render(
      <ChartIntervalSelect interval="oneMonth" onIntervalChange={ onIntervalChange }/>,
    );

    (container.querySelector('[data-id="oneYear"]') as HTMLElement).click();

    expect(onIntervalChange).toHaveBeenCalledWith('oneYear');
  });

  it('keeps a narrow viewport on the select while the wide one takes the tags', () => {
    const { container } = render(
      <ChartIntervalSelect interval="oneMonth" onIntervalChange={ vi.fn() }/>,
    );

    const root = container.querySelector('[data-chart-interval-select]') as HTMLElement;

    expect(root.querySelector('[data-id]')).not.toBeNull();
    expect(root.querySelector('[role="combobox"], button[aria-haspopup]')).not.toBeNull();
  });
});
