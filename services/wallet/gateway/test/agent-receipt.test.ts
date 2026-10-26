import { randomBytes } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import Fastify, { type FastifyInstance } from 'fastify';
import { decodeFunctionData, erc20Abi, getAddress, keccak256, parseTransaction, recoverTransactionAddress, type Hex } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { SequencerReceiptResult } from '../src/rpc/pool.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { newAgentKey, type AgentKey } from './support/identity.js';

const recipient = '0x00000000000000000000000000000000000000e1';
const token = '0x00000000000000000000000000000000000000e2';
const zeroAddress = '0x0000000000000000000000000000000000000000';
const broadcasts = new Map<Hex, Hex>();
const receiptRequests: Hex[] = [];
const receiptResults = new Map<Hex, SequencerReceiptResult | null>();
let pendingReceipt = false;
let rpcServer: Server;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let database: typeof import('../src/db/pool.js');
let rpc: typeof import('../src/rpc/pool.js');
let verifier: typeof import('../src/agent/verify.js');

function sequencerReceipt(raw: Hex): SequencerReceiptResult {
  const tx = parseTransaction(raw);
  return {
    txHash: keccak256(raw), nonce: `0x${tx.nonce!.toString(16)}`, rung: 'receipted',
    seq: '0x2a', height: '0x64', index: '0x0', timestamp: '0x6553f100', status: '0x1',
    returnData: '0x', logs: [{ address: token, topics: [`0x${'12'.repeat(32)}`], data: '0x1234', logIndex: '0x0' }],
    gasUsed: '0x5208', cumulativeGasUsed: '0xa410', effectiveGasPrice: '0x3b9aca00',
    feeDenom: 'upax', feeAmount: '0x7', sponsor: zeroAddress,
    from: zeroAddress, to: tx.to!, contractAddress: zeroAddress, vmError: '',
    keyId: `0x${'34'.repeat(32)}`, receipt: '0x010203040506070809',
  };
}

beforeAll(async () => {
  rpcServer = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
      id: number; method: string; params: Hex[];
    };
    let result: unknown;
    switch (body.method) {
      case 'eth_blockNumber': result = '0x64'; break;
      case 'eth_getTransactionCount': result = '0x0'; break;
      case 'eth_estimateGas': result = '0x186a0'; break;
      case 'eth_getBlockByNumber': result = { number: '0x64', baseFeePerGas: '0x3b9aca00' }; break;
      case 'eth_maxPriorityFeePerGas': result = '0x3b9aca00'; break;
      case 'eth_sendRawTransaction': {
        const raw = body.params[0]!;
        const hash = keccak256(raw);
        broadcasts.set(hash, raw);
        const receipt = pendingReceipt ? null : {
          ...sequencerReceipt(raw), from: await recoverTransactionAddress({ serializedTransaction: raw }),
        };
        receiptResults.set(hash, receipt);
        result = hash;
        break;
      }
      case 'pax_sequencerReceipt': {
        const hash = body.params[0]!;
        receiptRequests.push(hash);
        result = receiptResults.get(hash);
        break;
      }
      default:
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, error: { code: -32601, message: 'method not found' } }));
        return;
    }
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
  });
  await new Promise<void>((resolve) => rpcServer.listen(0, '127.0.0.1', resolve));
  const rpcUrl = `http://127.0.0.1:${(rpcServer.address() as AddressInfo).port}`;
  process.env.RPC_URLS = rpcUrl;
  process.env.HYPERPAXEER_RPC_URL = rpcUrl;
  process.env.SEQUENCER_RPC_URL = rpcUrl;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  pg = await startPostgres();
  database = await import('../src/db/pool.js');
  rpc = await import('../src/rpc/pool.js');
  verifier = await import('../src/agent/verify.js');
  const { agentRoutes } = await import('../src/routes/agent.js');
  app = Fastify();
  verifier.installRawBodyParser(app);
  await app.register(agentRoutes);
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  rpc?.sharedRpcPool().stop();
  await database?.closePool();
  await pg?.stop();
  await new Promise<void>((resolve, reject) => {
    if (!rpcServer) return resolve();
    rpcServer.close((error) => error ? reject(error) : resolve());
  });
}, 120_000);

async function provisionAgent(label: string): Promise<{ agent: AgentKey; address: Hex }> {
  const agent = newAgentKey(label);
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  await agents.upsertPrincipalOnVerify({
    did: agent.did, label, keyFingerprint: agent.publicKeyHex.slice(0, 16), publicKey: agent.publicKeyHex,
  });
  await agents.setPrincipalFrozen(agent.did, false);
  await database.query(
    `update agent_policies set mode = 'full', allow_native_transfer = true,
       withdrawal_allowlist_only = false, max_tx_value_wei = 1, max_daily_value_wei = 1,
       max_approve_wei = 1000 where did = $1`, [agent.did],
  );
  const { wallet } = await wallets.provisionWalletForUser(wallets.agentWalletUserId(agent.did), 'agent');
  await agents.setPrincipalWallet(agent.did, wallet.id);
  return { agent, address: wallet.address };
}

async function signedPost(agent: AgentKey, url: string, payload: unknown) {
  const body = JSON.stringify(payload);
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
  const digest = verifier.agentRequestDigest({
    method: `POST ${url}`, keyId: agent.did, nonce, expiry, body: Buffer.from(body),
  });
  return app.inject({
    method: 'POST', url, payload: body,
    headers: {
      'content-type': 'application/json', 'x-agent-key': agent.did,
      'x-agent-nonce': nonce.toString('hex'), 'x-agent-expires': expiry.toString(),
      'x-agent-signature': agent.sign(digest),
    },
  });
}

async function agentSendReturnsSequencerReceipt() {
  pendingReceipt = false;
  const { agent, address } = await provisionAgent('receipt-send');
  const response = await signedPost(agent, '/v1/agent/send', {
    tx: { to: recipient, value: '1', gas: '21000', maxFeePerGas: '2000000000', maxPriorityFeePerGas: '1000000000' },
  });
  expect(response.statusCode, response.body).toBe(200);
  const body = response.json();
  const raw = broadcasts.get(body.tx_hash)!;
  expect(raw).toBeDefined();
  expect(body.tx_hash).toBe(keccak256(raw));
  expect(body).toEqual({ tx_hash: keccak256(raw), address, chain_id: 125, op: 'send', rung: 'receipted', receipt: receiptResults.get(body.tx_hash) });
  expect(body.receipt).not.toBeNull();
  expect(body.receipt.txHash).toBe(body.tx_hash);
  expect(receiptRequests.filter((hash) => hash === body.tx_hash)).toEqual([body.tx_hash]);
  const transaction = parseTransaction(raw);
  expect(transaction.to?.toLowerCase()).toBe(recipient);
  expect(transaction.value).toBe(1n);
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(address.toLowerCase());
}

async function agentTransferReturnsSequencerReceipt() {
  pendingReceipt = false;
  const { agent, address } = await provisionAgent('receipt-transfer');
  const response = await signedPost(agent, '/v1/agent/transfer', { token, to: recipient, amount: '27' });
  expect(response.statusCode, response.body).toBe(200);
  const body = response.json();
  const raw = broadcasts.get(body.tx_hash)!;
  expect(raw).toBeDefined();
  expect(body).toEqual({ tx_hash: keccak256(raw), address, chain_id: 125, op: 'erc20.transfer', rung: 'receipted', receipt: receiptResults.get(body.tx_hash) });
  expect(body.receipt).not.toBeNull();
  expect(body.receipt.txHash).toBe(body.tx_hash);
  expect(receiptRequests.filter((hash) => hash === body.tx_hash)).toEqual([body.tx_hash]);
  const transaction = parseTransaction(raw);
  expect(transaction.to?.toLowerCase()).toBe(token);
  expect(transaction.value ?? 0n).toBe(0n);
  expect(decodeFunctionData({ abi: erc20Abi, data: transaction.data! })).toEqual({ functionName: 'transfer', args: [getAddress(recipient), 27n] });
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(address.toLowerCase());
}

async function agentApproveReturnsSequencerReceipt() {
  pendingReceipt = true;
  const { agent, address } = await provisionAgent('receipt-approve');
  const response = await signedPost(agent, '/v1/agent/approve', { token, spender: recipient, amount: '41' });
  expect(response.statusCode, response.body).toBe(200);
  const body = response.json();
  const raw = broadcasts.get(body.tx_hash)!;
  expect(raw).toBeDefined();
  expect(receiptResults.has(body.tx_hash)).toBe(true);
  expect(receiptResults.get(body.tx_hash)).toBeNull();
  expect(body).toEqual({ tx_hash: keccak256(raw), address, chain_id: 125, op: 'erc20.approve', rung: 'submitted', receipt: null });
  expect(receiptRequests.filter((hash) => hash === body.tx_hash)).toEqual([body.tx_hash]);
  const transaction = parseTransaction(raw);
  expect(transaction.to?.toLowerCase()).toBe(token);
  expect(decodeFunctionData({ abi: erc20Abi, data: transaction.data! })).toEqual({ functionName: 'approve', args: [getAddress(recipient), 41n] });
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(address.toLowerCase());
}

test(agentSendReturnsSequencerReceipt, agentSendReturnsSequencerReceipt);
test(agentTransferReturnsSequencerReceipt, agentTransferReturnsSequencerReceipt);
test(agentApproveReturnsSequencerReceipt, agentApproveReturnsSequencerReceipt);
