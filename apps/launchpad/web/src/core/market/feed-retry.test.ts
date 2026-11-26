// @vitest-environment jsdom
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { act, createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { HttpError, SdkClient } from '@kindle/sdk';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { marketRetryPace } from '@/core/retry-pace';
import { queryKeys } from '@/core/query-keys';
import { transientRetryFallbackMs } from '@/core/transient-retry';
import { MarketClientContext } from './client';
import { useFeed, type FeedPage } from './use-feed';
import type { Loadable } from './unsupported';

let server: ChildProcess;
let serverExit: Promise<number | null>;
let origin: string;
let directory: string;
let output = '';
let root: Root | undefined;
let queries: QueryClient | undefined;
let container: HTMLDivElement | undefined;

beforeAll(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  const path = process.env.JOINED_CONFIG_FILE;
  if (!path) throw new Error('Joined service configuration required');
  const joined = JSON.parse(readFileSync(path, 'utf8')) as { processes: { role: string; env?: { DATABASE_URL: string; REDIS_URL: string } }[] };
  const environment = joined.processes.find(process => process.role === 'api')?.env;
  if (!environment) throw new Error('Joined database configuration required');
  const redis = new URL(environment.REDIS_URL);
  redis.pathname = '/14';
  const moduleRoot = resolve(__dirname, '../../../../../../services/launchpad/api');
  directory = mkdtempSync(resolve(tmpdir(), 'feed-retry-'));
  const overlay = resolve(directory, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(moduleRoot, 'internal/rest/feed_retry_web_test.go')]: resolve(__dirname, '../test-fixtures/market-production/feed-retry_test.go.txt'),
  } }));
  server = spawn('go', ['test', `-overlay=${overlay}`, '-count=1', '-v', '-run=^TestFeedRetryWebService$', './internal/rest'], {
    cwd: moduleRoot, stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, GOWORK: 'off', API_TEST_DATABASE_URL: environment.DATABASE_URL, CORE_API_TEST_POSTGRES_URL: environment.DATABASE_URL, CORE_API_TEST_REDIS_URL: redis.toString() },
  });
  serverExit = new Promise((done, reject) => { server.once('error', reject); server.once('exit', done); });
  server.stderr!.on('data', chunk => { output += String(chunk); });
  const lines = createInterface({ input: server.stdout! });
  origin = await new Promise<string>((done, reject) => {
    lines.on('line', line => {
      output += line + '\n';
      if (line.startsWith('FEED_RETRY_SERVICE ')) done(line.slice('FEED_RETRY_SERVICE '.length));
    });
    void serverExit.then(code => reject(new Error(`Feed service exited ${code}: ${output}`)), reject);
  });
}, 600_000);

afterEach(async () => {
  if (root) await act(async () => root!.unmount());
  await queries?.cancelQueries();
  queries?.clear();
  container?.remove();
  root = undefined;
  queries = undefined;
  container = undefined;
  marketRetryPace.clear();
});

afterAll(async () => {
  try {
    if (origin) {
      expect((await fetch(origin + '/stop', { method: 'POST' })).status).toBe(204);
      expect(await serverExit, output).toBe(0);
    } else server?.kill('SIGTERM');
  } finally {
    if (directory) rmSync(directory, { recursive: true });
  }
});

async function waitFor(predicate: () => boolean) {
  const deadline = Date.now() + 5_000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error('Feed did not settle');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}

function mountFeed() {
  const streamOrigin = origin.replace('http:', 'ws:');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  queries = new QueryClient();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  const states: Loadable<FeedPage>[] = [];
  function Feed() {
    const state = useFeed();
    states.push(state);
    return createElement('p', null, state.state);
  }
  act(() => root!.render(createElement(QueryClientProvider, { client: queries! }, createElement(MarketClientContext.Provider, { value: client }, createElement(Feed)))));
  return states;
}

async function reads(): Promise<{ status: number; at: number }[]> {
  const response = await fetch(origin + '/reads');
  expect(response.status).toBe(200);
  return response.json();
}

async function feedReadIsTriedOnceMoreAfterARetryable503WithoutDelay() {
  expect((await fetch(origin + '/reset/1', { method: 'POST' })).status).toBe(204);
  const states = mountFeed();
  const key = queryKeys.feed('relevance', null, 50);
  await waitFor(() => queries!.getQueryState(key)?.fetchFailureCount === 1);
  const error = queries!.getQueryState(key)?.fetchFailureReason;
  expect(error).toBeInstanceOf(HttpError);
  expect(error).toMatchObject({ status: 503, envelope: { retryable: true, retryAfterMs: null } });
  expect(await reads()).toHaveLength(1);
  expect(states.every(state => state.state === 'loading')).toBe(true);
  expect(container!.textContent).toBe('loading');
  await waitFor(() => states.at(-1)?.state === 'ready');
  const result = states.at(-1);
  expect(result).toMatchObject({ state: 'ready', data: { Entries: [{ Body: 'Public callout', State: 'available' }] } });
  expect(result?.state === 'ready' && result.data).toEqual(queries!.getQueryData(key));
  expect(states.slice(0, -1).every(state => state.state === 'loading')).toBe(true);
  const requests = await reads();
  expect(requests.map(read => read.status)).toEqual([503, 200]);
  expect(requests[1].at - requests[0].at).toBeGreaterThanOrEqual(transientRetryFallbackMs);
}
test(feedReadIsTriedOnceMoreAfterARetryable503WithoutDelay.name, feedReadIsTriedOnceMoreAfterARetryable503WithoutDelay);

async function feedReadShowsItsFailureWhenTheSecondAttemptFails() {
  expect((await fetch(origin + '/reset/2', { method: 'POST' })).status).toBe(204);
  const states = mountFeed();
  const key = queryKeys.feed('relevance', null, 50);
  await waitFor(() => queries!.getQueryState(key)?.fetchFailureCount === 1);
  expect(states.every(state => state.state === 'loading')).toBe(true);
  await waitFor(() => queries!.getQueryState(key)?.status === 'error');
  expect(queries!.getQueryState(key)).toMatchObject({ fetchFailureCount: 2, fetchStatus: 'idle' });
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(states.at(-1)).toMatchObject({ state: 'unsupported', status: 503 });
  expect(container!.textContent).toBe('unsupported');
  expect((await reads()).map(read => read.status)).toEqual([503, 503]);
}
test(feedReadShowsItsFailureWhenTheSecondAttemptFails.name, feedReadShowsItsFailureWhenTheSecondAttemptFails);
