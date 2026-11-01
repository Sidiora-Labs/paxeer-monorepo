import { test } from 'node:test';
import assert from 'node:assert/strict';
import { operationDefinitions } from '../dist/operations.gen.js';
import { ROUTE_STATUS, routeStatus } from '../dist/index.js';

function routeStatusCoversEveryOperation() {
  const operationNames = Object.keys(operationDefinitions).sort();
  assert.deepEqual(Object.keys(ROUTE_STATUS).sort(), operationNames);

  const unrouted = new Set([
    'IF_ACCOUNT_DELETE',
    'IF_ACCOUNT_DELETE_STATUS',
    'IF_ACCOUNT_EXPORT',
    'IF_ACCOUNT_EXPORT_STATUS',
    'IF_AUTOMATION_ACTION',
    'IF_AUTOMATION_CREATE',
    'IF_AUTOMATION_PUBLIC_DISCLOSURE',
    'IF_AUTOMATION_STATUS',
    'IF_DEPOSIT_INTENT',
    'IF_DEVICE_REGISTER',
    'IF_DEVICE_REVOKE',
    'IF_DM_CREATE',
    'IF_DM_LIST',
    'IF_FUNDING_STATUS',
    'IF_LIVE_WEBHOOK',
    'IF_MODERATE',
    'IF_OPERATOR_DLQ',
    'IF_OPERATOR_HEALTH',
    'IF_OPERATOR_JOB_STATUS',
    'IF_OPERATOR_METRICS',
    'IF_OPERATOR_REDRIVE',
    'IF_OPERATOR_REPLAY',
    'IF_QUOTE',
    'IF_REPORT_LIST',
    'IF_REWARD_CLAIM',
    'IF_SQUAD_CREATE',
    'IF_SQUAD_MEMBERSHIP',
    'IF_SQUAD_READ',
    'IF_VOICE_END',
    'IF_VOICE_LEAVE',
    'IF_VOICE_READ',
    'IF_WITHDRAW_PREPARE',
    'IF_WITHDRAW_STATUS',
  ]);
  const development = new Set([
    'IF_ALERT_CREATE',
    'IF_ALERT_DELETE',
    'IF_CUP_CURRENT',
    'IF_CUP_GET',
    'IF_CUP_RESULTS',
    'IF_CUP_STANDINGS',
    'IF_EVENTS_SSE',
    'IF_FUNDING_RAILS',
    'IF_NARRATIVES',
    'IF_NARRATIVE_FEED',
    'IF_NOTIFICATIONS',
    'IF_NOTIFICATION_READ',
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
  assert.deepEqual(counts, { served: 153, development: 15, unrouted: 33 });
}

test(routeStatusCoversEveryOperation);
