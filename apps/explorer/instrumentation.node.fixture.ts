import type * as TelemetryApi from '@opentelemetry/api';
import type { NodeSDK } from '@opentelemetry/sdk-node';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import * as v from 'valibot';

import { expect, vi } from 'vitest';

const requireTelemetry = createRequire(import.meta.url);
const telemetry: typeof TelemetryApi = requireTelemetry('@opentelemetry/api');

function resetTelemetryApi(api: typeof telemetry) {
  api.context.disable();
  api.trace.disable();
  api.metrics.disable();
  api.propagation.disable();
  api.diag.disable();
}

async function shutdownTelemetry(sdk: NodeSDK) {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    await Promise.race([
      sdk.shutdown(),
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => reject(new Error('Real telemetry shutdown exceeded 1000ms')), 1000);
      }),
    ]);
  } finally {
    clearTimeout(timeout);
  }
}

const exportSchema = v.object({
  resourceSpans: v.array(v.object({
    resource: v.object({ attributes: v.array(v.object({ key: v.string(), value: v.object({ stringValue: v.string() }) })) }),
    scopeSpans: v.array(v.object({
      spans: v.array(v.object({ name: v.string(), traceId: v.string(), spanId: v.string(), parentSpanId: v.optional(v.string()) })),
    })),
  })),
});

export async function preloadTelemetryDependencies() {
  await Promise.all([
    import('@opentelemetry/auto-instrumentations-node'),
    import('@opentelemetry/exporter-metrics-otlp-proto'),
    import('@opentelemetry/exporter-trace-otlp-http'),
    import('@opentelemetry/sdk-node'),
  ]);
}

export async function exerciseTelemetryStartup(value: string | undefined) {
  const {
    createTraceState, defaultTextMapGetter, defaultTextMapSetter, isSpanContextValid, propagation, ROOT_CONTEXT, trace,
  } = telemetry;
  const requests: Array<{ path: string | undefined; body: string }> = [];
  const collector = createServer((request, response) => {
    const chunks: Array<string> = [];
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => chunks.push(chunk));
    request.on('end', () => {
      requests.push({ path: request.url, body: chunks.join('') });
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end('{}');
    });
  });
  collector.listen(0, '127.0.0.1');
  await once(collector, 'listening');
  const address = collector.address();
  if (!address || typeof address === 'string') {
    throw new Error('The telemetry collector needs a loopback TCP address');
  }

  const environment = {
    OTEL_SDK_ENABLED: value,
    OTEL_SDK_DISABLED: 'false',
    OTEL_EXPORTER_OTLP_ENDPOINT: `http://127.0.0.1:${ address.port }`,
    OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: `http://127.0.0.1:${ address.port }/v1/traces`,
    OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: `http://127.0.0.1:${ address.port }/v1/metrics`,
    OTEL_EXPORTER_OTLP_HEADERS: '',
    OTEL_EXPORTER_OTLP_TRACES_HEADERS: '',
    OTEL_EXPORTER_OTLP_METRICS_HEADERS: '',
    OTEL_TRACES_SAMPLER: 'always_on',
    OTEL_PROPAGATORS: 'tracecontext,baggage',
    OTEL_NODE_RESOURCE_DETECTORS: 'none',
    OTEL_NODE_ENABLED_INSTRUMENTATIONS: 'http',
    OTEL_METRIC_EXPORT_INTERVAL: '60000',
  };
  const saved = Object.fromEntries(Object.keys(environment).map((name) => [ name, process.env[name] ]));
  for (const [ name, setting ] of Object.entries(environment)) {
    if (setting === undefined) {
      delete process.env[name];
    } else {
      process.env[name] = setting;
    }
  }
  const originalListeners = process.listeners('SIGTERM');
  let sdk: NodeSDK | undefined;
  let shutdown: Promise<void> | undefined;
  vi.resetModules();
  const moduleApi = await import('@opentelemetry/api');
  resetTelemetryApi(telemetry);
  resetTelemetryApi(moduleApi);
  try {
    ({ sdk } = await import('./instrumentation.node'));
    const sigtermHandlersAdded = process.listeners('SIGTERM').filter((listener) => !originalListeners.includes(listener)).length;
    const tracer = trace.getTracer('explorer-startup-regression');
    const parent = tracer.startSpan('startup-parent', undefined, ROOT_CONTEXT);
    const parentContext = parent.spanContext();
    if (value === 'true') {
      parentContext.traceState = createTraceState('explorer=startup');
    }
    const enabled = isSpanContextValid(parentContext);
    const carrier: Record<string, string> = {};
    propagation.inject(trace.setSpan(ROOT_CONTEXT, parent), carrier, defaultTextMapSetter);
    const extracted = propagation.extract(ROOT_CONTEXT, carrier, defaultTextMapGetter);
    const child = tracer.startSpan('startup-child', undefined, extracted);
    const childContext = child.spanContext();
    child.end();
    parent.end();

    if (value === 'true') {
      expect(enabled).toBe(true);
      expect(carrier.traceparent).toBe(`00-${ parentContext.traceId }-${ parentContext.spanId }-01`);
      expect(carrier.tracestate).toBe('explorer=startup');
      expect(trace.getSpanContext(extracted)).toMatchObject({
        traceId: parentContext.traceId,
        spanId: parentContext.spanId,
        traceFlags: parentContext.traceFlags,
        isRemote: true,
      });
      expect(trace.getSpanContext(extracted)?.traceState?.serialize()).toBe('explorer=startup');
      expect(childContext.traceState?.serialize()).toBe('explorer=startup');
      expect(childContext.traceId).toBe(parentContext.traceId);
      expect(childContext.spanId).not.toBe(parentContext.spanId);
    }
    shutdown = shutdownTelemetry(sdk);
    await shutdown;

    const traceRequests = requests.filter((request) => request.path === '/v1/traces');
    if (value === 'true') {
      expect(traceRequests.length).toBeGreaterThan(0);
      const exports = traceRequests.flatMap((request) => v.parse(exportSchema, JSON.parse(request.body)).resourceSpans);
      expect(exports.every((item) => item.resource.attributes.some((attribute) =>
        attribute.key === 'service.name' && attribute.value.stringValue === 'blockscout_frontend'))).toBe(true);
      const spans = exports.flatMap((item) => item.scopeSpans.flatMap((scope) => scope.spans));
      expect(spans.filter((span) => span.name.startsWith('startup-'))).toHaveLength(2);
      expect(spans).toEqual(expect.arrayContaining([
        expect.objectContaining({ name: 'startup-parent', traceId: parentContext.traceId, spanId: parentContext.spanId }),
        expect.objectContaining({
          name: 'startup-child', traceId: parentContext.traceId, spanId: childContext.spanId, parentSpanId: parentContext.spanId,
        }),
      ]));
    } else {
      expect(carrier).toEqual({});
      expect(requests).toEqual([]);
    }
    return { enabled, sigtermHandlersAdded, traceRequests: traceRequests.length };
  } finally {
    try {
      if (sdk && !shutdown) {
        await shutdownTelemetry(sdk);
      }
    } finally {
      for (const listener of process.listeners('SIGTERM')) {
        if (!originalListeners.includes(listener)) {
          process.removeListener('SIGTERM', listener);
        }
      }
      resetTelemetryApi(telemetry);
      resetTelemetryApi(moduleApi);
      for (const [ name, setting ] of Object.entries(saved)) {
        if (setting === undefined) {
          delete process.env[name];
        } else {
          process.env[name] = setting;
        }
      }
      const closed = new Promise<void>((resolve, reject) => collector.close((error) => error ? reject(error) : resolve()));
      collector.closeAllConnections();
      await closed;
    }
  }
}
