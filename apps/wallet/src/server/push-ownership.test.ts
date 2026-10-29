import { mkdtemp, readFile, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { Wallet } from 'ethers';
import { NextRequest } from 'next/server';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { HttpBoundaryError } from './http';
import { pushOwnershipMessage, verifyPushOwnership } from './push-ownership';

let directory: string;
let post: typeof import('../app/api/push/subscribe/route').POST;
let getSubscription: typeof import('./push-store').getSubscription;
const originalDataDirectory = process.env.PUSH_DATA_DIR;

beforeAll(async () => {
  directory = await mkdtemp(path.join(os.tmpdir(), 'paxport-push-ownership-'));
  process.env.PUSH_DATA_DIR = directory;
  ({ POST: post } = await import('../app/api/push/subscribe/route'));
  ({ getSubscription } = await import('./push-store'));
});

afterAll(async () => {
  if (originalDataDirectory === undefined) delete process.env.PUSH_DATA_DIR;
  else process.env.PUSH_DATA_DIR = originalDataDirectory;
  await rm(directory, { recursive: true, force: true });
});

function subscriptionRequest(input: Record<string, unknown>): NextRequest {
  return new NextRequest('https://wallet.example/wallet/api/push/subscribe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      keys: { p256dh: 'a'.repeat(87), auth: 'b'.repeat(22) },
      ...input,
    }),
  });
}

async function acceptsSubscriptionSignedByWallet(): Promise<void> {
  const wallet = Wallet.createRandom();
  const endpoint = 'https://push.example/subscriptions/owned';
  const now = Math.floor(Date.now() / 1_000);
  for (const issuedAt of [now, now - 300, now + 60]) {
    const input = {
      endpoint,
      walletAddress: wallet.address,
      issuedAt,
      signature: await wallet.signMessage(pushOwnershipMessage(endpoint, wallet.address, issuedAt)),
    };
    expect(() => verifyPushOwnership(input, now)).not.toThrow();
    expect(() =>
      verifyPushOwnership({ ...input, walletAddress: wallet.address.toLowerCase() }, now)
    ).not.toThrow();
  }
  const input = {
    endpoint,
    walletAddress: wallet.address,
    issuedAt: now,
    signature: await wallet.signMessage(pushOwnershipMessage(endpoint, wallet.address, now)),
  };
  const response = await post(subscriptionRequest(input));
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ ok: true });
  expect(await getSubscription(endpoint)).toMatchObject({
    endpoint,
    walletAddress: wallet.address.toLowerCase(),
  });
}

async function refusesSubscriptionSignedByAnotherKey(): Promise<void> {
  const wallet = Wallet.createRandom();
  const foreignWallet = Wallet.createRandom();
  const now = Math.floor(Date.now() / 1_000);
  const storePath = path.join(directory, 'subscriptions.json');
  const before = await readFile(storePath, 'utf8').catch((error: NodeJS.ErrnoException) => {
    if (error.code === 'ENOENT') return null;
    throw error;
  });
  for (const [kind, signer, issuedAt] of [
    ['foreign', foreignWallet, now],
    ['stale', wallet, now - 301],
    ['future', wallet, now + 120],
  ] as const) {
    const endpoint = `https://push.example/subscriptions/${kind}`;
    const input = {
      endpoint,
      walletAddress: wallet.address,
      issuedAt,
      signature: await signer.signMessage(pushOwnershipMessage(endpoint, wallet.address, issuedAt)),
    };
    expect(() => verifyPushOwnership(input, now)).toThrow(HttpBoundaryError);
    expect(() => verifyPushOwnership(input, now)).toThrow(
      expect.objectContaining({
        status: 401,
        code: 'SUBSCRIPTION_UNPROVEN',
      })
    );
    const response = await post(subscriptionRequest(input));
    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({ error: { code: 'SUBSCRIPTION_UNPROVEN' } });
    expect(await getSubscription(endpoint)).toBeNull();
  }
  const endpoint = 'https://push.example/subscriptions/tampered';
  const signature = await wallet.signMessage(pushOwnershipMessage(endpoint, wallet.address, now));
  for (const proof of [
    { signature, issuedAt: now, endpoint: `${endpoint}/changed` },
    { signature, issuedAt: now + 1 },
    { signature: 'invalid', issuedAt: now },
    { signature, issuedAt: 'now' },
    { signature, issuedAt: null },
    { issuedAt: now },
    { signature },
  ]) {
    const response = await post(
      subscriptionRequest({ endpoint, walletAddress: wallet.address, ...proof })
    );
    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({ error: { code: 'SUBSCRIPTION_UNPROVEN' } });
  }
  const after = await readFile(storePath, 'utf8').catch((error: NodeJS.ErrnoException) => {
    if (error.code === 'ENOENT') return null;
    throw error;
  });
  expect(after).toBe(before);
}

test('accepts a subscription signed by the wallet', acceptsSubscriptionSignedByWallet);
test('refuses a subscription signed by another key', refusesSubscriptionSignedByAnotherKey);
