import Fastify, { type FastifyInstance } from 'fastify';
import { spawn, type ChildProcess } from 'node:child_process';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import { subscribe, unsubscribe } from 'node:diagnostics_channel';
import type { ClientRequest } from 'node:http';
import { createServer, type Server } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { Pool } from 'pg';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { AttestorClient, LxReviewResult } from '../src/attestor/client.js';
import type { WalletInventoryKey } from '../src/agent/authority.js';
import { buildAttestor, CHAIN_ID, makePki, startIdentity, type Identity, type Pki } from './e2e/attestors.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const owner = randomUUID();
const walletId = randomUUID();
const walletAddress = privateKeyToAccount(generatePrivateKey()).address;
const keyId = `wallet:${owner}:standard:ed25519:0`;
const requestPaths: string[] = [];
function observeRequest(message: unknown): void {
  const { request: outgoing } = message as { request: ClientRequest };
  if (nodes.some(node => outgoing.getHeader('host') === new URL(node.endpoint).host)) {
    requestPaths.push(outgoing.path);
  }
}

interface Review extends Omit<LxReviewResult, 'audit'> {
  id: string;
  state: 'reviewed' | 'approved';
  principal: string;
  session_id: string;
  expires_at: string;
}

interface KeyReply {
  public_key: string;
  epoch: number;
}

interface Node {
  id: string;
  endpoint: string;
  process: ChildProcess;
  logs: string[];
}

let identity: Identity;
let postgres: EphemeralPostgres;
let pki: Pki;
let work: string;
let gatewayAgent: Agent;
let pool: Pool;
let database: typeof import('../src/db/pool.js');
let attestors: AttestorClient;
let app: FastifyInstance;
let publicKey: string;
let authorization: { authorization: string };
const nodes: Node[] = [];

function call<T>(agent: Agent, endpoint: string, path: string, body?: unknown): Promise<{ status: number; body: T }> {
  const raw = body === undefined ? undefined : JSON.stringify(body);
  return new Promise((resolve, reject) => {
    const req = request(new URL(path, endpoint), {
      agent, method: raw === undefined ? 'GET' : 'POST', timeout: 180_000,
      headers: raw === undefined ? {} : { 'content-type': 'application/json', 'content-length': Buffer.byteLength(raw) },
    }, (res) => {
      const chunks: Buffer[] = [];
      res.on('data', (chunk: Buffer) => chunks.push(chunk));
      res.on('error', reject);
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode!, body: JSON.parse(Buffer.concat(chunks).toString('utf8')) as T });
        } catch (error) {
          reject(error);
        }
      });
    });
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error(`${path} timed out`)));
    req.end(raw);
  });
}

async function reservePorts(): Promise<{ ports: number[]; release: () => Promise<void> }> {
  const servers: Server[] = [];
  const ports: number[] = [];
  for (let i = 0; i < 10; i++) {
    const server = createServer();
    servers.push(server);
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('attestor port reservation failed');
    ports.push(address.port);
  }
  return {
    ports,
    release: async () => {
      await Promise.all(servers.map((server) => new Promise<void>((resolve, reject) => {
        server.close((error) => error ? reject(error) : resolve());
      })));
    },
  };
}

beforeAll(async () => {
  identity = await startIdentity();
  work = mkdtempSync(join(tmpdir(), 'gateway-lx-approvals-'));
  const ids = Array.from({ length: 5 }, (_, i) => `node-${i + 1}`);
  pki = makePki(ids);
  const producer = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const inventorySigner = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const producerFile = join(work, 'producer.key');
  const producerPublic = join(work, 'producer.pub');
  const inventoryFile = join(work, 'inventory.jwt');
  const inventoryPublic = join(work, 'inventory.pub');
  writeFileSync(producerFile, producer.privateKey.export({ format: 'pem', type: 'pkcs8' }), { mode: 0o600 });
  writeFileSync(producerPublic, producer.publicKey.export({ format: 'pem', type: 'spki' }));
  writeFileSync(inventoryPublic, inventorySigner.publicKey.export({ format: 'pem', type: 'spki' }));
  let sequence = 0;
  const approveInventory = (key: WalletInventoryKey): void => {
    const now = Math.floor(Date.now() / 1000);
    const header = Buffer.from(JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })).toString('base64url');
    const payload = Buffer.from(JSON.stringify({
      version: 1, iss: identity.issuer, aud: 'wallet-custody-inventory', tenant: 'lx-approvals-test',
      sequence: String(++sequence), iat: now - 1, exp: now + 3600, protocol: 'wallet', threshold: 3,
      members: ids.map((id) => ({ id, spki_sha256: pki.node(id).pin })), keys: [key],
    })).toString('base64url');
    const input = `${header}.${payload}`;
    const signature = sign('sha256', Buffer.from(input), { key: inventorySigner.privateKey, dsaEncoding: 'ieee-p1363' });
    writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`, { mode: 0o600 });
  };
  approveInventory({ key_id: keyId, owner, curve: 'ed25519', epoch: 0, public_key: '', operations: ['generate'] });
  const policyFile = join(work, 'policy.json');
  writeFileSync(policyFile, JSON.stringify({
    version: 1,
    defaults: {
      chain_id: CHAIN_ID, kinds: ['lx_activity', 'lx_send_authorization'], rate_per_minute: 600,
    },
  }));
  const kernelPolicyFile = join(work, 'kernel-policy.json');
  writeFileSync(kernelPolicyFile, JSON.stringify({ version: 1, defaults: { modules: { asset: [5] }, caps: { native: { per_operation: '6000000', daily: '8000000' } } } }));
  const bin = buildAttestor(work);
  const reserved = await reservePorts();
  await reserved.release();
  for (const [i, id] of ids.entries()) {
    const nodeKey = join(work, `${id}.key`);
    const nodeDir = join(work, id);
    mkdirSync(nodeDir);
    writeFileSync(nodeKey, randomBytes(32).toString('hex'), { mode: 0o600 });
    const others = ids.filter((peer) => peer !== id);
    const child = spawn(bin, [], {
      env: {
        PATH: process.env.PATH, HOME: process.env.HOME,
        ATTESTOR_NODE_ID: id, ATTESTOR_REGION: `region-${i + 1}`,
        ATTESTOR_LISTEN_ADDR: `127.0.0.1:${reserved.ports[i * 2]}`,
        ATTESTOR_PEER_LISTEN_ADDR: `127.0.0.1:${reserved.ports[i * 2 + 1]}`,
        ATTESTOR_PEERS: others.map((peer) => `${peer}=127.0.0.1:${reserved.ports[ids.indexOf(peer) * 2 + 1]}`).join(','),
        ATTESTOR_PEER_PINS: others.map((peer) => `${peer}=${pki.node(peer).pin}`).join(','),
        ATTESTOR_NODE_KEY_FILE: nodeKey, ATTESTOR_DATA_DIR: nodeDir, ATTESTOR_CHAIN_ID: String(CHAIN_ID),
        ATTESTOR_JWKS_URL: `${identity.issuer}/.well-known/jwks.json`,
        ATTESTOR_JWT_ISSUER: identity.issuer, ATTESTOR_JWT_AUDIENCE: 'authenticated',
        ATTESTOR_POLICY_FILE: policyFile, ATTESTOR_KERNEL_POLICY_FILE: kernelPolicyFile,
        ATTESTOR_RPC_URL: 'http://127.0.0.1:1', ATTESTOR_TLS_CERT_FILE: pki.node(id).cert,
        ATTESTOR_TLS_KEY_FILE: pki.node(id).key, ATTESTOR_TLS_CA_FILE: pki.caFile,
        ATTESTOR_OPERATOR_CA_FILE: pki.operatorCaFile,
        ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE: producerPublic,
        ATTESTOR_AUTHORITY_ISSUER: identity.issuer, ATTESTOR_AUTHORITY_TENANT: 'lx-approvals-test',
        ATTESTOR_INVENTORY_PUBLIC_KEY_FILE: inventoryPublic, ATTESTOR_INVENTORY_FILE: inventoryFile,
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    const node: Node = { id, endpoint: `https://127.0.0.1:${reserved.ports[i * 2]}`, process: child, logs: [] };
    child.stdout!.on('data', (chunk: Buffer) => node.logs.push(chunk.toString('utf8')));
    child.stderr!.on('data', (chunk: Buffer) => node.logs.push(chunk.toString('utf8')));
    nodes.push(node);
  }
  const tls = { ca: readFileSync(pki.caFile), cert: readFileSync(pki.clientCert), key: readFileSync(pki.clientKey) };
  gatewayAgent = new Agent({ ...tls, rejectUnauthorized: true, minVersion: 'TLSv1.3' });
  const deadline = Date.now() + 60_000;
  for (const node of nodes) {
    for (;;) {
      if (node.process.exitCode !== null) throw new Error(`${node.id} exited: ${node.logs.join('')}`);
      const health = await call<{ reachable_peers: number }>(gatewayAgent, node.endpoint, '/health').catch(() => null);
      if (health && health.body.reachable_peers === 4) break;
      if (Date.now() >= deadline) throw new Error(`${node.id} not ready: ${JSON.stringify(health)} ${node.logs.join('')}`);
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
  const generated = await Promise.all(nodes.map((node) => call<KeyReply>(gatewayAgent, node.endpoint, '/v1/keys/generate', {
    key_id: keyId, owner, curve: 'ed25519', account: walletAddress, session_id: 'lx-approval-key-generation',
  })));
  for (const answer of generated) expect(answer.status, JSON.stringify(answer.body)).toBe(200);
  const key = generated[0]!.body;
  for (const answer of generated) expect(answer.body.public_key).toBe(key.public_key);
  approveInventory({ key_id: keyId, owner, curve: 'ed25519', epoch: 0, public_key: key.public_key, operations: ['sign', 'refresh'] });
  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_CHAIN_ID = String(CHAIN_ID);
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = producerFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'lx-approvals-test';
  process.env.WALLET_CUSTODY_INVENTORY_FILE = inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = inventoryPublic;
  process.env.ATTESTOR_ENDPOINTS = nodes.map((node) => node.endpoint).join(',');
  process.env.ATTESTOR_QUORUM = '3';
  process.env.ATTESTOR_CLIENT_CERT_FILE = pki.clientCert;
  process.env.ATTESTOR_CLIENT_KEY_FILE = pki.clientKey;
  process.env.ATTESTOR_CA_FILE = pki.caFile;
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  pool = database.getPool();
  const { didFromPublicKey, mainAccountId } = await import('../src/provision/bind.js');
  const did = didFromPublicKey(key.public_key);
  await pool.query(
    `insert into wallets(id,user_id,address,chain_id,attestor_key_id,layerx_key_id,did,main_account_id,binding_state,migrated_at)
     values($1,$2,$3,$4,$5,$5,$6,$7,'bound',now())`,
    [walletId, owner, walletAddress, CHAIN_ID, keyId, did, mainAccountId(did)],
  );
  publicKey = key.public_key;
  const { AttestorClient } = await import('../src/attestor/client.js');
  attestors = new AttestorClient({
    endpoints: nodes.map((node) => node.endpoint), tls, quorum: 3,
    healthIntervalMs: 60_000, timeoutMs: 30_000,
  });
  const { RateLimiter } = await import('../src/audit.js');
  const { signRoutes } = await import('../src/routes/sign.js');
  app = Fastify({ logger: true });
  await signRoutes(app, { pool, attestors, limiter: new RateLimiter({ pool, clientPerMinute: 60, accountPerMinute: 60 }) });
  await app.ready();
  authorization = { authorization: `Bearer ${await identity.mint(owner)}` };
  subscribe('http.client.request.start', observeRequest);
}, 600_000);

afterAll(async () => {
  unsubscribe('http.client.request.start', observeRequest);
  if (database) {
    const { sharedRpcPool } = await import('../src/rpc/pool.js');
    sharedRpcPool().stop();
  }
  await app?.close();
  attestors?.stop();
  gatewayAgent?.destroy();
  await database?.closePool();
  await postgres?.stop();
  await Promise.all(nodes.map(async (node) => {
    if (node.process.exitCode !== null || node.process.signalCode !== null) return;
    const exited = new Promise<void>((resolve) => node.process.once('exit', () => resolve()));
    node.process.kill('SIGTERM');
    const timer = setTimeout(() => node.process.kill('SIGKILL'), 10_000);
    await exited;
    clearTimeout(timer);
  }));
  await identity?.close();
  if (work) rmSync(work, { recursive: true, force: true });
  if (pki) rmSync(pki.dir, { recursive: true, force: true });
}, 120_000);

function integer(value: bigint | number, size: number): Buffer {
  return Buffer.from(BigInt(value).toString(16).padStart(size * 2, '0'), 'hex');
}

function sized(bytes: Buffer): Buffer {
  return Buffer.concat([integer(bytes.length, 4), bytes]);
}

function sendActivity(key: string, version = 3, operation = 0x10005, conditions = 0) {
  const now = Math.floor(Date.now() / 1000);
  const did = `did:layerx:${key}`;
  const accountName = Buffer.from(`agent:${did}:main`);
  const account = createHash('sha256').update('LX:ACCOUNT:v1').update(sized(accountName)).digest();
  const destination = Buffer.alloc(32, 0x22);
  const context = Buffer.alloc(32, 0x33);
  const idempotency = randomBytes(32);
  const validity = Buffer.concat([integer(now - 10, 8), integer(now + 600, 8)]);
  const common = Buffer.concat([
    account, destination, Buffer.alloc(32), integer(500, 16), integer(7, 8),
    idempotency, integer(now + 600, 8), context, integer(conditions, 1),
    ...Array.from({ length: conditions }, (_, i) => Buffer.concat([
      integer(i % 2 + 1, 1), integer(i % 2 ? now + 600 : now - 10, 8),
    ])),
  ]);
  const authorization = Buffer.concat([integer(1, 1), account]);
  const scope = Buffer.concat([context, integer(CHAIN_ID, 4), integer(version, 2)]);
  const message = Buffer.concat([integer(0x5301, 2), common, authorization, scope]);
  const digest = createHash('sha256').update('LXP/v1/signature-preimage\0').update(message).digest('hex');
  const payload = Buffer.concat([
    integer(0x5301, 2), integer(10, 2), common, authorization,
    Buffer.from(key, 'hex'), Buffer.alloc(64), scope,
  ]);
  const payloadHash = createHash('sha256').update('LXP/v1/payload-hash\0').update(payload).digest();
  const fields = [
    integer(version, 2), integer(CHAIN_ID, 4), integer(operation, 4),
    sized(Buffer.from(did)), sized(Buffer.from(key, 'hex')), integer(7, 8),
    validity, sized(idempotency), integer(1000, 16), sized(payloadHash), sized(payload),
  ];
  const raw = Buffer.concat([
    integer(version, 2), integer(0x1001, 2), integer(11, 1),
    ...fields.flatMap((field, index) => [integer(index + 1, 1), field]),
  ]);
  return { activity: raw.toString('hex'), digest, raw, account: account.toString('hex'), destination: destination.toString('hex') };
}

async function lxReviewRefusesActivityKind(): Promise<void> {
  const { activity } = sendActivity(publicKey);
  const before = requestPaths.length;
  const response = await app.inject({
    method: 'POST', url: '/v1/wallet/lx/review', headers: authorization,
    payload: { activity, kind: 'lx_activity' },
  });
  expect(response.statusCode).toBe(400);
  expect(response.json()).toEqual({ error: 'invalid_lx_review' });
  expect(requestPaths.slice(before)).toEqual([]);
  expect((await pool.query('select id from wallet_lx_approvals where principal=$1', [owner])).rows).toEqual([]);
}

async function lxReviewDefaultsToSendAuthorization(): Promise<void> {
  const { activity, digest, account, destination } = sendActivity(publicKey);
  const before = requestPaths.length;
  const response = await app.inject({
    method: 'POST', url: '/v1/wallet/lx/review', headers: authorization,
    payload: { activity },
  });
  expect(response.statusCode, response.body).toBe(200);
  expect(requestPaths.slice(before).filter(path => path === '/v1/lx/review')).toHaveLength(3);
  const review = response.json<Review>();
  expect(review).toMatchObject({
    kind: 'lx_send_authorization', activity, signing_preimage: digest,
    key_id: keyId, public_key: publicKey, epoch: 0, principal: owner,
    account_id: account, network_id: CHAIN_ID, protocol_version: 3, state: 'reviewed',
    disclosure: {
      account, module: 'asset', operation: 5, sequence: '7',
      amounts: [{ asset: '00'.repeat(32), amount: '500' }], destinations: [destination],
    },
  });
  const saved = await pool.query('select artifact,review_evidence from wallet_lx_approvals where id=$1', [review.id]);
  const { state, ...artifact } = review;
  expect(saved.rows[0].artifact).toEqual(artifact);
  expect(saved.rows[0].review_evidence).toHaveLength(3);
  const status = await app.inject({ method: 'GET', url: `/v1/wallet/lx/approvals/${review.id}`, headers: authorization });
  expect(status.statusCode, status.body).toBe(200);
  expect(status.json()).toEqual(review);
  const unsigned = await app.inject({ method: 'POST', url: '/v1/wallet/lx/sign', headers: authorization, payload: { approval_id: review.id } });
  expect(unsigned.statusCode).toBe(403);
  expect(unsigned.json()).toEqual({ error: 'explicit_lx_approval_required' });
  const approved = await app.inject({
    method: 'POST', url: '/v1/wallet/lx/approve', headers: authorization,
    payload: { review_id: review.id, activity, signing_preimage: digest, disclosure: review.disclosure, expires_at: review.expires_at, decision: 'approve' },
  });
  expect(approved.statusCode, approved.body).toBe(200);
  expect(approved.json()).toMatchObject({
    ...review, state: 'approved', approval: { activity_digest: digest, principal: owner, key_id: keyId },
  });
  for (const [url, error] of [
    ['/v1/wallet/lx/approve', 'explicit_lx_approval_required'],
    ['/v1/wallet/lx/sign', 'retained_lx_approval_required'],
  ]) {
    const invalid = await app.inject({ method: 'POST', url, headers: authorization, payload: {} });
    expect(invalid.statusCode).toBe(400);
    expect(invalid.json()).toEqual({ error });
  }
  const invalidId = await app.inject({ method: 'GET', url: '/v1/wallet/lx/approvals/invalid', headers: authorization });
  expect(invalidId.statusCode).toBe(400);
  expect(invalidId.json()).toEqual({ error: 'invalid_lx_approval_id' });
}

async function lxSigningPreimageCoversSendAuthorizationOnly(): Promise<void> {
  const { lxSigningPreimage, AttestorSessionError } = await import('../src/attestor/client.js');
  for (const version of [1, 2, 3]) {
    for (const conditions of [0, 2, 8]) {
      const { activity, digest, raw } = sendActivity(publicKey, version, 0x10005, conditions);
      expect(lxSigningPreimage(activity, 'lx_send_authorization')).toBe(digest);
      expect(digest).not.toBe(createHash('sha256').update('LXP/v1/signature-preimage\0').update(raw).digest('hex'));
    }
  }
  for (const operation of [0x10001, 0x10007, 0x20001]) {
    const { activity } = sendActivity(publicKey, 3, operation);
    expect(() => lxSigningPreimage(activity, 'lx_send_authorization')).toThrow(AttestorSessionError);
    expect(() => lxSigningPreimage(activity, 'lx_send_authorization')).toThrow('owner send authorization layout required');
  }
}

test(lxReviewRefusesActivityKind);
test(lxReviewDefaultsToSendAuthorization);
test(lxSigningPreimageCoversSendAuthorizationOnly);
