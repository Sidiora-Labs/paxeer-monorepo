// @vitest-environment jsdom

import React from 'react';

import type { TimeChartData } from 'toolkit/components/charts/types';

import { render } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import ChainIndicatorChartContent from './ChainIndicatorChartContent';

const data: TimeChartData = [ {
  id: 'daily_txs',
  name: 'Tx/day',
  charts: [ { type: 'area', gradient: { startColor: 'blue.100', stopColor: 'blue.500' } } ],
  items: Array.from({ length: 14 }, (item, index) => ({
    date: new Date(Date.UTC(2022, 10, index + 1)),
    value: 1000 + index * 10,
  })),
} ];

// jsdom implements <path> as SVGElement and leaves the geometry methods of SVGGeometryElement out,
// so the chart line animation of the toolkit has nothing to measure without this shim.
if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

describe('ChainIndicatorChartContent', () => {
  it('draws the series as a sparkline', () => {
    const { container } = render(<ChainIndicatorChartContent data={ data }/>);

    const svg = container.querySelector('[data-label="sparkline"]') as SVGSVGElement;

    expect(svg).not.toBeNull();
    expect(svg.querySelector('path')).not.toBeNull();
    expect(svg.getAttribute('role')).toBe('img');
    expect(svg.getAttribute('aria-label')).toBe('Tx/day');
    const description = document.getElementById(svg.getAttribute('aria-describedby')!);
    expect(description?.textContent).toContain(data[0].items[0].date.toLocaleDateString());
    expect(description?.textContent).toContain(data[0].items[0].value.toLocaleString());
    expect(description?.textContent).toContain(data[0].items[13].value.toLocaleString());
  });

  it('labels the sparkline on both axes', () => {
    const { container } = render(<ChainIndicatorChartContent data={ data }/>);

    expect(container.querySelector('[data-label="sparkline-y-axis"]')).not.toBeNull();
    expect(container.querySelector('[data-label="sparkline-x-axis"]')).not.toBeNull();
  });

  it('dates the ends of the series along the bottom axis', () => {
    const { container } = render(<ChainIndicatorChartContent data={ data }/>);

    const xAxis = container.querySelector('[data-label="sparkline-x-axis"]') as SVGGElement;

    expect(xAxis.querySelectorAll('text').length).toBeGreaterThan(0);
  });

  it('reserves the widest value label and its gap inside the chart', function reservesValueAxisRoom() {
    for (const value of [ 1, 12, 123, 1234 ]) {
      const samples: TimeChartData = [ {
        ...data[0],
        items: data[0].items.map((item, index) => ({ ...item, value: value * (1 + index / 100) })),
      } ];
      const { container, unmount } = render(<ChainIndicatorChartContent data={ samples }/>);
      const axis = container.querySelector('[data-label="sparkline-y-axis"]')!;
      const labels = Array.from(axis.querySelectorAll('text'));
      const widestLabel = Math.max(...labels.map((label) => label.textContent!.length * 8));
      const leftMargin = Number(axis.parentElement!.getAttribute('transform')!.match(/translate\(([^,]+)/)![1]);

      expect(labels.length).toBeGreaterThan(0);
      expect(leftMargin).toBe(widestLabel + 9);
      for (const label of labels) {
        expect(leftMargin + Number(label.getAttribute('x'))).toBeGreaterThanOrEqual(widestLabel);
      }
      unmount();
    }
  });

  it('anchors the end date labels towards the plot', function anchorsDateLabelsInward() {
    const { container } = render(<ChainIndicatorChartContent data={ data }/>);
    const labels = container.querySelectorAll('[data-label="sparkline-x-axis"] .tick text');

    expect(labels.length).toBeGreaterThan(1);
    expect(getComputedStyle(labels[0]).textAnchor).toBe('start');
    expect(getComputedStyle(labels[labels.length - 1]).textAnchor).toBe('end');
    expect(getComputedStyle(container.querySelector('svg')!.parentElement!).paddingRight).toBe('');
  });
});
