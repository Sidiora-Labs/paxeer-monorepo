import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { operationDefinitions } from '../../../../../../sdk/launchpad/packages/sdk/src/operations.gen';
import { ROUTE_STATUS } from '../../../../../../sdk/launchpad/packages/sdk/src/served-routes';
import { prepareRewardClaim } from './rewards-view';
import { AccountApiError, accountEpoch, clearAccountSession, installAccountSession, requireAccount } from '../profile/account-transport';

interface Observation {
  method: string;
  path: string;
  authorization: string;
  contentType: string;
  keys: string[];
  body: string;
  validBody: boolean;
  request: { rewardIds: string[]; recipient: string };
  status: number;
  code: string;
}

const owner = '0x1111111111111111111111111111111111111111';
const previousOrigin = process.env.NEXT_PUBLIC_PNL_API_URL;
const observations: Observation[] = [];
let server: ChildProcess;
let origin: string;
let serverExit: Promise<number | null>;
let output = '';
let overlayDirectory: string | undefined;

beforeAll(async () => {
  const repository = resolve(__dirname, '../../../../../..');
  const moduleRoot = resolve(repository, 'services/launchpad/pnl-tracker');
  // Overlay adds only this test to the real package; every production file and
  // every existing test remains intact. The overlay stays inside this task's
  // scope, never in the service's directory.
  const overlay = { Replace: {
    [resolve(moduleRoot, 'internal/app/web_reward_claim_proof_test.go')]: resolve(__dirname, 'registered-reward-handler_test.go'),
  } };
  overlayDirectory = mkdtempSync(resolve(__dirname, '.reward-route-'));
  const overlayPath = resolve(overlayDirectory, 'overlay.json');
  writeFileSync(overlayPath, JSON.stringify(overlay));
  server = spawn('go', ['test', `-overlay=${overlayPath}`, '-count=1', '-v', '-run=^TestWebRewardClaimHandler$', './internal/app'], {
    cwd: moduleRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  serverExit = new Promise((done, reject) => {
    server.once('error', reject);
    server.once('exit', done);
  });
  server.stderr!.on('data', chunk => { output += String(chunk); });
  const lines = createInterface({ input: server.stdout! });
  origin = await new Promise<string>((done, reject) => {
    lines.on('line', line => {
      output += `${line}\n`;
      if (line.startsWith('REWARD_ORIGIN ')) done(line.slice('REWARD_ORIGIN '.length));
      if (line.startsWith('REWARD_OBSERVATION ')) observations.push(JSON.parse(line.slice('REWARD_OBSERVATION '.length)) as Observation);
    });
    void serverExit.then(code => reject(new Error(`Registered handler exited ${code}:\n${output}`)), reject);
  });
  process.env.NEXT_PUBLIC_PNL_API_URL = origin;
}, 600_000);

afterAll(async () => {
  clearAccountSession(false);
  if (previousOrigin === undefined) delete process.env.NEXT_PUBLIC_PNL_API_URL;
  else process.env.NEXT_PUBLIC_PNL_API_URL = previousOrigin;
  try {
    if (origin) {
      const stopped = await fetch(`${origin}/__reward_proof/stop`, { method: 'POST' });
      expect(stopped.status).toBe(204);
      expect(await serverExit, output).toBe(0);
    } else if (server) server.kill('SIGTERM');
  } finally {
    if (overlayDirectory) rmSync(overlayDirectory, { recursive: true });
  }
});

async function reward_claim_route_reaches_registered_handler() {
  expect(ROUTE_STATUS.IF_REWARD_CLAIM).toBe('served');
  expect(operationDefinitions.IF_REWARD_CLAIM).toMatchObject({
    method: 'POST', path: '/v2/me/rewards:prepare-claim',
    authenticated: true, idempotencyKey: true, body: 'RewardClaimBody',
  });
  // The screen must continue to invoke the tested producer, including the
  // selected reward and its current authenticated scope.
  const view = readFileSync(resolve(__dirname, 'rewards-view.tsx'), 'utf8');
  expect(view).toMatch(/await prepareRewardClaim\(row.rewardId,\s*scope\)/);
  installAccountSession({
    wallet: owner, sessionId: 'reward-route-owner', deviceLabel: 'Web browser',
    accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(),
    refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(),
    revokedAt: null, scopes: ['account'],
  }, 'reward-route-access', accountEpoch());
  const scope = requireAccount(owner);
  for (let attempt = 0; attempt < 2; attempt++) {
    await expect(prepareRewardClaim('creator:1', scope)).rejects.toMatchObject({
      status: 503, code: 'DEPENDENCY_UNAVAILABLE',
    });
  }
  await expect.poll(() => observations.length).toBe(2);
  const keys: string[] = [];
  for (const observed of observations) {
    expect(observed).toMatchObject({
      method: operationDefinitions.IF_REWARD_CLAIM.method,
      path: operationDefinitions.IF_REWARD_CLAIM.path,
      authorization: 'Bearer reward-route-access', contentType: 'application/json',
      validBody: true, status: 503, code: 'DEPENDENCY_UNAVAILABLE',
      request: { rewardIds: ['creator:1'], recipient: owner },
    });
    expect(JSON.parse(observed.body)).toEqual({ rewardIds: ['creator:1'], recipient: owner });
    expect(observed.keys).toHaveLength(1);
    expect(observed.keys[0]).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i);
    keys.push(observed.keys[0]);
  }
  expect(new Set(keys).size).toBe(2);
  const legacy = await fetch(`${origin}/v2/me/rewards/claim`, {
    method: 'POST', headers: { Authorization: `Bearer ${scope.accessToken}`, 'Idempotency-Key': crypto.randomUUID(), 'Content-Type': 'application/json' },
    body: observations[0].body,
  });
  expect(legacy.status).toBe(404);
  clearAccountSession(false);
  await expect(prepareRewardClaim('creator:1', scope)).rejects.toBeInstanceOf(AccountApiError);
  await expect(prepareRewardClaim('creator:1', scope)).rejects.toMatchObject({ code: 'ACCOUNT_CHANGED' });
  // Both stale-owner attempts must be rejected before reaching HTTP.
  expect(observations.filter(item => item.path === operationDefinitions.IF_REWARD_CLAIM.path)).toHaveLength(2);
}

test(reward_claim_route_reaches_registered_handler.name, reward_claim_route_reaches_registered_handler);
