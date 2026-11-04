// Repair record 22.33.1059: finding 34 is proved by the finding34* tests below.
// No out-of-path wiring is required. The baseline gate was not run: the task's
// ordered gate rules require complete implementation before qualification.
// The verified revision and private evidence are recorded by hf-done.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { gzipSync } from 'node:zlib';
import {
  SdkClient,
  HttpError,
  CapabilityUnavailableError,
  StreamTransportError,
} from '../dist/index.js';

const JSON_LIMIT = 8 * 1024 * 1024;
const ERROR_LIMIT = 64 * 1024;
const CHUNK_BYTES = 16 * 1024;
const errorEnvelope = {
  code: 'DEPENDENCY_UNAVAILABLE',
  message: 'Private upstream message',
  requestId: '11111111-1111-4111-8111-111111111111',
  retryable: true,
  retryAfterMs: 0,
  fieldErrors: [],
  details: { dependency: 'private-upstream' },
};

async function withServer(handler, run) {
  const server = createServer(handler);
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const origin = `http://127.0.0.1:${server.address().port}`;
  const stream = origin.replace('http:', 'ws:');
  const client = new SdkClient({
    apiOrigin: origin,
    identityOrigin: origin,
    mediaOrigin: origin,
    streamOrigin: stream,
    approvedStreamOrigins: [stream],
    production: false,
  });
  try {
    await run(client);
  } finally {
    const closed = once(server, 'close');
    server.close();
    server.closeAllConnections();
    await closed;
  }
}

function streamResponse(res, status, totalBytes, headers = {}) {
  const state = { sent: 0, finished: false };
  state.closed = once(res, 'close');
  res.writeHead(status, { 'Content-Type': 'application/json', ...headers });
  res.flushHeaders();
  let timer;
  function send() {
    if (res.destroyed) return;
    const count = Math.min(CHUNK_BYTES, totalBytes - state.sent);
    const chunk = Buffer.alloc(count, ' ');
    if (state.sent === 0) chunk.write('"42"');
    state.sent += count;
    const writable = res.write(chunk);
    if (state.sent === totalBytes) {
      state.finished = true;
      res.end();
    } else if (writable) {
      timer = setTimeout(send, 1);
    } else {
      res.once('drain', () => {
        timer = setTimeout(send, 1);
      });
    }
  }
  res.once('close', () => clearTimeout(timer));
  timer = setTimeout(send, 1);
  return state;
}

function isTooLarge(status) {
  return (error) => {
    assert.ok(error instanceof HttpError, String(error));
    assert.equal(error.status, status);
    assert.equal(error.code, 'RESPONSE_TOO_LARGE');
    assert.ok(!(error instanceof StreamTransportError));
    return true;
  };
}

async function finding34BoundsStreamedSuccessBeforeBuffering() {
  for (const [major, request] of [
    [1, 'request'],
    [1, 'requestV1Generated'],
    [2, 'requestV2'],
    [3, 'requestV3'],
  ]) {
    const streams = [];
    await withServer(
      (req, res) => {
        streams.push(streamResponse(res, 200, JSON_LIMIT * 2));
      },
      async (client) => {
        await assert.rejects(
          client[request](`/v${major}/oversized`, 'UInt256', {
            idempotentRead: true,
          }),
          isTooLarge(200),
        );
        assert.equal(streams.length, 1);
        await streams[0].closed;
        assert.equal(streams[0].finished, false);
        assert.ok(streams[0].sent < JSON_LIMIT + 1024 * 1024);
      },
    );
  }
}

async function finding34BoundsErrorsWithoutCloningOrRetryingOversize() {
  for (const [major, status, binary] of [
    [2, 400, false],
    [2, 404, false],
    [2, 503, false],
    [3, 500, false],
    [2, 500, true],
  ]) {
    const streams = [];
    await withServer(
      (req, res) => {
        streams.push(streamResponse(res, status, JSON_LIMIT));
      },
      async (client) => {
        const options = { idempotentRead: true };
        await assert.rejects(
          binary
            ? client.requestGeneratedBinary('/v2/download', options, 2)
            : client[major === 2 ? 'requestV2' : 'requestV3'](
                `/v${major}/error`,
                'Error',
                options,
              ),
          isTooLarge(status),
        );
        assert.equal(streams.length, 1);
        await streams[0].closed;
        assert.equal(streams[0].finished, false);
        assert.ok(streams[0].sent < ERROR_LIMIT + 1024 * 1024);
      },
    );
  }
}

async function finding34RejectsDeclaredOversizeBeforeReading() {
  for (const status of [200, 400]) {
    let stream;
    await withServer(
      (req, res) => {
        stream = streamResponse(res, status, JSON_LIMIT * 2, {
          'Content-Length': String(JSON_LIMIT * 2),
        });
      },
      async (client) => {
        await assert.rejects(
          client.requestV2('/v2/declared', 'UInt256'),
          isTooLarge(status),
        );
        await stream.closed;
        assert.equal(stream.finished, false);
        assert.ok(stream.sent < ERROR_LIMIT);
      },
    );
  }
}

async function finding34CountsDecodedBytesAndAcceptsExactLimits() {
  const envelope = JSON.stringify(errorEnvelope);
  const exactError =
    envelope + ' '.repeat(ERROR_LIMIT - Buffer.byteLength(envelope));
  const compressedSuccess = gzipSync('"42"' + ' '.repeat(JSON_LIMIT));
  const compressedError = gzipSync(envelope + ' '.repeat(ERROR_LIMIT));
  await withServer(
    (req, res) => {
      if (
        req.url === '/v2/compressed-success' ||
        req.url === '/v2/compressed-error'
      ) {
        const success = req.url.endsWith('success');
        const body = success ? compressedSuccess : compressedError;
        res.writeHead(success ? 200 : 400, {
          'Content-Encoding': 'gzip',
          'Content-Length': body.length,
        });
        res.end(body);
      } else if (req.url === '/v2/exact-error') {
        res.writeHead(400);
        res.end(exactError);
      } else if (req.url === '/v2/utf8-error') {
        res.writeHead(400);
        res.end(
          JSON.stringify({
            ...errorEnvelope,
            padding: 'é'.repeat(ERROR_LIMIT / 2),
          }),
        );
      } else if (req.url === '/v2/unicode') {
        const body = Buffer.from(
          JSON.stringify({ ...errorEnvelope, message: 'café 😀' }),
        );
        const split = body.indexOf(Buffer.from('😀')) + 1;
        res.writeHead(400);
        res.write(body.subarray(0, split));
        setTimeout(() => res.end(body.subarray(split)), 5);
      } else {
        res.end('"42"' + ' '.repeat(JSON_LIMIT - 4));
      }
    },
    async (client) => {
      assert.equal(await client.requestV2('/v2/exact', 'UInt256'), '42');
      await assert.rejects(
        client.requestV2('/v2/exact-error', 'Error'),
        (error) => {
          assert.equal(error.code, errorEnvelope.code);
          assert.deepEqual(error.envelope, {
            ...errorEnvelope,
            message: errorEnvelope.code,
            details: null,
          });
          return true;
        },
      );
      await assert.rejects(
        client.requestV2('/v2/unicode', 'Error'),
        (error) => {
          assert.equal(error.code, errorEnvelope.code);
          return true;
        },
      );
      await assert.rejects(
        client.requestV2('/v2/utf8-error', 'Error'),
        isTooLarge(400),
      );
      await assert.rejects(
        client.requestV2('/v2/compressed-success', 'UInt256'),
        isTooLarge(200),
      );
      await assert.rejects(
        client.requestV2('/v2/compressed-error', 'Error'),
        isTooLarge(400),
      );
    },
  );
}

async function finding34PreservesStableErrorsAndRetryLimits() {
  const counts = new Map();
  await withServer(
    (req, res) => {
      counts.set(req.url, (counts.get(req.url) ?? 0) + 1);
      if (req.url === '/v2/network') {
        req.socket.destroy();
        return;
      }
      if (req.url === '/v2/redirect') {
        res.writeHead(302, { Location: '/v2/target' });
        res.end();
        return;
      }
      if (req.url === '/v2/html') {
        res.end('<html>proxy</html>');
        return;
      }
      if (req.url === '/v2/missing') {
        res.writeHead(404);
        res.end('not found');
        return;
      }
      if (req.url === '/v2/malformed') {
        res.writeHead(500);
        res.end('{broken');
        return;
      }
      if (req.url === '/v2/success-after-retry' && counts.get(req.url) === 3) {
        res.end('"42"');
        return;
      }
      res.writeHead(req.url === '/v2/domain-missing' ? 404 : 503, {
        'Retry-After': '0',
      });
      res.end(
        JSON.stringify({
          ...errorEnvelope,
          retryable: req.url !== '/v2/no-retry',
        }),
      );
    },
    async (client) => {
      for (const [path, options, attempts] of [
        ['retry', { idempotentRead: true }, 3],
        ['post', { method: 'POST', idempotentRead: true }, 1],
        ['not-idempotent', {}, 1],
        ['no-retry', { idempotentRead: true }, 1],
        ['domain-missing', { idempotentRead: true }, 1],
      ]) {
        await assert.rejects(
          client.requestV2(`/v2/${path}`, 'Error', options),
          (error) => {
            assert.ok(error instanceof HttpError);
            assert.equal(error.code, errorEnvelope.code);
            assert.equal(error.envelope.message, errorEnvelope.code);
            assert.equal(error.envelope.details, null);
            return true;
          },
        );
        assert.equal(counts.get(`/v2/${path}`), attempts);
      }
      assert.equal(
        await client.requestV2('/v2/success-after-retry', 'UInt256', {
          idempotentRead: true,
        }),
        '42',
      );
      assert.equal(counts.get('/v2/success-after-retry'), 3);
      for (const [path, status, code] of [
        ['redirect', 302, 'UNEXPECTED_REDIRECT'],
        ['html', 200, 'NON_JSON_RESPONSE'],
        ['malformed', 500, 'TRANSPORT_ERROR'],
      ]) {
        await assert.rejects(
          client.requestV2(`/v2/${path}`, 'Error', { idempotentRead: true }),
          (error) => {
            assert.ok(error instanceof HttpError);
            assert.equal(error.status, status);
            assert.equal(error.code, code);
            return true;
          },
        );
        assert.equal(counts.get(`/v2/${path}`), 1);
      }
      assert.equal(counts.has('/v2/target'), false);
      await assert.rejects(
        client.requestV2('/v2/missing', 'Error'),
        (error) =>
          error instanceof CapabilityUnavailableError && error.status === 404,
      );
      await assert.rejects(
        client.requestV2('/v2/network', 'Error', { idempotentRead: true }),
        (error) =>
          error instanceof StreamTransportError &&
          error.code === 'NETWORK_INTERRUPTED',
      );
      assert.equal(counts.get('/v2/network'), 3);
    },
  );
}

async function finding34CancelsUnusedBodiesAndLeavesBinaryStreaming() {
  for (const [major, status, code] of [
    [1, 404, 'capability_unavailable'],
    [1, 500, 'TRANSPORT_ERROR'],
    [2, 501, 'capability_unavailable'],
    [3, 501, 'capability_unavailable'],
    [2, 302, 'UNEXPECTED_REDIRECT'],
  ]) {
    let stream;
    await withServer(
      (req, res) => {
        stream = streamResponse(res, status, JSON_LIMIT);
      },
      async (client) => {
        await assert.rejects(
          client[
            major === 1 ? 'request' : major === 2 ? 'requestV2' : 'requestV3'
          ](`/v${major}/unused`, 'UInt256'),
          (error) => error.code === code && error.status === status,
        );
        await stream.closed;
        assert.equal(stream.finished, false);
        assert.ok(stream.sent < ERROR_LIMIT);
      },
    );
  }
  let stream;
  await withServer(
    (req, res) => {
      if (req.url === '/v2/empty') {
        res.writeHead(204);
        res.end();
        return;
      }
      stream = streamResponse(res, 200, JSON_LIMIT * 2, {
        'Content-Type': 'text/event-stream',
      });
    },
    async (client) => {
      assert.equal(await client.requestV2Empty('/v2/empty', {}), undefined);
      const response = await client.requestGeneratedBinary('/v2/events', {}, 2);
      assert.ok(response instanceof Response);
      assert.equal(response.headers.get('Content-Type'), 'text/event-stream');
      assert.equal(response.bodyUsed, false);
      assert.ok(stream.sent < ERROR_LIMIT);
      await response.body.cancel();
      await stream.closed;
      assert.equal(stream.finished, false);
    },
  );
}

async function finding34PreservesDeadlineAndCallerCancellationWhileReading() {
  for (const status of [200, 400]) {
    for (const cancel of [false, true]) {
      let closed;
      let abortTimer;
      const controller = new AbortController();
      const reason = new DOMException('Caller left', 'AbortError');
      await withServer(
        (req, res) => {
          closed = once(res, 'close');
          res.writeHead(status);
          res.write(' ');
          if (cancel)
            abortTimer = setTimeout(() => controller.abort(reason), 20);
        },
        async (client) => {
          try {
            await assert.rejects(
              client.requestV2('/v2/stalled', 'UInt256', {
                timeoutMs: cancel ? 2000 : 100,
                signal: controller.signal,
              }),
              (error) =>
                cancel ? error === reason : error.name === 'TimeoutError',
            );
            await closed;
          } finally {
            clearTimeout(abortTimer);
          }
        },
      );
    }
  }
}

test(finding34BoundsStreamedSuccessBeforeBuffering);
test(finding34BoundsErrorsWithoutCloningOrRetryingOversize);
test(finding34RejectsDeclaredOversizeBeforeReading);
test(finding34CountsDecodedBytesAndAcceptsExactLimits);
test(finding34PreservesStableErrorsAndRetryLimits);
test(finding34CancelsUnusedBodiesAndLeavesBinaryStreaming);
test(finding34PreservesDeadlineAndCallerCancellationWhileReading);
