import Sentry from '@sentry/nextjs';
import { NextRequest } from 'next/server';
import { afterAll, beforeAll, beforeEach, expect, test } from 'vitest';
import { POST } from './route';

const BYTE_CAP = 64 * 1024;
const encoder = new TextEncoder();
type SentryClient = NonNullable<ReturnType<typeof Sentry.init>>;
type SentryTransport = NonNullable<ReturnType<SentryClient['getTransport']>>;
type SentryEnvelope = Parameters<SentryTransport['send']>[0];
const envelopes: SentryEnvelope[] = [];
let client: SentryClient;
const transport: SentryTransport = {
  send(envelope) {
    envelopes.push(envelope);
    return Promise.resolve({ statusCode: 200 });
  },
  flush() {
    return Promise.resolve(true);
  },
};

beforeAll(() => {
  const initialized = Sentry.init({
    dsn: 'http://public@127.0.0.1/1',
    maxValueLength: BYTE_CAP,
    defaultIntegrations: false,
    sendClientReports: false,
    transport: () => transport,
  });
  if (!initialized) throw new Error('Sentry client did not initialize');
  client = initialized;
});

beforeEach(async () => {
  expect(await client.flush(2000)).toBe(true);
  envelopes.length = 0;
});

afterAll(async () => {
  expect(await client.close(2000)).toBe(true);
});

function streamedRequest(chunks: Uint8Array[], headers?: HeadersInit) {
  let supplied = 0;
  let cancellations = 0;
  const body = new ReadableStream<Uint8Array>(
    {
      pull(controller) {
        if (supplied === chunks.length) {
          controller.close();
          return;
        }
        controller.enqueue(chunks[supplied++]);
      },
      cancel() {
        cancellations++;
      },
    },
    { highWaterMark: 0 },
  );
  return {
    request: new NextRequest('http://localhost/api/errors', {
      method: 'POST',
      body,
      headers,
    }),
    supplied: () => supplied,
    cancellations: () => cancellations,
  };
}

function capturedEvents() {
  return envelopes.flatMap(([, items]) =>
    items.filter(([header]) => header.type === 'event').map(([, payload]) => payload),
  );
}

async function errorBeaconByteLimitCancelsChunkedUploads() {
  const bytes = encoder.encode(JSON.stringify({ message: 'x'.repeat(BYTE_CAP) }));
  const fixture = streamedRequest([
    bytes.subarray(0, BYTE_CAP),
    bytes.subarray(BYTE_CAP, BYTE_CAP + 1),
    bytes.subarray(BYTE_CAP + 1),
  ]);
  expect(fixture.request.headers.has('content-length')).toBe(false);
  const response = await POST(fixture.request);
  expect(response.status).toBe(413);
  expect(await response.text()).toBe('');
  expect(fixture.cancellations()).toBe(1);
  expect(fixture.supplied()).toBe(2);
  expect(fixture.request.body?.locked).toBe(false);
  expect(await client.flush(2000)).toBe(true);
  expect(capturedEvents()).toEqual([]);
}

async function errorBeaconByteLimitCountsMultibyteBytes() {
  const raw = JSON.stringify({ message: '😀'.repeat(BYTE_CAP / 4) });
  const bytes = encoder.encode(raw);
  expect(raw.length).toBeLessThan(BYTE_CAP);
  expect(bytes.byteLength).toBeGreaterThan(BYTE_CAP);
  for (const chunks of [[bytes], [bytes.subarray(0, 13), bytes.subarray(13)]]) {
    const fixture = streamedRequest(chunks, { 'content-length': '1' });
    expect((await POST(fixture.request)).status).toBe(413);
    expect(fixture.cancellations()).toBe(1);
    expect(fixture.request.body?.locked).toBe(false);
  }
  expect(await client.flush(2000)).toBe(true);
  expect(capturedEvents()).toEqual([]);
}

async function errorBeaconByteLimitPreservesExactBoundaryDelivery() {
  const overhead = encoder.encode(JSON.stringify({ message: '' })).byteLength;
  for (const prefix of ['boundary-ascii:', 'boundary-multibyte:😀é']) {
    const message = prefix + 'x'.repeat(BYTE_CAP - overhead - encoder.encode(prefix).byteLength);
    const bytes = encoder.encode(JSON.stringify({ message }));
    expect(bytes.byteLength).toBe(BYTE_CAP);
    const fixture = streamedRequest([bytes.subarray(0, 31), bytes.subarray(31)]);
    const response = await POST(fixture.request);
    expect(response.status).toBe(204);
    expect(await response.text()).toBe('');
    expect(fixture.cancellations()).toBe(0);
    expect(fixture.request.body?.locked).toBe(false);
    expect(await client.flush(2000)).toBe(true);
    expect(capturedEvents()).toHaveLength(1);
    expect(capturedEvents()[0]).toMatchObject({
      exception: { values: [{ type: 'Error', value: message }] },
      tags: { source: 'client-beacon' },
    });
    envelopes.length = 0;
  }
}

async function errorBeaconByteLimitPreservesSplitUtf8AndMetadata() {
  const beacon = {
    message: 'split UTF-8: 😀 café 日本語',
    stack: 'Error: split UTF-8\n    at beacon (http://localhost/beacon.js:1:2)',
    context: { area: 'checkout', attempt: 2 },
    url: 'http://localhost/checkout',
    timestamp: 1700000000000,
  };
  const chunks = Array.from(encoder.encode(JSON.stringify(beacon)), (byte) => Uint8Array.of(byte));
  const fixture = streamedRequest(chunks, { 'user-agent': 'beacon-repair-test' });
  expect((await POST(fixture.request)).status).toBe(204);
  expect(fixture.cancellations()).toBe(0);
  expect(await client.flush(2000)).toBe(true);
  expect(capturedEvents()).toHaveLength(1);
  expect(capturedEvents()[0]).toMatchObject({
    level: 'error',
    exception: {
      values: [{
        type: 'Error',
        value: beacon.message,
        stacktrace: { frames: [{ filename: 'http://localhost/beacon.js', lineno: 1, colno: 2 }] },
      }],
    },
    tags: { source: 'client-beacon' },
    extra: {
      ...beacon.context,
      clientUrl: beacon.url,
      clientTimestamp: beacon.timestamp,
      userAgent: 'beacon-repair-test',
    },
  });
}

async function errorBeaconByteLimitPreservesInvalidAndFailedReadResponses() {
  for (const raw of ['', '{', '{}', '{"message":""}', '{"message":"bad","context":[]}']) {
    const fixture = streamedRequest([encoder.encode(raw)]);
    const response = await POST(fixture.request);
    expect(response.status).toBe(400);
    expect(await response.text()).toBe('');
    expect(fixture.cancellations()).toBe(0);
    expect(fixture.request.body?.locked).toBe(false);
  }
  expect((await POST(new NextRequest('http://localhost/api/errors', { method: 'POST' }))).status).toBe(400);
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.error(new Error('upload disconnected'));
    },
  });
  const request = new NextRequest('http://localhost/api/errors', { method: 'POST', body });
  expect((await POST(request)).status).toBe(204);
  expect(request.body?.locked).toBe(false);
  expect(await client.flush(2000)).toBe(true);
  expect(capturedEvents()).toEqual([]);
}

test(errorBeaconByteLimitCancelsChunkedUploads.name, errorBeaconByteLimitCancelsChunkedUploads);
test(errorBeaconByteLimitCountsMultibyteBytes.name, errorBeaconByteLimitCountsMultibyteBytes);
test(errorBeaconByteLimitPreservesExactBoundaryDelivery.name, errorBeaconByteLimitPreservesExactBoundaryDelivery);
test(errorBeaconByteLimitPreservesSplitUtf8AndMetadata.name, errorBeaconByteLimitPreservesSplitUtf8AndMetadata);
test(errorBeaconByteLimitPreservesInvalidAndFailedReadResponses.name, errorBeaconByteLimitPreservesInvalidAndFailedReadResponses);
