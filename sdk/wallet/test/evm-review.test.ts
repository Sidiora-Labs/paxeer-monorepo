import { createRequire } from 'node:module';
import { PaxeerWallet, type EvmTransactionReview, type EvmApproval } from '../src/index.js';
import { spawn, type ChildProcess } from 'node:child_process';
import { generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import { createServer, type Server } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterAll, beforeAll, expect, test } from 'vitest';
import { encodeFunctionData, erc20Abi, keccak256, serializeTransaction } from 'viem';
import type { AttestorClient } from '../../../services/wallet/gateway/src/attestor/client.js';
import type { WalletInventoryKey } from '../../../services/wallet/gateway/src/agent/authority.js';
import { buildAttestor, CHAIN_ID, makePki, startIdentity, type Identity, type Pki } from '../../../services/wallet/gateway/test/e2e/attestors.js';
import { startPostgres, type EphemeralPostgres } from '../../../services/wallet/gateway/test/support/postgres.js';

type Gateway = Parameters<typeof import('../../../services/wallet/gateway/src/routes/evmApprovals.js')['evmApprovalRoutes']>[0];
const requireGateway = createRequire(new URL('../../../services/wallet/gateway/package.json', import.meta.url));
const Fastify = requireGateway('fastify') as () => Gateway;

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
let pool: ReturnType<typeof import('../../../services/wallet/gateway/src/db/pool.js')['getPool']>;
let database: typeof import('../../../services/wallet/gateway/src/db/pool.js');
let attestors: AttestorClient;
let app: ReturnType<typeof Fastify>;
let apiUrl: string;
let tamper: 'to' | 'value' | 'data' | 'signing_hash' | null = null;
const requests: { method: string; url: string; body: unknown; authorization: string | undefined }[] = [];
const clients: PaxeerWallet[] = [];
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
  process.env.WALLET_MASTER_KEY = randomBytes(32).toString('base64');
  process.env.CORS_ORIGINS = 'http://localhost:3000';
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
  const { existsSync, readdirSync } = await import('node:fs');
  if (!process.env.PG_BIN_DIR) {
    const versions = readdirSync('/usr/lib/postgresql').filter((version) => existsSync(`/usr/lib/postgresql/${version}/bin/initdb`));
    if (versions.length !== 1) throw new Error('Set PG_BIN_DIR to the PostgreSQL installation');
    process.env.PG_BIN_DIR = `/usr/lib/postgresql/${versions[0]}/bin`;
  }
  postgres = await startPostgres();
  database = await import('../../../services/wallet/gateway/src/db/pool.js');
  pool = database.getPool();
  await pool.query(
    `insert into wallets(id,user_id,address,chain_id,attestor_key_id,migrated_at)
     values($1,$2,$3,$4,$5,now())`, [walletId, owner, key.address, CHAIN_ID, keyId],
  );
  const { AttestorClient } = await import('../../../services/wallet/gateway/src/attestor/client.js');
  attestors = new AttestorClient({
    endpoints: nodes.map((node) => node.endpoint), tls, quorum: 3,
    healthIntervalMs: 60_000, timeoutMs: 30_000,
  });
  const { RateLimiter } = await import('../../../services/wallet/gateway/src/audit.js');
  const { evmApprovalRoutes } = await import('../../../services/wallet/gateway/src/routes/evmApprovals.js');
  app = Fastify();
  app.addHook('preHandler', async (req) => {
    if (req.url.startsWith('/v1/wallet/evm/')) requests.push({
      method: req.method, url: req.url, body: req.body, authorization: req.headers.authorization,
    });
  });
  app.addHook('onSend', async (req, _reply, payload) => {
    if (tamper && req.url === '/v1/wallet/evm/review' && typeof payload === 'string') {
      const artifact = JSON.parse(payload);
      artifact[tamper] = tamper === 'to' ? recipient : tamper === 'value' ? '1'
        : tamper === 'data' ? '0x00' : `0x${'ff'.repeat(32)}`;
      return JSON.stringify(artifact);
    }
    return payload;
  });
  const { requireAuth } = await import('../../../services/wallet/gateway/src/middleware/auth.js');
  app.get('/auth/v1/user', { preHandler: requireAuth }, async (req) => ({
    id: req.user!.id, aud: 'authenticated', role: 'authenticated',
    app_metadata: {}, user_metadata: {}, created_at: new Date(0).toISOString(),
  }));
  await evmApprovalRoutes(app, pool, attestors, new RateLimiter({ pool, clientPerMinute: 60, accountPerMinute: 60 }));
  apiUrl = await app.listen({ port: 0, host: '127.0.0.1' });
  authorization = { authorization: `Bearer ${await identity.mint(owner)}` };
}, 600_000);

afterAll(async () => {
  for (const client of clients) client.supabase.auth.stopAutoRefresh();
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

async function wallet(principal = owner): Promise<PaxeerWallet> {
  const storageDir = join(work, 'sdk-storage');
  mkdirSync(storageDir, { recursive: true });
  const client = new PaxeerWallet({
    apiUrl, supabaseUrl: apiUrl, supabaseAnonKey: 'integration-publishable-key',
    evmReview: { storage: {
      getItem(key) {
        try { return readFileSync(join(storageDir, encodeURIComponent(key)), 'utf8'); }
        catch (error) { if ((error as NodeJS.ErrnoException).code === 'ENOENT') return null; throw error; }
      },
      setItem(key, value) { writeFileSync(join(storageDir, encodeURIComponent(key)), value, { mode: 0o600 }); },
    } },
  });
  clients.push(client);
  const { error } = await client.supabase.auth.setSession({
    access_token: principal === owner ? authorization.authorization.slice('Bearer '.length) : await identity.mint(principal),
    refresh_token: randomUUID(),
  });
  expect(error).toBeNull();
  return client;
}

async function evm_review_answers_to_value_data_and_signing_hash_of_the_transaction(): Promise<void> {
  const client = await wallet();
  const before = requests.length;
  const review: EvmTransactionReview = await client.reviewEvmTransaction(tx);
  expect(review).toMatchObject({
    to: tokenAddress, value: '0', data, signing_hash: keccak256(unsigned),
    state: 'reviewed', principal: owner, chain_id: CHAIN_ID,
  });
  expect(requests.slice(before)).toEqual([{
    method: 'POST', url: '/v1/wallet/evm/review', body: { transaction: unsigned },
    authorization: authorization.authorization,
  }]);
  const retained = JSON.parse(readFileSync(join(work, 'sdk-storage', encodeURIComponent(`paxeer:evm-review:v1:${owner}`)), 'utf8'));
  expect(retained).toContainEqual(review);
  for (const field of ['to', 'value', 'data', 'signing_hash'] as const) {
    tamper = field;
    try {
      await expect(client.reviewEvmTransaction(tx))
        .rejects.toThrow(new TypeError('review does not match the transaction'));
    } finally { tamper = null; }
  }
  expect(JSON.parse(readFileSync(join(work, 'sdk-storage', encodeURIComponent(`paxeer:evm-review:v1:${owner}`)), 'utf8')))
    .toEqual(retained);
  const native = { ...tx, to: recipient, value: 17n, data: undefined } as const;
  expect(await client.reviewEvmTransaction(native)).toMatchObject({
    to: recipient, value: '17', data: '0x', signing_hash: keccak256(serializeTransaction(native)),
  });
  const invalidBefore = requests.length;
  await expect(client.reviewEvmTransaction({ ...tx, type: 'legacy', gasPrice: 2n }))
    .rejects.toThrow('unsigned EIP-1559 transaction required');
  await expect(client.reviewEvmTransaction({ ...tx, r: `0x${'01'.repeat(32)}` }))
    .rejects.toThrow('unsigned EIP-1559 transaction required');
  expect(requests).toHaveLength(invalidBefore);
}

async function evm_review_approves_and_reads_the_approval_status(): Promise<void> {
  const client = await wallet();
  const review = await client.reviewEvmTransaction(tx);
  const resumed = await wallet();
  const before = requests.length;
  const approval: EvmApproval = await resumed.approveEvmTransaction(review.id);
  expect(approval).toMatchObject({ ...review, state: 'approved', approval: {
    version: 1, principal: owner, signing_hash: review.signing_hash,
  } });
  expect(await resumed.evmApprovalStatus(review.id)).toEqual(approval);
  expect(requests.slice(before)).toEqual([
    { method: 'POST', url: '/v1/wallet/evm/approve', authorization: authorization.authorization,
      body: { review_id: review.id, signing_hash: review.signing_hash, decision: 'approve' } },
    { method: 'GET', url: `/v1/wallet/evm/approvals/${review.id}`, body: undefined,
      authorization: authorization.authorization },
  ]);
  const saved = JSON.parse(readFileSync(join(work, 'sdk-storage', encodeURIComponent(`paxeer:evm-review:v1:${owner}`)), 'utf8'));
  expect(saved).toContainEqual(approval);
  const foreign = await wallet(randomUUID());
  const foreignBefore = requests.length;
  await expect(foreign.approveEvmTransaction(review.id)).rejects.toThrow('original EVM review is required');
  expect(requests).toHaveLength(foreignBefore);
  const row = await pool.query('select state,approval from wallet_evm_approvals where id=$1', [review.id]);
  expect(row.rows[0]).toEqual({ state: 'approved', approval: approval.approval });
}

test(evm_review_answers_to_value_data_and_signing_hash_of_the_transaction, 120_000);
test(evm_review_approves_and_reads_the_approval_status, 120_000);
