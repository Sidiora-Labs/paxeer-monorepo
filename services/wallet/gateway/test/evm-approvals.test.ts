import Fastify, { type FastifyInstance } from 'fastify';
import { spawn, type ChildProcess } from 'node:child_process';
import { generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import { createServer, type Server } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { Pool } from 'pg';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { encodeFunctionData, erc20Abi, keccak256, serializeTransaction, type Hex } from 'viem';
import type { AttestorClient, EvmReviewResult } from '../src/attestor/client.js';
import type { WalletInventoryKey } from '../src/agent/authority.js';
import { buildAttestor, CHAIN_ID, makePki, startIdentity, type Identity, type Pki } from './e2e/attestors.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const owner = randomUUID();
const walletId = randomUUID();
const keyId = `wallet:${owner}:standard:secp256k1:0`;
const tokenAddress = '0x2222222222222222222222222222222222222222';
const recipient = '0x3333333333333333333333333333333333333333';
const data = encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [recipient, 600n] });
const tx = {
  type: 'eip1559', chainId: CHAIN_ID, nonce: 7, to: tokenAddress, value: 0n,
  data, gas: 80_000n, maxFeePerGas: 2n, maxPriorityFeePerGas: 1n,
} as const;
const unsigned = serializeTransaction(tx);
const canonical = serializeTransaction(tx, { r: '0x0', s: '0x0', yParity: 0 });

interface Review extends Omit<EvmReviewResult, 'audit'> {
  id: string;
  state: 'reviewed' | 'approved';
  transaction: Hex;
  key_id: string;
  principal: string;
  session_id: string;
  expires_at: string;
  approval?: {
    version: number;
    principal: string;
    key_id: string;
    session_id: string;
    signing_hash: string;
    expires_at: string;
  };
}

interface KeyReply {
  public_key: string;
  address: string;
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
let operatorAgent: Agent;
let pool: Pool;
let database: typeof import('../src/db/pool.js');
let attestors: AttestorClient;
let app: FastifyInstance;
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
  work = mkdtempSync(join(tmpdir(), 'gateway-evm-approvals-'));
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
      version: 1, iss: identity.issuer, aud: 'wallet-custody-inventory', tenant: 'evm-approvals-test',
      sequence: String(++sequence), iat: now - 1, exp: now + 3600, protocol: 'wallet', threshold: 3,
      members: ids.map((id) => ({ id, spki_sha256: pki.node(id).pin })), keys: [key],
    })).toString('base64url');
    const input = `${header}.${payload}`;
    const signature = sign('sha256', Buffer.from(input), { key: inventorySigner.privateKey, dsaEncoding: 'ieee-p1363' });
    writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`, { mode: 0o600 });
  };
  approveInventory({ key_id: keyId, owner, curve: 'secp256k1', epoch: 0, public_key: '', operations: ['generate'] });
  const policyFile = join(work, 'policy.json');
  writeFileSync(policyFile, JSON.stringify({
    version: 1,
    defaults: {
      chain_id: CHAIN_ID, kinds: ['evm_tx'], rate_per_minute: 600,
      caps: { native: { per_transaction: '1000000', daily: '10000000' }, [tokenAddress]: { per_transaction: '1000', daily: '10000' } },
      selectors: { [tokenAddress]: ['0xa9059cbb'] },
    },
  }));
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
        ATTESTOR_POLICY_FILE: policyFile, ATTESTOR_TLS_CERT_FILE: pki.node(id).cert,
        ATTESTOR_TLS_KEY_FILE: pki.node(id).key, ATTESTOR_TLS_CA_FILE: pki.caFile,
        ATTESTOR_OPERATOR_CA_FILE: pki.operatorCaFile,
        ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE: producerPublic,
        ATTESTOR_AUTHORITY_ISSUER: identity.issuer, ATTESTOR_AUTHORITY_TENANT: 'evm-approvals-test',
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
  operatorAgent = new Agent({ ca: tls.ca, cert: readFileSync(pki.operatorCert), key: readFileSync(pki.operatorKey), rejectUnauthorized: true });
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
    key_id: keyId, owner, curve: 'secp256k1', session_id: 'evm-approval-key-generation',
  })));
  for (const answer of generated) expect(answer.status, JSON.stringify(answer.body)).toBe(200);
  const key = generated[0]!.body;
  for (const answer of generated) expect(answer.body.public_key).toBe(key.public_key);
  approveInventory({ key_id: keyId, owner, curve: 'secp256k1', epoch: 0, public_key: key.public_key, operations: ['sign', 'refresh'] });
  const refreshed = await Promise.all(nodes.map((node) => call<KeyReply>(operatorAgent, node.endpoint, '/v1/keys/refresh', {
    key_id: keyId, session_id: 'evm-approval-key-refresh',
  })));
  for (const answer of refreshed) {
    expect(answer.status, JSON.stringify(answer.body)).toBe(200);
    expect(answer.body.epoch).toBe(1);
  }
  approveInventory({ key_id: keyId, owner, curve: 'secp256k1', epoch: 1, public_key: key.public_key, operations: ['sign', 'refresh'] });
  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_CHAIN_ID = String(CHAIN_ID);
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = producerFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'evm-approvals-test';
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
  await pool.query(
    `insert into wallets(id,user_id,address,chain_id,attestor_key_id,migrated_at)
     values($1,$2,$3,$4,$5,now())`, [walletId, owner, key.address, CHAIN_ID, keyId],
  );
  const { AttestorClient } = await import('../src/attestor/client.js');
  attestors = new AttestorClient({
    endpoints: nodes.map((node) => node.endpoint), tls, quorum: 3,
    healthIntervalMs: 60_000, timeoutMs: 30_000,
  });
  const { RateLimiter } = await import('../src/audit.js');
  const { evmApprovalRoutes } = await import('../src/routes/evmApprovals.js');
  app = Fastify({ logger: true });
  await evmApprovalRoutes(app, pool, attestors, new RateLimiter({ pool, clientPerMinute: 60, accountPerMinute: 60 }));
  await app.ready();
  authorization = { authorization: `Bearer ${await identity.mint(owner)}` };
}, 600_000);

afterAll(async () => {
  await app?.close();
  attestors?.stop();
  gatewayAgent?.destroy();
  operatorAgent?.destroy();
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

async function reviewTransaction(transaction: Hex = canonical): Promise<Review> {
  const result = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/review', headers: authorization, payload: { transaction },
  });
  expect(result.statusCode, result.body).toBe(200);
  return result.json<Review>();
}

async function evmReviewRetainsDecodedDisclosure(): Promise<void> {
  const before = Math.floor(Date.now() / 1000);
  const review = await reviewTransaction(unsigned);
  expect(review).toMatchObject({
    to: tokenAddress, value: '0', data, method: 'transfer', destination: 'erc20',
    signing_hash: keccak256(unsigned), state: 'reviewed', transaction: canonical,
    chain_id: CHAIN_ID, key_id: keyId, principal: owner,
  });
  expect(review.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(review.participants).toHaveLength(3);
  expect(new Set(review.participants).size).toBe(3);
  expect(Number(review.expires_at)).toBeGreaterThanOrEqual(before + 600);
  expect(Number(review.expires_at)).toBeLessThanOrEqual(Math.floor(Date.now() / 1000) + 600);
  const saved = await pool.query(
    'select * from wallet_evm_approvals where id=$1', [review.id],
  );
  expect(saved.rows).toHaveLength(1);
  const { state, ...artifact } = review;
  expect(saved.rows[0]).toMatchObject({
    wallet_id: walletId, principal: owner, session_id: review.session_id,
    state, artifact, approval: null, evidence: null, approved_at: null, attempted_at: null,
  });
  expect(saved.rows[0].expires_at.getTime()).toBe(Number(review.expires_at) * 1000);
  expect(saved.rows[0].review_evidence).toEqual(review.participants.map((node_id) => ({
    node_id, audit_sequence: expect.any(Number),
  })));
  for (const evidence of saved.rows[0].review_evidence) expect(evidence.audit_sequence).toBeGreaterThan(0);
  const invalid = await app.inject({ method: 'POST', url: '/v1/wallet/evm/review', headers: authorization, payload: { transaction: '0x0200' } });
  expect(invalid.statusCode).toBe(400);
  const missingAuth = await app.inject({ method: 'POST', url: '/v1/wallet/evm/review', payload: { transaction: canonical } });
  expect(missingAuth.statusCode).toBe(401);
}

async function evmApproveRequiresExactSigningHash(): Promise<void> {
  const review = await reviewTransaction();
  const approve = (signing_hash: string, headers = authorization) => app.inject({
    method: 'POST', url: '/v1/wallet/evm/approve', headers,
    payload: { review_id: review.id, signing_hash, decision: 'approve' },
  });
  const changed = await approve(`0x${'00'.repeat(32)}`);
  expect(changed.statusCode).toBe(409);
  expect(changed.json()).toEqual({ error: 'evm_review_changed' });
  const saved = await pool.query('select state,approval,approved_at from wallet_evm_approvals where id=$1', [review.id]);
  expect(saved.rows).toEqual([{ state: 'reviewed', approval: null, approved_at: null }]);
  const foreign = await approve(review.signing_hash, { authorization: `Bearer ${await identity.mint(randomUUID())}` });
  expect(foreign.statusCode).toBe(404);
  expect(foreign.json()).toEqual({ error: 'evm_approval_not_found' });
  const accepted = await approve(review.signing_hash);
  expect(accepted.statusCode, accepted.body).toBe(200);
  const approval = {
    version: 1, principal: owner, key_id: keyId, session_id: review.session_id,
    signing_hash: review.signing_hash, expires_at: review.expires_at,
  };
  expect(accepted.json()).toEqual({ ...review, state: 'approved', approval });
  const retained = await pool.query('select state,approval,approved_at,artifact from wallet_evm_approvals where id=$1', [review.id]);
  expect(retained.rows[0]).toMatchObject({ state: 'approved', approval, approved_at: expect.any(Date) });
  expect(retained.rows[0].artifact.signing_hash).toBe(review.signing_hash);
  const repeated = await approve(review.signing_hash);
  expect(repeated.statusCode).toBe(200);
  expect(repeated.json()).toEqual(accepted.json());
  expect((await approve(`0x${'11'.repeat(32)}`)).statusCode).toBe(409);
  const expired = await reviewTransaction();
  const expiry = String(Math.floor(Date.now() / 1000) - 1);
  await pool.query(
    "update wallet_evm_approvals set expires_at=to_timestamp($2::double precision),artifact=jsonb_set(artifact,'{expires_at}',to_jsonb($2::text)) where id=$1",
    [expired.id, expiry],
  );
  const refused = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/approve', headers: authorization,
    payload: { review_id: expired.id, signing_hash: expired.signing_hash, decision: 'approve' },
  });
  expect(refused.statusCode).toBe(403);
  expect(refused.json()).toEqual({ error: 'evm_approval_expired' });
}

async function evmApprovalStatusReturnsRetainedRow(): Promise<void> {
  const review = await reviewTransaction();
  const url = `/v1/wallet/evm/approvals/${review.id}`;
  const found = await app.inject({ method: 'GET', url, headers: authorization });
  expect(found.statusCode, found.body).toBe(200);
  expect(found.json()).toEqual(review);
  const foreign = await app.inject({ method: 'GET', url, headers: { authorization: `Bearer ${await identity.mint(randomUUID())}` } });
  expect(foreign.statusCode).toBe(404);
  expect(foreign.json()).toEqual({ error: 'evm_approval_not_found' });
  const absent = await app.inject({ method: 'GET', url: `/v1/wallet/evm/approvals/${randomUUID()}`, headers: authorization });
  expect(absent.statusCode).toBe(404);
  expect(absent.json()).toEqual({ error: 'evm_approval_not_found' });
  const invalid = await app.inject({ method: 'GET', url: '/v1/wallet/evm/approvals/not-a-uuid', headers: authorization });
  expect(invalid.statusCode).toBe(400);
  expect(invalid.json()).toEqual({ error: 'invalid_evm_approval_id' });
  const approved = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/approve', headers: authorization,
    payload: { review_id: review.id, signing_hash: review.signing_hash, decision: 'approve' },
  });
  expect(approved.statusCode, approved.body).toBe(200);
  const status = await app.inject({ method: 'GET', url, headers: authorization });
  expect(status.statusCode).toBe(200);
  expect(status.json()).toEqual(approved.json());
  expect((await app.inject({ method: 'GET', url })).statusCode).toBe(401);
}

test(evmReviewRetainsDecodedDisclosure, evmReviewRetainsDecodedDisclosure, 120_000);
test(evmApproveRequiresExactSigningHash, evmApproveRequiresExactSigningHash, 120_000);
test(evmApprovalStatusReturnsRetainedRow, evmApprovalStatusReturnsRetainedRow, 120_000);
