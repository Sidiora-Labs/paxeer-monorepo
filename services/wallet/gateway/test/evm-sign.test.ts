import Fastify, { type FastifyInstance } from 'fastify';
import { spawn, type ChildProcess } from 'node:child_process';
import { generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import { createServer, type Server } from 'node:net';
import { createServer as createHttpServer, type Server as HttpServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { Pool } from 'pg';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { encodeFunctionData, erc20Abi, keccak256, parseTransaction, recoverTransactionAddress, serializeTransaction, toHex, type Hex } from 'viem';
import type { SequencerReceiptResult } from '../src/rpc/pool.js';
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
const canonical = serializeTransaction(tx, { r: '0x0', s: '0x0', yParity: 0 });

interface Review extends Omit<EvmReviewResult, 'audit'> {
  id: string;
  state: 'reviewed' | 'approved' | 'signing_unknown' | 'signed';
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
const rpcServers: HttpServer[] = [];
const receipts = new Map<Hex, SequencerReceiptResult>();
const sequencerKey = generateKeyPairSync('ed25519');
const sequencerKeyId = keccak256(toHex(sequencerKey.publicKey.export({ type: 'spki', format: 'der' })));
const zeroAddress = '0x0000000000000000000000000000000000000000';
let rpcModule: typeof import('../src/rpc/pool.js');
interface RpcExchange {
  method: string;
  params: Hex[];
  result: unknown;
}
const sequencerExchanges: RpcExchange[] = [];
const readerExchanges: RpcExchange[] = [];

async function startRpc(exchanges: RpcExchange[], sequencer: boolean): Promise<string> {
  const server = createHttpServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as { id: number; method: string; params: Hex[] };
      let result: unknown;
      if (sequencer && body.method === 'eth_sendRawTransaction') {
        const raw = body.params[0]!;
        const hash = keccak256(raw);
        const signed = parseTransaction(raw);
        const from = await recoverTransactionAddress({ serializedTransaction: raw });
        receipts.set(hash, {
          rung: 'receipted', txHash: hash, nonce: toHex(signed.nonce!),
          seq: toHex(receipts.size + 1), height: '0x1', index: '0x0', timestamp: '0x65000000',
          status: '0x1', returnData: '0x', logs: [], gasUsed: toHex(tx.gas), cumulativeGasUsed: toHex(tx.gas),
          effectiveGasPrice: toHex(signed.maxFeePerGas!), feeDenom: 'PAX',
          feeAmount: toHex(tx.gas * signed.maxFeePerGas!), sponsor: zeroAddress,
          from, to: signed.to!, contractAddress: zeroAddress, vmError: '', keyId: sequencerKeyId,
          receipt: toHex(sign(null, Buffer.from(hash.slice(2), 'hex'), sequencerKey.privateKey)),
        });
        result = hash;
      } else if (sequencer && body.method === 'pax_sequencerReceipt') {
        if (!receipts.has(body.params[0]!)) throw new Error('receipt requested before broadcast');
        result = receipts.get(body.params[0]!);
      } else if (!sequencer && body.method === 'eth_blockNumber') {
        result = '0x1';
      } else {
        exchanges.push({ ...body, result: undefined });
        throw new Error(`unexpected RPC method ${body.method}`);
      }
      exchanges.push({ ...body, result });
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
    } catch (error) {
      res.writeHead(500, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: String(error) }));
    }
  });
  rpcServers.push(server);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

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
  work = mkdtempSync(join(tmpdir(), 'gateway-evm-sign-'));
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
  process.env.SEQUENCER_RPC_URL = await startRpc(sequencerExchanges, true);
  process.env.RPC_URLS = await startRpc(readerExchanges, false);
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
  rpcModule = await import('../src/rpc/pool.js');
  const { RateLimiter } = await import('../src/audit.js');
  const { evmApprovalRoutes } = await import('../src/routes/evmApprovals.js');
  app = Fastify({ logger: true });
  await evmApprovalRoutes(app, pool, attestors, new RateLimiter({ pool, clientPerMinute: 60, accountPerMinute: 60 }));
  await app.ready();
  authorization = { authorization: `Bearer ${await identity.mint(owner)}` };
}, 600_000);

afterAll(async () => {
  await app?.close();
  rpcModule?.sharedRpcPool().stop();
  await Promise.all(rpcServers.map((server) => new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
  })));
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

function signApproval(approval_id: string) {
  return app.inject({ method: 'POST', url: '/v1/wallet/evm/sign', headers: authorization, payload: { approval_id } });
}

async function approveTransaction(review: Review): Promise<Review> {
  const answer = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/approve', headers: authorization,
    payload: { review_id: review.id, signing_hash: review.signing_hash, decision: 'approve' },
  });
  expect(answer.statusCode, answer.body).toBe(200);
  return answer.json<Review>();
}

async function evmSignRequiresApprovedRow(): Promise<void> {
  const review = await reviewTransaction();
  const refused = await signApproval(review.id);
  expect(refused.statusCode, refused.body).toBe(403);
  expect(refused.json()).toEqual({ error: 'explicit_evm_approval_required' });
  expect((await pool.query('select state,attempted_at,evidence from wallet_evm_approvals where id=$1', [review.id])).rows)
    .toEqual([{ state: 'reviewed', attempted_at: null, evidence: null }]);
  expect(sequencerExchanges.filter((entry) => entry.method === 'eth_sendRawTransaction')).toHaveLength(0);

  const missingApproval = await reviewTransaction();
  await expect(pool.query("update wallet_evm_approvals set state='approved' where id=$1", [missingApproval.id]))
    .rejects.toMatchObject({ code: '23514', constraint: 'wallet_evm_approvals_check' });
  const missingApprovalAnswer = await signApproval(missingApproval.id);
  expect(missingApprovalAnswer.statusCode, missingApprovalAnswer.body).toBe(403);
  expect(missingApprovalAnswer.json()).toEqual({ error: 'explicit_evm_approval_required' });
  expect((await pool.query('select state,attempted_at,evidence from wallet_evm_approvals where id=$1', [missingApproval.id])).rows)
    .toEqual([{ state: 'reviewed', attempted_at: null, evidence: null }]);

  expect((await signApproval(randomUUID())).statusCode).toBe(404);
  const foreign = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/sign',
    headers: { authorization: `Bearer ${await identity.mint(randomUUID())}` }, payload: { approval_id: review.id },
  });
  expect(foreign.statusCode).toBe(404);
  expect((await app.inject({ method: 'POST', url: '/v1/wallet/evm/sign', payload: { approval_id: review.id } })).statusCode).toBe(401);
  const malformed = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/sign', headers: authorization, payload: { approval_id: 'invalid' },
  });
  expect(malformed.statusCode).toBe(400);
  const approved = await approveTransaction(review);
  const expiry = String(Math.floor(Date.now() / 1000) - 1);
  await pool.query(
    `update wallet_evm_approvals set expires_at=to_timestamp($2::double precision),
     artifact=jsonb_set(artifact,'{expires_at}',to_jsonb($2::text)),
     approval=jsonb_set(approval,'{expires_at}',to_jsonb($2::text)) where id=$1`, [approved.id, expiry],
  );
  const expired = await signApproval(approved.id);
  expect(expired.statusCode).toBe(403);
  expect(expired.json()).toEqual({ error: 'evm_approval_expired' });
  expect((await pool.query('select state,attempted_at from wallet_evm_approvals where id=$1', [approved.id])).rows)
    .toEqual([{ state: 'approved', attempted_at: null }]);

  const changed = await approveTransaction(await reviewTransaction());
  await pool.query(
    `update wallet_evm_approvals set artifact=jsonb_set(artifact,'{signing_hash}',to_jsonb($2::text)),
     approval=jsonb_set(approval,'{signing_hash}',to_jsonb($2::text)) where id=$1`,
    [changed.id, `0x${'11'.repeat(32)}`],
  );
  const mismatch = await signApproval(changed.id);
  expect(mismatch.statusCode, mismatch.body).toBe(502);
  expect(mismatch.json()).toEqual({ error: 'evm_signature_evidence_mismatch' });
  const unknown = (await pool.query('select state,attempted_at,evidence from wallet_evm_approvals where id=$1', [changed.id])).rows[0];
  expect(unknown).toEqual({ state: 'signing_unknown', attempted_at: expect.any(Date), evidence: null });
  const repeated = await signApproval(changed.id);
  expect(repeated.statusCode).toBe(202);
  expect(repeated.json().state).toBe('signing_unknown');
  expect(sequencerExchanges.filter((entry) => entry.method === 'eth_sendRawTransaction')).toHaveLength(0);
}

async function evmSignSendsThroughSequencerWithReceipt(): Promise<void> {
  const wallet = (await pool.query<{ address: Hex }>('select address from wallets where id=$1', [walletId])).rows[0]!;
  const review = await reviewTransaction();
  expect(review).toMatchObject({ to: tokenAddress, value: '0', data, method: 'transfer', destination: 'erc20' });
  const approved = await approveTransaction(review);
  const attempts = await Promise.all([signApproval(approved.id), signApproval(approved.id)]);
  expect(attempts.map((answer) => answer.statusCode).sort()).toEqual([200, 202]);
  const answer = attempts.find((result) => result.statusCode === 200)!;
  const body = answer.json<Review & { tx_hash: Hex; rung: SequencerReceiptResult['rung']; receipt: SequencerReceiptResult | null }>();
  const sends = sequencerExchanges.filter((entry) => entry.method === 'eth_sendRawTransaction');
  expect(sends).toHaveLength(1);
  const raw = sends[0]!.params[0]!;
  expect(body.tx_hash).toBe(keccak256(raw));
  expect(body.tx_hash).toBe(sends[0]!.result);
  expect(body.state).toBe('signed');
  expect(body.approval).toEqual(approved.approval);
  const signed = parseTransaction(raw);
  const { value: zero, ...fields } = tx;
  expect(signed).toMatchObject(fields);
  expect(signed.value ?? 0n).toBe(zero);
  expect(serializeTransaction({ ...signed, value: tx.value })).toBe(raw);
  const unsigned = { ...signed, r: undefined, s: undefined, v: undefined, yParity: undefined };
  expect(keccak256(serializeTransaction(unsigned))).toBe(approved.signing_hash);
  expect(serializeTransaction(unsigned, { r: '0x0', s: '0x0', yParity: 0 })).toBe(approved.transaction);
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(wallet.address.toLowerCase());
  const receipts = sequencerExchanges.filter((entry) => entry.method === 'pax_sequencerReceipt');
  expect(receipts).toHaveLength(1);
  expect(receipts[0]!.params).toEqual([body.tx_hash]);
  expect(body.receipt).not.toBeNull();
  expect(body.receipt).toEqual(receipts[0]!.result);
  expect(body.receipt).toMatchObject({ rung: 'receipted', txHash: body.tx_hash, from: wallet.address, to: tokenAddress });
  expect(body.rung).toBe(body.receipt === null ? 'submitted' : body.receipt.rung);
  if (body.receipt !== null) expect(body.receipt.txHash).toBe(body.tx_hash);
  expect(readerExchanges.filter((entry) => entry.method === 'eth_sendRawTransaction')).toHaveLength(0);
  const saved = (await pool.query('select * from wallet_evm_approvals where id=$1', [review.id])).rows[0];
  expect(saved).toMatchObject({ state: 'signed', attempted_at: expect.any(Date), completed_at: expect.any(Date) });
  expect(saved.evidence).toMatchObject({ tx_hash: body.tx_hash, receipt: body.receipt, rung: body.rung });
  expect(saved.evidence.attestor_audit).toHaveLength(3);
  for (const audit of saved.evidence.attestor_audit) expect(audit.audit_sequence).toBeGreaterThan(0);
  const repeated = await signApproval(review.id);
  expect(repeated.statusCode).toBe(200);
  expect(repeated.json()).toEqual(body);
  const status = await app.inject({ method: 'GET', url: `/v1/wallet/evm/approvals/${review.id}`, headers: authorization });
  expect(status.statusCode).toBe(200);
  expect(status.json()).toEqual(body);
  expect(sequencerExchanges.filter((entry) => entry.method === 'eth_sendRawTransaction')).toHaveLength(1);
}

test(evmSignRequiresApprovedRow, evmSignRequiresApprovedRow, 120_000);
test(evmSignSendsThroughSequencerWithReceipt, evmSignSendsThroughSequencerWithReceipt, 180_000);
