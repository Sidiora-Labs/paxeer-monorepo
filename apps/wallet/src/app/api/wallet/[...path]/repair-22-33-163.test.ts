import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { NextRequest } from 'next/server';
import { afterAll, beforeAll, expect, test, vi } from 'vitest';

type Method = 'GET' | 'HEAD';
const hits = new Map<string, number>();
const holds = new Map<string, Promise<void>>();
const releases = new Set<() => void>();
let active = 0;
let peakActive = 0;
let origin: string;
let directory: string;
let route: typeof import('./route');
const envNames = ['BLOCKSCOUT_UPSTREAM_BASE', 'RATE_LIMIT_STORE_PATH'] as const;
const originalEnv = envNames.map((name) => [name, process.env[name]] as const);

function upstreamPath(id: string): string {
  return `/api/v2/stats/${id}`;
}

function requestKey(method: Method, id: string): string {
  return `${method} ${upstreamPath(id)}`;
}

function count(method: Method, id: string): number {
  return hits.get(requestKey(method, id)) ?? 0;
}

function payload(id: string): string {
  if (id.startsWith('bytes-')) return '€'.repeat(131_000);
  if (id.startsWith('oversize-')) return '€'.repeat(180_000);
  return '';
}

const server = createServer((request, response) => {
  const key = `${request.method} ${request.url}`;
  const id = request.url?.split('/').at(-1) ?? '';
  hits.set(key, (hits.get(key) ?? 0) + 1);
  active += 1;
  peakActive = Math.max(peakActive, active);
  const finish = () => {
    active -= 1;
    const body = JSON.stringify({
      path: request.url,
      method: request.method,
      payload: payload(id),
    });
    response.writeHead(id.startsWith('failure-') ? 503 : 200, {
      'Content-Type': 'application/json; charset=utf-8',
      'Content-Length': Buffer.byteLength(body),
      'Cache-Control': request.method === 'HEAD' ? 'private, max-age=12' : 'private, max-age=11',
    });
    response.end(request.method === 'HEAD' ? undefined : body);
  };
  const held = holds.get(key);
  if (held) void held.then(finish);
  else finish();
});

function hold(method: Method, id: string): () => void {
  let resolve!: () => void;
  const gate = new Promise<void>((release) => {
    resolve = release;
  });
  const key = requestKey(method, id);
  holds.set(key, gate);
  const release = () => {
    holds.delete(key);
    releases.delete(release);
    resolve();
  };
  releases.add(release);
  return release;
}

async function waitForHit(method: Method, id: string): Promise<void> {
  await vi.waitFor(() => expect(count(method, id)).toBe(1), { timeout: 5_000, interval: 10 });
}

function request(method: Method, id: string): Promise<Response> {
  const input = new NextRequest(`${origin}/api/wallet${upstreamPath(id)}`, { method });
  const context = { params: Promise.resolve({ path: ['api', 'v2', 'stats', id] }) };
  return route[method](input, context);
}

async function assertGet(id: string): Promise<void> {
  const response = await request('GET', id);
  expect(response.status).toBe(200);
  expect(response.headers.get('content-type')).toBe('application/json; charset=utf-8');
  expect(response.headers.get('cache-control')).toBe('private, max-age=11');
  expect(await response.json()).toEqual({
    path: upstreamPath(id),
    method: 'GET',
    payload: payload(id),
  });
}

async function assertHead(response: Response): Promise<void> {
  expect(response.status).toBe(200);
  expect(response.headers.get('cache-control')).toBe('private, max-age=12');
  expect(await response.text()).toBe('');
}

beforeAll(async () => {
  directory = await mkdtemp(path.join(tmpdir(), 'wallet-repair-22-33-163-'));
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
  origin = `http://127.0.0.1:${address.port}`;
  process.env.BLOCKSCOUT_UPSTREAM_BASE = origin;
  process.env.RATE_LIMIT_STORE_PATH = path.join(directory, 'rate-limits.json');
  vi.resetModules();
  route = await import('./route');
});

afterAll(async () => {
  for (const release of releases) release();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
  for (const [name, value] of originalEnv) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  if (directory) await rm(directory, { recursive: true, force: true });
});

async function proxy_cache_capacity_evicts_lru_entries() {
  for (let index = 0; index < 256; index += 1) await assertGet(`entries-${index}`);
  await assertGet('entries-0');
  expect(count('GET', 'entries-0')).toBe(1);
  await assertGet('entries-256');
  await assertGet('entries-0');
  expect(count('GET', 'entries-0')).toBe(1);
  await assertGet('entries-1');
  expect(count('GET', 'entries-1')).toBe(2);
  await assertGet('entries-256');
  expect(count('GET', 'entries-256')).toBe(1);
}

async function proxy_cache_capacity_bounds_aggregate_utf8_bytes() {
  // Each body is below 512 KiB, but 25 bodies exceed the 8 MiB aggregate budget.
  expect(Buffer.byteLength(payload('bytes-0'))).toBe(393_000);
  for (let index = 0; index < 25; index += 1) await assertGet(`bytes-${index}`);
  await assertGet('bytes-0');
  expect(count('GET', 'bytes-0')).toBe(2);
  await assertGet('bytes-24');
  expect(count('GET', 'bytes-24')).toBe(1);

  // UTF-8 byte size, rather than JS string length, also enforces the per-entry budget.
  expect(payload('oversize-0').length).toBeLessThan(512 * 1024);
  expect(Buffer.byteLength(payload('oversize-0'))).toBeGreaterThan(512 * 1024);
  await assertGet('oversize-0');
  await assertGet('oversize-0');
  expect(count('GET', 'oversize-0')).toBe(2);
}

async function proxy_cache_capacity_bounds_inflight_and_preserves_deduplication() {
  peakActive = 0;
  const ids = Array.from({ length: 16 }, (_, index) => `inflight-${index}`);
  const release = ids.map((id) => hold('GET', id));
  const pending = ids.map((id) => request('GET', id));
  let duplicate: Promise<Response> | undefined;
  try {
    await Promise.all(ids.map((id) => waitForHit('GET', id)));
    duplicate = request('GET', ids[0]);
    const overflow = await request('GET', 'inflight-overflow');
    expect(overflow.status).toBe(503);
    expect(await overflow.json()).toEqual({
      error: { code: 'UPSTREAM_BUSY', message: 'Wallet data is busy; retry shortly' },
    });
    expect(count('GET', 'inflight-overflow')).toBe(0);

    release[0]();
    for (const response of await Promise.all([pending[0], duplicate])) {
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({
        path: upstreamPath(ids[0]),
        method: 'GET',
        payload: '',
      });
    }
    expect(count('GET', ids[0])).toBe(1);
    await assertGet('inflight-overflow');
    expect(count('GET', 'inflight-overflow')).toBe(1);
    expect(peakActive).toBe(16);
  } finally {
    for (const unblock of release) unblock();
    await Promise.allSettled([...pending, ...(duplicate ? [duplicate] : [])]);
  }
  for (const response of await Promise.all(pending)) expect(response.status).toBe(200);
  for (const id of ids) expect(count('GET', id)).toBe(1);
}

async function proxy_cache_capacity_releases_failed_inflight_slots() {
  const ids = Array.from({ length: 16 }, (_, index) => `failure-${index}`);
  const release = ids.map((id) => hold('GET', id));
  const pending = ids.map((id) => request('GET', id));
  try {
    await Promise.all(ids.map((id) => waitForHit('GET', id)));
  } finally {
    for (const unblock of release) unblock();
    await Promise.allSettled(pending);
  }
  for (const response of await Promise.all(pending)) {
    expect(response.status).toBe(503);
    expect((await response.json()).method).toBe('GET');
  }
  await assertGet('after-failure');
  const retry = await request('GET', ids[0]);
  expect(retry.status).toBe(503);
  expect(count('GET', ids[0])).toBe(2);
}

async function proxy_head_get_collision_head_before_get() {
  const id = 'head-first';
  await assertHead(await request('HEAD', id));
  await assertGet(id);
  await assertHead(await request('HEAD', id));
  await assertGet(id);
  expect(count('HEAD', id)).toBe(1);
  expect(count('GET', id)).toBe(1);
}

async function concurrentMethods(first: Method, second: Method, id: string): Promise<void> {
  const releaseFirst = hold(first, id);
  const releaseSecond = hold(second, id);
  const pending: Array<Promise<Response>> = [];
  try {
    pending.push(request(first, id));
    await waitForHit(first, id);
    pending.push(request(second, id));
    await waitForHit(second, id);
  } finally {
    releaseFirst();
    releaseSecond();
    await Promise.allSettled(pending);
  }
  const responses = await Promise.all(pending);
  const head = responses[first === 'HEAD' ? 0 : 1];
  const get = responses[first === 'GET' ? 0 : 1];
  await assertHead(head);
  expect(get.status).toBe(200);
  expect(get.headers.get('cache-control')).toBe('private, max-age=11');
  expect(await get.json()).toEqual({ path: upstreamPath(id), method: 'GET', payload: '' });
  await assertGet(id);
  await assertHead(await request('HEAD', id));
  expect(count('HEAD', id)).toBe(1);
  expect(count('GET', id)).toBe(1);
}

async function proxy_head_get_collision_concurrent_head_then_get() {
  await concurrentMethods('HEAD', 'GET', 'concurrent-head-first');
}

async function proxy_head_get_collision_concurrent_get_then_head() {
  await concurrentMethods('GET', 'HEAD', 'concurrent-get-first');
}

test(proxy_cache_capacity_evicts_lru_entries, proxy_cache_capacity_evicts_lru_entries, 30_000);
test(
  proxy_cache_capacity_bounds_aggregate_utf8_bytes,
  proxy_cache_capacity_bounds_aggregate_utf8_bytes,
  30_000
);
test(
  proxy_cache_capacity_bounds_inflight_and_preserves_deduplication,
  proxy_cache_capacity_bounds_inflight_and_preserves_deduplication,
  30_000
);
test(
  proxy_cache_capacity_releases_failed_inflight_slots,
  proxy_cache_capacity_releases_failed_inflight_slots,
  30_000
);
test(proxy_head_get_collision_head_before_get, proxy_head_get_collision_head_before_get, 30_000);
test(
  proxy_head_get_collision_concurrent_head_then_get,
  proxy_head_get_collision_concurrent_head_then_get,
  30_000
);
test(
  proxy_head_get_collision_concurrent_get_then_head,
  proxy_head_get_collision_concurrent_get_then_head,
  30_000
);
