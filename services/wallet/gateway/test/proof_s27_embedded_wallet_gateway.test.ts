import { TestChain } from './support/chain.js';
/**
 * Proves: walletRoutes/provisionAccount, RPC_URLS, RpcPool, backfillAccounts.main and
 * precompiles.servedAddress -> index.buildApp HTTP + JSON-RPC + PostgreSQL ->
 * signRoutes, readinessRoutes, NonceStore and the authenticated wallet consumer.
 *
 * Repairs: RpcPool.checkHealth admitted nodes on the wrong chain; RPC_URLS accepted
 * malformed URLs; signRoutes discarded an accepted hash when sequencerReceipt failed;
 * servedAddress returned undefined for unknown runtime keys; runBackfill accepted
 * invalid programmatic batch limits. Provision and readiness replies must not cache
 * identity or health state. The proof uses real PostgreSQL, an ES256 issuer and an
 * loopback EVM JSON-RPC peer, with production routes, signing and persistence.
 *
 * The EVM peer exposes no Paxeer kernel, gas station or attestor network. Their unavailable
 * boundaries are asserted explicitly, including the peer's explicit -32601 response.
 * No workspace dist entry is imported: gateway consumers load their own source.
 */
import { spawn, type ChildProcess } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { createServer, type AddressInfo } from 'node:net';
import { fileURLToPath } from 'node:url';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  decodeFunctionData,
  keccak256,
  parseTransaction,
  recoverMessageAddress,
  recoverTransactionAddress,
  type Hex,
} from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';

const gatewayDir = fileURLToPath(new URL('..', import.meta.url));
const children: Array<{ child: ChildProcess; logs: string[] }> = [];
let pg: EphemeralPostgres;
let identity: IdentityProvider;
let gateway: ChildProcess;
let gatewayPort: number;
let base: string;
let rpcUrl: string;
const chain = new TestChain(125, 'evm');
let db: typeof import('../src/db/pool.js');
let rpcModule: typeof import('../src/rpc/pool.js');
let rpc: InstanceType<typeof import('../src/rpc/pool.js').RpcPool>;
let childEnv: NodeJS.ProcessEnv;

async function freePort(): Promise<number> {
  const server = createServer();
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const port = (server.address() as AddressInfo).port;
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
  return port;
}

function launch(command: string, args: string[], env: NodeJS.ProcessEnv = childEnv) {
  const child = spawn(command, args, {
    cwd: gatewayDir,
    env,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const logs: string[] = [];
  child.stdout!.on('data', (chunk: Buffer) => logs.push(chunk.toString()));
  child.stderr!.on('data', (chunk: Buffer) => logs.push(chunk.toString()));
  child.on('error', (error) => logs.push(error.message));
  const entry = { child, logs };
  children.push(entry);
  return entry;
}

async function stop(child: ChildProcess | undefined): Promise<void> {
  if (!child?.pid || child.exitCode !== null || child.signalCode !== null) return;
  const exited = new Promise<void>((resolve) => child.once('exit', () => resolve()));
  child.kill('SIGTERM');
  const timer = setTimeout(() => child.kill('SIGKILL'), 5_000);
  await exited;
  clearTimeout(timer);
}

async function waitFor(url: string, child: ChildProcess, logs: string[], method = 'GET') {
  const deadline = Date.now() + 20_000;
  for (;;) {
    if (!child.pid || child.exitCode !== null || child.signalCode !== null)
      throw new Error(`process exited before serving ${url}: ${logs.join('')}`);
    const response = await fetch(url, {
      method,
      ...(method === 'POST'
        ? {
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({
              jsonrpc: '2.0',
              id: 1,
              method: 'eth_chainId',
              params: [],
            }),
          }
        : {}),
      signal: AbortSignal.timeout(1_000),
    }).catch(() => null);
    if (response?.ok) {
      await response.text();
      return;
    }
    if (response) await response.text();
    if (Date.now() >= deadline) throw new Error(`process did not serve ${url}: ${logs.join('')}`);
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}

async function startGateway() {
  const launched = launch(process.execPath, ['--import', 'tsx', 'src/index.ts']);
  gateway = launched.child;
  await waitFor(`${base}/healthz`, gateway, launched.logs);
}

async function request(path: string, token?: string, body?: unknown) {
  const response = await fetch(`${base}${path}`, {
    method: body === undefined ? 'GET' : 'POST',
    headers: {
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      ...(body === undefined ? {} : { 'content-type': 'application/json' }),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    signal: AbortSignal.timeout(10_000),
  });
  expect(response.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  return { response, body: await response.json() };
}

beforeAll(async () => {
  identity = await startIdentityProvider();
  await chain.start();
  gatewayPort = await freePort();
  rpcUrl = chain.url;
  base = `http://127.0.0.1:${gatewayPort}`;
  process.env.SUPABASE_URL = identity.url;
  process.env.RPC_URLS = rpcUrl;
  process.env.HYPERPAXEER_RPC_URL = rpcUrl;
  process.env.SEQUENCER_RPC_URL = rpcUrl;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  for (const key of [
    'ATTESTOR_ENDPOINTS',
    'WALLET_GAS_STATION_URL',
    'WALLET_IDENTITY_BINDING_TENANT',
    'WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE',
    'WALLET_CUSTODY_INVENTORY_FILE',
    'WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE',
  ])
    delete process.env[key];
  pg = await startPostgres();
  childEnv = { ...process.env, PORT: String(gatewayPort), API_WORKERS: '1' };
  db = await import('../src/db/pool.js');
  rpcModule = await import('../src/rpc/pool.js');
  const { env } = await import('../src/env.js');
  rpc = rpcModule.rpcPoolFromConfig(env);
  await startGateway();
}, 60_000);

afterAll(async () => {
  for (const entry of [...children].reverse()) await stop(entry.child);
  rpc?.stop();
  await db?.closePool();
  await pg?.stop();
  await identity?.stop();
  await chain.stop();
}, 60_000);

async function s27ConfigurationCrossesRealRpcAndReadiness() {
  const { RPC_URLS, EnvSchema, env } = await import('../src/env.js');
  expect(RPC_URLS.parse(` ${rpcUrl}, ${rpcUrl} `)).toEqual([rpcUrl, rpcUrl]);
  expect(env.RPC_URLS).toEqual([rpcUrl]);
  for (const malformed of [
    '',
    'http://[broken',
    'http://localhost:bad',
    'ftp://localhost',
    'https://a b',
  ])
    expect(RPC_URLS.safeParse(malformed).success).toBe(false);
  expect(EnvSchema.parse({ ...process.env, SEQUENCER_RPC_URL: undefined }).SEQUENCER_RPC_URL).toBe(
    rpcUrl
  );
  expect(await rpc.checkHealth()).toMatchObject([{ state: 'healthy', head: 0n }]);
  expect(await rpc.request('eth_chainId')).toBe('0x7d');
  const unsupported = await rpc.request('s27_unknown_method').catch((error: unknown) => error);
  expect(unsupported).toBeInstanceOf(rpcModule.RpcResponseError);
  expect(unsupported).toMatchObject({ code: -32601, memberIndex: 0 });
  expect(rpc.healthyCount()).toBe(1);

  const wrongChain = rpcModule.rpcPoolFromConfig({ ...env, HYPERPAXEER_CHAIN_ID: 126 });
  try {
    expect(await wrongChain.checkHealth()).toMatchObject([
      {
        state: 'down',
        head: null,
        lastError: 'RPC chain id does not match configured chain 126',
      },
    ]);
    await expect(wrongChain.request('eth_chainId')).rejects.toBeInstanceOf(
      rpcModule.RpcUnavailableError
    );
  } finally {
    wrongChain.stop();
  }

  const { response, body } = await request('/readyz');
  expect(response.status).toBe(503);
  expect(response.headers.get('cache-control')).toBe('no-store');
  expect(body).toMatchObject({
    ready: false,
    components: {
      rpc_pool: {
        state: 'up',
        reason: null,
      },
      nonce_store: { state: 'up' },
      identity_provider: { state: 'up', reason: null },
      attestors: { state: 'down', reason: 'attestor_unconfigured' },
    },
  });
}

test(s27ConfigurationCrossesRealRpcAndReadiness, s27ConfigurationCrossesRealRpcAndReadiness);

async function s27WalletAndNonceSurviveRestartAndAcceptedBroadcastKeepsHash() {
  const userId = randomUUID();
  const token = await identity.mintUserToken(userId);
  const provision = await request('/v1/wallet/provision', token, {});
  expect(provision.response.status).toBe(200);
  expect(provision.response.headers.get('cache-control')).toBe('no-store');
  const { id, address, chain_id } = provision.body.wallet as {
    id: string;
    address: Hex;
    chain_id: number;
  };
  const wallet = { id, address, chain_id };
  expect(wallet.chain_id).toBe(125);
  expect(provision.body.identityBinding).toBeNull();
  expect(provision.body.wallet).not.toHaveProperty('encrypted_private_key');
  const accounts = await rpc.request<Hex[]>('eth_accounts');
  await rpc.request('eth_sendTransaction', [
    { from: accounts[0], to: wallet.address, value: '0xde0b6b3a7640000' },
  ]);
  const recipient = privateKeyToAccount(generatePrivateKey()).address;
  const tx = {
    to: recipient,
    value: '7',
    gas: '21000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
  };
  const signed = await request('/v1/wallet/sign', token, { tx });
  expect(signed.response.status).toBe(200);
  expect(signed.body.nonce).toBe(0);
  const raw = signed.body.signed_tx as Hex;
  const hash = keccak256(raw);
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(
    wallet.address.toLowerCase()
  );
  expect(parseTransaction(raw)).toMatchObject({
    nonce: 0,
    chainId: 125,
    to: recipient.toLowerCase(),
    value: 7n,
  });
  const before = await db
    .getPool()
    .query('select nonce::text, tx_hash, member from nonce_outstanding where address = $1', [
      wallet.address.toLowerCase(),
    ]);
  expect(before.rows).toEqual([{ nonce: '0', tx_hash: hash, member: null }]);

  await stop(gateway);
  const closedPool = db.getPool();
  await db.closePool();
  await startGateway();
  expect(db.getPool()).not.toBe(closedPool);
  const me = await request('/v1/wallet/me', token);
  expect(me.response.status).toBe(200);
  expect(me.body.wallet).toMatchObject(wallet);
  // src/provision/bind.ts::readKernelAvailability: an EVM without the Paxeer
  // address precompile must report unavailable, never manufacture a binding.
  expect(me.body.kernel).toMatchObject({ state: 'unavailable' });
  const after = await db
    .getPool()
    .query('select nonce::text, tx_hash, member from nonce_outstanding where address = $1', [
      wallet.address.toLowerCase(),
    ]);
  expect(after.rows).toEqual(before.rows);
  const repeated = await request('/v1/wallet/provision', token, {});
  expect(repeated.response.status).toBe(200);
  expect(repeated.body.wallet).toMatchObject(wallet);
  expect(
    (
      await db
        .getPool()
        .query('select count(*)::text as count from wallets where user_id = $1', [userId])
    ).rows
  ).toEqual([{ count: '1' }]);
  const duplicate = await request('/v1/wallet/sign', token, { tx: { ...tx, nonce: 0 } });
  expect(duplicate.response.status).toBe(409);
  expect(duplicate.body).toMatchObject({ error: 'nonce_outstanding', nonce: 0 });

  expect(await rpc.sendRawTransaction(raw)).toBe(hash);
  expect(await rpc.sendRawTransaction(raw)).toBe(hash);
  expect(await rpc.transactionKnown(hash, null)).toBe(true);
  expect(await rpc.request('eth_getTransactionReceipt', [hash])).toMatchObject({
    transactionHash: hash,
    status: '0x1',
  });
  expect(await rpc.getTransactionCount(wallet.address)).toBe(1);
  const sent = await request('/v1/wallet/send', token, { tx: { ...tx, value: '11' } });
  expect(sent.response.status).toBe(202);
  expect(sent.response.headers.get('cache-control')).toBe('no-store');
  expect(sent.body).toMatchObject({
    error: 'sequencer_receipt_unavailable',
    address: wallet.address,
    chain_id: 125,
    rung: 'submitted',
    retry: 'poll_receipt',
    poll: 'eth_getTransactionReceipt',
    must_not_resubmit: true,
  });
  expect(sent.body.tx_hash).toMatch(/^0x[0-9a-f]{64}$/);
  expect(await rpc.request('eth_getTransactionReceipt', [sent.body.tx_hash])).toMatchObject({
    status: '0x1',
  });
  expect(BigInt(await rpc.request<Hex>('eth_getBalance', [recipient, 'latest']))).toBe(18n);
  const audit = await db
    .getPool()
    .query('select decision, path, tx_hash, nonce::text from signing_audit where request_id = $1', [
      sent.response.headers.get('x-request-id'),
    ]);
  expect(audit.rows).toEqual([
    { decision: 'broadcast', path: 'envelope', tx_hash: sent.body.tx_hash, nonce: '1' },
  ]);
  const replay = await request('/v1/wallet/send', token, { tx: { ...tx, nonce: 0 } });
  expect(replay.response.status).toBe(409);
  expect(replay.body).toMatchObject({ error: 'nonce_below_latest', nonce: 0, latest: 2 });
  const message = await request('/v1/wallet/sign-message', token, {
    message: 's27 persisted owner',
  });
  expect(message.response.status).toBe(200);
  expect(
    (
      await recoverMessageAddress({
        message: 's27 persisted owner',
        signature: message.body.signature,
      })
    ).toLowerCase()
  ).toBe(wallet.address.toLowerCase());
  const other = await identity.mintUserToken(randomUUID());
  const sponsor = privateKeyToAccount(generatePrivateKey());
  const statusBody = {
    account: wallet.address,
    sponsor: sponsor.address,
    quoteNonce: '0',
    relayerSignature: await sponsor.signMessage({ message: wallet.address }),
  };
  const ownStatus = await request('/v1/wallet/sponsored/status', token, statusBody);
  expect(ownStatus.response.status).toBe(503);
  expect(ownStatus.body).toEqual({
    error: 'gas_station_unconfigured',
    status: 'pending',
  });
  const foreignStatus = await request('/v1/wallet/sponsored/status', other, statusBody);
  expect(foreignStatus.response.status).toBe(403);
  expect(foreignStatus.body).toEqual({
    error: 'wallet_owner_mismatch',
    status: 'pending',
  });
  for (const path of ['/v1/wallet/provision', '/v1/wallet/send', '/v1/wallet/sponsored/status']) {
    const unauthenticated = await request(path, undefined, {});
    expect(unauthenticated.response.status).toBe(401);
    expect(unauthenticated.body).toEqual({
      error: 'missing or malformed Authorization header',
    });
  }
}

test(
  s27WalletAndNonceSurviveRestartAndAcceptedBroadcastKeepsHash,
  s27WalletAndNonceSurviveRestartAndAcceptedBroadcastKeepsHash,
  60_000
);

async function s27PrecompileProducerCrossesTheRealSigningConsumer() {
  const precompiles = await import('../src/precompiles.js');
  const { PRECOMPILE_ADDRESSES, servedAddress, scheduler, SCHEDULER_ABI } = precompiles;
  expect(Object.isFrozen(PRECOMPILE_ADDRESSES)).toBe(true);
  for (const key of Object.keys(PRECOMPILE_ADDRESSES) as Array<keyof typeof PRECOMPILE_ADDRESSES>)
    expect(servedAddress(key)).toBe(PRECOMPILE_ADDRESSES[key]);
  expect(() => Reflect.apply(servedAddress, undefined, ['unknown'])).toThrow(
    'unknown precompile unknown'
  );
  const token = await identity.mintUserToken(randomUUID());
  const provision = await request('/v1/wallet/provision', token, {});
  expect(provision.response.status).toBe(200);
  const target = privateKeyToAccount(generatePrivateKey()).address;
  const call = scheduler.schedule({
    target,
    callData: '0x1234',
    executeAtBlock: 100n,
    gasLimit: 50_000n,
    depositWei: 0n,
  });
  expect(call.to).toBe(servedAddress('scheduler'));
  const signed = await request('/v1/wallet/sign', token, {
    tx: {
      ...call,
      gas: '200000',
      maxFeePerGas: '2000000000',
      maxPriorityFeePerGas: '1000000000',
    },
  });
  expect(signed.response.status).toBe(200);
  const parsed = parseTransaction(signed.body.signed_tx);
  expect(parsed.to).toBe(call.to);
  expect(parsed.data).toBe(call.data);
  expect(decodeFunctionData({ abi: SCHEDULER_ABI, data: parsed.data! })).toMatchObject({
    functionName: 'schedule',
    args: [target, '0x1234', 100n, 50_000n],
  });
  expect(
    (
      await recoverTransactionAddress({ serializedTransaction: signed.body.signed_tx })
    ).toLowerCase()
  ).toBe(provision.body.wallet.address.toLowerCase());
  // The EVM peer without this native precompile returns empty bytes. The exported
  // precompiles.ts::ensureServed consumer must refuse that result.
  await expect(precompiles.ensureServed('scheduler')).rejects.toBeInstanceOf(
    precompiles.PrecompileNotServedError
  );
  const malformed = await request('/v1/wallet/send', token, { tx: { to: 'invalid' } });
  expect(malformed.response.status).toBe(400);
  expect(malformed.body.error).toBe('invalid_body');
}

test(
  s27PrecompileProducerCrossesTheRealSigningConsumer,
  s27PrecompileProducerCrossesTheRealSigningConsumer
);

async function s27ProvisioningAndBackfillPreserveTypedRefusals() {
  const { NonceStore } = await import('../src/nonce/store.js');
  const { provisionAccount, ProvisionError, requireProvisionStation } = await import(
    '../src/provision/state.js'
  );
  const jobs = await import('../src/jobs/backfillAccounts.js');
  const deps = {
    pool: db.getPool(),
    rpc,
    attestors: null,
    station: null,
    nonces: new NonceStore({
      pool: db.getPool(),
      chainId: 125,
      pendingCount: (address: Hex) => rpc.getTransactionCount(address),
      lookupTransaction: (hash: Hex, member: string | null) => rpc.lookupTransaction(hash, member),
    }),
    chainId: 125,
    gasCapWei: 1_000_000n,
    receiptTimeoutMs: 1_000,
    receiptPollMs: 10,
  };
  expect(jobs.parseBackfillArgs(['--batch-size', '1', '--max-batches=2'])).toEqual({
    batchSize: 1,
    maxBatches: 2,
  });
  expect(jobs.parseBackfillArgs(['--batch-size=500'])).toMatchObject({ batchSize: 500 });
  for (const args of [
    ['--batch-size=501'],
    ['--max-batches=0'],
    ['--batch-size=1.5'],
    ['--unknown'],
  ])
    expect(() => jobs.parseBackfillArgs(args)).toThrow(jobs.BackfillUsageError);
  for (const opts of [
    { batchSize: 0, maxBatches: 1 },
    { batchSize: 1, maxBatches: Number.MAX_SAFE_INTEGER + 1 },
  ])
    await expect(jobs.runBackfill(deps, opts)).rejects.toBeInstanceOf(jobs.BackfillUsageError);
  expect(() => requireProvisionStation(deps)).toThrow(ProvisionError);
  await expect(
    provisionAccount(deps, { kind: 'standard', userId: randomUUID(), token: null })
  ).rejects.toMatchObject({ code: 'gas_station_unconfigured', status: 503 });
  await expect(jobs.runBackfill(deps, { batchSize: 1, maxBatches: 1 })).rejects.toMatchObject({
    code: 'gas_station_unconfigured',
    status: 503,
  });
  const invalid = launch(process.execPath, [
    '--import',
    'tsx',
    'src/jobs/backfillAccounts.ts',
    '--batch-size',
    '501',
  ]);
  expect(await new Promise((resolve) => invalid.child.once('close', resolve))).toBe(2);
  expect(invalid.logs.join('')).toContain('--batch-size must be between 1 and 500');
  const unconfigured = launch(process.execPath, [
    '--import',
    'tsx',
    'src/jobs/backfillAccounts.ts',
    '--batch-size',
    '1',
    '--max-batches',
    '1',
  ]);
  expect(await new Promise((resolve) => unconfigured.child.once('close', resolve))).toBe(1);
  expect(unconfigured.logs.join('')).toContain(
    'refusing to start without the attestor configuration'
  );
}

test(
  s27ProvisioningAndBackfillPreserveTypedRefusals,
  s27ProvisioningAndBackfillPreserveTypedRefusals,
  20_000
);
