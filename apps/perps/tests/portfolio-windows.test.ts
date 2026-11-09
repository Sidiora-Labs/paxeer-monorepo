import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import type { DailyPnl } from '../src/hooks/perps/useDailyPnl';
import type { AssetsTimeRange } from '../src/widgets/portfolio/PortfolioAssetsChart';

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= 'https://perps-gateway.invalid';

const day = (date: string, pnl = '1000000000000000000', volume = '2000000000000000000', trades = 1): DailyPnl => ({ date, pnl, volume, trades });
const NOW = new Date(Date.UTC(2026, 9, 21, 23, 59));

test('windowsAreCalendarDaysOnUtcBoundaries', async () => {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  assert.equal(grid.windowStartKey('7D', NOW), '2026-10-15');
  assert.equal(grid.windowStartKey('7D', new Date(Date.UTC(2026, 9, 22, 0, 0))), '2026-10-16');
  assert.equal(grid.windowStartKey('30D', NOW), '2026-09-22');
  assert.equal(grid.windowStartKey('90D', NOW), '2026-07-24');
  const rows = [day('2026-06-01'), day('2026-09-20'), day('2026-09-21'), day('2026-10-13'), day('2026-10-14T00:00:00Z'), day('2026-10-21')];
  assert.deepEqual(grid.sliceByRange(rows, '7D', NOW).map(r => r.date), ['2026-10-21']);
  assert.deepEqual(grid.sliceByRange(rows, '30D', NOW).map(r => r.date), ['2026-10-13', '2026-10-14T00:00:00Z', '2026-10-21']);
  assert.equal(grid.sliceByRange(rows, '90D', NOW).length, 5);
  assert.equal(grid.sliceByRange(rows, 'ALL', NOW).length, 6);
});

// Boundary: an N-day window is the last N calendar UTC days including today.
// Clock fixed at 2026-10-21T23:59Z, so 7D is 2026-10-15..2026-10-21 (seven
// day keys) and 2026-10-14 is outside; one minute later, at 2026-10-22T00:00Z,
// today rolls over and 7D becomes 2026-10-16..2026-10-22.
test('sevenDayWindowIsSevenUtcDaysIncludingToday', async () => {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  const keys = Array.from({ length: 14 }, (_, i) => new Date(Date.UTC(2026, 9, 9 + i)).toISOString().slice(0, 10));
  assert.equal(keys[0], '2026-10-09');
  assert.equal(keys[13], '2026-10-22');
  const rows = keys.map(k => day(k));
  const week = grid.sliceByRange(rows.slice(0, 13), '7D', NOW).map(r => r.date);
  assert.deepEqual(week, ['2026-10-15', '2026-10-16', '2026-10-17', '2026-10-18', '2026-10-19', '2026-10-20', '2026-10-21']);
  const rolled = grid.sliceByRange(rows, '7D', new Date(Date.UTC(2026, 9, 22, 0, 0))).map(r => r.date);
  assert.deepEqual(rolled, ['2026-10-16', '2026-10-17', '2026-10-18', '2026-10-19', '2026-10-20', '2026-10-21', '2026-10-22']);
  assert.equal(grid.windowTotals(grid.sliceByRange(rows.slice(0, 13), '7D', NOW))!.trades, 7);
});

test('emptyWindowIsNoDataNotZero', async () => {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  const stale = [day('2026-01-01'), day('2026-01-02')];
  assert.deepEqual(grid.sliceByRange(stale, '7D', NOW), []);
  assert.equal(grid.windowTotals(grid.sliceByRange(stale, '7D', NOW)), null);
  assert.deepEqual(grid.buildSeries(stale, '7D', NOW), []);
  const html = renderToStaticMarkup(createElement(grid.PortfolioPerformanceGrid, { dailyPnl: [] }));
  assert.equal((html.match(/No data yet/g) ?? []).length, 4);
  assert.doesNotMatch(html, /\$0\.00/);
});

async function assetsCalendarWindowsMatchPerformance() {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  const { buildAssetSeries, PortfolioAssetsChart } = await import('../src/widgets/portfolio/PortfolioAssetsChart');
  const stale = [day('2026-01-02', '2000000000000000000'), day('2026-01-01')];
  assert.deepEqual(buildAssetSeries(stale, 100, '7D', NOW), []);
  const allStale = [
    { date: '01-01', value: 98 },
    { date: '01-02', value: 100 },
  ];
  assert.deepEqual(buildAssetSeries(stale, 100, 'ALL', NOW), allStale);
  assert.deepEqual(
    buildAssetSeries(stale, 100, 'ALL', NOW).map(point => point.date),
    grid.sliceByRange(stale, 'ALL', NOW).map(row => row.date.slice(5, 10)),
  );

  const midnight = new Date('2026-10-22T00:00:00Z');
  const cases: Array<{
    range: Exclude<AssetsTimeRange, 'ALL'>;
    before: string;
    expected: string[];
    rolled: string[];
  }> = [
    {
      range: '7D',
      before: '2026-10-14',
      expected: ['2026-10-15', '2026-10-16', '2026-10-21'],
      rolled: ['2026-10-16', '2026-10-21'],
    },
    {
      range: '30D',
      before: '2026-09-21',
      expected: ['2026-09-22', '2026-09-23', '2026-10-21'],
      rolled: ['2026-09-23', '2026-10-21'],
    },
    {
      range: '90D',
      before: '2026-07-23',
      expected: ['2026-07-24', '2026-07-25', '2026-10-21'],
      rolled: ['2026-07-25', '2026-10-21'],
    },
  ];
  for (const { range, before, expected, rolled } of cases) {
    const rows = [
      day('2026-10-21'),
      day(`${expected[1]}T00:00:00Z`),
      day(before, '1000000000000000000000'),
      day(`${expected[0]}T23:59:00Z`),
    ];
    const original = rows.map(row => ({ ...row }));
    const assets = buildAssetSeries(rows, 100, range, NOW);
    const performance = grid.sliceByRange(rows, range, NOW);
    assert.deepEqual(performance.map(row => row.date.slice(0, 10)), expected, range);
    assert.deepEqual(assets.map(point => point.date), expected.map(date => date.slice(5)), range);
    assert.deepEqual(assets.map(point => point.date), performance.map(row => row.date.slice(5, 10)), range);
    assert.deepEqual(assets.map(point => point.value), [98, 99, 100], range);

    const rolledAssets = buildAssetSeries(rows, 100, range, midnight);
    const rolledPerformance = grid.sliceByRange(rows, range, midnight);
    assert.deepEqual(rolledPerformance.map(row => row.date.slice(0, 10)), rolled, range);
    assert.deepEqual(rolledAssets.map(point => point.date), rolled.map(date => date.slice(5)), range);
    assert.deepEqual(rolledAssets.map(point => point.date), rolledPerformance.map(row => row.date.slice(5, 10)), range);
    assert.deepEqual(rolledAssets.map(point => point.value), [99, 100], range);
    assert.deepEqual(rows, original, 'window selection preserves the supplied history');
    assert.deepEqual(buildAssetSeries(stale, 100, range, NOW), [], range);
  }
  assert.deepEqual(buildAssetSeries(stale, 100, 'ALL', midnight), allStale);
  assert.deepEqual(buildAssetSeries([], 100, '7D', NOW), []);
  const today = new Date().toISOString().slice(0, 10);
  assert.deepEqual(buildAssetSeries([day(today)], 123.45, '7D'), [
    { date: today.slice(5), value: 123.45 },
  ]);

  const large = 10n ** 40n;
  const exactRows = [
    day('2026-10-21', large.toString()),
    day('2026-10-15', '0'),
    day('2026-10-20'),
    day('2026-10-17', (-large).toString()),
  ];
  const exactSeries = [
    { date: '10-15', value: 99 },
    { date: '10-17', value: 0 },
    { date: '10-20', value: 0 },
    { date: '10-21', value: 100 },
  ];
  for (const range of ['7D', '30D', '90D', 'ALL'] as const) {
    assert.deepEqual(buildAssetSeries(exactRows, 100, range, NOW), exactSeries, range);
  }

  const capped = renderToStaticMarkup(createElement(PortfolioAssetsChart, {
    dailyPnl: stale, totalVaultUsd: 100, historyCapped: true, skippedDays: 2, range: 'ALL',
  }));
  assert.match(capped, /<option value="ALL" selected="">Last 90 active days<\/option>/);
  assert.match(capped, /History limited to the last 90 active days/);
  assert.match(capped, /2 days skipped: incomplete values/);
  assert.match(capped, /PnL-only estimate/);
  assert.match(capped, /excludes deposits and withdrawals/);
  const uncapped = renderToStaticMarkup(createElement(PortfolioAssetsChart, {
    dailyPnl: stale, totalVaultUsd: 100, range: 'ALL',
  }));
  assert.match(uncapped, /<option value="ALL" selected="">ALL<\/option>/);
  assert.doesNotMatch(uncapped, /History limited|days skipped/);
  assert.match(uncapped, /PnL-only estimate/);
  assert.match(uncapped, /excludes deposits and withdrawals/);
  const skippedDay = renderToStaticMarkup(createElement(PortfolioAssetsChart, {
    dailyPnl: stale, totalVaultUsd: 100, skippedDays: 1, range: 'ALL',
  }));
  assert.match(skippedDay, /1 day skipped: incomplete values/);
}
test(assetsCalendarWindowsMatchPerformance);

test('bigintSumsDoNotDriftAbove2pow53Wei', async () => {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  const big = '9007199254740993';
  const rows = [day('2026-10-20', big, big, 2), day('2026-10-21', big, '1000e+15', 3)];
  const totals = grid.windowTotals(rows)!;
  assert.equal(totals.pnlWei, 18014398509481986n);
  assert.equal(totals.volumeWei, 9007199254740993n + 1_000_000_000_000_000_000n);
  assert.equal(totals.trades, 5);
  assert.notEqual(BigInt(Number(big) + Number(big)), totals.pnlWei);
  const roi = grid.windowTotals([day('2026-10-21', '1000000000000000000', '3000000000000000000')])!;
  assert.equal(roi.roiHundredthsPct, 3333n);
  assert.equal(grid.windowTotals([day('2026-10-21', '5', '0')])!.roiHundredthsPct, null);
});

test('failureAndSkippedDaysAreShown', async () => {
  const grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
  const failed = renderToStaticMarkup(createElement(grid.PortfolioPerformanceGrid, { dailyPnl: [], error: new Error('down') }));
  assert.match(failed, /Performance history is unavailable/);
  assert.equal((failed.match(/Could not load data/g) ?? []).length, 4);
  assert.doesNotMatch(failed, /No data yet/);
  const skipped = renderToStaticMarkup(createElement(grid.PortfolioPerformanceGrid, { dailyPnl: [], skippedDays: 2 }));
  assert.match(skipped, /2 days skipped/);
});
