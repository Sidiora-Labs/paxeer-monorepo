import { beforeAll, expect, test } from 'vitest';

import { exerciseTelemetryStartup, preloadTelemetryDependencies } from './instrumentation.node.fixture';

beforeAll(preloadTelemetryDependencies);

test('telemetryFalseFlagKeepsSdkDisabled', async function telemetryFalseFlagKeepsSdkDisabled() {
  const result = await exerciseTelemetryStartup('false');
  expect(result).toEqual({ enabled: false, sigtermHandlersAdded: 0, traceRequests: 0 });
});

test('telemetryTrueFlagStartsSdkAndExportsSpans', async function telemetryTrueFlagStartsSdkAndExportsSpans() {
  const result = await exerciseTelemetryStartup('true');
  expect(result.enabled).toBe(true);
  expect(result.sigtermHandlersAdded).toBe(1);
  expect(result.traceRequests).toBeGreaterThan(0);
});

test('telemetryUnsetFlagKeepsSdkDisabled', async function telemetryUnsetFlagKeepsSdkDisabled() {
  const result = await exerciseTelemetryStartup(undefined);
  expect(result).toEqual({ enabled: false, sigtermHandlersAdded: 0, traceRequests: 0 });
});
