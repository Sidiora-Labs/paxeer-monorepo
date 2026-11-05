import {
  defaultTextMapGetter, defaultTextMapSetter, context, isSpanContextValid, propagation, ROOT_CONTEXT, trace,
} from '@opentelemetry/api';
import { JaegerPropagator } from '@opentelemetry/propagator-jaeger';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { AlwaysOnSampler, InMemorySpanExporter, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-node';
import type { Dependency, FirstLevelDependency } from '@yarnpkg/lockfile';
import { parse } from '@yarnpkg/lockfile';
import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { gte, intersects, major, satisfies, validRange } from 'semver';
import * as v from 'valibot';

import { beforeAll, expect, test } from 'vitest';

import { exerciseTelemetryStartup, preloadTelemetryDependencies } from './instrumentation.node.fixture';

beforeAll(preloadTelemetryDependencies);

function readInstalledTelemetryManifest(name: string, requirePackage: ReturnType<typeof createRequire>) {
  let directory = path.dirname(requirePackage.resolve(name));
  while (true) {
    const installedPath = path.join(directory, 'package.json');
    if (existsSync(installedPath)) {
      const installed: unknown = JSON.parse(readFileSync(installedPath, 'utf8'));
      if (v.is(v.object({ name: v.literal(name) }), installed)) {
        return v.parse(v.object({
          name: v.literal(name),
          version: v.string(),
          dependencies: v.optional(v.record(v.string(), v.string())),
          peerDependencies: v.optional(v.record(v.string(), v.string())),
        }), installed);
      }
    }
    const parent = path.dirname(directory);
    if (parent === directory) {
      throw new Error(`Could not find the installed manifest for ${ name }`);
    }
    directory = parent;
  }
}

async function test_explorer_telemetry_dependency_coherence() {
  const root = path.resolve(__dirname, '../..');
  const manifestPath = path.join(__dirname, 'package.json');
  const manifest = v.parse(v.object({
    dependencies: v.record(v.string(), v.string()),
    devDependencies: v.record(v.string(), v.string()),
    resolutions: v.record(v.string(), v.string()),
  }), JSON.parse(readFileSync(manifestPath, 'utf8')));
  const parsed = parse(readFileSync(path.join(__dirname, 'yarn.lock'), 'utf8'));
  expect(parsed.type).toBe('success');
  const lock: Record<string, FirstLevelDependency & { optionalDependencies?: Dependency }> = parsed.object;
  const coreName = '@opentelemetry/core';
  const coreEntries = Object.entries(lock).filter(([ selector ]) => selector.startsWith(`${ coreName }@`));
  expect(coreEntries.length).toBeGreaterThan(0);
  expect(manifest.dependencies).not.toHaveProperty('@opentelemetry/exporter-jaeger');
  expect(Object.keys(lock).some((selector) => selector.startsWith('@opentelemetry/exporter-jaeger@'))).toBe(false);

  for (const [ selector, entry ] of coreEntries) {
    const range = selector.slice(coreName.length + 1);
    expect(validRange(range), selector).not.toBeNull();
    expect(satisfies(entry.version, range), `${ selector } resolved to ${ entry.version }`).toBe(true);
    expect(gte(entry.version, '2.8.0'), selector).toBe(true);
    expect(major(entry.version), selector).toBeGreaterThanOrEqual(2);
  }

  let coreRequests = 0;
  for (const [ selector, entry ] of Object.entries(lock)) {
    for (const dependencies of [ entry.dependencies, entry.optionalDependencies ]) {
      const range = dependencies?.[coreName];
      if (!range) {
        continue;
      }
      coreRequests++;
      const resolved = lock[`${ coreName }@${ range }`];
      expect(resolved, `${ selector } requests ${ range }`).toBeDefined();
      expect(validRange(range), selector).not.toBeNull();
      expect(satisfies(resolved.version, range), `${ selector } requests ${ range }, got ${ resolved.version }`).toBe(true);
      expect(gte(resolved.version, '2.8.0'), selector).toBe(true);
      expect(intersects(range, '>=1.0.0 <2.0.0'), `${ selector } must use a modern core range`).toBe(false);
    }
  }
  expect(coreRequests).toBeGreaterThan(0);

  for (const dependencies of [ manifest.dependencies, manifest.devDependencies, manifest.resolutions ]) {
    const range = dependencies[coreName];
    if (range) {
      expect(validRange(range)).not.toBeNull();
      expect(satisfies(lock[`${ coreName }@${ range }`].version, range)).toBe(true);
    }
  }

  const jaeger = lock['@opentelemetry/propagator-jaeger@2.9.0'];
  expect(manifest.dependencies['@opentelemetry/propagator-jaeger']).toBe('2.9.0');
  expect(jaeger.version).toBe('2.9.0');
  expect(jaeger.dependencies?.[coreName]).toBe('2.9.0');
  expect(lock[`${ coreName }@2.9.0`].version).toBe('2.9.0');

  const requireInstalled = createRequire(manifestPath);
  const requireJaeger = createRequire(requireInstalled.resolve('@opentelemetry/propagator-jaeger'));
  const installedApi = readInstalledTelemetryManifest('@opentelemetry/api', requireInstalled);
  const installedSdk = readInstalledTelemetryManifest('@opentelemetry/sdk-node', requireInstalled);
  const sdkApiRange = v.parse(v.string(), installedSdk.peerDependencies?.['@opentelemetry/api']);
  expect(satisfies(installedApi.version, sdkApiRange)).toBe(true);
  const installedJaeger = readInstalledTelemetryManifest('@opentelemetry/propagator-jaeger', requireInstalled);
  expect(installedJaeger.version).toBe('2.9.0');
  expect(installedJaeger.dependencies?.[coreName]).toBe('2.9.0');
  expect(readInstalledTelemetryManifest(coreName, requireJaeger).version).toBe('2.9.0');
  for (const [ name, range ] of Object.entries(manifest.dependencies)) {
    if (name.startsWith('@opentelemetry/')) {
      const installed = readInstalledTelemetryManifest(name, requireInstalled);
      expect(satisfies(installed.version, range), `${ name } installed version`).toBe(true);
    }
  }

  const floorCheck = spawnSync('python3', [ 'tools/deps/lockfloor.py', 'tools/deps/floors/explorer-web.json' ], {
    cwd: root,
    encoding: 'utf8',
  });
  expect(floorCheck.error).toBeUndefined();
  expect(floorCheck.status, `${ floorCheck.stdout }${ floorCheck.stderr }`).toBe(0);

  const exporter = new InMemorySpanExporter();
  const sdk = new NodeSDK({
    autoDetectResources: false,
    sampler: new AlwaysOnSampler(),
    resource: resourceFromAttributes({ 'service.name': 'explorer-telemetry-coherence' }),
    spanProcessors: [ new SimpleSpanProcessor(exporter) ],
    textMapPropagator: new JaegerPropagator(),
  });
  sdk.start();
  try {
    const tracer = trace.getTracer('explorer-telemetry-coherence');
    const span = tracer.startSpan('inject', undefined, ROOT_CONTEXT);
    const original = span.spanContext();
    expect(isSpanContextValid(original)).toBe(true);
    const carrier: Record<string, string> = {};
    propagation.inject(trace.setSpan(ROOT_CONTEXT, span), carrier, defaultTextMapSetter);
    expect(carrier['uber-trace-id']).toBe(`${ original.traceId }:${ original.spanId }:0:01`);
    const extractedContext = propagation.extract(ROOT_CONTEXT, carrier, defaultTextMapGetter);
    const extracted = trace.getSpanContext(extractedContext);
    expect(extracted).toMatchObject({
      traceId: original.traceId,
      spanId: original.spanId,
      traceFlags: original.traceFlags,
      isRemote: true,
    });
    const child = tracer.startSpan('extract', undefined, extractedContext);
    expect(child.spanContext().traceId).toBe(original.traceId);
    expect(child.spanContext().spanId).not.toBe(original.spanId);
    child.end();
    span.end();
    const finished = exporter.getFinishedSpans();
    expect(finished).toHaveLength(2);
    expect(finished.find((item) => item.name === 'extract')?.parentSpanContext?.spanId).toBe(original.spanId);
  } finally {
    await sdk.shutdown();
    context.disable();
    trace.disable();
    propagation.disable();
  }
}

test('test_explorer_telemetry_dependency_coherence', test_explorer_telemetry_dependency_coherence);

test.each([
  { value: 'true', enabled: true },
  { value: 'false', enabled: false },
  { value: undefined, enabled: false },
])('actual telemetry module startup with OTEL_SDK_ENABLED=$value', async function telemetryModuleStartup({ value, enabled }) {
  const result = await exerciseTelemetryStartup(value);
  expect(result.enabled).toBe(enabled);
  expect(result.sigtermHandlersAdded).toBe(enabled ? 1 : 0);
});
