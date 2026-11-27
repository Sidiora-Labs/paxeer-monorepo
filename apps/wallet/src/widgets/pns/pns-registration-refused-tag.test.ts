// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import * as workerThreads from 'node:worker_threads';
import { ethers } from 'ethers';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { controllerAbi, REGISTRATION_DURATION_1Y } from '@/lib/pns';
import { PnsController, PnsRegistrationJournal, PnsRegistrationSession } from './pnsRegistration';

const wallet = new ethers.Wallet(`0x${'01'.padStart(64, '0')}`);
const account = wallet.address;
const blockHash = ethers.id('pns numbered block');
const requests: { method: string; tag: string | null }[] = [];
const transactions = new Map<string, Record<string, unknown>>();
const receipts = new Map<string, Record<string, unknown>>();
const unexpectedErrors: unknown[] = [];
let servesPending = false;
let refusesLatestCount = false;
let pendingBlock: 'block' | 'null' = 'block';
let numberedBlock: 'block' | 'null' | 'error' = 'block';
let losesResponse = false;
let mined = true;
let server: Server;
let provider: ethers.JsonRpcProvider;
let signer: ethers.JsonRpcSigner;
const deployment = {
  controller: '0x0000000000000000000000000000000000000123',
  publicResolver: '0x0000000000000000000000000000000000000456',
};
const abi = new ethers.Interface([
  ...controllerAbi,
  'function commitments(bytes32) view returns (uint256)',
  'function maxCommitmentAge() view returns (uint256)',
]);
let client: PnsController;

class RpcRefusal extends Error {}

function counts() {
  return requests.filter(({ method }) => method === 'eth_getTransactionCount');
}

function blocks() {
  return requests.filter(({ method }) => method === 'eth_getBlockByNumber');
}

function sends() {
  return requests.filter(({ method }) => method === 'eth_sendTransaction');
}

function block(prefetch: boolean) {
  return {
    number: '0x1',
    hash: blockHash,
    parentHash: ethers.ZeroHash,
    timestamp: '0x6553f100',
    nonce: '0x0000000000000000',
    difficulty: '0x0',
    gasLimit: '0x1c9c380',
    gasUsed: '0x30d40',
    miner: ethers.ZeroAddress,
    extraData: '0x',
    transactions: [...transactions.keys()].map((hash) =>
      prefetch ? transactions.get(hash) : hash
    ),
  };
}

function mineReceipts() {
  for (const [hash, tx] of transactions) {
    tx.blockHash = blockHash;
    tx.blockNumber = '0x1';
    tx.transactionIndex = '0x0';
    receipts.set(hash, {
      transactionHash: hash,
      transactionIndex: '0x0',
      blockHash,
      blockNumber: '0x1',
      from: account,
      to: tx.to,
      contractAddress: null,
      status: '0x1',
      type: '0x0',
      gasUsed: '0x30d40',
      cumulativeGasUsed: '0x30d40',
      effectiveGasPrice: '0x3b9aca00',
      logsBloom: `0x${'00'.repeat(256)}`,
      logs: [],
    });
  }
}

async function rpc(method: string, params: unknown[]): Promise<unknown> {
  const tag =
    method === 'eth_getBlockByNumber'
      ? String(params[0])
      : method === 'eth_getTransactionCount' || method === 'eth_call'
        ? String(params[1])
        : null;
  requests.push({ method, tag });
  if (tag === 'pending' && !servesPending) throw new RpcRefusal('pending served by sequencer');
  switch (method) {
    case 'eth_chainId':
      return '0x7d';
    case 'eth_accounts':
      return [account];
    case 'eth_blockNumber':
      return '0x1';
    case 'eth_estimateGas':
      return '0x30d40';
    case 'eth_getTransactionCount':
      if (tag === 'latest' && refusesLatestCount) throw new RpcRefusal('latest count unavailable');
      return tag === 'pending' ? '0x9' : '0x4';
    case 'eth_getBlockByNumber':
      if (tag === 'pending' && pendingBlock === 'null') return null;
      if (tag !== 'pending' && tag !== 'latest') {
        if (numberedBlock === 'error') throw new RpcRefusal('numbered block unavailable');
        if (numberedBlock === 'null') return null;
      }
      return block(params[1] === true);
    case 'eth_getTransactionByHash':
      return transactions.get(String(params[0])) ?? null;
    case 'eth_getTransactionReceipt':
      return receipts.get(String(params[0])) ?? null;
    case 'eth_call': {
      const call = params[0] as { to: string; data: string };
      expect(call.to.toLowerCase()).toBe(deployment.controller);
      const decoded = abi.parseTransaction(call)!;
      switch (decoded.name) {
        case 'available':
          return abi.encodeFunctionResult('available', [true]);
        case 'rentPrice':
          return abi.encodeFunctionResult('rentPrice', [100n, 0n]);
        case 'makeCommitment':
          return abi.encodeFunctionResult('makeCommitment', [
            ethers.keccak256(
              ethers.AbiCoder.defaultAbiCoder().encode(
                ['tuple(string,address,uint256,bytes32,address,bytes[],uint8,bytes32)'],
                [decoded.args[0]]
              )
            ),
          ]);
        case 'commitments':
          return abi.encodeFunctionResult('commitments', [1700000000n]);
        case 'minCommitmentAge':
          return abi.encodeFunctionResult('minCommitmentAge', [60n]);
        case 'maxCommitmentAge':
          return abi.encodeFunctionResult('maxCommitmentAge', [86400n]);
        default:
          throw new Error(`Unexpected contract call ${decoded.name}`);
      }
    }
    case 'eth_sendTransaction': {
      const request = params[0] as {
        from: string;
        to: string;
        data: string;
        nonce: string;
        value?: string;
      };
      const saved = journal().current()!.transactions.at(-1)!;
      expect(saved).toMatchObject({
        hash: null,
        nonce: Number(BigInt(request.nonce)),
        data: request.data,
      });
      expect(request.from.toLowerCase()).toBe(account.toLowerCase());
      expect(abi.parseTransaction(request)?.name).toBe('commit');
      const raw = await wallet.signTransaction({
        chainId: 125,
        nonce: Number(BigInt(request.nonce)),
        to: request.to,
        data: request.data,
        value: request.value ?? 0,
        gasLimit: 500000,
        gasPrice: 1000000000,
        type: 0,
      });
      const tx = ethers.Transaction.from(raw);
      const hash = tx.hash!;
      transactions.set(hash, {
        hash,
        from: tx.from,
        to: tx.to,
        input: tx.data,
        value: ethers.toQuantity(tx.value),
        nonce: ethers.toQuantity(tx.nonce),
        chainId: '0x7d',
        type: '0x0',
        gas: '0x7a120',
        gasPrice: '0x3b9aca00',
        v: ethers.toQuantity(tx.signature!.networkV!),
        r: tx.signature!.r,
        s: tx.signature!.s,
        blockNumber: null,
        blockHash: null,
        transactionIndex: null,
      });
      if (mined) mineReceipts();
      if (losesResponse) throw new RpcRefusal('transaction response unavailable');
      return hash;
    }
    default:
      throw new Error(`Unexpected JSON-RPC method ${method}`);
  }
}

beforeAll(async () => {
  const locks: unknown = Reflect.get(workerThreads, 'locks');
  if (
    typeof locks !== 'object' ||
    locks === null ||
    !('request' in locks) ||
    typeof locks.request !== 'function'
  )
    throw new Error('Transaction recovery requires Node Web Locks');
  Object.defineProperty(navigator, 'locks', { value: locks, configurable: true });
  server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString()) as {
      id: number;
      method: string;
      params: unknown[];
    };
    response.setHeader('content-type', 'application/json');
    try {
      const result = await rpc(body.method, body.params);
      response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
    } catch (error) {
      if (!(error instanceof RpcRefusal)) unexpectedErrors.push(error);
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          error: {
            code: -32000,
            message: error instanceof Error ? error.message : String(error),
          },
        })
      );
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string')
    throw new Error('JSON-RPC server did not bind a port');
  provider = new ethers.JsonRpcProvider(`http://127.0.0.1:${address.port}`, 125, {
    staticNetwork: true,
    batchMaxCount: 1,
    cacheTimeout: -1,
  });
  signer = await provider.getSigner(account);
  client = new PnsController(provider, deployment);
});

afterAll(async () => {
  provider?.destroy();
  if (server)
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
});

beforeEach(() => {
  requests.length = 0;
  transactions.clear();
  receipts.clear();
  unexpectedErrors.length = 0;
  localStorage.clear();
  servesPending = false;
  refusesLatestCount = false;
  pendingBlock = 'block';
  numberedBlock = 'block';
  losesResponse = false;
  mined = true;
});

afterEach(() => {
  expect(unexpectedErrors).toEqual([]);
});

function journal() {
  return new PnsRegistrationJournal(account, 125, deployment.controller);
}
function session() {
  return new PnsRegistrationSession(
    journal(),
    client,
    () => {},
    () => true
  );
}
function start(boundary: PnsRegistrationSession) {
  return boundary.start(signer, 'register', REGISTRATION_DURATION_1Y);
}

async function pns_commit_goes_out_when_the_node_refuses_the_pending_tag() {
  const boundary = session();
  await start(boundary);
  expect(counts()).toEqual([
    { method: 'eth_getTransactionCount', tag: 'pending' },
    { method: 'eth_getTransactionCount', tag: 'latest' },
  ]);
  expect(sends()).toHaveLength(1);
  expect(requests.indexOf(counts()[1])).toBeLessThan(requests.indexOf(sends()[0]));
  const saved = journal().current()!.transactions[0];
  expect(saved).toMatchObject({ kind: 'commit', nonce: 4 });
  expect(saved.hash).toBe([...transactions.keys()][0]);
  expect(transactions.get(saved.hash!)?.nonce).toBe('0x4');
  boundary.close();
}

async function pns_resume_finds_a_saved_transaction_without_the_pending_block() {
  losesResponse = true;
  const first = session();
  await expect(start(first)).rejects.toThrow('transaction response unavailable');
  const original = journal().current()!;
  expect(original.transactions[0].hash).toBeNull();
  expect(sends()).toHaveLength(1);
  first.close();
  const reopened = session();
  servesPending = true;
  pendingBlock = 'null';
  requests.length = 0;
  await expect(reopened.resume(signer, false)).rejects.toThrow('could not read a block');
  expect(blocks()).toEqual([{ method: 'eth_getBlockByNumber', tag: 'pending' }]);
  servesPending = false;
  for (const mode of ['error', 'null'] as const) {
    numberedBlock = mode;
    requests.length = 0;
    await expect(reopened.resume(signer, false)).rejects.toThrow(
      mode === 'error' ? 'numbered block unavailable' : 'could not read a block'
    );
    expect(blocks()).toEqual([
      { method: 'eth_getBlockByNumber', tag: 'pending' },
      { method: 'eth_getBlockByNumber', tag: '0x1' },
    ]);
    expect(journal().current()!.transactions[0]).toEqual(original.transactions[0]);
    expect(sends()).toEqual([]);
  }
  numberedBlock = 'block';
  requests.length = 0;
  await reopened.resume(signer, false);
  const recovered = journal().current()!;
  expect(recovered).toMatchObject({
    secret: original.secret,
    commitmentHash: original.commitmentHash,
    status: 'waiting',
  });
  expect(recovered.transactions[0]).toMatchObject({
    hash: [...transactions.keys()][0],
    nonce: 4,
    scannedThrough: 1,
  });
  expect(blocks()).toEqual([
    { method: 'eth_getBlockByNumber', tag: 'pending' },
    { method: 'eth_getBlockByNumber', tag: '0x1' },
    { method: 'eth_getBlockByNumber', tag: 'latest' },
  ]);
  expect(counts()).toEqual([]);
  expect(sends()).toEqual([]);
  reopened.close();
}

async function pns_count_stays_at_the_pending_tag_where_the_node_serves_it() {
  servesPending = true;
  losesResponse = true;
  mined = false;
  const first = session();
  await expect(start(first)).rejects.toThrow('transaction response unavailable');
  expect(counts()).toEqual([{ method: 'eth_getTransactionCount', tag: 'pending' }]);
  expect(journal().current()!.transactions[0]).toMatchObject({ nonce: 9, hash: null });
  first.close();
  const reopened = session();
  await reopened.resume(signer, false);
  expect(blocks()).toEqual([{ method: 'eth_getBlockByNumber', tag: 'pending' }]);
  expect(journal().current()!.transactions[0].hash).toBe([...transactions.keys()][0]);
  expect(transactions.values().next().value?.nonce).toBe('0x9');
  expect(sends()).toHaveLength(1);
  reopened.close();
}

async function pns_commit_ends_when_no_count_can_be_read() {
  refusesLatestCount = true;
  const boundary = session();
  await expect(start(boundary)).rejects.toMatchObject({
    code: 'UNKNOWN_ERROR',
    shortMessage: 'could not coalesce error',
    error: { code: -32000, message: 'latest count unavailable' },
  });
  expect(counts()).toEqual([
    { method: 'eth_getTransactionCount', tag: 'pending' },
    { method: 'eth_getTransactionCount', tag: 'latest' },
  ]);
  expect(sends()).toEqual([]);
  expect(transactions.size).toBe(0);
  expect(journal().current()!.transactions).toEqual([]);
  boundary.close();
}

test(
  pns_commit_goes_out_when_the_node_refuses_the_pending_tag.name,
  { timeout: 4000 },
  pns_commit_goes_out_when_the_node_refuses_the_pending_tag
);

test(
  pns_resume_finds_a_saved_transaction_without_the_pending_block.name,
  { timeout: 4000 },
  pns_resume_finds_a_saved_transaction_without_the_pending_block
);

test(
  pns_count_stays_at_the_pending_tag_where_the_node_serves_it.name,
  { timeout: 4000 },
  pns_count_stays_at_the_pending_tag_where_the_node_serves_it
);

test(
  pns_commit_ends_when_no_count_can_be_read.name,
  { timeout: 4000 },
  pns_commit_ends_when_no_count_can_be_read
);
