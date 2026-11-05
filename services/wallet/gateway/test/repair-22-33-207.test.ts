import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import Fastify, { type FastifyInstance } from 'fastify';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  decodeFunctionData,
  encodeFunctionResult,
  keccak256,
  recoverPublicKey,
  toHex,
  type Abi,
  type Hex,
} from 'viem';
import { privateKeyToAccount, publicKeyToAddress } from 'viem/accounts';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';
import { verifySequencerReceipt } from '../../../agent/sdk/typescript/src/verifier.js';
import { encodeSequencerReceipt } from '../../../agent/sdk/typescript/src/sequencer-receipt-codec.js';
import { newAgentKey, startIdentityProvider, type IdentityProvider } from './support/identity.js';
import './migrations-evm.test.js';

export const repair2233207Record = {
  task: '22.33.207',
  findings: {
    'tee-selector-order': 'teeSelectorOrderDistinguishesAvailabilityProbe',
    'retired-agent-route-auth': 'retainedActionRejectsTokenWhileRetiredActionIsAbsent',
    'attestor-inventory-fixture': 'recordedSigningIdentityMatchesInventoryOwnerAndKey',
    'backfill-key-admission': 'generationAdmissionAdvancesToExactHeldKeyAndEpoch',
    'hash-only-evm-sign-receipt': 'canonicalEvmEvidenceSurvivesDatabaseRestart',
    'migration-tail-assumption':
      'evmApprovalsTableAppliesAfter016/currentEvmApprovalsMigrationChain',
    'read-pool-write-retry': 'ambiguousWriteRecoversWithoutReadPoolBroadcastOrReplacement',
    'station-empty-receipt': 'stationReplayPreservesCanonicalNodeExecutionEvidence',
  },
  baseline:
    'No pre-change gate was run: the explicit gate-only order forbids running verify_cmd by hand or before implementation.',
  continuation: {
    previousRevision: '9e2d7c38271235266bdad9d61d5e4729edbe7bb6',
    previousGates:
      'Attempt 1 gate ledger: verify run 1 failed with transient gas_station_unavailable and a closed database connection; verify run 2 passed eight cases and failed the Buffer/Uint8Array receipt comparison (gotcha 3164).',
    gates:
      'Attempt 2 build and verify revision, command, exit and private logs are recorded by hf-build/hf-done in /out/run/gates.jsonl; the final hf-done entry qualifies this unchanged candidate.',
  },
  wiring: [
    'Wallet domain 12: services/wallet/attestor/internal/server/authority.go must expose recorded owner/key/epoch admission and refusal transcripts to the TypeScript-only gate; the real-daemon cases remain in attestor.test.ts.',
    'Wallet domain 12: services/wallet/gateway/test/e2e/attestors.ts generate must publish generated public key and epoch before its refresh; backfill-command.test.ts now drives the admitted transitions directly.',
    'Interop domain 9 / node domain 6: services/interop/crates/layerx-gas-station/tests and node/sequencer must publish a canonical sponsored addr-bind execution/replay/recovery transcript; the current committed station capture proves delegated execution and authenticated receipts but contains no addr bind.',
  ],
} as const;

interface ReceiptVector {
  id: string;
  rawTransaction: Hex;
  signedReceipt: Hex;
  receiptJson: {
    rung: 'receipted';
    txHash: Hex;
    nonce: Hex;
    height: Hex;
    seq: Hex;
    keyId: Hex;
    signingKey: Hex;
    receipt: Hex;
    paymentDenom: string;
    from: Hex;
    to: Hex;
  };
  pinnedKey: { keyId: Hex; publicKey: Hex; firstHeight: number; lastHeight: number };
}

const fixture = <T>(path: string): T =>
  JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8')) as T;
const vectors = fixture<{ cases: ReceiptVector[] }>(
  '../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'
);
const vector = vectors.cases.find((entry) => entry.id === 'V1-pax-transfer')!;
const bytes = (hex: string): Buffer => Buffer.from(hex.replace(/^0x/, ''), 'hex');
const pins = (entry: ReceiptVector) => [
  {
    keyId: bytes(entry.pinnedKey.keyId),
    publicKey: bytes(entry.pinnedKey.publicKey),
    first_height: BigInt(entry.pinnedKey.firstHeight),
    last_height: BigInt(entry.pinnedKey.lastHeight),
  },
];
const inventorySigner = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
let inventorySequence = 0;
let directory: string;
let db: PGlite;
let socket: PGLiteSocketServer;
let server: Server;
let app: FastifyInstance;
let configuration: typeof import('../src/env.js');
let database: typeof import('../src/db/pool.js');
let precompiles: typeof import('../src/precompiles.js');
let authority: typeof import('../src/agent/authority.js');
let rpcModule: typeof import('../src/rpc/pool.js');
let nonceModule: typeof import('../src/nonce/store.js');
let baseUrl: string;
let token: string;
let identityProvider: IdentityProvider;
let agent: ReturnType<typeof newAgentKey>;
let acceptedRaw: Hex | null = null;
let loseWriteResponse = false;
const exchanges: Array<{ path: string; method: string; params: unknown[] }> = [];
const errors: unknown[] = [];
const teeAbi = JSON.parse(
  readFileSync(
    new URL('../../../../precompiles/layerxagent/teeattestor/teeattestor.go', import.meta.url),
    'utf8'
  ).match(/const abiJSON = `([\s\S]*?)`/)![1]!
) as Abi;

async function openDatabase(): Promise<void> {
  db = await PGlite.create({ dataDir: join(directory, 'database'), extensions: { pgcrypto } });
  socket = new PGLiteSocketServer({ db, host: '127.0.0.1', port: 0, maxConnections: 10 });
  await socket.start();
  process.env.DATABASE_URL = `postgres://postgres:postgres@${socket.getServerConn()}/postgres`;
  if (configuration) configuration.env.DATABASE_URL = process.env.DATABASE_URL;
}

async function closeDatabase(): Promise<void> {
  await database?.closePool();
  await socket?.stop();
  await db?.close();
}

async function restartDatabase(): Promise<void> {
  await closeDatabase();
  await openDatabase();
  await (await import('../src/db/migrate.js')).runMigrations();
}

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), 'repair-22-33-207-'));
  identityProvider = await startIdentityProvider();
  process.env.SUPABASE_URL = identityProvider.url;
  await openDatabase();
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  server = createServer(async (req, res) => {
    try {
      let text = '';
      for await (const chunk of req) text += chunk;
      const body = JSON.parse(text) as { id: number; method: string; params: unknown[] };
      exchanges.push({ path: req.url!, method: body.method, params: body.params });
      let result: unknown;
      let rpcError: string | undefined;
      if (body.method === 'eth_call' && req.url === '/tee') {
        const call = body.params[0] as { to: string; data: Hex };
        expect(call.to.toLowerCase()).toBe('0x0000000000000000000000000000000000000907');
        if (call.data === '0xffffffff') rpcError = 'LXP_PRECOMPILE_UNKNOWN_METHOD';
        else {
          const decoded = decodeFunctionData({ abi: teeAbi, data: call.data });
          if (decoded.functionName === 'rootCount')
            result = encodeFunctionResult({ abi: teeAbi, functionName: 'rootCount', result: 0n });
          else if (decoded.functionName === 'rootOf')
            result = encodeFunctionResult({ abi: teeAbi, functionName: 'rootOf', result: '0x' });
          else rpcError = 'LXP_AGENT_TEE_INVALID_QUOTE';
        }
      } else if (body.method === 'eth_sendRawTransaction' && req.url === '/write') {
        acceptedRaw = body.params[0] as Hex;
        result = keccak256(acceptedRaw);
        if (loseWriteResponse) return;
      } else if (body.method === 'eth_getTransactionByHash') {
        result =
          acceptedRaw && body.params[0] === keccak256(acceptedRaw)
            ? { hash: body.params[0] }
            : null;
      } else if (body.method === 'eth_getTransactionCount') result = '0x0';
      else if (body.method === 'eth_blockNumber') result = '0x64';
      else if (body.method === 'pax_sequencerReceipt' && req.url === '/evidence')
        result = vector.receiptJson;
      else if (body.method === 'px_getReceiptKeys') result = { keys: [vector.pinnedKey] };
      else throw new Error(`unrecorded exchange ${req.url} ${body.method}`);
      res.setHeader('content-type', 'application/json');
      res.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          ...(rpcError ? { error: { code: -32000, message: rpcError } } : { result }),
        })
      );
    } catch (error) {
      errors.push(error);
      res.statusCode = 500;
      res.end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.HYPERPAXEER_RPC_URL = `${baseUrl}/tee`;
  configuration = await import('../src/env.js');
  database = await import('../src/db/pool.js');
  await (await import('../src/db/migrate.js')).runMigrations();
  precompiles = await import('../src/precompiles.js');
  authority = await import('../src/agent/authority.js');
  rpcModule = await import('../src/rpc/pool.js');
  nonceModule = await import('../src/nonce/store.js');
  const publicFile = join(directory, 'inventory.pub');
  writeFileSync(publicFile, inventorySigner.publicKey.export({ type: 'spki', format: 'pem' }));
  configuration.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = publicFile;
  configuration.env.WALLET_CUSTODY_INVENTORY_FILE = join(directory, 'inventory.jwt');
  configuration.env.WALLET_IDENTITY_BINDING_TENANT = 'repair-22-33-207';
  agent = newAgentKey('retained-action');
  await (
    await import('../src/db/agents.js')
  ).upsertPrincipalOnVerify({
    did: agent.did,
    publicKey: agent.publicKeyHex,
    label: 'repair',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
  });
  token = (
    await (
      await import('../src/auth/agentToken.js')
    ).mintAgentToken({ did: agent.did, ownerUserId: null })
  ).token;
  app = Fastify();
  await app.register((await import('../src/routes/agentActions.js')).agentActionRoutes);
  await app.register((await import('../src/routes/agentPrecompiles.js')).agentPrecompileRoutes);
  await app.ready();
}, 30_000);

afterAll(async () => {
  await app?.close();
  await closeDatabase();
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
  if (directory) await rm(directory, { recursive: true, force: true });
  await identityProvider?.stop();
  expect(errors).toEqual([]);
});

function approveInventory(keys: import('../src/agent/authority.js').WalletInventoryKey[]): void {
  const now = Math.floor(Date.now() / 1000);
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })
  ).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({
      version: 1,
      iss: `${configuration.env.SUPABASE_URL}/auth/v1`,
      aud: 'wallet-custody-inventory',
      tenant: 'repair-22-33-207',
      sequence: String(++inventorySequence),
      iat: now - 1,
      exp: now + 600,
      protocol: 'wallet',
      threshold: 3,
      members: Array.from({ length: 5 }, (_, i) => ({
        id: `node-${i + 1}`,
        spki_sha256: createHash('sha256').update(`member-${i}`).digest('hex'),
      })),
      keys,
    })
  ).toString('base64url');
  const input = `${header}.${payload}`;
  const signature = sign('sha256', Buffer.from(input), {
    key: inventorySigner.privateKey,
    dsaEncoding: 'ieee-p1363',
  });
  writeFileSync(
    configuration.env.WALLET_CUSTODY_INVENTORY_FILE!,
    `${input}.${signature.toString('base64url')}`
  );
}

async function teeSelectorOrderDistinguishesAvailabilityProbe(): Promise<void> {
  const start = exchanges.length;
  await precompiles.ensureServed('teeAttestor');
  expect(exchanges.slice(start)).toEqual([
    {
      path: '/tee',
      method: 'eth_call',
      params: [{ to: precompiles.PRECOMPILE_ADDRESSES.teeAttestor, data: '0xffffffff' }, 'latest'],
    },
  ]);
  await precompiles.ensureServed('teeAttestor');
  expect(exchanges.slice(start)).toHaveLength(1);
  const report = `0x${'42'.repeat(32)}`;
  for (const example of [
    {
      path: 'verify',
      payload: { family: 'intel_tdx', quote: '0x0102' },
      name: 'verify',
      args: [0, '0x0102'],
      status: 422,
    },
    {
      path: 'verify',
      payload: { family: 1, quote: '0x0102', expected_report_data: report },
      name: 'verifyAndExpect',
      args: [1, '0x0102', report],
      status: 422,
    },
    {
      path: 'root-count',
      payload: { family: 'nvidia_h100' },
      name: 'rootCount',
      args: [2],
      status: 200,
    },
    {
      path: 'root-of',
      payload: { family: 'intel_sgx', index: (1n << 255n).toString() },
      name: 'rootOf',
      args: [3, 1n << 255n],
      status: 200,
    },
  ]) {
    const offset = exchanges.length;
    const response = await app.inject({
      method: 'POST',
      url: `/v1/agent/precompiles/tee/${example.path}`,
      headers: { authorization: `Bearer ${token}` },
      payload: example.payload,
    });
    expect(response.statusCode, response.body).toBe(example.status);
    const calls = exchanges.slice(offset).filter((call) => call.method === 'eth_call');
    expect(calls).toHaveLength(1);
    const call = calls[0]!.params[0] as { to: string; data: Hex };
    expect(decodeFunctionData({ abi: teeAbi, data: call.data })).toEqual({
      functionName: example.name,
      args: example.args,
    });
  }
}

async function retainedActionRejectsTokenWhileRetiredActionIsAbsent(): Promise<void> {
  const { ALLOWANCE_AND_CALL_ROUTE } = await import('../src/routes/agentActions.js');
  const response = await app.inject({
    method: 'POST',
    url: ALLOWANCE_AND_CALL_ROUTE,
    headers: { authorization: `Bearer ${token}` },
    payload: { idempotency_key: 'token-write' },
  });
  expect(response.statusCode).toBe(403);
  expect(response.json()).toMatchObject({ error: 'agent_token_read_only' });
  const url = '/v1/agent/actions/layerx/deposit';
  const payload = JSON.stringify({ amount: '1' });
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
  const digest = (await import('../src/agent/verify.js')).agentRequestDigest({
    method: `POST ${url}`,
    keyId: agent.did,
    nonce,
    expiry,
    body: Buffer.from(payload),
  });
  const signed = {
    'x-agent-key': agent.did,
    'x-agent-nonce': nonce.toString('hex'),
    'x-agent-expires': expiry.toString(),
    'x-agent-signature': agent.sign(digest),
  };
  for (const headers of [{}, { authorization: `Bearer ${token}` }, signed, signed]) {
    const retired = await app.inject({
      method: 'POST',
      url,
      payload,
      headers: { 'content-type': 'application/json', ...headers },
    });
    expect(retired.statusCode).toBe(404);
  }
  expect((await database.query('select count(*)::int as n from agent_actions')).rows).toEqual([
    { n: 0 },
  ]);
}

async function recordedSigningIdentityMatchesInventoryOwnerAndKey(): Promise<void> {
  const recorded = fixture<{ signed_bytes: string; signature: string }>(
    './fixtures/attestor/sign-personal-message.response.json'
  );
  const wallet = fixture<{ key_id: string; address: Hex }>('./fixtures/attestor/wallet.json');
  const publicKey = await recoverPublicKey({
    hash: `0x${recorded.signed_bytes}`,
    signature: `0x${recorded.signature}`,
  });
  expect(publicKeyToAddress(publicKey)).toBe(wallet.address);
  const owner = randomUUID();
  const entry = {
    key_id: wallet.key_id,
    owner,
    curve: 'secp256k1' as const,
    epoch: 1,
    public_key: publicKey.slice(2).toLowerCase(),
    operations: ['sign' as const],
  };
  approveInventory([entry]);
  expect(authority.requireInventoryKey(wallet.key_id, 'sign', owner)).toEqual(entry);
  expect(() => authority.requireInventoryKey(wallet.key_id, 'sign', randomUUID())).toThrow(
    authority.CustodyAuthorityError
  );
  const { evmApprovalRoutes } = await import('../src/routes/evmApprovals.js');
  const { RateLimiter } = await import('../src/audit.js');
  const instance = Fastify();
  const rpc = new rpcModule.RpcPool({
    urls: [`${baseUrl}/read-a`],
    sequencerUrl: `${baseUrl}/evidence`,
    chainId: 125,
    timeoutMs: 500,
    lagThresholdBlocks: 20,
    healthIntervalMs: 60_000,
  });
  try {
    const walletId = randomUUID();
    const id = randomUUID();
    const sessionId = randomUUID();
    await database.query(
      `insert into wallets(id,user_id,address,chain_id,attestor_key_id,migrated_at)
      values($1,$2,$3,125,$4,now())`,
      [walletId, owner, wallet.address, wallet.key_id]
    );
    const artifact = {
      id,
      principal: owner,
      key_id: wallet.key_id,
      public_key: entry.public_key,
      epoch: entry.epoch,
      chain_id: 125,
      session_id: sessionId,
    };
    await database.query(
      `insert into wallet_evm_approvals(id,wallet_id,principal,session_id,state,artifact,review_evidence,expires_at)
      values($1,$2,$3,$4,'reviewed',$5,'[{},{},{}]',now()+interval '1 hour')`,
      [id, walletId, owner, sessionId, artifact]
    );
    await evmApprovalRoutes(
      instance,
      database.getPool(),
      null,
      new RateLimiter({ pool: database.getPool(), clientPerMinute: 60, accountPerMinute: 60 }),
      rpc
    );
    const headers = { authorization: `Bearer ${await identityProvider.mintUserToken(owner)}` };
    const read = () =>
      instance.inject({ method: 'GET', url: `/v1/wallet/evm/approvals/${id}`, headers });
    approveInventory([entry]);
    const initial = await read();
    expect(initial.statusCode, initial.body).toBe(200);
    expect(initial.json()).toMatchObject({ ...artifact, state: 'reviewed' });
    approveInventory([{ ...entry, owner: randomUUID() }]);
    const wrongOwner = await read();
    expect(wrongOwner.statusCode, wrongOwner.body).toBe(503);
    expect(wrongOwner.json()).toEqual({ error: 'custody_authority_unavailable' });
    approveInventory([
      { ...entry, public_key: privateKeyToAccount(`0x${'29'.repeat(32)}`).publicKey.slice(2) },
    ]);
    const wrongKey = await read();
    expect(wrongKey.statusCode, wrongKey.body).toBe(403);
    expect(wrongKey.json()).toEqual({ error: 'original_evm_identity_mismatch' });
    approveInventory([{ ...entry, epoch: entry.epoch + 1 }]);
    const wrongEpoch = await read();
    expect(wrongEpoch.statusCode, wrongEpoch.body).toBe(409);
    expect(wrongEpoch.json()).toEqual({ error: 'evm_approval_identity_changed' });
    approveInventory([entry]);
    expect(authority.requireInventoryKey(wallet.key_id, 'sign', owner).public_key).toBe(
      publicKey.slice(2).toLowerCase()
    );
    expect(
      publicKeyToAddress(
        `0x${authority.requireInventoryKey(wallet.key_id, 'sign', owner).public_key}`
      )
    ).toBe(wallet.address);
    expect((await read()).statusCode).toBe(200);
  } finally {
    await instance.close();
    rpc.stop();
  }
}

async function generationAdmissionAdvancesToExactHeldKeyAndEpoch(): Promise<void> {
  const owner = randomUUID();
  const keyId = `wallet:${owner}:standard:secp256k1:0`;
  const account = privateKeyToAccount(`0x${'28'.repeat(32)}`);
  const admission = {
    key_id: keyId,
    owner,
    curve: 'secp256k1' as const,
    epoch: 0,
    public_key: '',
    operations: ['generate' as const],
  };
  approveInventory([admission]);
  expect(authority.requireInventoryKey(keyId, 'generate', owner)).toEqual(admission);
  expect(() => authority.requireInventoryKey(keyId, 'sign', owner)).toThrow(
    authority.CustodyAuthorityError
  );
  expect(() => authority.requireInventoryKey(keyId, 'generate', randomUUID())).toThrow(
    authority.CustodyAuthorityError
  );
  const generated = {
    ...admission,
    public_key: account.publicKey.slice(2),
    operations: ['sign' as const, 'refresh' as const],
  };
  approveInventory([generated]);
  expect(authority.requireInventoryKey(keyId, 'refresh', owner)).toEqual(generated);
  const refreshed = { ...generated, epoch: 1 };
  approveInventory([refreshed]);
  const admitted = authority.requireInventoryKey(keyId, 'sign', owner);
  expect(admitted).toEqual(refreshed);
  expect(publicKeyToAddress(`0x${admitted.public_key}`)).toBe(account.address);
  const previous = readFileSync(configuration.env.WALLET_CUSTODY_INVENTORY_FILE!, 'utf8');
  approveInventory([{ ...refreshed, public_key: '' }]);
  expect(() => authority.loadWalletInventory()).toThrow(authority.CustodyAuthorityError);
  writeFileSync(configuration.env.WALLET_CUSTODY_INVENTORY_FILE!, previous);
  expect(authority.requireInventoryKey(keyId, 'sign', owner)).toEqual(refreshed);
}

async function canonicalEvmEvidenceSurvivesDatabaseRestart(): Promise<void> {
  const rpc = new rpcModule.RpcPool({
    urls: [`${baseUrl}/read-a`],
    sequencerUrl: `${baseUrl}/evidence`,
    chainId: 125,
    timeoutMs: 500,
    lagThresholdBlocks: 20,
    healthIntervalMs: 60_000,
  });
  try {
    const answer = await rpc.sequencerReceipt(vector.receiptJson.txHash);
    const governed = await rpc.request<{ keys: ReceiptVector['pinnedKey'][] }>(
      'px_getReceiptKeys',
      []
    );
    expect(governed.keys).toEqual([vector.pinnedKey]);
    const verified = await verifySequencerReceipt(
      bytes((answer as typeof vector.receiptJson).receipt),
      bytes(vector.rawTransaction),
      pins({ ...vector, pinnedKey: governed.keys[0]! })
    );
    expect(toHex(encodeSequencerReceipt(verified.receipt))).toBe(vector.signedReceipt);
    expect(toHex(verified.receipt.activityId)).toBe(vector.receiptJson.txHash);
    expect(toHex(verified.receiptKey.publicKey)).toBe(vector.receiptJson.signingKey);
    expect(toHex(verified.receipt.contextHash)).toBe(vector.receiptJson.keyId);
    expect(verified.height).toBe(BigInt(vector.receiptJson.height));
    const walletId = randomUUID();
    const principal = randomUUID();
    const id = randomUUID();
    await database.query(
      'insert into wallets(id,user_id,address,attestor_key_id) values($1,$2,$3,$4)',
      [walletId, principal, vector.receiptJson.from, `receipt:${principal}`]
    );
    const evidence = { tx_hash: vector.receiptJson.txHash, rung: 'receipted', receipt: answer };
    await database.query(
      `insert into wallet_evm_approvals(id,wallet_id,principal,session_id,state,artifact,
      review_evidence,approval,evidence,expires_at,approved_at,attempted_at,completed_at)
      values($1,$2,$3,$4,'signed','{}','[{},{},{}]','{}',$5,now()+interval '1 hour',now(),now(),now())`,
      [id, walletId, principal, randomUUID(), evidence]
    );
    await restartDatabase();
    const recovered = (
      await database.query('select evidence from wallet_evm_approvals where id=$1', [id])
    ).rows[0]!.evidence;
    expect(recovered).toEqual(evidence);
    expect(
      await verifySequencerReceipt(
        bytes(recovered.receipt.receipt),
        bytes(vector.rawTransaction),
        pins(vector)
      )
    ).toMatchObject({ height: BigInt(vector.receiptJson.height) });
    const hashOnly = sign(
      null,
      bytes(vector.receiptJson.txHash),
      generateKeyPairSync('ed25519').privateKey
    );
    await expect(
      verifySequencerReceipt(hashOnly, bytes(vector.rawTransaction), pins(vector))
    ).rejects.toThrow();
    await expect(
      verifySequencerReceipt(bytes(recovered.receipt.receipt), bytes(vector.rawTransaction), [
        { ...pins(vector)[0]!, publicKey: Buffer.alloc(32, 1) },
      ])
    ).rejects.toThrow();
    await expect(
      verifySequencerReceipt(bytes(recovered.receipt.receipt), bytes(vector.rawTransaction), [
        { ...pins(vector)[0]!, first_height: BigInt(vector.receiptJson.height) + 1n },
      ])
    ).rejects.toThrow();
  } finally {
    rpc.stop();
  }
}

async function ambiguousWriteRecoversWithoutReadPoolBroadcastOrReplacement(): Promise<void> {
  const rpc = new rpcModule.RpcPool({
    urls: [`${baseUrl}/read-a`, `${baseUrl}/read-b`],
    sequencerUrl: `${baseUrl}/write`,
    chainId: 125,
    timeoutMs: 100,
    lagThresholdBlocks: 20,
    healthIntervalMs: 60_000,
  });
  const account = privateKeyToAccount(`0x${'31'.repeat(32)}`);
  const raw = await account.signTransaction({
    type: 'eip1559',
    chainId: 125,
    nonce: 0,
    to: vector.receiptJson.to,
    value: 1n,
    gas: 21000n,
    maxFeePerGas: 2n,
    maxPriorityFeePerGas: 1n,
  });
  const store = () =>
    new nonceModule.NonceStore({
      pool: database.getPool(),
      chainId: 125,
      pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
      lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
    });
  const offset = exchanges.length;
  loseWriteResponse = true;
  try {
    const sent = await store().withLock(account.address, async (lease) => {
      const nonce = await lease.next();
      expect(nonce).toBe(0);
      return nonceModule.broadcastReserved(lease, nonce, raw, (signed) =>
        rpc.sendRawTransactionVia(signed)
      );
    });
    expect(sent.hash).toBeNull();
    expect(sent.error).toBeInstanceOf(rpcModule.BroadcastOutcomeUnknownError);
    const before = (
      await database.query('select nonce::text,tx_hash from nonce_outstanding where address=$1', [
        account.address.toLowerCase(),
      ])
    ).rows;
    expect(before).toEqual([{ nonce: '0', tx_hash: keccak256(raw) }]);
    await database.query(
      "update nonce_outstanding set created_at=now()-interval '1 hour' where address=$1",
      [account.address.toLowerCase()]
    );
    await restartDatabase();
    await store().withLock(account.address, async (lease) => {
      expect(await lease.peek()).toBe(1);
      await expect(lease.reserveClientNonce(0)).rejects.toBeInstanceOf(
        nonceModule.NonceOutstandingError
      );
    });
    expect(
      (
        await database.query('select nonce::text,tx_hash from nonce_outstanding where address=$1', [
          account.address.toLowerCase(),
        ])
      ).rows
    ).toEqual(before);
    const writes = exchanges
      .slice(offset)
      .filter((call) => call.method === 'eth_sendRawTransaction');
    expect(writes).toEqual([{ path: '/write', method: 'eth_sendRawTransaction', params: [raw] }]);
    expect(acceptedRaw).toBe(raw);
  } finally {
    loseWriteResponse = false;
    rpc.stop();
  }
}

async function stationReplayPreservesCanonicalNodeExecutionEvidence(): Promise<void> {
  const recording = fixture<{
    publisher_config: {
      sequencer_receipt_keys: Array<{
        keyId: string;
        publicKey: string;
        firstHeight: string;
        lastHeight: string;
      }>;
    };
    phases: { service: Array<{ method: string; params: Hex[]; result: unknown }> };
    service: {
      submission: { call: { to: string; data: Hex }; batch: { account: string } };
      status_body: unknown;
      status: {
        state: string;
        completion: { transactionHash: Hex; outcome: string; rung: string };
        submission: { transactionHash: Hex; sponsorNonce: string };
      };
      receipt: { receipt: Hex; txHash: Hex; signingKey: Hex; status: Hex; rung: string };
    };
  }>('../../../interop/crates/layerx-gas-station/tests/fixtures/producer_22_33_71.json');
  // The committed station producer tape contains a delegated EIP-7702 execution.
  // Exercise the real HTTP adapter and receipt codec, including station restart.
  const delegated = recording.service;
  const submission = recording.phases.service.find(
    (entry) => entry.method === 'eth_sendRawTransaction'
  )!;
  const raw = submission.params[0]!;
  const governedKeys = recording.publisher_config.sequencer_receipt_keys.map((key) => ({
    keyId: Buffer.from(key.keyId, 'base64'),
    publicKey: Buffer.from(key.publicKey, 'base64'),
    first_height: BigInt(key.firstHeight),
    last_height: BigInt(key.lastHeight),
  }));
  expect(raw.slice(0, 4)).toBe('0x04');
  expect(keccak256(raw)).toBe(delegated.receipt.txHash);
  const verified = await verifySequencerReceipt(
    bytes(delegated.receipt.receipt),
    bytes(raw),
    governedKeys
  );
  expect(verified.receipt.operation).toBe(4);
  expect(verified.receipt.resultCode).toBe(1);
  expect(delegated.receipt.status).toBe('0x0');
  expect(toHex(verified.receiptKey.publicKey)).toBe(delegated.receipt.signingKey);
  expect(toHex(encodeSequencerReceipt(verified.receipt))).toBe(delegated.receipt.receipt);
  const response = { transactionHash: delegated.status.submission.transactionHash };
  const { GasStationClient } = await import('../src/sponsor/station.js');
  const requests: Array<{ path: string; body: unknown }> = [];
  let loseResponse = true;
  const station = createServer(async (req, res) => {
    let text = '';
    for await (const chunk of req) text += chunk;
    const body = JSON.parse(text);
    requests.push({ path: req.url!, body });
    res.setHeader('content-type', 'application/json');
    // A completed exchange must release its connection before this fixture
    // stops and reopens the listener on the same address.
    res.setHeader('connection', 'close');
    if (req.url === '/node') {
      const result =
        body.method === 'pax_sequencerReceipt'
          ? delegated.receipt
          : body.method === 'eth_blockNumber'
            ? '0x25'
            : body.method === 'px_getReceiptKeys'
              ? {
                  keys: recording.publisher_config.sequencer_receipt_keys.map((key) => ({
                    keyId: toHex(Buffer.from(key.keyId, 'base64')),
                    publicKey: toHex(Buffer.from(key.publicKey, 'base64')),
                    firstHeight: key.firstHeight,
                    lastHeight: key.lastHeight,
                  })),
                }
              : null;
      res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
      return;
    }
    if (req.url === '/submit' && loseResponse) {
      loseResponse = false;
      res.statusCode = 503;
      res.end('{"error":"gas_station_unavailable"}');
    } else res.end(JSON.stringify(req.url === '/status' ? delegated.status : response));
  });
  await new Promise<void>((resolve) => station.listen(0, '127.0.0.1', resolve));
  const port = (station.address() as AddressInfo).port;
  const url = `http://127.0.0.1:${port}`;
  const rpc = new rpcModule.RpcPool({
    urls: [`${url}/node`],
    sequencerUrl: `${url}/node`,
    chainId: 713714,
    timeoutMs: 500,
    lagThresholdBlocks: 20,
    healthIntervalMs: 60_000,
  });
  try {
    const client = new GasStationClient(url);
    await expect(client.submit(delegated.submission)).rejects.toMatchObject({
      code: 'gas_station_unavailable',
    });
    const result = await client.submit(delegated.submission);
    expect(result).toEqual(response);
    await new Promise<void>((resolve) => station.close(() => resolve()));
    await new Promise<void>((resolve) => station.listen(port, '127.0.0.1', resolve));
    const recovered = new GasStationClient(url);
    expect(await recovered.status(delegated.status_body)).toEqual(delegated.status);
    expect(requests[0]!.body).toEqual(requests[1]!.body);
    const nodeReceipt = await rpc.sequencerReceipt(delegated.receipt.txHash);
    expect(nodeReceipt).toEqual(delegated.receipt);
    const evidence = await verifySequencerReceipt(
      bytes((nodeReceipt as typeof delegated.receipt).receipt),
      bytes(raw),
      governedKeys
    );
    expect(Buffer.from(evidence.receipt.activityId)).toEqual(bytes(delegated.receipt.txHash));
    expect(evidence.receipt.resultCode).toBe(1);
    expect(delegated.status.completion).toMatchObject({
      transactionHash: delegated.receipt.txHash,
      outcome: 'reverted',
      rung: 'receipted',
    });
    await expect(verifySequencerReceipt(bytes('0x'), bytes(raw), governedKeys)).rejects.toThrow();
    await expect(
      verifySequencerReceipt(bytes(delegated.receipt.receipt), bytes(raw), [])
    ).rejects.toThrow();
  } finally {
    rpc.stop();
    station.closeAllConnections();
    await new Promise<void>((resolve) => station.close(() => resolve()));
  }
}

test(
  teeSelectorOrderDistinguishesAvailabilityProbe.name,
  teeSelectorOrderDistinguishesAvailabilityProbe
);
test(
  retainedActionRejectsTokenWhileRetiredActionIsAbsent.name,
  retainedActionRejectsTokenWhileRetiredActionIsAbsent
);
test(
  recordedSigningIdentityMatchesInventoryOwnerAndKey.name,
  recordedSigningIdentityMatchesInventoryOwnerAndKey
);
test(
  generationAdmissionAdvancesToExactHeldKeyAndEpoch.name,
  generationAdmissionAdvancesToExactHeldKeyAndEpoch
);
test(
  canonicalEvmEvidenceSurvivesDatabaseRestart.name,
  canonicalEvmEvidenceSurvivesDatabaseRestart,
  30_000
);
test(
  ambiguousWriteRecoversWithoutReadPoolBroadcastOrReplacement.name,
  ambiguousWriteRecoversWithoutReadPoolBroadcastOrReplacement,
  30_000
);
test(
  stationReplayPreservesCanonicalNodeExecutionEvidence.name,
  stationReplayPreservesCanonicalNodeExecutionEvidence
);
