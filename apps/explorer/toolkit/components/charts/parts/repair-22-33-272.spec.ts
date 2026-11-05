import { parse } from 'papaparse';

import type { TimeChartData, TimeChartItem } from '../types';

import { expect, test } from 'vitest';

import { createChartCsv } from './ChartMenu';

function point(date: string, value: number, dateLabel?: string, dateTo?: string): TimeChartItem {
  return { date: new Date(date), value, dateLabel, date_to: dateTo === undefined ? undefined : new Date(dateTo) };
}

async function readExport(charts: TimeChartData) {
  const blob = createChartCsv(charts);
  expect(blob.type).toBe('text/csv;charset=utf-8;');
  const result = parse<Array<string>>(await blob.text(), { delimiter: ',' });
  expect(result.errors).toEqual([]);
  return result.data;
}

test('chartCsvUnequalLengths', async function chartCsvUnequalLengths() {
  const charts: TimeChartData = [
    { id: 'fees', name: 'Fees, "paid"', units: 'PAX', charts: [], items: [
      point('2026-01-03', 3), point('2026-01-01', 0, 'Opening, "day"\nUTC'), point('2026-01-02', 2),
    ] },
    { id: 'gas', name: 'Gas', units: 'Gwei', charts: [], items: [ point('2026-01-02', 0.125) ] },
    { id: 'empty', name: 'Pending', charts: [], items: [] },
  ];

  expect(await readExport(charts)).toEqual([
    [ 'Date', 'Fees, "paid" (PAX)', 'Gas (Gwei)', 'Pending' ],
    [ 'Opening, "day"\nUTC', '0', '', '' ],
    [ '2026-01-02', '2', '0.125', '' ],
    [ '2026-01-03', '3', '', '' ],
  ]);
});

test('chartCsvDifferentSamplingDates', async function chartCsvDifferentSamplingDates() {
  expect(await readExport([
    { id: 'a', name: 'A', charts: [], items: [ point('2026-01-03', 30), point('2026-01-01', 10) ] },
    { id: 'b', name: 'B', charts: [], items: [ point('2026-01-04', 400), point('2026-01-02', 200, 'Second day') ] },
  ])).toEqual([
    [ 'Date', 'A', 'B' ],
    [ '2026-01-01', '10', '' ],
    [ 'Second day', '', '200' ],
    [ '2026-01-03', '30', '' ],
    [ '2026-01-04', '', '400' ],
  ]);
});

test('chartCsvDateRanges', async function chartCsvDateRanges() {
  expect(await readExport([
    { id: 'a', name: 'A', charts: [], items: [
      point('2026-01-01', 1),
      point('2026-01-01', 7, undefined, '2026-01-07'),
      point('2026-01-01', 31, undefined, '2026-01-31'),
    ] },
    { id: 'b', name: 'B', charts: [], items: [
      point('2025-12-31T19:00:00-05:00', 70, 'First week', '2026-01-07'),
      point('2026-01-01', 140, undefined, '2026-01-14'),
    ] },
  ])).toEqual([
    [ 'Date', 'A', 'B' ],
    [ '2026-01-01', '1', '' ],
    [ 'First week', '7', '70' ],
    [ '2026-01-01 - 2026-01-14', '', '140' ],
    [ '2026-01-01 - 2026-01-31', '31', '' ],
  ]);
});

test('chartCsvIntradayTimestamps', async function chartCsvIntradayTimestamps() {
  expect(await readExport([
    { id: 'a', name: 'A', charts: [], items: [ point('2026-01-01T01:00:00Z', 1) ] },
    { id: 'b', name: 'B', charts: [], items: [
      point('2025-12-31T20:00:00-05:00', 10), point('2026-01-01T02:00:00Z', 20),
    ] },
  ])).toEqual([
    [ 'Date', 'A', 'B' ],
    [ '2026-01-01T01:00:00.000Z', '1', '10' ],
    [ '2026-01-01T02:00:00.000Z', '', '20' ],
  ]);
});

test('chartCsvEmptySeries', async function chartCsvEmptySeries() {
  expect(await readExport([])).toEqual([ [ 'Date' ] ]);
  expect(await readExport([
    { id: 'a', name: 'A', charts: [], items: [] },
    { id: 'b', name: 'B', charts: [], items: [ point('2026-01-02', 2) ] },
  ])).toEqual([ [ 'Date', 'A', 'B' ], [ '2026-01-02', '', '2' ] ]);
});
