import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import { once } from 'node:events';
import { createServer, request } from 'node:http';
import { Writable } from 'node:stream';
import pinoHttp from 'pino-http';
import { Counter, Registry } from 'prom-client';

import { test, expect } from 'vitest';

import { createInvalidApiSchemaHandler } from './invalid-api-schema';

type SchemaDiagnostic = { resource?: string; url?: string; errorCategory: 'invalid_response_schema' | 'invalid_report' };

function isSchemaDiagnostic(value: unknown): value is SchemaDiagnostic {
  return value !== null && typeof value === 'object' &&
    'errorCategory' in value && (value.errorCategory === 'invalid_response_schema' || value.errorCategory === 'invalid_report') &&
    (!('resource' in value) || typeof value.resource === 'string') &&
    (!('url' in value) || typeof value.url === 'string');
}

async function openMonitoringRoute() {
  const registry = new Registry();
  const counter = new Counter({
    name: 'invalid_api_schema',
    help: 'Number of invalid external API schema events',
    labelNames: [ 'resource', 'url' ] as const,
    registers: [ registry ],
  });
  const diagnostics: Array<SchemaDiagnostic> = [];
  const destination = new Writable({
    write(chunk, _encoding, callback) {
      const diagnostic: unknown = JSON.parse(chunk.toString());
      if (!isSchemaDiagnostic(diagnostic)) {
        callback(new Error('Unexpected schema diagnostic'));
        return;
      }
      diagnostics.push(diagnostic);
      callback();
    },
  });
  const handler = createInvalidApiSchemaHandler(counter, pinoHttp({}, destination).logger);
  const server = createServer((req, res) => {
    void apiResolver(req, res, {}, { 'default': handler }, {
      previewModeId: '',
      previewModeEncryptionKey: '',
      previewModeSigningKey: '',
      dev: false,
    }, false);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback server address');
  }

  return {
    counter,
    registry,
    diagnostics,
    async report(payload: unknown, contentType = 'application/json') {
      const body = typeof payload === 'string' ? payload : JSON.stringify(payload);
      return new Promise<{ status: number | undefined; body: unknown }>((resolve, reject) => {
        const req = request({
          hostname: '127.0.0.1',
          port: address.port,
          path: '/api/monitoring/invalid-api-schema',
          method: 'POST',
          headers: { 'content-type': contentType },
        }, (res) => {
          let responseBody = '';
          res.setEncoding('utf8');
          res.on('data', (chunk: string) => responseBody += chunk);
          res.on('error', reject);
          res.on('end', () => resolve({ status: res.statusCode, body: JSON.parse(responseBody) }));
        });
        req.on('error', reject);
        req.end(body);
      });
    },
    async close() {
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      destination.end();
    },
  };
}

test('schema-metric-cardinality: distinct URLs keep one series per known resource', async function schemaMetricCardinality() {
  const route = await openMonitoringRoute();
  try {
    for (let index = 0; index < 200; index++) {
      const result = await route.report({
        resource: 'general:address_xstar_score',
        url: `https://example.test/addresses/${ index }?query=${ index }#${ index }`,
      }, index % 2 ? 'text/plain' : 'application/json');
      expect(result).toEqual({ status: 200, body: { status: 'ok' } });
    }
    await route.report({ resource: 'general:address_xstar_score' });
    await route.report({ resource: 'general:contract_solidity_scan_report', url: 'https://example.test/report?key=other' });

    const metric = await route.counter.get();
    expect(metric.values).toEqual([
      { labels: { resource: 'general:address_xstar_score', url: 'invalid_response_schema' }, value: 201 },
      { labels: { resource: 'general:contract_solidity_scan_report', url: 'invalid_response_schema' }, value: 1 },
    ]);
    expect(await route.registry.metrics()).not.toContain('example.test');
    expect(route.diagnostics).toHaveLength(10);
  } finally {
    await route.close();
  }
});

test('schema-metric-cardinality: diagnostics strip credentials and queries and bound URL length', async function boundedSchemaDiagnostics() {
  const route = await openMonitoringRoute();
  try {
    await route.report({
      resource: 'general:address_xstar_score',
      url: 'https://username:password@example.test/report?token=secret#private',
    });
    await route.report({ resource: 'general:address_xstar_score', url: `https://example.test/${ 'a'.repeat(2048) }?token=secret` });
    expect(route.diagnostics).toHaveLength(2);
    expect(route.diagnostics[0]).toMatchObject({
      resource: 'general:address_xstar_score',
      errorCategory: 'invalid_response_schema',
      url: 'https://example.test/report',
    });
    expect(route.diagnostics[1].url).toHaveLength(512);
    expect(JSON.stringify(route.diagnostics)).not.toMatch(/username|password|secret|private/);
    expect((await route.counter.get()).values).toEqual([
      { labels: { resource: 'general:address_xstar_score', url: 'invalid_response_schema' }, value: 2 },
    ]);
  } finally {
    await route.close();
  }
});

test('schema-metric-cardinality: unregistered and aliased resources cannot create series', async function finiteSchemaResources() {
  const route = await openMonitoringRoute();
  try {
    for (let index = 0; index < 30; index++) {
      for (const resource of [ `general:unknown_${ index }`, `general:address_xstar_score:${ index }`, 'general:constructor', 'constructor:prototype' ]) {
        expect(await route.report({ resource, url: `https://example.test/?q=${ index }` })).toEqual({ status: 200, body: { status: 'ok' } });
      }
    }
    for (const payload of [ null, {}, { resource: 1 }, { resource: 'general:address_xstar_score', url: 'invalid URL' } ]) {
      expect(await route.report(payload)).toEqual({ status: 200, body: { status: 'ok' } });
    }
    expect((await route.counter.get()).values).toEqual([]);
    expect(route.diagnostics).toHaveLength(10);
    expect(route.diagnostics.every((diagnostic) => diagnostic.errorCategory === 'invalid_report')).toBe(true);
    expect(route.diagnostics.every((diagnostic) => diagnostic.resource === undefined && diagnostic.url === undefined)).toBe(true);
  } finally {
    await route.close();
  }
});
