// Proves: operations.gen.ts operationDefinitions and Operations.call -> @kindle/sdk exported module / SdkClient native HTTP -> market operationsFor, fetchPlatformMetricsV2, fetchPoolDetailListing, resolvePoolSnapshot, loadSparkline, fetchStatsBatch and cups-contract readers.
// Repairs: Operations.call did not reject non-object inputs explicitly;
// pool-listings.fetchPoolDetailListing ignored the stats identity and quarantine;
// pool-resolver.resolvePoolSnapshot ignored the metadata envelope watermark;
// use-stats-batch.fetchStatsBatch ignored the envelope watermark;
// use-sparkline.loadSparkline compared canonical candle addresses with mixed-case input.
// The anchor owns no durable state. Sequential reads must reach the listener again.
// The loopback application owns its domain records and emits responses using the
// real SDK contract decoder. No SDK, transport, consumer or fetch is substituted.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { randomUUID } from 'node:crypto';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { registerHooks } from 'node:module';
import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { SdkClient, Operations, decodeV2Contract, CapabilityUnavailableError, routeStatus } from '../dist/index.js';
import {
  operationDefinitions, IF_ALERT_CREATE, IF_ALERT_DELETE, IF_CUP_CURRENT,
  IF_CUP_GET, IF_CUP_RESULTS, IF_CUP_STANDINGS, IF_ORDER_CANCEL,
} from '../dist/operations.gen.js';
import { ROUTE_CORRECTIONS } from '../dist/route-corrections.js';

const web = new URL('../../../../../apps/launchpad/web/src/', import.meta.url);
const sdkEntry = new URL('../dist/index.js', import.meta.url).href;

// Execute the actual web TypeScript with its existing @/ alias. This hook only
// resolves files and erases types/JSX; every imported implementation is unchanged.
function installWebLoader() {
  return registerHooks({
    resolve(specifier, context, next) {
      if (specifier === '@kindle/sdk') return { url: sdkEntry, shortCircuit: true };
      if (specifier.startsWith('@/') || (specifier.startsWith('.') && context.parentURL?.startsWith(web.href))) {
        const base = specifier.startsWith('@/') ? new URL(specifier.slice(2), web) : new URL(specifier, context.parentURL);
        for (const suffix of ['', '.ts', '.tsx', '/index.ts', '/index.tsx']) {
          const candidate = base.href + suffix;
          if (existsSync(fileURLToPath(candidate)) && /\.(?:ts|tsx|json)$/.test(candidate)) return { url: candidate, shortCircuit: true };
        }
      }
      return next(specifier, context);
    },
    load(url, context, next) {
      if (url.startsWith(web.href) && /\.(?:ts|tsx)$/.test(url)) {
        const source = ts.transpileModule(readFileSync(fileURLToPath(url), 'utf8'), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText;
        return { format: 'module', source, shortCircuit: true };
      }
      return next(url, context);
    },
  });
}

const pool = '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd';
const token = '0x2222222222222222222222222222222222222222';
const wallet = '0x3333333333333333333333333333333333333333';
const otherPool = '0x4444444444444444444444444444444444444444';
const cupId = randomUUID();
const orderId = randomUUID();
const snapshotId = randomUUID();
const at = '2026-01-02T12:00:00.000Z';
const from = '2026-01-02T11:00:00.000Z';
const hash = '0x' + 'a'.repeat(64);
const position = { chainId: '125', height: '100', blockHash: hash, transactionHash: hash, transactionIndex: '0', logIndex: '0' };
const watermark = { chainId: '125', height: '100', blockHash: hash, integrity: 'healthy', observedAt: at, projectionRevision: '1' };
const price = {
  numerator: '2', denominator: '1',
  baseAsset: { kind: 'erc20', chainId: '125', address: token, currencyCode: null, decimals: 18 },
  quoteAsset: { kind: 'erc20', chainId: '125', address: wallet, currencyCode: null, decimals: 6 },
  source: { kind: 'post_swap_spot', amountBasis: 'effective_reserves', sourceEventId: null, position, providerId: null, observedAt: at, sourceReference: null },
};

function envelope(data) {
  return { data, requestId: randomUUID(), generatedAt: at, quality: 'complete', watermark: { ...watermark }, revision: '1', nextCursor: null };
}

// The application derives every response from owned records, including mutations
// made by the decoded SDK requests. Responses are validated before serialization.
class LaunchpadApplication {
  requests = [];
  failure = null;
  fault = null;
  alerts = new Map();
  generation = 1;
  order = { orderId, chainOrderId: '1', inputRaw: '7', kind: 'limit', poolAddress: pool, status: 'active', triggerPrice: price, wallet };
  cup = { cupId, name: 'SDK cup', startsAt: from, endsAt: at, mode: 'solo', scoring: 'pnl', status: 'finished', prizePoolRaw: '10', prizeLadder: [{ rank: 1, amountRaw: '10' }], rulesMarkdown: 'Highest PnL wins.' };
  pool = { chainId: '125', poolAddress: pool, tokenAddress: token, creator: wallet, createdPosition: position, createdAt: from, effectivePricingUsdlReserveRaw: '2000000', tokenReserveRaw: '1000000000000000000', quoteAssetAddress: wallet, quoteAssetDecimals: 6, feeBps: 30, realUsdlBalanceRaw: '1000000', virtualUsdlReserveRaw: '1000000' };
  token = { address: token, name: 'Proof token', symbol: 'PROOF', decimals: 18, decimalsQuality: 'verified', logoAssetId: null, metadataRevision: '1', sourcePosition: position, supplyQuality: 'verified', totalSupplyRaw: '1000000000000000000' };
  stats() {
    return { poolAddress: pool, buys24h: '1', sells24h: '0', changeBaselineQuality: 'complete', circulatingMarketCapUsdlRaw: null, holderCount: '1', indicativeTvlUsdlRaw: null, priceChangeBps24h: null, realUsdlBalanceRaw: '1000000', sourceUpperPosition: position, supplyValuationQuality: 'verified', totalSupplyValuationUsdlRaw: null, valuationChangeUsdlRaw: null, valuationMethod: null, volume24hRaw: String(this.generation), watermark: { ...watermark }, windowStart: from, windowEnd: at };
  }
  candle() {
    return { poolAddress: pool, start: from, interval: '1h', open: price, high: price, low: price, close: { ...price, numerator: String(this.generation) }, coverage: 'complete', firstPosition: position, lastPosition: position, revision: '1', seriesModelVersion: 'proof-v1', snapshotId, synthetic: false, trades: '1', buyVolumeUsdlRaw: '1', sellVolumeUsdlRaw: '0', volumeTokenRaw: '1', volumeUsdlRaw: '1', largeTradeCount: null, largeTradePolicyId: null, largeTradeThresholdUsdlRaw: null, uniqueTraderCount: '1' };
  }
  async handle(req, res) {
    try {
      let text = '';
      for await (const chunk of req) text += chunk;
      const url = new URL(req.url, 'http://localhost');
      const request = { method: req.method, path: url.pathname, query: Object.fromEntries(url.searchParams), headers: req.headers, body: text ? JSON.parse(text) : undefined };
      this.requests.push(request);
      const match = Object.entries(operationDefinitions).find(([name, definition]) => {
        const route = ROUTE_CORRECTIONS[name] ?? definition.path;
        return definition.method === req.method && new RegExp('^' + route.replace(/\{[^}]+\}/g, '[^/]+') + '$').test(url.pathname);
      });
      assert.ok(match, `No real generated operation for ${req.method} ${url.pathname}`);
      const [name, definition] = match;
      request.operation = name;
      if (definition.authenticated) assert.equal(req.headers.authorization, 'Bearer proof-session');
      let data;
      switch (name) {
        case IF_ALERT_CREATE: {
          const body = decodeV2Contract('AlertCreateBody', request.body);
          assert.ok(req.headers['idempotency-key']);
          const alert = { ...body, alertId: randomUUID(), wallet, state: 'enabled' };
          this.alerts.set(alert.alertId, alert);
          data = { alert };
          break;
        }
        case IF_ALERT_DELETE: {
          const alertId = url.pathname.split('/').at(-1);
          assert.equal(this.alerts.delete(alertId), true);
          if (this.fault === name) { res.writeHead(200, { 'Content-Type': 'application/json' }); res.end('{}'); return; }
          res.writeHead(204); res.end(); return;
        }
        case IF_CUP_CURRENT:
        case IF_CUP_GET: data = { cup: { ...this.cup, name: `SDK cup ${this.generation}` } }; break;
        case IF_CUP_RESULTS: data = { cup: { ...this.cup, name: `SDK cup ${this.generation}` }, items: [{ rank: 1, wallet, metricSigned: String(this.generation), prizeRaw: '10' }] }; break;
        case IF_CUP_STANDINGS: data = { items: [{ rank: 1, wallet, metricSigned: String(this.generation) }] }; break;
        case IF_ORDER_CANCEL:
          assert.equal(url.pathname, `/v2/me/orders/${orderId}:cancel`);
          assert.ok(req.headers['idempotency-key']);
          this.order.status = 'cancelled'; data = { order: { ...this.order }, transactionRequest: null }; break;
        case 'IF_PLATFORM': data = { volumeUsdlRaw: String(this.generation), pools: '1', activeWallets: '1', newPools: '1', trades: '1', totalMarketCapUsdlRaw: '2' }; break;
        case 'IF_POOL_DETAIL': data = { pool: this.pool, token: this.token, stats: this.stats() }; break;
        case 'IF_METADATA_GET': data = { token: this.token, description: 'Owned by the application harness.', displayName: null, displaySymbol: null, revision: '1', socialLinks: [], tags: [] }; break;
        case 'IF_STATS_BATCH':
          assert.deepEqual(request.body, { poolAddresses: [pool] });
          data = { items: [this.stats()], missing: [] }; break;
        case 'IF_CANDLES':
          assert.deepEqual(request.query, { interval: '1h', from, to: at, limit: '24' });
          data = { items: [this.candle()], seriesModelVersion: 'proof-v1', snapshotId }; break;
        default: assert.fail(`Application has no producer for ${name}`);
      }
      const response = decodeV2Contract(definition.response, envelope(data));
      // Deliberate producer corruption crosses the same native HTTP boundary.
      if (this.fault === name) response.data = null;
      if (this.fault === 'stats-identity' && name === 'IF_POOL_DETAIL') response.data.stats.poolAddress = otherPool;
      if (this.fault === 'stats-quarantine' && name === 'IF_POOL_DETAIL') response.data.stats.watermark.integrity = 'quarantined';
      if (this.fault === 'metadata-quarantine' && name === 'IF_METADATA_GET') response.watermark.integrity = 'quarantined';
      if (this.fault === 'batch-quarantine' && name === 'IF_STATS_BATCH') response.watermark.integrity = 'quarantined';
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(response));
    } catch (error) {
      this.failure = error;
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ message: error.message }));
    }
  }
  count(name) { return this.requests.filter(request => request.operation === name).length; }
}

function config(origin, production = false) {
  const streamOrigin = origin.replace(/^http/, 'ws');
  return { apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production, session: { mode: 'bearer', token: () => 'proof-session' } };
}

async function runConsumerProof() {
  const hooks = installWebLoader();
  const load = path => import(new URL(path, web).href);
  const { operationsFor } = await load('core/market/operations.ts');
  const { fetchPlatformMetricsV2 } = await load('core/market/platform-metrics.ts');
  const { fetchPoolDetailListing } = await load('core/market/pool-listings.ts');
  const { resolvePoolSnapshot } = await load('core/market/pool-resolver.ts');
  const { loadSparkline } = await load('core/market/use-sparkline.ts');
  const { fetchStatsBatch, statsByPool } = await load('core/market/use-stats-batch.ts');
  const cups = await load('widgets/leaderboard/cups/cups-contract.ts');
  const app = new LaunchpadApplication();
  const server = createServer((req, res) => { void app.handle(req, res); });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const client = new SdkClient(config(`http://127.0.0.1:${server.address().port}`));
    const operations = operationsFor(client);
    assert.ok(operations instanceof Operations);
    assert.equal(operationsFor(client), operations);
    assert.notEqual(operationsFor(new SdkClient(client.config)), operations);
    const body = { poolAddress: pool, direction: 'crosses_above', cooldownSeconds: 300, threshold: price };
    const named = [IF_ALERT_CREATE, IF_ALERT_DELETE, IF_CUP_CURRENT, IF_CUP_GET, IF_CUP_RESULTS, IF_CUP_STANDINGS, IF_ORDER_CANCEL];
    for (const name of named) assert.equal(routeStatus(name), 'development');
    assert.equal(ROUTE_CORRECTIONS.IF_REWARD_CLAIM, '/v2/me/rewards/claim');

    // Every named producer runs twice; changing domain records makes a cached
    // result visible. HTTP counts prove the consumers did not take a shortcut.
    for (const generation of [1, 2]) {
      app.generation = generation;
      const created = await operations.call(IF_ALERT_CREATE, { body }, { headers: { 'Idempotency-Key': randomUUID() } });
      assert.deepEqual(created.data.alert.threshold, body.threshold);
      assert.equal(created.data.alert.poolAddress, pool);
      assert.equal(created.data.alert.wallet, wallet);
      assert.ok(app.alerts.has(created.data.alert.alertId));
      assert.equal(await operations.call(IF_ALERT_DELETE, { path: { alertId: created.data.alert.alertId } }), undefined);
      assert.equal(app.alerts.has(created.data.alert.alertId), false);
      assert.equal((await cups.fetchCurrentCup(operations)).name, `SDK cup ${generation}`);
      assert.equal((await cups.fetchCup(operations, cupId)).cupId, cupId);
      assert.equal((await cups.fetchCup(operations, cupId)).name, `SDK cup ${generation}`);
      assert.equal((await cups.fetchCupStandings(operations, cupId, null, 10)).items[0].metricSigned, String(generation));
      assert.equal((await cups.fetchCupResults(operations, cupId)).items[0].prizeRaw, '10');
      const cancelled = await operations.call(IF_ORDER_CANCEL, { path: { orderId } }, { headers: { 'Idempotency-Key': randomUUID() } });
      assert.equal(cancelled.data.order.status, 'cancelled');
      assert.equal(cancelled.data.order.orderId, orderId);
      const platform = await fetchPlatformMetricsV2(operations);
      assert.equal(platform.volumeUsdlRaw, String(generation));
      assert.equal(platform.window, '24h');
      const listing = await fetchPoolDetailListing(operations, pool);
      assert.deepEqual(listing, { poolAddress: pool, tokenAddress: token, creator: wallet, createdAtMs: Date.parse(from), spotPrice: '2' });
      const snapshot = await resolvePoolSnapshot(pool, { operations });
      assert.deepEqual(snapshot.pool, { chainId: '125', poolAddress: pool, tokenAddress: token });
      assert.equal(snapshot.stats.volume24hRaw, String(generation));
      assert.equal(snapshot.metadata.description, 'Owned by the application harness.');
      assert.deepEqual(snapshot.unavailable, { stats: null, metadata: null, registry: null });
      const sparkline = await loadSparkline(operations, '0x' + pool.slice(2).toUpperCase(), { from, to: at });
      assert.deepEqual(sparkline.closes, [generation]);
      assert.deepEqual(sparkline.starts, [from]);
      const batch = await fetchStatsBatch(operations, [pool, pool]);
      assert.equal(statsByPool(batch).get(pool).volume24hRaw, String(generation));
    }
    for (const name of named) assert.equal(app.count(name), name === IF_CUP_GET ? 4 : 2, name);
    for (const name of ['IF_PLATFORM', 'IF_METADATA_GET', 'IF_CANDLES', 'IF_STATS_BATCH']) assert.equal(app.count(name), 2, name);
    assert.equal(app.count('IF_POOL_DETAIL'), 4);
    assert.equal(app.requests.find(request => request.operation === 'IF_PLATFORM').query.window, '24h');

    const production = operationsFor(new SdkClient(config('https://api.example.com', true)));
    const before = app.requests.length;
    for (const name of named) await assert.rejects(production.call(name, {}), error => error instanceof CapabilityUnavailableError && error.message === `Operation ${name} is development-only`);
    assert.equal(app.requests.length, before);
    await assert.rejects(operations.call(IF_CUP_GET, { path: { cupId: 'unknown' } }), /Invalid IF_CUP_GET path/);
    await assert.rejects(operations.call(IF_CUP_CURRENT, null), /Invalid IF_CUP_CURRENT input/);
    assert.equal(app.requests.length, before);

    const malformedCalls = new Map([
      [IF_ALERT_CREATE, () => operations.call(IF_ALERT_CREATE, { body }, { headers: { 'Idempotency-Key': randomUUID() } })],
      [IF_CUP_CURRENT, () => cups.fetchCurrentCup(operations)],
      [IF_CUP_GET, () => cups.fetchCup(operations, cupId)],
      [IF_CUP_RESULTS, () => cups.fetchCupResults(operations, cupId)],
      [IF_CUP_STANDINGS, () => cups.fetchCupStandings(operations, cupId, null, 10)],
      [IF_ORDER_CANCEL, () => operations.call(IF_ORDER_CANCEL, { path: { orderId } }, { headers: { 'Idempotency-Key': randomUUID() } })],
      ['IF_PLATFORM', () => fetchPlatformMetricsV2(operations)],
      ['IF_POOL_DETAIL', () => fetchPoolDetailListing(operations, pool)],
      ['IF_METADATA_GET', () => resolvePoolSnapshot(pool, { operations })],
      ['IF_CANDLES', () => loadSparkline(operations, pool, { from, to: at })],
      ['IF_STATS_BATCH', () => fetchStatsBatch(operations, [pool])],
    ]);
    for (const [name, call] of malformedCalls) {
      app.fault = name;
      const count = app.count(name);
      await assert.rejects(call(), error => error instanceof TypeError && error.message.includes('Invalid v2 contract'), name);
      assert.equal(app.count(name), count + 1, name);
    }
    app.fault = null;
    const disposable = await operations.call(IF_ALERT_CREATE, { body }, { headers: { 'Idempotency-Key': randomUUID() } });
    app.fault = IF_ALERT_DELETE;
    await assert.rejects(operations.call(IF_ALERT_DELETE, { path: { alertId: disposable.data.alert.alertId } }), /Expected empty operation response/);
    for (const [fault, call, message] of [
      ['stats-identity', () => fetchPoolDetailListing(operations, pool), /Pool identity disagreement/],
      ['stats-quarantine', () => fetchPoolDetailListing(operations, pool), /Pool integrity quarantined/],
      ['metadata-quarantine', () => resolvePoolSnapshot(pool, { operations }), /Metadata identity disagreement or integrity quarantine/],
      ['batch-quarantine', () => fetchStatsBatch(operations, [pool]), /Statistics integrity quarantined/],
    ]) {
      app.fault = fault;
      await assert.rejects(call(), message);
    }
    app.fault = null;
    assert.equal((await fetchPlatformMetricsV2(operations)).volumeUsdlRaw, '2');
    assert.equal(app.failure, null, app.failure?.stack);
    return { namedProducers: named.length + 1, marketConsumers: 6, requests: app.requests.length, malformedResults: malformedCalls.size + 1 };
  } finally {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
    hooks.deregister();
  }
}

async function proof_launchpad_sdk_sdk() {
  // A real subprocess imports the built SDK entry and the actual consumer modules.
  const { stdout } = await promisify(execFile)(process.execPath, [fileURLToPath(import.meta.url), '--consumer'], { timeout: 30000, maxBuffer: 1024 * 1024 });
  const observed = JSON.parse(stdout);
  assert.equal(observed.namedProducers, 8);
  assert.equal(observed.marketConsumers, 6);
  assert.equal(observed.malformedResults, 12);
  assert.ok(observed.requests >= 40);
}

if (process.argv.includes('--consumer')) {
  process.stdout.write(JSON.stringify(await runConsumerProof()));
} else {
  test(proof_launchpad_sdk_sdk);
}
