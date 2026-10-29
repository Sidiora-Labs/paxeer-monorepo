import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';

type DailyModule = typeof import('../src/hooks/perps/useDailyPnl');
type GridModule = typeof import('../src/widgets/portfolio/PortfolioPerformanceGrid');

const ADDRESS = '0x00000000000000000000000000000000000000a1';
let server: Server;
let daily: DailyModule;
let grid: GridModule;
let graphql: { status: number; body: unknown } = { status: 200, body: {} };
let rest: { status: number; body: unknown } = { status: 200, body: [] };
const requests: string[] = [];

before(async () => {
  server = createServer((req, res) => {
    requests.push(`${req.method} ${req.url}`);
    const reply = req.url === '/graphql' ? graphql : req.url === `/v2/user/${ADDRESS}/daily-pnl` ? rest : { status: 404, body: {} };
    req.resume();
    req.on('end', () => {
      res.writeHead(reply.status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(reply.body));
    });
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  daily = await import('../src/hooks/perps/useDailyPnl');
  grid = await import('../src/widgets/portfolio/PortfolioPerformanceGrid');
});

after(async () => {
  server.closeAllConnections();
  await new Promise(resolve => server.close(resolve));
});

const row = (date: string | null, pnl: string | null, volume: string | null, trades: number | null) => ({ date, pnl, volume, trades });

async function allTimeLabelDisclosesCappedHistory() {
  assert.equal(daily.DAILY_PNL_LIMIT, 90);
  const completeRows = Array.from({ length: daily.DAILY_PNL_LIMIT + 1 }, (_, index) =>
    row(new Date(Date.UTC(2026, 9, 28 - index)).toISOString().slice(0, 10), '1', '2', 1),
  );

  for (const count of [daily.DAILY_PNL_LIMIT, daily.DAILY_PNL_LIMIT - 1, daily.DAILY_PNL_LIMIT + 1]) {
    requests.length = 0;
    graphql = { status: 200, body: { data: { userDailyPnl: completeRows.slice(0, count) } } };
    const result = await daily.fetchDailyPnl(ADDRESS);
    const capped = count >= daily.DAILY_PNL_LIMIT;
    assert.equal(result.rows.length, count);
    assert.equal(result.skippedDays, 0);
    assert.equal(result.historyCapped, capped);
    assert.equal(
      grid.rangeLabelFor('ALL', result.historyCapped),
      capped ? 'Last 90 active days' : 'All-time',
    );
    if (capped) assert.notEqual(grid.rangeLabelFor('ALL', result.historyCapped), 'All-time');
    assert.deepEqual(requests, ['POST /graphql']);
  }

  graphql = {
    status: 200,
    body: { data: { userDailyPnl: [null, ...completeRows.slice(0, daily.DAILY_PNL_LIMIT - 1)] } },
  };
  const filtered = await daily.fetchDailyPnl(ADDRESS);
  assert.equal(filtered.rows.length, daily.DAILY_PNL_LIMIT - 1);
  assert.equal(filtered.skippedDays, 1);
  assert.equal(filtered.historyCapped, true);
  assert.equal(grid.rangeLabelFor('ALL', filtered.historyCapped), 'Last 90 active days');

  requests.length = 0;
  graphql = { status: 200, body: { data: { userDailyPnl: null } } };
  rest = { status: 200, body: { dailyPnl: completeRows } };
  const fallback = await daily.fetchDailyPnl(ADDRESS);
  assert.equal(fallback.rows.length, daily.DAILY_PNL_LIMIT + 1);
  assert.equal(fallback.historyCapped, false);
  assert.equal(grid.rangeLabelFor('ALL', fallback.historyCapped), 'All-time');
  assert.deepEqual(requests, ['POST /graphql', `GET /v2/user/${ADDRESS}/daily-pnl`]);

  for (const range of ['7D', '30D', '90D'] as const) {
    assert.equal(grid.rangeLabelFor(range, true), range);
    assert.equal(grid.rangeLabelFor(range, false), range);
  }
}

test(allTimeLabelDisclosesCappedHistory);

test('graphqlRowsAscendingWithoutNullDates', async () => {
  requests.length = 0;
  graphql = { status: 200, body: { data: { userDailyPnl: [
    row('2026-10-03', '3000000000000000000', '9000000000000000000', 3),
    row(null, '1', '1', 1),
    row('2026-10-01', '1000000000000000000', '4000000000000000000', 1),
    row('2026-10-02', '-2000000000000000000', '5000000000000000000', 2),
  ] } } };
  const { rows, skippedDays } = await daily.fetchDailyPnl(ADDRESS);
  assert.deepEqual(rows.map(r => r.date), ['2026-10-01', '2026-10-02', '2026-10-03']);
  assert.equal(skippedDays, 1);
  assert.deepEqual(requests, ['POST /graphql']);
});

test('restFallbackWhenGraphqlFails', async () => {
  requests.length = 0;
  graphql = { status: 500, body: { error: 'down' } };
  rest = { status: 200, body: { dailyPnl: [row('2026-10-05', '5', '6', 1), row('2026-10-04', '7', '8', 2)] } };
  const { rows } = await daily.fetchDailyPnl(ADDRESS);
  assert.deepEqual(rows.map(r => r.date), ['2026-10-04', '2026-10-05']);
  assert.equal(requests.at(-1), `GET /v2/user/${ADDRESS}/daily-pnl`);
});

test('nullNumericRowsDroppedAndCounted', async () => {
  requests.length = 0;
  graphql = { status: 200, body: { data: { userDailyPnl: [
    row('2026-10-01', null, '4', 1),
    row('2026-10-02', '2', null, 1),
    row('2026-10-03', '3', '9', null),
    row('2026-10-04', 'not-a-number', '9', 1),
    null,
    row('2026-10-05', '1000e+15', '-5e-3', 0),
  ] } } };
  const { rows, skippedDays } = await daily.fetchDailyPnl(ADDRESS);
  assert.deepEqual(rows.map(r => r.date), ['2026-10-05']);
  assert.equal(skippedDays, 5);
  assert.deepEqual(requests, ['POST /graphql']);
});

test('nullGraphqlListFallsBackToRest', async () => {
  requests.length = 0;
  graphql = { status: 200, body: { data: { userDailyPnl: null } } };
  rest = { status: 200, body: [row('2026-10-06', '1', '2', 1)] };
  const { rows, skippedDays } = await daily.fetchDailyPnl(ADDRESS);
  assert.deepEqual(rows.map(r => r.date), ['2026-10-06']);
  assert.equal(skippedDays, 0);
  assert.deepEqual(requests, ['POST /graphql', `GET /v2/user/${ADDRESS}/daily-pnl`]);
});

test('nullListFromBothSourcesRejects', async () => {
  graphql = { status: 200, body: { data: { userDailyPnl: null } } };
  rest = { status: 200, body: { dailyPnl: null } };
  await assert.rejects(daily.fetchDailyPnl(ADDRESS), /REST returned no daily PnL list/);
});

test('rejectsWhenBothFail', async () => {
  graphql = { status: 500, body: {} };
  rest = { status: 503, body: {} };
  await assert.rejects(daily.fetchDailyPnl(ADDRESS));
});

test('performanceFromDailyRowsAlone', () => {
  const rows = [
    row('2026-10-01', '1000000000000000000', '2000000000000000000', 1),
    row('2026-10-02', '-500000000000000000', '3000000000000000000', 2),
  ] as import('../src/hooks/perps/useDailyPnl').DailyPnl[];
  const now = new Date(Date.UTC(2026, 9, 3, 12));
  for (const range of ['7D', 'ALL'] as const) {
    assert.equal(grid.sliceByRange(rows, range, now).length, 2);
    const series = grid.buildSeries(rows, range, now);
    assert.equal(series.length, 2);
    assert.equal(series.reduce((sum, p) => sum + p.pnl, 0), 0.5);
    assert.equal(series.reduce((sum, p) => sum + p.volume, 0), 5);
    assert.equal(series.at(-1)!.cumPnl, 0.5);
    assert.equal(series.at(-1)!.cumVolume, 5);
  }
});
