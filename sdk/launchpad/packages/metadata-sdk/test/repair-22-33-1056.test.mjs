import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once, getEventListeners } from 'node:events';
import { gzipSync } from 'node:zlib';
import {
  createMetadataClient,
  MetadataHttpError,
  MetadataUploadError,
  MetadataProtocolError,
  MetadataTimeoutError,
  MetadataResponseTooLargeError,
  METADATA_DEFAULT_TIMEOUT_MS,
  METADATA_MAX_TIMEOUT_MS,
  METADATA_RESPONSE_LIMITS,
} from '../dist/index.js';

const token = `0x${'a'.repeat(40)}`;
const metadata = {
  token_address: token, pool_address: null, name: null, symbol: null,
  decimals: 6, total_supply: '1000000000', creator: null, description: null,
  socials: { website: null, twitter: null, telegram: null, discord: null },
  tags: [], images: { logo: null, banner: null }, created_at: null, updated_at: null,
};
const upload = { success: true, metadata_updated: true, logo_url: null, banner_url: null };
const uploadInput = { wallet: token, signature: 'signed', message: 'authorized' };
const endpoints = [
  { name: 'metadata', limit: METADATA_RESPONSE_LIMITS.metadata, call: (client, options) => client.getMetadata(token, options) },
  { name: 'batch', limit: METADATA_RESPONSE_LIMITS.batch, call: (client, options) => client.getMetadataBatch([token], options) },
  { name: 'upload', limit: METADATA_RESPONSE_LIMITS.upload, call: (client, options) => client.uploadMetadata(token, uploadInput, options) },
];

async function withServer(handler, run) {
  const server = createServer(handler);
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    await run(`http://127.0.0.1:${server.address().port}`);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}

async function within(promise, milliseconds = 2000) {
  let timer;
  try {
    return await Promise.race([
      promise,
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Request failed to settle within test bound')), milliseconds); }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

function assertNoListeners(signal) {
  assert.equal(getEventListeners(signal, 'abort').length, 0, 'caller abort listener must be released');
}

async function Repair31DefaultDeadlineBoundsStalledHeaders() {
  assert.equal(METADATA_DEFAULT_TIMEOUT_MS, 10_000);
  await withServer(req => req.resume(), async baseUrl => {
    const started = performance.now();
    const client = createMetadataClient({ baseUrl });
    await assert.rejects(within(client.getMetadata(token), 12_000), error =>
      error instanceof MetadataTimeoutError && error.timeoutMs === METADATA_DEFAULT_TIMEOUT_MS);
    assert.ok(performance.now() - started < 12_000);
  });
}

async function Repair31ConfiguredDeadlineCoversHeadersAndBodies() {
  for (const endpoint of endpoints) {
    for (const phase of ['headers', 'body', 'error-body']) {
      let closed;
      const connectionClosed = new Promise(resolve => { closed = resolve; });
      await withServer((req, res) => {
        req.resume();
        res.once('close', closed);
        if (phase !== 'headers') {
          res.writeHead(phase === 'error-body' ? 503 : 200, { 'Content-Type': 'application/json' });
          res.write('{');
        }
      }, async baseUrl => {
        const caller = new AbortController();
        const client = createMetadataClient({ baseUrl, timeoutMs: 150 });
        await assert.rejects(within(endpoint.call(client, { signal: caller.signal })), error =>
          error instanceof MetadataTimeoutError && error.timeoutMs === 150, `${endpoint.name} ${phase}`);
        assertNoListeners(caller.signal);
        await within(connectionClosed);
      });
    }
  }
}

async function Repair31CallerCancellationAndListenerCleanup() {
  for (const phase of ['headers', 'body']) {
    let arrived;
    const requestArrived = new Promise(resolve => { arrived = resolve; });
    await withServer((req, res) => {
      req.resume();
      if (phase === 'body') res.write('{');
      arrived();
    }, async baseUrl => {
      const client = createMetadataClient({ baseUrl, timeoutMs: 1500 });
      const caller = new AbortController();
      const reason = new Error(`caller cancelled ${phase}`);
      const result = client.getMetadata(token, { signal: caller.signal });
      await within(requestArrived);
      // Let the body reader attach before cancelling the partial body.
      if (phase === 'body') await new Promise(resolve => setTimeout(resolve, 30));
      caller.abort(reason);
      await assert.rejects(within(result), error => error === reason);
      assertNoListeners(caller.signal);
    });
  }
  let requests = 0;
  await withServer((req, res) => { requests++; req.resume(); res.end(JSON.stringify(metadata)); }, async baseUrl => {
    const client = createMetadataClient({ baseUrl });
    const caller = new AbortController();
    const reason = new Error('already cancelled');
    caller.abort(reason);
    await assert.rejects(client.getMetadata(token, { signal: caller.signal }), error => error === reason);
    assertNoListeners(caller.signal);
    assert.equal(requests, 0);
    const shared = new AbortController();
    for (let i = 0; i < 12; i++) {
      assert.deepEqual(await client.getMetadata(token, { signal: shared.signal }), metadata);
      assertNoListeners(shared.signal);
    }
    shared.abort(new Error('after completion'));
  });
}

async function Repair31StreamedByteCapsRefuseBeforeFullBody() {
  assert.deepEqual(METADATA_RESPONSE_LIMITS, { metadata: 262144, batch: 4194304, upload: 65536, error: 16384 });
  for (const endpoint of endpoints) {
    for (const status of [200, 503]) {
      const limit = status === 200 ? endpoint.limit : METADATA_RESPONSE_LIMITS.error;
      const total = limit * 4;
      let sent = 0;
      let closed;
      const connectionClosed = new Promise(resolve => { closed = resolve; });
      await withServer((req, res) => {
        req.resume();
        res.writeHead(status, { 'Content-Type': 'application/json' });
        const interval = setInterval(() => {
          sent += 8192;
          res.write(Buffer.alloc(8192, 32));
          if (sent >= total) { clearInterval(interval); res.end(); }
        }, 1);
        res.once('close', () => { clearInterval(interval); closed(); });
      }, async baseUrl => {
        const caller = new AbortController();
        const client = createMetadataClient({ baseUrl, timeoutMs: 5000 });
        await assert.rejects(within(endpoint.call(client, { signal: caller.signal })), error =>
          error instanceof MetadataResponseTooLargeError && error.status === status && error.limitBytes === limit);
        assertNoListeners(caller.signal);
        await within(connectionClosed);
        assert.ok(sent < total, `${endpoint.name} ${status}: transport must stop the producer before the full body`);
      });
    }
  }
}

async function Repair31DeclaredAndDecompressedByteCaps() {
  for (const encoding of ['declared', 'gzip', 'utf8']) {
    let closed;
    const connectionClosed = new Promise(resolve => { closed = resolve; });
    await withServer((req, res) => {
      req.resume();
      res.once('close', closed);
      const limit = METADATA_RESPONSE_LIMITS.metadata;
      if (encoding === 'declared') {
        res.writeHead(200, { 'Content-Length': limit + 1 });
        res.flushHeaders();
      } else {
        const value = { ...metadata, description: '🙂'.repeat(limit / 4) };
        const body = Buffer.from(JSON.stringify(value));
        assert.ok(body.length > limit);
        assert.ok(JSON.stringify(value).length < limit, 'cap must count bytes, not UTF-16 code units');
        if (encoding === 'gzip') {
          res.setHeader('Content-Encoding', 'gzip');
          res.end(gzipSync(body));
        } else res.end(body);
      }
    }, async baseUrl => {
      const client = createMetadataClient({ baseUrl, timeoutMs: 1500 });
      await assert.rejects(within(client.getMetadata(token)), error =>
        error instanceof MetadataResponseTooLargeError && error.limitBytes === METADATA_RESPONSE_LIMITS.metadata);
      await within(connectionClosed);
    });
  }
}

async function Repair31PreservesTypedErrorsShapesAndBoundaryResponses() {
  let payload = metadata;
  let status = 200;
  let body;
  const requests = [];
  await withServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    requests.push({ url: req.url, method: req.method, contentType: req.headers['content-type'], body: Buffer.concat(chunks).toString() });
    res.writeHead(status, { 'Content-Type': 'application/json' });
    const bytes = Buffer.from(body ?? JSON.stringify(payload));
    // Split inside a multibyte character, exercising streaming UTF-8 decoding.
    const split = bytes.indexOf(Buffer.from('🙂')) + 1;
    res.write(bytes.subarray(0, Math.max(1, split)));
    setImmediate(() => res.end(bytes.subarray(Math.max(1, split))));
  }, async baseUrl => {
    const client = createMetadataClient({ baseUrl });
    const caller = new AbortController();
    const options = { signal: caller.signal };
    async function rejects(promise, error) {
      await assert.rejects(promise, error);
      assertNoListeners(caller.signal);
    }
    payload = { ...metadata, description: '🙂' };
    assert.deepEqual(await client.getMetadata(token, options), payload);
    assertNoListeners(caller.signal);
    payload = { [token]: metadata };
    assert.deepEqual(await client.getMetadataBatch([token, token], options), payload);
    payload = upload;
    assert.deepEqual(await client.uploadMetadata(token, { ...uploadInput, logo: new Blob(['image']) }, options), upload);
    assert.equal(requests[1].url, `/metadata/batch?addresses=${token}`);
    assert.equal(requests[2].method, 'POST');
    assert.match(requests[2].contentType, /^multipart\/form-data; boundary=/);
    for (const field of ['wallet', 'signature', 'message', 'logo']) assert.ok(requests[2].body.includes(`name="${field}"`));
    payload = { ...metadata, description: '' };
    payload.description = 'a'.repeat(METADATA_RESPONSE_LIMITS.metadata - Buffer.byteLength(JSON.stringify(payload)));
    assert.equal(Buffer.byteLength(JSON.stringify(payload)), METADATA_RESPONSE_LIMITS.metadata);
    assert.deepEqual(await client.getMetadata(token, options), payload);
    payload = { ...metadata, decimals: '6' };
    await rejects(client.getMetadata(token, options), MetadataProtocolError);
    payload = {};
    await rejects(client.getMetadataBatch([token], options), MetadataProtocolError);
    payload = { [token]: { ...metadata, token_address: `0x${'b'.repeat(40)}` } };
    await rejects(client.getMetadataBatch([token], options), MetadataProtocolError);
    payload = { ...upload, success: 'true' };
    await rejects(client.uploadMetadata(token, uploadInput, options), MetadataProtocolError);
    payload = { ...upload, success: false, errors: ['logo rejected'] };
    await rejects(client.uploadMetadata(token, uploadInput, options), error =>
      error instanceof MetadataUploadError && error.response.metadata_updated && error.message === 'logo rejected');
    body = '{invalid json';
    await rejects(client.getMetadata(token, options), SyntaxError);
    status = 503;
    body = 'unavailable';
    await rejects(client.getMetadata(token, options), error =>
      error instanceof MetadataHttpError && error.status === 503 && error.body === body);
    assertNoListeners(caller.signal);
  });
}

function Repair31RejectsUnboundedDeadlinesAtomically() {
  const baseUrl = 'http://127.0.0.1';
  assert.equal(METADATA_MAX_TIMEOUT_MS, 120_000);
  const client = createMetadataClient({ baseUrl, timeoutMs: 150 });
  for (const timeoutMs of [0, -1, 0.5, NaN, Infinity, METADATA_MAX_TIMEOUT_MS + 1]) {
    assert.throws(() => createMetadataClient({ baseUrl, timeoutMs }), RangeError);
    assert.throws(() => client.configure({ baseUrl, timeoutMs }), RangeError);
  }
  client.configure({ baseUrl, timeoutMs: METADATA_MAX_TIMEOUT_MS });
}

test(Repair31DefaultDeadlineBoundsStalledHeaders);
test(Repair31ConfiguredDeadlineCoversHeadersAndBodies);
test(Repair31CallerCancellationAndListenerCleanup);
test(Repair31StreamedByteCapsRefuseBeforeFullBody);
test(Repair31DeclaredAndDecompressedByteCaps);
test(Repair31PreservesTypedErrorsShapesAndBoundaryResponses);
test(Repair31RejectsUnboundedDeadlinesAtomically);
