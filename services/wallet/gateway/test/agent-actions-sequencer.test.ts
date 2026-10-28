import { spawn, type ChildProcess } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { once } from 'node:events';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { createPublicClient, http, keccak256, type Hex } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type { ActionTransactionDraft } from '../src/db/actions.js';
import type { RpcPool } from '../src/rpc/pool.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const chainId = 125;
const account = privateKeyToAccount(generatePrivateKey());
const calls: { path: string; method: string; params: unknown[] }[] = [];
const errors: unknown[] = [];
let node: ChildProcess;
let proxy: Server;
let postgres: EphemeralPostgres;
let rpc: RpcPool;
let database: typeof import('../src/db/pool.js');
let sequencerUrl: string;
let readUrl: string;
let nodeUrl: string;

beforeAll(async () => {
  node = spawn(process.env.ANVIL_BIN ?? 'anvil', ['--port', '0', '--chain-id', String(chainId)], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  nodeUrl = await new Promise<string>((resolve, reject) => {
    let output = '';
    const timer = setTimeout(() => reject(new Error(`anvil did not start: ${output}`)), 10_000);
    node.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    node.once('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`anvil exited with ${code}: ${output}`));
    });
    node.stderr!.on('data', (chunk: Buffer) => {
      output += chunk.toString();
    });
    node.stdout!.on('data', (chunk: Buffer) => {
      output += chunk.toString();
      const match = /Listening on (127\.0\.0\.1:\d+)/u.exec(output);
      if (match) {
        clearTimeout(timer);
        resolve(`http://${match[1]}`);
      }
    });
  });
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString('utf8');
      const request = JSON.parse(body) as { method: string; params: unknown[] };
      calls.push({ path: req.url!, method: request.method, params: request.params });
      const upstream = await fetch(nodeUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
      });
      res.writeHead(upstream.status, { 'content-type': 'application/json' });
      res.end(await upstream.text());
    } catch (error) {
      errors.push(error);
      res.destroy(error instanceof Error ? error : new Error(String(error)));
    }
  });
  await new Promise<void>((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(0, '127.0.0.1', resolve);
  });
  const url = `http://127.0.0.1:${(proxy.address() as AddressInfo).port}`;
  sequencerUrl = `${url}/sequencer`;
  readUrl = `${url}/read`;
  process.env.HYPERPAXEER_RPC_URL = readUrl;
  process.env.RPC_URLS = readUrl;
  process.env.SEQUENCER_RPC_URL = sequencerUrl;
  process.env.HYPERPAXEER_CHAIN_ID = String(chainId);
  process.env.LOG_LEVEL = 'error';
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  const { sharedRpcPool } = await import('../src/rpc/pool.js');
  rpc = sharedRpcPool();
}, 120_000);

afterAll(async () => {
  rpc?.stop();
  await database?.closePool();
  await postgres?.stop();
  if (proxy) {
    await new Promise<void>((resolve, reject) =>
      proxy.close((error) => (error ? reject(error) : resolve()))
    );
  }
  if (node && node.exitCode === null && node.signalCode === null) {
    const exited = once(node, 'exit');
    node.kill('SIGTERM');
    await exited;
  }
}, 120_000);

async function retainedSubmissionReachesSequencer(): Promise<void> {
  const { upsertPrincipalOnVerify } = await import('../src/db/agents.js');
  const actions = await import('../src/db/actions.js');
  const { sharedNonceStore } = await import('../src/nonce/store.js');
  const { submitRetained } = await import('../src/agent/actions/orchestrator.js');
  const publicKey = randomBytes(32).toString('hex');
  const did = `did:matrix:retained:${publicKey.slice(0, 16)}`;
  await upsertPrincipalOnVerify({
    did,
    label: 'retained',
    keyFingerprint: publicKey.slice(0, 16),
    publicKey,
  });
  const { row } = await actions.createOrGetAction({
    did,
    walletId: null,
    walletAddress: account.address,
    kind: 'allowance_and_call',
    idempotencyKey: randomUUID(),
    request: {},
  });
  const funding = await fetch(nodeUrl, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'anvil_setBalance',
      params: [account.address, '0xde0b6b3a7640000'],
    }),
  });
  expect(await funding.json()).toMatchObject({ result: null });
  const draft: ActionTransactionDraft = {
    chainId,
    to: '0x000000000000000000000000000000000000c0de',
    data: '0x',
    value: '1',
    nonce: 0,
    gas: '21000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
    replaces: null,
  };
  await actions.retainActionTransactionDraft(row.id, 'call', draft);
  const raw = await account.signTransaction({
    type: 'eip1559',
    chainId,
    to: draft.to,
    data: draft.data,
    value: BigInt(draft.value),
    nonce: draft.nonce,
    gas: BigInt(draft.gas),
    maxFeePerGas: BigInt(draft.maxFeePerGas),
    maxPriorityFeePerGas: BigInt(draft.maxPriorityFeePerGas),
  });
  const hash = keccak256(raw);
  await actions.retainActionSignedTransaction(row.id, 'call', { draft, raw, hash });
  const retainedRow = await actions.getAction(row.id);
  expect(retainedRow).not.toBeNull();
  if (!retainedRow) throw new Error('retained action missing');
  const signed = actions.actionLegExecution(retainedRow, 'call').current;
  expect(signed).toEqual({ draft, raw, hash });
  if (!signed) throw new Error('retained signed transaction missing');
  const store = sharedNonceStore();
  const start = calls.length;
  await expect(
    store.withLock(
      account.address,
      (lease) => submitRetained(retainedRow, { ...signed, raw: `${raw}00` as Hex }, lease),
      row.id
    )
  ).rejects.toThrow('retained transaction integrity failure');
  expect(calls.slice(start).filter((call) => call.method === 'eth_sendRawTransaction')).toEqual([]);

  expect(
    await store.withLock(
      account.address,
      (lease) => submitRetained(retainedRow, signed, lease),
      row.id
    )
  ).toBeNull();
  expect(calls.slice(start).filter((call) => call.method === 'eth_sendRawTransaction')).toEqual([
    { path: '/sequencer', method: 'eth_sendRawTransaction', params: [raw] },
  ]);
  const outstanding = await database.query<{ tx_hash: Hex; member: string }>(
    'select tx_hash, member from nonce_outstanding where address = $1 and nonce = $2',
    [account.address.toLowerCase(), draft.nonce]
  );
  expect(outstanding.rows).toEqual([{ tx_hash: hash, member: sequencerUrl }]);
  const reservation = await database.query(
    'select 1 from custody_signing_reservations where action_id = $1',
    [row.id]
  );
  expect(reservation.rows).toEqual([]);
  const client = createPublicClient({ transport: http(nodeUrl) });
  const transaction = await client.getTransaction({ hash });
  expect(transaction).toMatchObject({ hash, nonce: draft.nonce, value: 1n });
  expect(transaction.from.toLowerCase()).toBe(account.address.toLowerCase());
  expect(transaction.to?.toLowerCase()).toBe(draft.to);
  expect(await client.getBalance({ address: draft.to })).toBe(1n);
  expect(sequencerUrl).not.toBe(readUrl);
  expect(errors).toEqual([]);
}

test(retainedSubmissionReachesSequencer);
