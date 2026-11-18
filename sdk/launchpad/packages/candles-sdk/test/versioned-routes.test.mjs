import assert from 'node:assert/strict';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { CandlesClient, CandlesHttpError, UdfResponseError, parseHistory, parseSymbol } from '../dist/index.js';

const wire = JSON.parse(await readFile(new URL('../../../../../apps/launchpad/web/src/core/test-fixtures/market-production/charts-wire.json', import.meta.url), 'utf8'));

async function productionRoutesPreserveHandlerDocuments() {
  const names = ['config', 'symbols', 'time', 'history', 'valuationHistory', 'unknownSymbol', 'statsDaily', 'statsHourly', 'statsFiveMinute'];
  let index = 0;
  const failures = [];
  const server = createServer((request, response) => {
    const exchange = wire[names[index++]];
    try {
      assert.ok(exchange);
      assert.equal(request.method, 'GET');
      const actual = new URL(request.url, 'http://localhost');
      const expected = new URL(exchange.path, 'http://localhost');
      assert.equal(actual.pathname, expected.pathname);
      assert.deepEqual([...actual.searchParams].sort(), [...expected.searchParams].sort());
      assert.equal(request.headers.accept, 'application/json');
      assert.equal(request.headers.cookie, undefined);
      assert.equal(request.headers.authorization, undefined);
      response.writeHead(exchange.status, exchange.headers);
      response.end(exchange.bodyBytes);
    } catch (error) {
      failures.push(error);
      response.writeHead(500);
      response.end();
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const client = new CandlesClient();
  client.configure({ baseUrl: `http://127.0.0.1:${server.address().port}/v2/udf/` });
  const parameters = name => {
    const query = new URL(wire[name].path, 'http://localhost').searchParams;
    return {
      symbol: query.get('symbol'), resolution: query.get('resolution'),
      from: Number(query.get('from')), to: Number(query.get('to')),
      ...(query.has('mode') ? { mode: query.get('mode') } : {}),
    };
  };
  try {
    assert.deepEqual(await client.getConfig(), JSON.parse(wire.config.bodyBytes));
    const symbol = await client.getSymbol(wire.provenance.pool);
    assert.deepEqual(symbol, JSON.parse(wire.symbols.bodyBytes));
    assert.equal(typeof symbol.metadataRevision, 'string');
    assert.equal(symbol.priceScale, symbol.pricescale);
    assert.equal(symbol.chainId, '125');
    assert.equal(await client.getTime(), JSON.parse(wire.time.bodyBytes));
    for (const name of ['history', 'valuationHistory']) {
      const history = await client.getHistory(parameters(name));
      assert.deepEqual(history, JSON.parse(wire[name].bodyBytes));
      assert.equal(typeof history.watermark.height, 'string');
      assert.equal(typeof history.watermark.projectionRevision, 'string');
      assert.equal(typeof history.watermark.blockHash, 'string');
      assert.equal(typeof history.watermark.observedAt, 'string');
      assert.equal(history.valuations[0].supply.length, 4);
      assert.equal(typeof history.valuations[0].supply[0].totalSupplyRaw, 'string');
      assert.equal(typeof history.valuations[0].supply[0].supplySourceBlock, 'string');
    }
    const unknown = new URL(wire.unknownSymbol.path, 'http://localhost').searchParams.get('symbol');
    for (const name of ['unknownSymbol', 'statsDaily', 'statsHourly', 'statsFiveMinute']) {
      await assert.rejects(name === 'unknownSymbol' ? client.getSymbol(unknown) : client.getHistory(parameters(name)), error => {
        assert.ok(error instanceof CandlesHttpError);
        assert.equal(error.status, wire[name].status);
        assert.deepEqual(error.response, JSON.parse(wire[name].bodyBytes));
        assert.equal(error.cause, undefined);
        return true;
      });
    }
    assert.equal(index, names.length);
    assert.deepEqual(failures, []);
  } finally {
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
  }
}
test(productionRoutesPreserveHandlerDocuments);

function versionedParsersRetainValidation() {
  const history = JSON.parse(wire.history.bodyBytes);
  for (const value of [
    { ...history, c: [] }, { ...history, h: [0] }, { ...history, t: [1.5] },
    { ...history, valuations: [] }, { ...history, mode: 'mcap' },
    { ...history, watermark: { ...history.watermark, height: 1 } },
    { ...history, valuations: [{ ...history.valuations[0], supply: [{ ...history.valuations[0].supply[0], totalSupplyRaw: 1000000000000000 }] }] },
  ]) assert.throws(() => parseHistory(value), UdfResponseError);
  const symbol = JSON.parse(wire.symbols.bodyBytes);
  assert.throws(() => parseSymbol({ ...symbol, baseDecimals: '6' }), UdfResponseError);
  assert.throws(() => parseSymbol({ ...symbol, metadataRevision: 1 }), UdfResponseError);
}
test(versionedParsersRetainValidation);
