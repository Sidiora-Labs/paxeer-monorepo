import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import './repair-22-33-1047-loader.mjs';
import { runPythonProof } from './repair-22-33-1047-python.mjs';

const { operationDefinitions } = await import('../dist/operations.gen.js');
const { decodeV2Contract } = await import('../dist/index.js');
const root = new URL('../../../../../', import.meta.url);
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const json = url => JSON.parse(readFileSync(url, 'utf8'));
const recorded = new URL('services/launchpad/api/internal/rest/testdata/contracts/v2/openapi.json', root);

async function finding6VersionedGenerationInputsMatchImmutableReleasePins() {
  await runPythonProof(`
generation = runpy.run_path(str(suite / 'generation.test.py'))
generation['test_versioned_clean_generations']()
`);
  for (const major of [1, 2]) {
    const packageName = major === 1 ? 'sdk' : 'sdk-v2';
    const packageRoot = new URL(`sdk/launchpad/packages/${packageName}/`, root);
    const release = json(new URL('release-manifest.json', packageRoot));
    assert.equal(release.apiMajor, major);
    assert.equal(release.streamMajor, major);
    for (const [name, digest] of Object.entries(release.outputs)) {
      assert.equal(sha(readFileSync(new URL(`src/${name}`, packageRoot))), digest, name);
    }
  }
  for (const [name, digest] of Object.entries({
    'src/operations.gen.ts': 'b7763579cac0429a3bf507370fcf055b0839b642c9c9d33c8488791fe3fcd42b',
    'examples/pagination.mjs': '1337efb63a0bca9106c17f8ec1d5d603241653e012a41dca9238d0996712ca9b',
    'examples/snapshot-resume.mjs': 'fa170082780b19a7bf906ab921fa32bc763895330f5fb13b083adbd65f9facda',
  })) {
    assert.equal(sha(readFileSync(new URL(`../${name}`, import.meta.url))), digest, name);
  }
}

async function finding7SupportedBundlesHaveUsableSchemas() {
  await runPythonProof(`
layout = load_module(suite / 'test_generate_layout.py', 'sdk_generate_layout_proof')
layout.test_repository_root_is_monorepo()
layout.test_default_contracts_use_launchpad_spec_prefix()
layout.test_unsupported_major_is_explicitly_refused()
`);
  const document = json(recorded);
  assert.equal(document.openapi, '3.1.0');
  assert.equal(document.info.version, 'v2');
  assert.ok(Object.keys(document.components.schemas).length > 0);
  const { SdkClient, Operations } = await import('../dist/index.js');
  const examples = json(new URL('examples.json', recorded));
  const response = examples.endpoints.IF_STATS.successes[0];
  assert.equal(response.status, 200);
  assert.deepEqual(decodeV2Contract('StatsResponse', response.body), response.body);
  assert.throws(() => decodeV2Contract('StatsResponse', {
    ...response.body, data: { stats: { ...response.body.data.stats, poolAddress: 'invalid' } },
  }), { name: 'TypeError', message: /Invalid v2 contract StatsResponse/ });

  const requests = [];
  const server = createServer((request, reply) => {
    requests.push({ method: request.method, path: request.url });
    reply.writeHead(response.status, { 'Content-Type': 'application/json' });
    reply.end(JSON.stringify(response.body));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const streamOrigin = origin.replace(/^http/, 'ws');
    const api = new Operations(new SdkClient({
      apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin,
      approvedStreamOrigins: [streamOrigin], production: false,
    }));
    const poolAddress = response.body.data.stats.poolAddress;
    const consumed = await api.call('IF_STATS', { path: { poolAddress } });
    assert.deepEqual(requests, [{ method: 'GET', path: `/v2/pools/${poolAddress}/stats` }]);
    assert.deepEqual(consumed, response.body);
  } finally {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  }
}

async function finding8ExamplesReadTheAdmittedOperationCatalog() {
  await runPythonProof(`
layout = load_module(suite / 'test_examples_layout.py', 'sdk_examples_layout_proof')
with tempfile.TemporaryDirectory() as directory:
    layout.test_examples_paths_use_monorepo_layout(pathlib.Path(directory))
`);
  const document = json(recorded);
  const operations = {};
  for (const [path, methods] of Object.entries(document.paths)) {
    for (const [method, operation] of Object.entries(methods)) {
      if (['get', 'post', 'patch', 'put', 'delete'].includes(method)) {
        assert.ok(!Object.hasOwn(operations, operation.operationId), operation.operationId);
        operations[operation.operationId] = { method: method.toUpperCase(), path };
        assert.ok(Object.hasOwn(operationDefinitions, operation.operationId), operation.operationId);
      }
    }
  }
  for (const [name, expected] of Object.entries({
    IF_TRADES: { method: 'GET', path: '/v2/pools/{poolAddress}/trades' },
    IF_TOPIC_SNAPSHOT_READ: { method: 'GET', path: '/v2/snapshots' },
  })) {
    assert.deepEqual(operations[name], expected, name);
    assert.equal(operationDefinitions[name].method, expected.method, name);
    assert.equal(operationDefinitions[name].path, expected.path, name);
  }
  assert.equal(operationDefinitions.IF_STREAM_TICKET_ISSUE.method, 'POST');
  assert.equal(operationDefinitions.IF_STREAM_TICKET_ISSUE.path, '/v2/stream/tickets');
}

async function finding8RealPaginationExampleConsumesOpaqueContinuation() {
  const pool = '0x1111111111111111111111111111111111111111';
  const position = {
    chainId: '125', height: '1', transactionIndex: '0', logIndex: '0',
    transactionHash: '0x' + 'a'.repeat(64), blockHash: '0x' + 'b'.repeat(64),
  };
  const trades = ['fill-1', 'fill-2'].map(eventId => ({
    eventId, poolAddress: pool, sender: pool, recipient: null, side: 'buy',
    tokenAmountRaw: '9007199254740993', usdlAmountRaw: '1000000', position,
    feeRaw: '10', feeAssetAddress: pool, feeAssetDecimals: 6, feeDisposition: 'distributed_usdl',
    paidInputRaw: '1000000', pricedInputRaw: '999990', ammInputAfterFeeRaw: '999980',
    inputDerivationQuality: 'exact', recipientAttribution: 'unavailable', recipientEvidenceRefs: [],
    emittingContract: pool, codeEpoch: '1',
  }));
  const cursor = 'page/2+next';
  const applicationOrigin = 'https://app.example.test';
  const requests = [];
  const responses = [];
  const failures = [];
  const server = createServer((request, response) => {
    try {
      const url = new URL(request.url, 'http://localhost');
      requests.push({ method: request.method, path: url.pathname, query: [...url.searchParams], origin: request.headers.origin });
      const page = requests.length - 1;
      assert.equal(request.method, 'GET');
      assert.equal(url.pathname, `/v2/pools/${pool}/trades`);
      assert.equal(request.headers.origin, applicationOrigin);
      assert.equal(url.searchParams.get('cursor'), page === 0 ? null : cursor);
      assert.equal(url.searchParams.get('limit'), '100');
      assert.ok(page < trades.length);
      const envelope = {
        requestId: '00000000-0000-4000-8000-000000000001',
        data: { items: [trades[page]] }, watermark: null, revision: null, quality: 'complete',
        nextCursor: page === 0 ? cursor : null, generatedAt: '2026-01-02T00:00:00.000Z',
      };
      responses.push(decodeV2Contract('TradesResponse', envelope));
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(envelope));
    } catch (error) {
      failures.push(error);
      response.writeHead(500);
      response.end();
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const directory = mkdtempSync(join(tmpdir(), 'repair-22-33-1047-'));
  const configPath = join(directory, 'config.json');
  const previousConfig = process.env.SDK_CONNECTED_CONFIG;
  const previousWebSocket = globalThis.WebSocket;
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const streamOrigin = origin.replace(/^http/, 'ws');
    writeFileSync(configPath, JSON.stringify({
      applicationOrigin, poolAddress: pool,
      transport: { apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false },
    }));
    process.env.SDK_CONNECTED_CONFIG = configPath;
    await import('../examples/pagination.mjs?repair-22-33-1047');
    assert.deepEqual(failures, []);
    assert.deepEqual(requests, [
      { method: 'GET', path: `/v2/pools/${pool}/trades`, query: [['limit', '100']], origin: applicationOrigin },
      { method: 'GET', path: `/v2/pools/${pool}/trades`, query: [['cursor', cursor], ['limit', '100']], origin: applicationOrigin },
    ]);
    assert.deepEqual(responses.map(value => value.data.items[0]), trades);
    assert.deepEqual(responses.map(value => value.nextCursor), [cursor, null]);
    assert.equal(responses[1].data.items[0].tokenAmountRaw, '9007199254740993');
  } finally {
    if (previousConfig === undefined) delete process.env.SDK_CONNECTED_CONFIG;
    else process.env.SDK_CONNECTED_CONFIG = previousConfig;
    globalThis.WebSocket = previousWebSocket;
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
    rmSync(directory, { recursive: true, force: true });
  }
}

test(finding6VersionedGenerationInputsMatchImmutableReleasePins);
test(finding7SupportedBundlesHaveUsableSchemas);
test(finding8ExamplesReadTheAdmittedOperationCatalog);
test(finding8RealPaginationExampleConsumesOpaqueContinuation);

// These retain the independent inventory, real reward-claim request/response
// walkthrough and both network refusal contracts in this focused gate.
await import('./served-routes.test.mjs');
await import('./route-corrections.test.mjs');
await import('./unrouted-refusal.test.mjs');
await import('./development-gating.test.mjs');
