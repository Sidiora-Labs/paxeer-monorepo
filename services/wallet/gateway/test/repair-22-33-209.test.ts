import assert from 'node:assert/strict';
import { createHash, generateKeyPairSync, randomUUID, sign, X509Certificate } from 'node:crypto';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { createServer as createHttpServer, type Server as HttpServer } from 'node:http';
import { createServer as createHttpsServer, type Server as HttpsServer } from 'node:https';
import { createConnection, createServer as createNetServer, type AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import Fastify, { type FastifyInstance } from 'fastify';
import { jwtVerify } from 'jose';
import { Pool } from 'pg';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, parseTransaction, serializeTransaction, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';
import type { AttestorClient } from '../src/attestor/client.js';
import type { RpcPool } from '../src/rpc/pool.js';
import { reserveAttestorPorts, startIdentity, type Identity } from './e2e/attestors.js';
import {
  assertRetainedSigningAuthentication,
  recoverEvmSigningReceipt,
  reviewAndApproveEvmSigning,
  type SigningAdapterRequest,
} from './e2e/wallet-signing-adapters.production.js';
import { TestChain } from './support/chain.js';

const repairRecord = {
  findings: {
    'e2e-port-reservation': 'attestorListenersStayReservedUntilDaemonHandoff',
    'retired-custody-e2e': 'retainedRoutesAuthenticateWhileCustodyStaysAbsent',
    'retired-production-adapters': 'evmApprovalRetainsFeesOwnershipAndReceiptAcrossRestart',
  },
  baseline:
    'Former callback-less TODO collection did not execute these assertions and cannot substantiate repair qualification.',
  wiring:
    'tools/qualification/paxeer-x/wallet-signing-adapters.py (tools-ops): invoke the retained EVM driver and expose arm-evm, returning null for the pre-broadcast receipt lookup, dropping the accepted broadcast reply, and withholding subsequent receipts until receipts is called.',
} as const;

const owner = randomUUID();
const foreign = randomUUID();
const keyId = `wallet:${owner}:standard:secp256k1:0`;
const account = privateKeyToAccount(`0x${'31'.repeat(32)}`);
const tx = {
  type: 'eip1559',
  chainId: 125,
  nonce: 0,
  to: '0x000000000000000000000000000000000000c0de',
  value: 1000n,
  gas: 21000n,
  maxFeePerGas: 2000000000n,
  maxPriorityFeePerGas: 1000000000n,
} as const;
const unsigned = serializeTransaction(tx);
const canonical = serializeTransaction(tx, { r: '0x0', s: '0x0', yParity: 0 });
let work: string;
let db: PGlite;
let socket: PGLiteSocketServer;
let authorityDb: PGlite;
let authoritySocket: PGLiteSocketServer;
let approvalPool: Pool;
let identity: Identity;
let app: FastifyInstance;
let gatewayUrl: string;
let token: string;
let foreignToken: string;
let invalidToken: string;
let database: typeof import('../src/db/pool.js');
let configuration: typeof import('../src/env.js');
let attestors: AttestorClient;
let rpc: RpcPool;
let chain: TestChain;
let proxy: HttpServer;
const peers: HttpsServer[] = [];
const fixtureErrors: unknown[] = [];
const requests: Array<{ node: string; path: string; session: string }> = [];
const broadcasts: Hex[] = [];
let showReceipts = false;
let dropBroadcastReply = true;

function der(tag: number, ...values: Buffer[]): Buffer {
  const bytes = Buffer.concat(values);
  let length = Buffer.from([bytes.length]);
  if (bytes.length >= 128) {
    const hex = bytes.length
      .toString(16)
      .padStart(Math.ceil(bytes.length.toString(16).length / 2) * 2, '0');
    const size = Buffer.from(hex, 'hex');
    length = Buffer.concat([Buffer.from([0x80 | size.length]), size]);
  }
  return Buffer.concat([Buffer.from([tag]), length, bytes]);
}

function certificate(name: string): { cert: string; key: string; pin: string } {
  const pair = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const algorithm = der(0x30, Buffer.from('06082a8648ce3d040302', 'hex'));
  const subject = der(
    0x30,
    der(0x31, der(0x30, Buffer.from('0603550403', 'hex'), der(0x0c, Buffer.from(name))))
  );
  const spki = pair.publicKey.export({ type: 'spki', format: 'der' });
  const tbs = der(
    0x30,
    der(0xa0, der(0x02, Buffer.from([2]))),
    der(0x02, Buffer.from([1])),
    algorithm,
    subject,
    der(0x30, der(0x17, Buffer.from('200101000000Z')), der(0x17, Buffer.from('491231235959Z'))),
    subject,
    spki,
    der(
      0xa3,
      der(
        0x30,
        der(
          0x30,
          Buffer.from('0603551d13', 'hex'),
          der(0x01, Buffer.from([0xff])),
          der(0x04, der(0x30, der(0x01, Buffer.from([0xff]))))
        ),
        der(
          0x30,
          Buffer.from('0603551d11', 'hex'),
          der(0x04, der(0x30, der(0x87, Buffer.from([127, 0, 0, 1]))))
        )
      )
    )
  );
  const bytes = der(
    0x30,
    tbs,
    algorithm,
    der(0x03, Buffer.from([0]), sign('sha256', tbs, pair.privateKey))
  );
  const cert = `-----BEGIN CERTIFICATE-----\n${bytes
    .toString('base64')
    .match(/.{1,64}/g)!
    .join('\n')}\n-----END CERTIFICATE-----\n`;
  assert(new X509Certificate(cert).verify(pair.publicKey));
  return {
    cert,
    key: pair.privateKey.export({ format: 'pem', type: 'pkcs8' }).toString(),
    pin: createHash('sha256').update(spki).digest('hex'),
  };
}

async function openDatabase(): Promise<void> {
  db = await PGlite.create({ dataDir: join(work, 'database'), extensions: { pgcrypto } });
  socket = new PGLiteSocketServer({ db, port: 0, host: '127.0.0.1', maxConnections: 16 });
  await socket.start();
  const url = `postgres://postgres:postgres@${socket.getServerConn()}/postgres`;
  if (!configuration) process.env.DATABASE_URL = url;
  approvalPool = new Pool({ connectionString: url });
}

async function closeDatabase(): Promise<void> {
  await approvalPool?.end();
  await socket?.stop();
  await db?.close();
}

const request: SigningAdapterRequest = async (path, body, bearer, method = 'POST') => {
  const response = await fetch(gatewayUrl + path, {
    method,
    headers: {
      'content-type': 'application/json',
      ...(bearer ? { authorization: `Bearer ${bearer}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  return { status: response.status, body: (await response.json()) as Record<string, unknown> };
};

async function openGateway(): Promise<void> {
  const { signRoutes } = await import('../src/routes/sign.js');
  const { evmApprovalRoutes } = await import('../src/routes/evmApprovals.js');
  const { RateLimiter } = await import('../src/audit.js');
  const { NonceStore } = await import('../src/nonce/store.js');
  const pool = approvalPool;
  const limiter = new RateLimiter({
    pool: database.getPool(),
    clientPerMinute: 600,
    accountPerMinute: 600,
  });
  const nonces = new NonceStore({
    pool,
    chainId: 125,
    pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
    lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
  });
  app = Fastify();
  await app.register(signRoutes, { pool, rpc, nonces, attestors, limiter });
  await evmApprovalRoutes(app, pool, attestors, limiter, rpc);
  gatewayUrl = await app.listen({ port: 0, host: '127.0.0.1' });
}

beforeAll(async () => {
  work = await mkdtemp(join(tmpdir(), 'repair-22-33-209-'));
  identity = await startIdentity();
  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  token = await identity.mint(owner);
  foreignToken = await identity.mint(foreign);
  invalidToken = await identity.mintForeign(owner);
  await openDatabase();
  chain = new TestChain(125);
  chain.balances.set(account.address.toLowerCase(), 10n ** 20n);
  await chain.start();
  proxy = createHttpServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks);
      const call = JSON.parse(body.toString('utf8')) as {
        id: number;
        method: string;
        params: Hex[];
      };
      if (call.method === 'pax_sequencerReceipt' && !showReceipts) {
        res.setHeader('content-type', 'application/json');
        res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result: null }));
        return;
      }
      const response = await fetch(chain.url, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
      });
      const answer = await response.text();
      if (call.method === 'eth_sendRawTransaction') {
        broadcasts.push(call.params[0]!);
        expect(JSON.parse(answer)).toMatchObject({ result: keccak256(call.params[0]!) });
        if (dropBroadcastReply) {
          dropBroadcastReply = false;
          res.destroy();
          return;
        }
      }
      res.setHeader('content-type', 'application/json');
      res.end(answer);
    } catch (error) {
      fixtureErrors.push(error);
      res.writeHead(500).end();
    }
  });
  await new Promise<void>((resolve) => proxy.listen(0, '127.0.0.1', resolve));
  const rpcUrl = `http://127.0.0.1:${(proxy.address() as AddressInfo).port}`;
  process.env.HYPERPAXEER_RPC_URL = rpcUrl;
  process.env.RPC_URLS = rpcUrl;
  process.env.SEQUENCER_RPC_URL = rpcUrl;

  const ids = Array.from({ length: 5 }, (_, i) => `node-${i + 1}`);
  const tls = ids.map((id) => certificate(id));
  const clientTls = certificate('gateway-transcript-client');
  const ca = tls.map((value) => value.cert).join('');
  const caFile = join(work, 'ca.pem');
  const certFile = join(work, 'client.pem');
  const keyFile = join(work, 'client.key');
  await writeFile(caFile, ca);
  await writeFile(certFile, clientTls.cert);
  await writeFile(keyFile, clientTls.key, { mode: 0o600 });
  process.env.ATTESTOR_CA_FILE = caFile;
  process.env.ATTESTOR_CLIENT_CERT_FILE = certFile;
  process.env.ATTESTOR_CLIENT_KEY_FILE = keyFile;
  const producer = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const producerFile = join(work, 'producer.pem');
  const inventoryFile = join(work, 'inventory.jwt');
  const publicFile = join(work, 'inventory-public.pem');
  await writeFile(producerFile, producer.privateKey.export({ format: 'pem', type: 'pkcs8' }), {
    mode: 0o600,
  });
  await writeFile(publicFile, producer.publicKey.export({ format: 'pem', type: 'spki' }));
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = producerFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'repair-22-33-209';
  process.env.WALLET_CUSTODY_INVENTORY_FILE = inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = publicFile;
  const now = Math.floor(Date.now() / 1000);
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })
  ).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({
      version: 1,
      iss: identity.issuer,
      aud: 'wallet-custody-inventory',
      tenant: 'repair-22-33-209',
      sequence: '1',
      iat: now - 1,
      exp: now + 3600,
      protocol: 'wallet',
      threshold: 3,
      members: ids.map((id, i) => ({ id, spki_sha256: tls[i]!.pin })),
      keys: [
        {
          key_id: keyId,
          epoch: 0,
          curve: 'secp256k1',
          public_key: account.publicKey.slice(2),
          owner,
          operations: ['sign'],
        },
      ],
    })
  ).toString('base64url');
  const signingInput = `${header}.${payload}`;
  await writeFile(
    inventoryFile,
    `${signingInput}.${sign('sha256', Buffer.from(signingInput), {
      key: producer.privateKey,
      dsaEncoding: 'ieee-p1363',
    }).toString('base64url')}`
  );

  const raw = await account.signTransaction(tx);
  const signed = parseTransaction(raw);
  const signature = `${signed.r!.slice(2).padStart(64, '0')}${signed.s!.slice(2).padStart(64, '0')}${signed.yParity!.toString(16).padStart(2, '0')}`;
  const endpoints: string[] = [];
  for (const [index, id] of ids.entries()) {
    const server = createHttpsServer(
      {
        cert: tls[index]!.cert,
        key: tls[index]!.key,
        ca: clientTls.cert,
        requestCert: true,
        rejectUnauthorized: true,
        minVersion: 'TLSv1.3',
      },
      async (req, res) => {
        try {
          res.setHeader('content-type', 'application/json');
          if (req.method === 'GET' && req.url === '/health') {
            res.end(
              JSON.stringify({
                node_id: id,
                region: id,
                share_count: 1,
                refresh_epoch: 0,
                audit_sequence: 1,
                audit_head: '00'.repeat(32),
                peers: {},
                reachable_peers: 4,
                ready: true,
              })
            );
            return;
          }
          const chunks: Buffer[] = [];
          for await (const chunk of req) chunks.push(Buffer.from(chunk));
          const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
          if (req.url === '/v1/authority') {
            const verified = await jwtVerify(body.token, producer.publicKey, {
              algorithms: ['ES256'],
              issuer: identity.issuer,
            });
            res.end(JSON.stringify({ node_id: id, sequence: verified.payload.sequence }));
            return;
          }
          const { verifyAccessToken } = await import('../src/auth/jwt.js');
          expect((await verifyAccessToken(req.headers.authorization!.slice(7))).sub).toBe(owner);
          expect(body.key_id).toBe(keyId);
          requests.push({ node: id, path: req.url!, session: body.session_id });
          if (req.url === '/v1/evm/review') {
            expect(body.transaction).toBe(canonical);
            res.end(
              JSON.stringify({
                node_id: id,
                key_id: keyId,
                audit_sequence: 2,
                to: tx.to,
                value: tx.value.toString(),
                data: '0x',
                method: '',
                destination: 'eoa',
                signing_hash: keccak256(unsigned),
                chain_id: 125,
              })
            );
          } else {
            expect(req.url).toBe('/v1/sign');
            const { signRequestBody } = await import('../src/attestor/client.js');
            expect(body).toEqual(
              signRequestBody(body.session_id, keyId, body.signers, {
                kind: 'evm_tx',
                transaction: canonical,
              })
            );
            expect(body.signers).toContain(id);
            expect(body.signers).toHaveLength(3);
            res.end(
              JSON.stringify({
                node_id: id,
                key_id: keyId,
                kind: 'evm_tx',
                signature,
                recovery_id: signed.yParity,
                signed_bytes: keccak256(unsigned).slice(2),
                audit_sequence: 3,
              })
            );
          }
        } catch (error) {
          fixtureErrors.push(error);
          res.writeHead(500).end(JSON.stringify({ error: String(error) }));
        }
      }
    );
    peers.push(server);
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    endpoints.push(`https://127.0.0.1:${(server.address() as AddressInfo).port}`);
  }
  process.env.ATTESTOR_ENDPOINTS = endpoints.join(',');
  process.env.ATTESTOR_QUORUM = '3';
  configuration = await import('../src/env.js');
  database = await import('../src/db/pool.js');
  await (await import('../src/db/migrate.js')).runMigrations();
  await database
    .getPool()
    .query(
      `insert into wallets(user_id,address,chain_id,attestor_key_id,migrated_at) values($1,$2,125,$3,now())`,
      [owner, account.address, keyId]
    );
  await database.closePool();
  authorityDb = await PGlite.create({ extensions: { pgcrypto } });
  authoritySocket = new PGLiteSocketServer({
    db: authorityDb,
    port: 0,
    host: '127.0.0.1',
    maxConnections: 16,
  });
  await authoritySocket.start();
  configuration.env.DATABASE_URL = `postgres://postgres:postgres@${authoritySocket.getServerConn()}/postgres`;
  await (await import('../src/db/migrate.js')).runMigrations();
  await database
    .getPool()
    .query(
      `insert into wallets(user_id,address,chain_id,attestor_key_id,migrated_at) values($1,$2,125,$3,now())`,
      [owner, account.address, keyId]
    );
  const { AttestorClient } = await import('../src/attestor/client.js');
  attestors = new AttestorClient({
    endpoints,
    tls: { cert: clientTls.cert, key: clientTls.key, ca },
    quorum: 3,
    healthIntervalMs: 60000,
    timeoutMs: 3000,
  });
  const { RpcPool } = await import('../src/rpc/pool.js');
  rpc = new RpcPool({
    urls: [rpcUrl],
    sequencerUrl: rpcUrl,
    chainId: 125,
    lagThresholdBlocks: 3,
    timeoutMs: 3000,
    healthIntervalMs: 60000,
  });
  await openGateway();
}, 60000);

afterAll(async () => {
  await app?.close();
  attestors?.stop();
  rpc?.stop();
  await closeDatabase();
  await database?.closePool();
  await authoritySocket?.stop();
  await authorityDb?.close();
  await Promise.all(
    peers.map(
      (server) =>
        new Promise<void>((resolve, reject) =>
          server.close((error) => (error ? reject(error) : resolve()))
        )
    )
  );
  if (proxy)
    await new Promise<void>((resolve, reject) =>
      proxy.close((error) => (error ? reject(error) : resolve()))
    );
  await chain?.stop();
  await identity?.close();
  if (work) await rm(work, { recursive: true, force: true });
});

async function attestorListenersStayReservedUntilDaemonHandoff(): Promise<void> {
  expect(repairRecord.findings['e2e-port-reservation']).toBe(
    attestorListenersStayReservedUntilDaemonHandoff.name
  );
  const reserved = await reserveAttestorPorts(5);
  const ports = reserved.flatMap((node) => [node.api, node.peer]);
  try {
    expect(ports).toHaveLength(10);
    expect(new Set(ports).size).toBe(10);
    for (const port of ports) {
      const contender = createNetServer();
      const error = await new Promise<NodeJS.ErrnoException>((resolve) => {
        contender.once('error', resolve);
        contender.listen(port, '127.0.0.1', () => {
          contender.close();
          resolve(new Error('reservation was released early'));
        });
      });
      expect(error.code).toBe('EADDRINUSE');
    }
    await reserved[0]!.release();
    const daemon = createNetServer((socket) => socket.end('reserved listener handoff'));
    try {
      await new Promise<void>((resolve, reject) => {
        daemon.once('error', reject);
        daemon.listen(reserved[0]!.api, '127.0.0.1', resolve);
      });
      const received = await new Promise<string>((resolve, reject) => {
        const client = createConnection({ host: '127.0.0.1', port: reserved[0]!.api });
        const chunks: Buffer[] = [];
        client.on('data', (chunk) => chunks.push(chunk));
        client.on('error', reject);
        client.on('end', () => resolve(Buffer.concat(chunks).toString()));
      });
      expect(received).toBe('reserved listener handoff');
      const other = createNetServer();
      const error = await new Promise<NodeJS.ErrnoException>((resolve) => {
        other.once('error', resolve);
        other.listen(reserved[1]!.peer, '127.0.0.1', () => {
          other.close();
          resolve(new Error('unstarted peer lost its reservation'));
        });
      });
      expect(error.code).toBe('EADDRINUSE');
    } finally {
      await new Promise<void>((resolve, reject) =>
        daemon.close((error) => (error ? reject(error) : resolve()))
      );
    }
  } finally {
    await Promise.all(reserved.map((node) => node.release()));
  }
}

async function retainedRoutesAuthenticateWhileCustodyStaysAbsent(): Promise<void> {
  expect(repairRecord.findings['retired-custody-e2e']).toBe(
    retainedRoutesAuthenticateWhileCustodyStaysAbsent.name
  );
  await assertRetainedSigningAuthentication(request, token, invalidToken);
  expect(requests).toEqual([]);
  expect(broadcasts).toEqual([]);
  expect(
    (await approvalPool.query('select count(*)::text as count from wallet_evm_approvals')).rows
  ).toEqual([{ count: '0' }]);
}

async function evmApprovalRetainsFeesOwnershipAndReceiptAcrossRestart(): Promise<void> {
  expect(repairRecord.findings['retired-production-adapters']).toBe(
    evmApprovalRetainsFeesOwnershipAndReceiptAcrossRestart.name
  );
  const review = await reviewAndApproveEvmSigning(request, unsigned, token, foreignToken);
  expect(review).toMatchObject({ principal: owner, key_id: keyId, transaction: canonical });
  expect(requests.filter((entry) => entry.path === '/v1/evm/review')).toHaveLength(3);
  expect(broadcasts).toEqual([]);
  const signed = await request('/v1/wallet/evm/sign', { approval_id: review.id }, token);
  expect(fixtureErrors).toEqual([]);
  expect(signed.status, JSON.stringify(signed.body)).toBe(200);
  expect(signed.body).toMatchObject({
    state: 'signed',
    signing_hash: keccak256(unsigned),
    rung: 'submitted',
    receipt: null,
  });
  expect(broadcasts).toHaveLength(1);
  const raw = broadcasts[0]!;
  const hash = keccak256(raw);
  expect(signed.body.tx_hash).toBe(hash);
  expect(raw).toBe(await account.signTransaction(tx));
  const signatures = requests.filter((entry) => entry.path === '/v1/sign');
  expect(signatures).toHaveLength(3);
  expect(signatures.map((entry) => entry.node).sort()).toEqual([...review.participants].sort());
  expect(new Set(signatures.map((entry) => entry.session))).toEqual(new Set([review.session_id]));
  expect(chain.sent).toHaveLength(1);
  await app.close();
  await closeDatabase();
  await openDatabase();
  showReceipts = true;
  await openGateway();
  const recovered = await recoverEvmSigningReceipt(request, review, hash, token, foreignToken);
  expect(recovered.receipt).toMatchObject({
    from: account.address.toLowerCase(),
    to: tx.to,
    status: '0x1',
  });
  const saved = await approvalPool.query(
    "select phase,evidence from wallet_approval_recovery where domain='evm' and approval_id=$1",
    [review.id]
  );
  expect(saved.rows).toHaveLength(1);
  expect(saved.rows[0]).toMatchObject({
    phase: 'settled',
    evidence: { raw, tx_hash: hash, result: { participants: review.participants } },
  });
  expect(requests.filter((entry) => entry.path === '/v1/sign')).toHaveLength(3);
  expect(broadcasts).toEqual([raw]);
  expect(chain.sent).toHaveLength(1);
  expect(fixtureErrors).toEqual([]);
}

test(
  attestorListenersStayReservedUntilDaemonHandoff.name,
  attestorListenersStayReservedUntilDaemonHandoff
);
test(
  retainedRoutesAuthenticateWhileCustodyStaysAbsent.name,
  retainedRoutesAuthenticateWhileCustodyStaysAbsent
);
test(
  evmApprovalRetainsFeesOwnershipAndReceiptAcrossRestart.name,
  evmApprovalRetainsFeesOwnershipAndReceiptAcrossRestart,
  60000
);
