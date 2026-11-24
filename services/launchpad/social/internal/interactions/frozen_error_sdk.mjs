import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { stripTypeScriptTypes } from 'node:module';

const [root, mode, origin, path, method, requestId, status, code, message, retryable] = process.argv.slice(2);
if (mode === 'prepare') {
  for (const name of ['transport', 'validate']) {
    const source = readFileSync(resolve(root, 'src', `${name}.ts`), 'utf8');
    writeFileSync(resolve(root, 'dist', `${name}.js`), stripTypeScriptTypes(source, { mode: 'transform' }));
  }
} else {
  const { decodeV2Contract, SdkClient, HttpError } = await import(pathToFileURL(resolve(root, 'dist/transport.js')));
  const response = await fetch(new URL(path, origin), { method, headers: { 'X-Request-ID': requestId } });
  assert.equal(response.status, Number(status));
  assert.equal(response.headers.get('X-Request-ID'), requestId);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.match(response.headers.get('Vary'), /Cookie/);
  assert.match(response.headers.get('Vary'), /Authorization/);
  const value = decodeV2Contract('Error', await response.json());
  assert.deepEqual(value, { code, message, requestId, retryable: retryable === 'true', retryAfterMs: null, fieldErrors: [], details: null });
  const stream = origin.replace('http:', 'ws:');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
  const error = await client.requestV2(path, 'FollowResponse', { method, requestId }).then(() => assert.fail('follow unexpectedly succeeded'), error => error);
  assert.ok(error instanceof HttpError, String(error));
  assert.equal(error.status, Number(status));
  assert.equal(error.code, code);
  assert.ok(error.envelope, 'SDK discarded the frozen error envelope');
  assert.equal(error.envelope.requestId, requestId);
  assert.equal(error.envelope.retryable, retryable === 'true');
  assert.equal(error.envelope.retryAfterMs, null);
  assert.deepEqual(error.envelope.fieldErrors, []);
  assert.equal(error.envelope.details, null);
}
