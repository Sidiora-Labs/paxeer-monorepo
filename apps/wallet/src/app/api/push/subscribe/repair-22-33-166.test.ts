import { mkdtemp, readFile, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { Wallet } from 'ethers';
import { NextRequest } from 'next/server';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { pushOwnershipMessage } from '@/server/push-ownership';
import { pushUnsubscribeMessage } from './unsubscribe-intent';

const owner = new Wallet(`0x${'11'.repeat(32)}`);
const other = new Wallet(`0x${'22'.repeat(32)}`);
const originalDataDirectory = process.env.PUSH_DATA_DIR;
let directory: string;
let routes: typeof import('./route');
let store: typeof import('@/server/push-store');

beforeAll(async () => {
  directory = await mkdtemp(path.join(os.tmpdir(), 'repair-22-33-166-'));
  process.env.PUSH_DATA_DIR = directory;
  routes = await import('./route');
  store = await import('@/server/push-store');
});

afterAll(async () => {
  if (originalDataDirectory === undefined) delete process.env.PUSH_DATA_DIR;
  else process.env.PUSH_DATA_DIR = originalDataDirectory;
  if (directory) await rm(directory, { recursive: true, force: true });
});

function request(method: 'POST' | 'DELETE', body: Record<string, unknown>): NextRequest {
  return new NextRequest('https://wallet.example.test/wallet/api/push/subscribe', {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

async function subscribe(endpoint: string, wallet = owner): Promise<void> {
  const issuedAt = Math.floor(Date.now() / 1_000);
  const response = await routes.POST(
    request('POST', {
      endpoint,
      keys: { p256dh: 'a'.repeat(87), auth: 'b'.repeat(22) },
      walletAddress: wallet.address,
      issuedAt,
      signature: await wallet.signMessage(pushOwnershipMessage(endpoint, wallet.address, issuedAt)),
    })
  );
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ ok: true });
  expect(await store.getSubscription(endpoint)).toMatchObject({
    endpoint,
    walletAddress: wallet.address.toLowerCase(),
  });
}

async function proof(endpoint: string, wallet = owner, issuedAt = Math.floor(Date.now() / 1_000)) {
  return {
    endpoint,
    walletAddress: wallet.address,
    issuedAt,
    signature: await wallet.signMessage(pushUnsubscribeMessage(endpoint, wallet.address, issuedAt)),
  };
}

async function refusesWithoutMutation(body: Record<string, unknown>): Promise<void> {
  const storePath = path.join(directory, 'subscriptions.json');
  const before = await readFile(storePath, 'utf8');
  const response = await routes.DELETE(request('DELETE', body));
  expect(response.status).toBe(401);
  expect(response.headers.get('cache-control')).toBe('no-store');
  expect(await response.json()).toMatchObject({ error: { code: 'SUBSCRIPTION_UNPROVEN' } });
  expect(await readFile(storePath, 'utf8')).toBe(before);
}

async function push_delete_ownership_requires_proof() {
  const endpoint = 'https://push.example.test/subscriptions/unsigned';
  await subscribe(endpoint);
  await refusesWithoutMutation({ endpoint });
  expect(await store.getSubscription(endpoint)).not.toBeNull();
}

async function push_delete_ownership_refuses_different_wallet() {
  const endpoint = 'https://push.example.test/subscriptions/foreign';
  await subscribe(endpoint);
  await refusesWithoutMutation(await proof(endpoint, other));
  const forgedOwner = await proof(endpoint);
  forgedOwner.signature = await other.signMessage(
    pushUnsubscribeMessage(endpoint, owner.address, forgedOwner.issuedAt)
  );
  await refusesWithoutMutation(forgedOwner);
  expect((await store.getSubscription(endpoint))?.walletAddress).toBe(owner.address.toLowerCase());
}

async function push_delete_ownership_binds_operation_endpoint_owner_and_time() {
  const endpoint = 'https://push.example.test/subscriptions/binding';
  const secondEndpoint = `${endpoint}/second`;
  await subscribe(endpoint);
  await subscribe(secondEndpoint);
  const valid = await proof(endpoint);
  await refusesWithoutMutation({ ...valid, endpoint: secondEndpoint });
  await refusesWithoutMutation({ ...valid, walletAddress: other.address });
  await refusesWithoutMutation({ ...valid, issuedAt: valid.issuedAt + 1 });
  await refusesWithoutMutation({
    ...valid,
    signature: await owner.signMessage(
      pushOwnershipMessage(endpoint, owner.address, valid.issuedAt)
    ),
  });
  for (const issuedAt of [valid.issuedAt - 301, valid.issuedAt + 120, valid.issuedAt + 0.5]) {
    await refusesWithoutMutation(await proof(endpoint, owner, issuedAt));
  }
  for (const signature of ['', 'invalid', `0x${'00'.repeat(65)}`]) {
    await refusesWithoutMutation({ ...valid, signature });
  }
  for (const missing of ['signature', 'issuedAt', 'walletAddress']) {
    const incomplete: Record<string, unknown> = { ...valid };
    delete incomplete[missing];
    await refusesWithoutMutation(incomplete);
  }
  expect(await store.getSubscription(endpoint)).not.toBeNull();
  expect(await store.getSubscription(secondEndpoint)).not.toBeNull();
}

async function push_delete_ownership_accepts_only_current_stored_owner() {
  const endpoint = 'https://push.example.test/subscriptions/transferred';
  await subscribe(endpoint);
  const previousOwnerProof = await proof(endpoint);
  await subscribe(endpoint, other);
  await refusesWithoutMutation(previousOwnerProof);
  const response = await routes.DELETE(request('DELETE', await proof(endpoint, other)));
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ ok: true, removed: true });
  expect(await store.getSubscription(endpoint)).toBeNull();
}

async function push_delete_ownership_accepts_fresh_unsubscribe_intent() {
  const untouchedEndpoint = 'https://push.example.test/subscriptions/untouched';
  await subscribe(untouchedEndpoint, other);
  for (const [index, offset] of [0, -240, 30].entries()) {
    const endpoint = `https://push.example.test/subscriptions/valid/${index}`;
    await subscribe(endpoint);
    const signed = await proof(endpoint, owner, Math.floor(Date.now() / 1_000) + offset);
    const response = await routes.DELETE(request('DELETE', signed));
    expect(response.status).toBe(200);
    expect(response.headers.get('cache-control')).toBe('no-store');
    expect(await response.json()).toEqual({ ok: true, removed: true });
    expect(await store.getSubscription(endpoint)).toBeNull();
  }
  expect((await store.getSubscription(untouchedEndpoint))?.walletAddress).toBe(
    other.address.toLowerCase()
  );
}

test(push_delete_ownership_requires_proof.name, push_delete_ownership_requires_proof);
test(
  push_delete_ownership_refuses_different_wallet.name,
  push_delete_ownership_refuses_different_wallet
);
test(
  push_delete_ownership_binds_operation_endpoint_owner_and_time.name,
  push_delete_ownership_binds_operation_endpoint_owner_and_time
);
test(
  push_delete_ownership_accepts_only_current_stored_owner.name,
  push_delete_ownership_accepts_only_current_stored_owner
);
test(
  push_delete_ownership_accepts_fresh_unsubscribe_intent.name,
  push_delete_ownership_accepts_fresh_unsubscribe_intent
);
