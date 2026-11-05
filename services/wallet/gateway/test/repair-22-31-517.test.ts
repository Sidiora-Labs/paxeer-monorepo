import Fastify, { type FastifyInstance } from 'fastify';
import { randomUUID } from 'node:crypto';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, recoverMessageAddress, recoverTransactionAddress, type Hex } from 'viem';
import { RateLimiter } from '../src/audit.js';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';
import { TestChain } from './support/chain.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';

const recipient = '0x0000000000000000000000000000000000000517';
const chain = new TestChain(125);
const apps: FastifyInstance[] = [];
let engine: PGlite;
let socket: PGLiteSocketServer;
let identity: IdentityProvider;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');
let configuration: typeof import('../src/env.js');
let rpc: import('../src/rpc/pool.js').RpcPool;
let nonces: import('../src/nonce/store.js').NonceStore;
let app: FastifyInstance;

async function mount(clientPerMinute = 120, accountPerMinute = 60): Promise<FastifyInstance> {
  const { signRoutes } = await import('../src/routes/sign.js');
  const { evmApprovalRoutes } = await import('../src/routes/evmApprovals.js');
  const pool = database.getPool();
  const limiter = new RateLimiter({ pool, clientPerMinute, accountPerMinute });
  const instance = Fastify();
  apps.push(instance);
  await instance.register(signRoutes, { pool, rpc, nonces, limiter, attestors: null });
  await evmApprovalRoutes(instance, pool, null, limiter, rpc);
  await instance.ready();
  return instance;
}

beforeAll(async () => {
  engine = await PGlite.create({ extensions: { pgcrypto } });
  socket = new PGLiteSocketServer({ db: engine, host: '127.0.0.1', port: 0 });
  await socket.start();
  identity = await startIdentityProvider();
  await chain.start();
  configuration = await import('../src/env.js');
  configuration.env.DATABASE_POOL_MAX = 1;
  configuration.env.DATABASE_URL = `postgres://postgres:postgres@${socket.getServerConn()}/postgres`;
  configuration.env.SUPABASE_URL = identity.url;
  database = await import('../src/db/pool.js');
  expect(database.getPool().options.max).toBe(1);
  const { runMigrations } = await import('../src/db/migrate.js');
  await runMigrations();
  wallets = await import('../src/db/wallets.js');
  const { RpcPool } = await import('../src/rpc/pool.js');
  const { NonceStore } = await import('../src/nonce/store.js');
  rpc = new RpcPool({
    urls: [chain.url],
    sequencerUrl: chain.url,
    chainId: 125,
    lagThresholdBlocks: 20,
    timeoutMs: 1_000,
    healthIntervalMs: 60_000,
  });
  await rpc.checkHealth();
  nonces = new NonceStore({
    pool: database.getPool(),
    chainId: 125,
    pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
    lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
  });
  app = await mount();
}, 30_000);

afterAll(async () => {
  for (const instance of apps) await instance.close();
  rpc?.stop();
  await database?.closePool();
  await socket?.stop();
  await engine?.close();
  await chain.stop();
  await identity?.stop();
}, 30_000);

async function caller() {
  const userId = randomUUID();
  const { row } = await wallets.provisionWalletForUser(userId);
  const token = await identity.mintUserToken(userId);
  chain.balances.set(row.address.toLowerCase(), 10n ** 20n);
  return { userId, row, headers: { authorization: `Bearer ${token}` } };
}

function transaction(value = '1', nonce?: number) {
  return {
    to: recipient,
    value,
    gas: '21000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
    ...(nonce === undefined ? {} : { nonce }),
  };
}

async function walletSendCompletesWithOneConnection(): Promise<void> {
  for (const nonce of [undefined, 0]) {
    const user = await caller();
    const before = chain.sent.length;
    const reply = await app.inject({
      method: 'POST',
      url: '/v1/wallet/send',
      headers: user.headers,
      payload: { tx: transaction('1', nonce) },
    });
    expect(reply.statusCode, reply.body).toBe(200);
    expect(chain.sent).toHaveLength(before + 1);
    const sent = chain.sent[before]!;
    expect(sent).toMatchObject({
      from: user.row.address.toLowerCase(),
      to: recipient,
      value: 1n,
      nonce: 0,
    });
    expect(reply.json()).toMatchObject({ tx_hash: sent.hash, rung: 'receipted' });
    const retained = await database.query(
      `select r.signed_evidence, r.tx_hash, s.value_wei::text
       from wallet_signature_reservations r join wallet_signatures s on s.id=r.signature_id
       where r.wallet_id=$1`,
      [user.row.id]
    );
    expect(retained.rows).toHaveLength(1);
    expect(retained.rows[0]).toMatchObject({ tx_hash: sent.hash, value_wei: '1' });
    const raw = retained.rows[0]!.signed_evidence as Hex;
    expect(keccak256(raw)).toBe(sent.hash);
    expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(
      user.row.address.toLowerCase()
    );
    expect(
      (
        await database.query('select tx_hash from nonce_outstanding where address=$1', [
          user.row.address.toLowerCase(),
        ])
      ).rows
    ).toEqual([{ tx_hash: sent.hash }]);
    expect(
      (
        await database.query(
          'select phase,evidence from wallet_approval_recovery where domain=$1 and binding=$2',
          ['human', sent.hash]
        )
      ).rows
    ).toEqual([
      {
        phase: 'signed',
        evidence: { raw, tx_hash: sent.hash, signature_id: expect.any(String) },
      },
    ]);
    expect(
      (
        await database.query(
          "select decision from signing_audit where wallet_id=$1 and route='/v1/wallet/send'",
          [user.row.id]
        )
      ).rows
    ).toEqual([{ decision: 'broadcast' }]);
  }
}

async function walletSendKeepsClientAccountPolicyRefusalOrder(): Promise<void> {
  const instance = await mount(1, 1);
  const limiter = new RateLimiter({
    pool: database.getPool(),
    clientPerMinute: 1,
    accountPerMinute: 1,
  });
  const overCap = (configuration.env.POLICY_MAX_TX_VALUE_WEI + 1n).toString();
  const before = chain.sent.length;
  const clientLimited = await caller();
  await limiter.consume('client', clientLimited.userId);
  await limiter.consume('account', clientLimited.row.address);
  const clientReply = await instance.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: clientLimited.headers,
    payload: { tx: transaction(overCap) },
  });
  expect(clientReply.statusCode, clientReply.body).toBe(429);
  expect(clientReply.json()).toMatchObject({
    error: 'rate_limited',
    scope: 'client',
    limit_per_minute: 1,
  });
  expect(Number(clientReply.headers['retry-after'])).toBeGreaterThan(0);
  expect(
    (
      await database.query(
        "select hits from rate_limit_windows where scope='account' and subject=$1",
        [clientLimited.row.address.toLowerCase()]
      )
    ).rows
  ).toEqual([{ hits: 1 }]);

  const accountLimited = await caller();
  await limiter.consume('account', accountLimited.row.address);
  const accountReply = await instance.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: accountLimited.headers,
    payload: { tx: transaction(overCap) },
  });
  expect(accountReply.statusCode, accountReply.body).toBe(429);
  expect(accountReply.json()).toMatchObject({
    error: 'rate_limited',
    scope: 'account',
    limit_per_minute: 1,
  });
  expect(Number(accountReply.headers['retry-after'])).toBeGreaterThan(0);
  expect(
    (
      await database.query(
        "select hits from rate_limit_windows where scope='account' and subject=$1",
        [accountLimited.row.address.toLowerCase()]
      )
    ).rows
  ).toEqual([{ hits: 2 }]);

  const policyLimited = await caller();
  const policyReply = await instance.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: policyLimited.headers,
    payload: { tx: transaction(overCap) },
  });
  expect(policyReply.statusCode, policyReply.body).toBe(403);
  expect(policyReply.json()).toMatchObject({ error: 'TX_VALUE_CAP' });
  expect(
    (
      await database.query(
        'select scope,hits from rate_limit_windows where subject in ($1,$2) order by scope',
        [policyLimited.userId, policyLimited.row.address.toLowerCase()]
      )
    ).rows
  ).toEqual([
    { scope: 'account', hits: 1 },
    { scope: 'client', hits: 1 },
  ]);
  expect(
    (
      await database.query('select id from wallet_signatures where user_id=any($1::uuid[])', [
        [clientLimited.userId, accountLimited.userId, policyLimited.userId],
      ])
    ).rows
  ).toEqual([]);
  expect(chain.sent).toHaveLength(before);
}

async function walletSendKeepsMissingArchivedDisabledResponses(): Promise<void> {
  const missing = randomUUID();
  const missingReply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: { authorization: `Bearer ${await identity.mintUserToken(missing)}` },
    payload: { tx: transaction() },
  });
  expect(missingReply.statusCode, missingReply.body).toBe(404);
  expect(missingReply.json()).toMatchObject({ error: 'no_wallet' });
  const archived = await caller();
  await database.query('update wallets set archived_at=now() where id=$1', [archived.row.id]);
  const archivedReply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: archived.headers,
    payload: { tx: transaction() },
  });
  expect(archivedReply.statusCode, archivedReply.body).toBe(410);
  expect(archivedReply.json()).toMatchObject({
    error: 'wallet_archived',
    wallet_id: archived.row.id,
  });
  const disabled = await caller();
  await database.query(
    "update wallets set is_disabled=true,disabled_reason='test refusal' where id=$1",
    [disabled.row.id]
  );
  const disabledReply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: disabled.headers,
    payload: { tx: transaction() },
  });
  expect(disabledReply.statusCode, disabledReply.body).toBe(403);
  expect(disabledReply.json()).toMatchObject({ error: 'WALLET_DISABLED', message: 'test refusal' });
}

async function messageSigningCommitsAccountingOnOneConnection(): Promise<void> {
  const user = await caller();
  const message = 'single connection signing proof';
  const reply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/sign-message',
    headers: user.headers,
    payload: { message },
  });
  expect(reply.statusCode, reply.body).toBe(200);
  expect(
    (await recoverMessageAddress({ message, signature: reply.json().signature })).toLowerCase()
  ).toBe(user.row.address.toLowerCase());
  expect(
    (
      await database.query(
        'select signed_evidence from wallet_signature_reservations where wallet_id=$1',
        [user.row.id]
      )
    ).rows
  ).toEqual([{ signed_evidence: reply.json().signature }]);
}

async function lxApprovalLimiterAndRecoveryCompleteWithOneConnection(): Promise<void> {
  const userId = randomUUID();
  const headers = { authorization: `Bearer ${await identity.mintUserToken(userId)}` };
  const id = randomUUID();
  const reply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/lx/approve',
    headers,
    payload: {
      review_id: id,
      activity: '000110010b',
      signing_preimage: '00'.repeat(32),
      disclosure: {},
      expires_at: String(Math.floor(Date.now() / 1000) + 600),
      decision: 'approve',
    },
  });
  expect(reply.statusCode, reply.body).toBe(403);
  expect(reply.json()).toEqual({ error: 'original_lx_identity_unavailable' });
  expect(
    (
      await database.query(
        "select hits from rate_limit_windows where scope='client' and subject=$1",
        [userId]
      )
    ).rows
  ).toEqual([{ hits: 1 }]);
  const recovered = await app.inject({
    method: 'GET',
    url: `/v1/wallet/lx/approvals/${id}`,
    headers,
  });
  expect(recovered.statusCode, recovered.body).toBe(403);
  expect(recovered.json()).toEqual({ error: 'original_lx_identity_unavailable' });
}

async function evmApprovalAndRecoveryCompleteWithOneConnection(): Promise<void> {
  const userId = randomUUID();
  const headers = { authorization: `Bearer ${await identity.mintUserToken(userId)}` };
  const id = randomUUID();
  const reply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/evm/approve',
    headers,
    payload: { review_id: id, signing_hash: `0x${'00'.repeat(32)}`, decision: 'approve' },
  });
  expect(reply.statusCode, reply.body).toBe(404);
  expect(reply.json()).toEqual({ error: 'evm_approval_not_found' });
  const recovered = await app.inject({
    method: 'GET',
    url: `/v1/wallet/evm/approvals/${id}`,
    headers,
  });
  expect(recovered.statusCode, recovered.body).toBe(404);
  expect(recovered.json()).toEqual({ error: 'evm_approval_not_found' });
}

async function parallelSendsKeepDistinctNoncesOnOneConnection(): Promise<void> {
  const user = await caller();
  const replies = await Promise.all(
    [1, 2].map((value) =>
      app.inject({
        method: 'POST',
        url: '/v1/wallet/send',
        headers: user.headers,
        payload: { tx: transaction(String(value)) },
      })
    )
  );
  for (const reply of replies) expect(reply.statusCode, reply.body).toBe(200);
  const sent = chain.sent.filter((tx) => tx.from === user.row.address.toLowerCase());
  expect(sent.map((tx) => tx.nonce).sort()).toEqual([0, 1]);
  expect(sent.map((tx) => tx.value).sort()).toEqual([1n, 2n]);
  expect(
    (
      await database.query(
        'select value_wei::text from wallet_signatures where wallet_id=$1 order by value_wei',
        [user.row.id]
      )
    ).rows
  ).toEqual([{ value_wei: '1' }, { value_wei: '2' }]);
}

async function sharedWalletLockReusesClientForDurableSigning(): Promise<void> {
  const { withWalletLock } = await import('../src/util/walletLock.js');
  const user = await caller();
  const message = 'shared route signing session';
  const signature = await withWalletLock(
    user.row.address,
    async () => {
      expect(database.signingSessionClient()).toBeDefined();
      const account = await wallets.getMigrationAwareSigningAccountForRow(
        user.row,
        undefined,
        null,
        undefined,
        { subject: user.userId, requestHash: randomUUID(), route: '/test/shared-signing' }
      );
      const signed = await account.signMessage({ message });
      expect(
        (
          await database.query(
            'select signed_evidence from wallet_signature_reservations where wallet_id=$1',
            [user.row.id]
          )
        ).rows
      ).toEqual([{ signed_evidence: signed }]);
      return signed;
    },
    nonces
  );
  expect(database.signingSessionClient()).toBeUndefined();
  expect((await recoverMessageAddress({ message, signature })).toLowerCase()).toBe(
    user.row.address.toLowerCase()
  );
}

async function signOnlyKeepsSignedBytesAndNonceWithOneConnection(): Promise<void> {
  const user = await caller();
  const before = chain.sent.length;
  const reply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/sign',
    headers: user.headers,
    payload: { tx: transaction() },
  });
  expect(reply.statusCode, reply.body).toBe(200);
  const raw = reply.json().signed_tx as Hex;
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(
    user.row.address.toLowerCase()
  );
  expect(
    (
      await database.query('select tx_hash from nonce_outstanding where address=$1', [
        user.row.address.toLowerCase(),
      ])
    ).rows
  ).toEqual([{ tx_hash: keccak256(raw) }]);
  expect(chain.sent).toHaveLength(before);
}

async function humanSendRecoveryReopensOnOneConnection(): Promise<void> {
  const user = await caller();
  const reply = await app.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    headers: user.headers,
    payload: { tx: transaction() },
  });
  expect(reply.statusCode, reply.body).toBe(200);
  const hash = reply.json().tx_hash as Hex;
  const before = chain.sent.length;
  const reopened = await mount();
  expect(reopened.hasRoute({ method: 'POST', url: '/v1/wallet/send' })).toBe(true);
  expect(
    (
      await database.query(
        "select phase,evidence->>'tx_hash' as tx_hash from wallet_approval_recovery where domain='human' and binding=$1",
        [hash]
      )
    ).rows
  ).toEqual([{ phase: 'settled', tx_hash: hash }]);
  expect(
    (
      await database.query(
        'select value_wei::text,tx_hash from wallet_signatures where wallet_id=$1',
        [user.row.id]
      )
    ).rows
  ).toEqual([{ value_wei: '1', tx_hash: hash }]);
  expect(chain.sent).toHaveLength(before);
}

// A pool acquisition timeout is 5 seconds. Every route proof must finish before it.
test(walletSendCompletesWithOneConnection, 4_000);
test(walletSendKeepsClientAccountPolicyRefusalOrder, 4_000);
test(walletSendKeepsMissingArchivedDisabledResponses, 4_000);
test(messageSigningCommitsAccountingOnOneConnection, 4_000);
test(lxApprovalLimiterAndRecoveryCompleteWithOneConnection, 4_000);
test(evmApprovalAndRecoveryCompleteWithOneConnection, 4_000);
test(parallelSendsKeepDistinctNoncesOnOneConnection, 4_000);
test(sharedWalletLockReusesClientForDurableSigning, 4_000);
test(signOnlyKeepsSignedBytesAndNonceWithOneConnection, 4_000);
test(humanSendRecoveryReopensOnOneConnection, 4_000);
