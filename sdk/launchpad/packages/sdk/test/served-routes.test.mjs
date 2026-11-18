import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { operationDefinitions } from '../dist/operations.gen.js';
import { ROUTE_STATUS, routeStatus } from '../dist/index.js';
import { ROUTE_CORRECTIONS } from '../dist/route-corrections.js';

const retiredConnectedRoutes = [
  { method: 'POST', path: '/v2/assets/uploads' },
  { method: 'GET', path: '/v2/assets/{assetId}' },
  { method: 'POST', path: '/v2/assets/{assetId}:complete' },
  { method: 'GET', path: '/v2/rooms/{roomId}/messages' },
  { method: 'POST', path: '/v2/rooms/{roomId}/messages' },
];

function retiredGatewayRoutesAreUnrouted() {
  const gateway = readFileSync(new URL('../../../../../services/launchpad/gateway/internal/proxy/connected.go', import.meta.url), 'utf8');
  const retired = gateway.match(/^func RetiredConnectedRoutes\(\) \[\]ConnectedRoute \{\n([\s\S]*?)^\}/m);
  assert.ok(retired);
  const routes = [...retired[1].matchAll(/\{Method: http\.Method(\w+), Template: "([^"]+)"\}/g)]
    .map(([, method, path]) => ({ method: method.toUpperCase(), path }));
  assert.deepEqual(routes, retiredConnectedRoutes);
  for (const { method, path } of retiredConnectedRoutes) {
    const operations = Object.entries(operationDefinitions).filter(([name, definition]) =>
      definition.method === method && (ROUTE_CORRECTIONS[name] ?? definition.path) === path);
    assert.ok(operations.length > 0, `${method} ${path}`);
    for (const [name] of operations) {
      assert.equal(ROUTE_STATUS[name], 'unrouted', name);
      assert.equal(routeStatus(name), 'unrouted', name);
    }
  }
}

function routeStatusCoversEveryOperation() {
  const operationNames = Object.keys(operationDefinitions).sort();
  assert.deepEqual(Object.keys(ROUTE_STATUS).sort(), operationNames);

  const unrouted = new Set([
    'IF_ASSET_GET',
    'IF_AUTOMATION_ACTION',
    'IF_AUTOMATION_CREATE',
    'IF_AUTOMATION_PUBLIC_DISCLOSURE',
    'IF_AUTOMATION_STATUS',
    'IF_DEPOSIT_INTENT',
    'IF_DM_CREATE',
    'IF_DM_LIST',
    'IF_FUNDING_STATUS',
    'IF_LIVE_WEBHOOK',
    'IF_MESSAGES_LIST',
    'IF_MESSAGE_SEND',
    'IF_MODERATE',
    'IF_OPERATOR_DLQ',
    'IF_OPERATOR_HEALTH',
    'IF_OPERATOR_JOB_STATUS',
    'IF_OPERATOR_METRICS',
    'IF_OPERATOR_REDRIVE',
    'IF_OPERATOR_REPLAY',
    'IF_QUOTE',
    'IF_REPORT_LIST',
    'IF_UPLOAD_COMPLETE',
    'IF_UPLOAD_CREATE',
    'IF_WITHDRAW_PREPARE',
    'IF_WITHDRAW_STATUS',
  ]);
  const development = new Set([
    'IF_CUP_CURRENT',
    'IF_CUP_GET',
    'IF_CUP_RESULTS',
    'IF_CUP_STANDINGS',
    'IF_FUNDING_RAILS',
    'IF_NARRATIVES',
    'IF_NARRATIVE_FEED',
    'IF_ORDER_CANCEL',
    'IF_ORDER_CREATE',
    'IF_ORDER_LIST',
  ]);

  const counts = { served: 0, development: 0, unrouted: 0 };
  for (const name of operationNames) {
    const expected = unrouted.has(name) ? 'unrouted' : development.has(name) ? 'development' : 'served';
    assert.equal(ROUTE_STATUS[name], expected, name);
    assert.equal(routeStatus(name), expected, name);
    counts[ROUTE_STATUS[name]] += 1;
  }
  assert.deepEqual(counts, { served: 169, development: 10, unrouted: 25 });
}

test(retiredGatewayRoutesAreUnrouted);
test(routeStatusCoversEveryOperation);
