// @vitest-environment jsdom

import React from 'react';

import type { TimeChartData } from 'toolkit/components/charts/types';

import { render } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import ChainIndicatorsChart from './ChainIndicatorsChart';
import { getChartData } from './utils/chart';

const buildData = (days: number): TimeChartData => ([ {
  id: 'daily_txs',
  name: 'Tx/day',
  charts: [ { type: 'area', gradient: { startColor: 'blue.100', stopColor: 'blue.500' } } ],
  items: Array.from({ length: days }, (item, index) => ({
    date: new Date(Date.UTC(2022, 10, index + 1)),
    value: 1000 + index,
  })),
} ]);

// jsdom implements <path> as SVGElement and leaves the geometry methods of SVGGeometryElement out,
// so the chart line animation of the toolkit has nothing to measure without this shim.
if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

describe('ChainIndicatorsChart', () => {
  it('titles the sparkline and draws it', () => {
    const { container } = render(
      <ChainIndicatorsChart
        isLoading={ false }
        title="Blockscout transaction history in 14 days"
        chartQuery={{ isError: false, isPending: false, data: buildData(30) }}
      />,
    );

    const chart = container.querySelector('[data-label="chain-indicator-chart"]') as HTMLElement;

    expect(chart.querySelector('[data-title]')?.textContent).toBe('Blockscout transaction history in 14 days');
    expect(chart.querySelector('[data-label="sparkline"]')).not.toBeNull();
  });

  it('renders the calendar-day window prepared by the producer', () => {
    const samples = Array.from({ length: 30 }, (_, index) => ({
      date: new Date(Date.UTC(2026, 0, index + 1)),
      value: index === 28 ? null : index + 1,
    }));
    const data = getChartData('daily_txs', samples, 14);
    const { container } = render(
      <ChainIndicatorsChart
        isLoading={ false }
        title="Blockscout transaction history in 14 days"
        chartQuery={{ isError: false, isPending: false, data }}
      />,
    );

    expect(container.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('13');
    expect(container.querySelector('[data-label="sparkline"]')).not.toBeNull();
    expect(data[0].items.map(({ date, value }) => [ date.toISOString().slice(0, 10), value ])).toEqual([
      [ '2026-01-17', 17 ], [ '2026-01-18', 18 ], [ '2026-01-19', 19 ], [ '2026-01-20', 20 ],
      [ '2026-01-21', 21 ], [ '2026-01-22', 22 ], [ '2026-01-23', 23 ], [ '2026-01-24', 24 ],
      [ '2026-01-25', 25 ], [ '2026-01-26', 26 ], [ '2026-01-27', 27 ], [ '2026-01-28', 28 ], [ '2026-01-30', 30 ],
    ]);
    expect(samples).toHaveLength(30);
  });

  it('falls back to a notice when the series cannot be fetched', () => {
    const { container } = render(
      <ChainIndicatorsChart
        isLoading={ false }
        title="Blockscout transaction history in 14 days"
        chartQuery={{ isError: true, isPending: false, data: [] }}
      />,
    );

    expect(container.querySelector('[data-label="sparkline"]')).toBeNull();
  });
});
