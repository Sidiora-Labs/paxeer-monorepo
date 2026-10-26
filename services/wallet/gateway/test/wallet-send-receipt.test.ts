import Fastify, { type FastifyInstance } from 'fastify';
import { generateKeyPairSync, randomUUID, sign } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, parseTransaction, toHex, type Hex } from 'viem';
import type { RpcPool, SequencerReceiptResult } from '../src/rpc/pool.js';
import { CHAIN_ID, startIdentity, type Identity } from './e2e/attestors.js';
import { TestChain } from './support/chain.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const RECIPIENT = '0x000000000000000000000000000000000000c0de';
const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000';
const sequencerKey = generateKeyPairSync('ed25519');
const keyId = keccak256(toHex(sequencerKey.publicKey.export({ type: 'spki', format: 'der' })));
const requests: { method: string; params: unknown[] }[] = [];
const receipts = new Map<Hex, SequencerReceiptResult>();
const chain = new TestChain(CHAIN_ID);
let pending = false;
let identity: Identity;
let postgres: EphemeralPostgres;
let sequencer: Server;
let rpc: RpcPool;
let app: FastifyInstance;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');

beforeAll(async () => {
  identity = await startIdentity();
  await chain.start();
  sequencer = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number; method: string; params: unknown[];
      };
      requests.push({ method: body.method, params: body.params });
      res.setHeader('content-type', 'application/json');
      if (body.method === 'pax_sequencerReceipt') {
        const hash = body.params[0] as Hex;
        if (!receipts.has(hash)) throw new Error('receipt requested before broadcast');
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: pending ? null : receipts.get(hash) }));
        return;
      }
      if (body.method !== 'eth_sendRawTransaction') throw new Error(`unexpected sequencer method ${body.method}`);
      const upstream = await fetch(chain.url, {
        method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body),
      });
      const answer = await upstream.json() as { result?: Hex; error?: unknown };
      if (answer.result) {
        const hash = answer.result;
        const raw = body.params[0] as Hex;
        const tx = parseTransaction(raw);
        const sent = chain.sent.find((entry) => entry.hash === hash)!;
        receipts.set(hash, {
          rung: 'receipted', txHash: hash, nonce: toHex(tx.nonce!),
          seq: toHex(chain.sent.length), height: toHex(chain.block), index: '0x0', timestamp: '0x65000000',
          status: '0x1', returnData: '0x', logs: [], gasUsed: '0x5208', cumulativeGasUsed: '0x5208',
          effectiveGasPrice: toHex(tx.maxFeePerGas!), feeDenom: 'PAX',
          feeAmount: toHex(21_000n * tx.maxFeePerGas!), sponsor: ZERO_ADDRESS,
          from: sent.from as Hex, to: RECIPIENT, contractAddress: ZERO_ADDRESS, vmError: '', keyId,
          receipt: toHex(sign(null, Buffer.from(hash.slice(2), 'hex'), sequencerKey.privateKey)),
        });
      }
      res.end(JSON.stringify(answer));
    } catch (error) {
      res.statusCode = 500;
      res.end(JSON.stringify({ error: String(error) }));
    }
  });
  await new Promise<void>((resolve) => sequencer.listen(0, '127.0.0.1', resolve));
  const sequencerUrl = `http://127.0.0.1:${(sequencer.address() as AddressInfo).port}`;
  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_RPC_URL = chain.url;
  process.env.HYPERPAXEER_CHAIN_ID = String(CHAIN_ID);
  process.env.RPC_URLS = chain.url;
  process.env.SEQUENCER_RPC_URL = sequencerUrl;
  process.env.LOG_LEVEL = 'error';
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  wallets = await import('../src/db/wallets.js');
  const { RpcPool } = await import('../src/rpc/pool.js');
  const { NonceStore } = await import('../src/nonce/store.js');
  const { signRoutes } = await import('../src/routes/sign.js');
  rpc = new RpcPool({
    urls: [chain.url], sequencerUrl, chainId: CHAIN_ID, lagThresholdBlocks: 20,
    timeoutMs: 3_000, healthIntervalMs: 60_000,
  });
  await rpc.checkHealth();
  const nonces = new NonceStore({
    pool: database.getPool(), chainId: CHAIN_ID,
    pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
    lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
  });
  app = Fastify();
  await app.register(signRoutes, { pool: database.getPool(), rpc, nonces, attestors: null });
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  rpc?.stop();
  await database?.closePool();
  await postgres?.stop();
  if (sequencer) await new Promise<void>((resolve, reject) => sequencer.close((error) => error ? reject(error) : resolve()));
  await chain.stop();
  await identity?.close();
}, 120_000);

async function sendAndAssert(receiptPending: boolean): Promise<void> {
  pending = receiptPending;
  for (const nonce of [undefined, 0]) {
    const userId = randomUUID();
    const { wallet } = await wallets.provisionWalletForUser(userId);
    chain.balances.set(wallet.address.toLowerCase(), 10n ** 18n);
    const start = requests.length;
    const sentBefore = chain.sent.length;
    const response = await app.inject({
      method: 'POST', url: '/v1/wallet/send',
      headers: { authorization: `Bearer ${await identity.mint(userId)}` },
      payload: { tx: {
        to: RECIPIENT, value: '1', gas: '21000', chainId: CHAIN_ID,
        maxFeePerGas: '2000000000', maxPriorityFeePerGas: '1000000000',
        ...(nonce === undefined ? {} : { nonce }),
      } },
    });
    expect(response.statusCode, response.body).toBe(200);
    expect(chain.sent).toHaveLength(sentBefore + 1);
    const sent = chain.sent[sentBefore]!;
    expect(sent.from).toBe(wallet.address.toLowerCase());
    expect(sent.to).toBe(RECIPIENT);
    expect(sent.value).toBe(1n);
    const calls = requests.slice(start);
    expect(calls.map((call) => call.method)).toEqual(['eth_sendRawTransaction', 'pax_sequencerReceipt']);
    expect(calls[1]!.params).toEqual([sent.hash]);
    expect(keccak256(calls[0]!.params[0] as Hex)).toBe(sent.hash);
    expect(receipts.has(sent.hash)).toBe(true);
    expect(response.json()).toEqual({
      tx_hash: sent.hash, address: wallet.address, chain_id: CHAIN_ID,
      rung: receiptPending ? 'submitted' : 'receipted',
      receipt: receiptPending ? null : receipts.get(sent.hash),
    });
  }
}

async function walletSendReturnsSequencerReceipt(): Promise<void> {
  await sendAndAssert(false);
}

async function walletSendReportsSubmittedWhenReceiptPending(): Promise<void> {
  await sendAndAssert(true);
}

test(walletSendReturnsSequencerReceipt);
test(walletSendReportsSubmittedWhenReceiptPending);
