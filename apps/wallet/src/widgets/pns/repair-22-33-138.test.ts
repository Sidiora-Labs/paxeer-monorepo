// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import * as workerThreads from 'node:worker_threads';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { ethers } from 'ethers';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { controllerAbi, REGISTRATION_DURATION_1Y } from '@/lib/pns';
import {
  PnsController,
  PnsRegistrationJournal,
  PnsRegistrationSession,
  pnsRegistrationTuple,
} from './pnsRegistration';
import { usePnsSearch } from './usePnsSearch';
import { usePnsRegistration } from './usePnsRegistration';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const fixtureWallet = new ethers.Wallet(`0x${'01'.padStart(64, '0')}`);
const account = fixtureWallet.address;
const deployment = {
  controller: '0x0000000000000000000000000000000000000123',
  publicResolver: '0x0000000000000000000000000000000000000456',
};
const abi = new ethers.Interface([
  ...controllerAbi,
  'function commitments(bytes32) view returns (uint256)',
  'function maxCommitmentAge() view returns (uint256)',
]);
const tupleType = 'tuple(string,address,uint256,bytes32,address,bytes[],uint8,bytes32)';
const blockHash = ethers.id('pns registration transcript block');
const transactions = new Map<string, Record<string, unknown>>();
const receipts = new Map<string, Record<string, unknown>>();
const commitments = new Map<string, bigint>();
const sent: string[] = [];
const requested: string[] = [];
const held = new Map<string, { promise: Promise<void>; release: () => void }>();
const fixtureErrors: unknown[] = [];
let now = 1700000000;
let mine = true;
let loseResponse = false;
let nextStatus = 1;
let server: Server;
let provider: ethers.JsonRpcProvider;
let signer: ethers.JsonRpcSigner;
let client: PnsController;
let root: Root | null = null;
let container: HTMLDivElement;
let searchState: ReturnType<typeof usePnsSearch> | null;
let registrationState: ReturnType<typeof usePnsRegistration> | null;
let currentAccount = account;
let registrationQuery = 'restart';
let registrationDuration = REGISTRATION_DURATION_1Y;

function journal(owner = account, chainId = 125) {
  return new PnsRegistrationJournal(owner, chainId, deployment.controller);
}

function commitmentHash(tuple: readonly unknown[]): string {
  return ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode([tupleType], [tuple]));
}

function fixtureBlock(pending: boolean, prefetch: boolean) {
  return {
    number: '0x1',
    hash: blockHash,
    parentHash: ethers.ZeroHash,
    timestamp: ethers.toQuantity(now),
    nonce: '0x0000000000000000',
    difficulty: '0x0',
    gasLimit: '0x1c9c380',
    gasUsed: '0x30d40',
    miner: ethers.ZeroAddress,
    extraData: '0x',
    transactions: [...transactions.keys()]
      .filter((hash) => pending || receipts.has(hash))
      .map((hash) => (prefetch ? transactions.get(hash) : hash)),
  };
}

function mineReceipts(): void {
  for (const [hash, tx] of transactions) {
    if (receipts.has(hash)) continue;
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
      status: ethers.toQuantity(nextStatus),
      type: '0x0',
      gasUsed: '0x30d40',
      cumulativeGasUsed: '0x30d40',
      effectiveGasPrice: '0x3b9aca00',
      logsBloom: `0x${'00'.repeat(256)}`,
      logs: [],
    });
    if (nextStatus === 0) continue;
    const decoded = abi.parseTransaction({ data: String(tx.input) })!;
    if (decoded.name === 'commit') commitments.set(decoded.args[0], BigInt(now));
    if (decoded.name === 'register') commitments.delete(commitmentHash(decoded.args[0]));
  }
}

async function rpc(method: string, params: unknown[]): Promise<unknown> {
  switch (method) {
    case 'eth_chainId':
      return '0x7d';
    case 'eth_accounts':
      return [account];
    case 'eth_blockNumber':
      return '0x1';
    case 'eth_getTransactionCount':
      return ethers.toQuantity(transactions.size);
    case 'eth_getTransactionByHash':
      return transactions.get(String(params[0])) ?? null;
    case 'eth_getTransactionReceipt':
      return receipts.get(String(params[0])) ?? null;
    case 'eth_getBlockByNumber':
      return fixtureBlock(params[0] === 'pending', params[1] === true);
    case 'eth_call': {
      const call = params[0] as { data: string; to: string };
      expect(call.to.toLowerCase()).toBe(deployment.controller);
      const decoded = abi.parseTransaction(call)!;
      if (decoded.name === 'available' || decoded.name === 'rentPrice') {
        const label = String(decoded.args[0]);
        const key = `${decoded.name}:${label}:${decoded.name === 'rentPrice' ? decoded.args[1] : ''}`;
        requested.push(key);
        await held.get(key)?.promise;
        return decoded.name === 'available'
          ? abi.encodeFunctionResult('available', [label !== 'taken'])
          : abi.encodeFunctionResult('rentPrice', [BigInt(label.length) * decoded.args[1], 0n]);
      }
      if (decoded.name === 'makeCommitment')
        return abi.encodeFunctionResult('makeCommitment', [commitmentHash(decoded.args[0])]);
      if (decoded.name === 'commitments')
        return abi.encodeFunctionResult('commitments', [commitments.get(decoded.args[0]) ?? 0n]);
      if (decoded.name === 'minCommitmentAge')
        return abi.encodeFunctionResult('minCommitmentAge', [60n]);
      if (decoded.name === 'maxCommitmentAge')
        return abi.encodeFunctionResult('maxCommitmentAge', [86400n]);
      throw new Error(`Unexpected contract call ${decoded.name}`);
    }
    case 'eth_sendTransaction': {
      const request = params[0] as {
        from: string;
        to: string;
        data: string;
        nonce: string;
        value?: string;
      };
      const saved = journal().current()!;
      expect(saved.account).toBe(account.toLowerCase());
      expect(saved.chainId).toBe('125');
      expect(saved.commitmentHash).toBe(commitmentHash(pnsRegistrationTuple(saved)));
      expect(saved.secret).toMatch(/^0x[0-9a-f]{64}$/);
      expect(saved.transactions.at(-1)).toMatchObject({
        hash: null,
        nonce: Number(BigInt(request.nonce)),
        data: request.data,
      });
      expect(request.from.toLowerCase()).toBe(saved.account);
      const decoded = abi.parseTransaction(request)!;
      sent.push(decoded.name);
      if (decoded.name === 'register') {
        const hash = commitmentHash(decoded.args[0]);
        const age = BigInt(now) - (commitments.get(hash) ?? 0n);
        expect(hash).toBe(saved.commitmentHash);
        expect(age >= 60n && age < 86400n).toBe(true);
        expect(decoded.args[0][3]).toBe(saved.secret);
        expect(BigInt(request.value!)).toBe(
          (BigInt(saved.label.length * saved.duration) * 110n) / 100n
        );
      }
      const raw = await fixtureWallet.signTransaction({
        to: request.to,
        data: request.data,
        nonce: Number(BigInt(request.nonce)),
        value: request.value ?? 0,
        chainId: 125,
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
      if (mine) mineReceipts();
      if (loseResponse) {
        loseResponse = false;
        throw new Error('Fixture lost response after accepting transaction');
      }
      return hash;
    }
    default:
      throw new Error(`Unexpected JSON-RPC method ${method}`);
  }
}

beforeAll(async () => {
  const locks: unknown = Reflect.get(workerThreads, 'locks');
  if (!locks) throw new Error('PNS recovery tests require Node Web Locks');
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
    } catch (reason) {
      if (!(reason instanceof Error && reason.message.startsWith('Fixture lost response')))
        fixtureErrors.push(reason);
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          error: {
            code: -32000,
            message: reason instanceof Error ? reason.message : String(reason),
          },
        })
      );
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('PNS fixture did not bind a port');
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
  transactions.clear();
  receipts.clear();
  commitments.clear();
  sent.length = 0;
  requested.length = 0;
  held.clear();
  fixtureErrors.length = 0;
  localStorage.clear();
  now = 1700000000;
  mine = true;
  loseResponse = false;
  nextStatus = 1;
  currentAccount = account;
  registrationQuery = 'restart';
  registrationDuration = REGISTRATION_DURATION_1Y;
  searchState = null;
  registrationState = null;
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(async () => {
  for (const item of held.values()) item.release();
  await stop();
  container.remove();
  expect(fixtureErrors).toEqual([]);
});

async function stop() {
  await act(async () => root?.unmount());
  root = null;
}

async function until(check: () => boolean) {
  await expect
    .poll(
      async () => {
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 10));
        });
        return check();
      },
      { timeout: 4000 }
    )
    .toBe(true);
}

function hold(key: string) {
  let release!: () => void;
  const promise = new Promise<void>((resolve) => {
    release = resolve;
  });
  held.set(key, { promise, release });
  return release;
}

function SearchConsumer({ query, duration }: { query: string; duration: number }) {
  const value = usePnsSearch(client, query, duration);
  useLayoutEffect(() => {
    searchState = value;
  });
  return createElement(
    'output',
    null,
    value.result
      ? `${value.result.label}:${value.result.duration}:${value.result.price.total}`
      : 'unvalidated'
  );
}

async function search(query: string, duration = REGISTRATION_DURATION_1Y) {
  await act(async () => {
    root ??= createRoot(container);
    root.render(createElement(SearchConsumer, { query, duration }));
  });
}

function RegistrationConsumer() {
  const search = usePnsSearch(client, registrationQuery, registrationDuration);
  useLayoutEffect(() => {
    searchState = search;
  });
  const value = usePnsRegistration({
    account: currentAccount,
    chainId: 125,
    client,
    getSigner: async () => signer,
    isCurrent: () => currentAccount === account,
  });
  useLayoutEffect(() => {
    registrationState = value;
  });
  return createElement(
    'output',
    null,
    value.intent
      ? `${value.intent.label}:${value.intent.status}:${value.intent.countdown}`
      : 'no intent'
  );
}

function registration() {
  if (!registrationState) throw new Error('PNS registration consumer did not render');
  return registrationState;
}

async function mount() {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(RegistrationConsumer));
  });
  await until(() => !registration().busy);
}

function session(owner = account) {
  return new PnsRegistrationSession(
    journal(owner),
    client,
    () => {},
    () => currentAccount === account
  );
}

async function pns_stale_search_rejects_out_of_order_query_duration_and_clear() {
  const releaseAlpha = hold('available:taken:');
  await search('taken');
  await until(() => requested.includes('available:taken:'));
  await search('bravo');
  await until(() => searchState?.result?.label === 'bravo');
  const bravo = searchState!.validated();
  expect(bravo).toMatchObject({
    label: 'bravo',
    duration: REGISTRATION_DURATION_1Y,
    available: true,
  });
  releaseAlpha();
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 50));
  });
  expect(searchState!.result?.available).toBe(true);
  expect(searchState!.validated()).toEqual(bravo);

  const releaseOldPrice = hold(`rentPrice:bravo:${REGISTRATION_DURATION_1Y * 2}`);
  await search('bravo', REGISTRATION_DURATION_1Y * 2);
  expect(searchState!.validated()).toBeNull();
  await until(() => requested.includes(`rentPrice:bravo:${REGISTRATION_DURATION_1Y * 2}`));
  await search('bravo', REGISTRATION_DURATION_1Y * 3);
  await until(() => searchState?.result?.duration === REGISTRATION_DURATION_1Y * 3);
  releaseOldPrice();
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 50));
  });
  expect(searchState!.validated()!.price.total).toBe(BigInt(5 * REGISTRATION_DURATION_1Y * 3));

  const releaseClear = hold('available:charlie:');
  await search('charlie');
  await until(() => requested.includes('available:charlie:'));
  await search('');
  releaseClear();
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 50));
  });
  expect(searchState!.validated()).toBeNull();
  expect(searchState!.result).toBeNull();
  expect(searchState!.searching).toBe(false);
  expect(container.textContent).toBe('unvalidated');
  expect(sent).toEqual([]);
}

async function pns_stale_search_cannot_authorize_a_different_registration_identity() {
  await mount();
  await until(() => searchState?.result?.label === 'restart');
  const validate = searchState!.validated;
  registrationQuery = 'taken';
  await act(async () => root!.render(createElement(RegistrationConsumer)));
  await act(async () => registration().start(validate));
  expect(journal().current()).toBeNull();
  expect(sent).toEqual([]);
  await until(() => searchState?.result?.label === 'taken');
  await act(async () => registration().start(searchState!.validated));
  expect(sent).toEqual([]);
  registrationQuery = 'winner';
  registrationDuration = REGISTRATION_DURATION_1Y * 2;
  await act(async () => root!.render(createElement(RegistrationConsumer)));
  await until(() => searchState?.result?.label === 'winner');
  await act(async () => registration().start(searchState!.validated));
  expect(journal().current()).toMatchObject({
    label: 'winner',
    duration: REGISTRATION_DURATION_1Y * 2,
  });
  expect(sent).toEqual(['commit']);
}

async function pns_registration_journal_recovers_original_secret_after_component_restart() {
  await mount();
  await until(() => searchState?.result?.label === 'restart');
  await act(async () => registration().start(searchState!.validated));
  const original = journal().current()!;
  expect(original.status).toBe('waiting');
  expect(original.countdown).toBe(60);
  expect(original.transactions[0].hash).toMatch(/^0x[0-9a-f]{64}$/);
  await stop();
  currentAccount = '0x0000000000000000000000000000000000000002';
  await mount();
  expect(registration().intent).toBeNull();
  expect(journal().current()!.secret).toBe(original.secret);
  await stop();
  currentAccount = account;
  now += 30;
  await mount();
  expect(registration().intent).toMatchObject({
    secret: original.secret,
    commitmentHash: original.commitmentHash,
    label: 'restart',
    countdown: 30,
    status: 'waiting',
  });
  expect(sent).toEqual(['commit']);
  await stop();
  now += 31;
  await mount();
  expect(registration().intent?.status).toBe('ready');
  expect(sent).toEqual(['commit']);
  await act(async () => registration().resume());
  await until(() => registration().intent?.status === 'done');
  expect(sent).toEqual(['commit', 'register']);
  expect(registration().intent?.secret).toBe(original.secret);
  await stop();
  await mount();
  expect(registration().intent?.status).toBe('done');
  expect(sent).toEqual(['commit', 'register']);
}

async function pns_registration_recovers_lost_commit_response_without_rebroadcast() {
  loseResponse = true;
  const first = session();
  await expect(first.start(signer, 'response', REGISTRATION_DURATION_1Y)).rejects.toThrow();
  const original = journal().current()!;
  expect(original.transactions[0].hash).toBeNull();
  first.close();
  const reopened = session();
  await reopened.resume(signer, false);
  expect(journal().current()).toMatchObject({ secret: original.secret, status: 'waiting' });
  expect(journal().current()!.transactions[0].hash).toMatch(/^0x[0-9a-f]{64}$/);
  now += 61;
  await reopened.resume(signer, true);
  await reopened.resume(signer, false);
  expect(journal().current()!.status).toBe('done');
  expect(sent).toEqual(['commit', 'register']);
}

async function pns_registration_recovers_lost_reveal_response_without_duplicate_spend() {
  const first = session();
  await first.start(signer, 'reveal', REGISTRATION_DURATION_1Y);
  await first.resume(signer, false);
  now += 61;
  loseResponse = true;
  await expect(first.resume(signer, true)).rejects.toThrow();
  expect(journal().current()!.transactions[1].hash).toBeNull();
  first.close();
  const reopened = session();
  await reopened.resume(signer, true);
  expect(journal().current()!.status).toBe('done');
  expect(journal().current()!.transactions[1].hash).toMatch(/^0x[0-9a-f]{64}$/);
  expect(sent).toEqual(['commit', 'register']);
}

async function pns_registration_retains_pending_and_expires_only_reconciled_commitment() {
  mine = false;
  const first = session();
  await first.start(signer, 'pending', REGISTRATION_DURATION_1Y);
  await first.resume(signer, true);
  expect(journal().current()!.status).toBe('committing');
  expect(() => journal().acknowledge()).toThrow('Reconcile the original');
  first.close();
  now += 90000;
  const reopened = session();
  await reopened.resume(signer, true);
  expect(journal().current()!.status).toBe('committing');
  expect(sent).toEqual(['commit']);
  mineReceipts();
  await reopened.resume(signer, true);
  expect(journal().current()!.status).toBe('waiting');
  now += 86400;
  await reopened.resume(signer, true);
  expect(journal().current()!.status).toBe('expired');
  expect(sent).toEqual(['commit']);
  journal().acknowledge();
  expect(journal().current()).toBeNull();
}

async function pns_registration_refuses_cross_account_chain_and_undurable_intent() {
  const wrong = session('0x0000000000000000000000000000000000000002');
  await expect(wrong.start(signer, 'wrong', REGISTRATION_DURATION_1Y)).rejects.toThrow(
    'original account'
  );
  const wrongChain = new PnsRegistrationSession(
    journal(account, 126),
    client,
    () => {},
    () => true
  );
  await expect(wrongChain.start(signer, 'wrong', REGISTRATION_DURATION_1Y)).rejects.toThrow(
    'original account'
  );
  expect(journal('0x0000000000000000000000000000000000000002').current()).toBeNull();
  expect(journal(account, 126).current()).toBeNull();
  localStorage.setItem(journal().key, '{corrupt');
  await expect(session().start(signer, 'corrupt', REGISTRATION_DURATION_1Y)).rejects.toThrow();
  expect(sent).toEqual([]);
  localStorage.clear();
  localStorage.setItem('pns-fixture-storage-quota', 'x'.repeat(4_999_500));
  await expect(session().start(signer, 'undurable', REGISTRATION_DURATION_1Y)).rejects.toThrow(
    'could not be saved'
  );
  expect(sent).toEqual([]);
}

async function pns_registration_reconciles_reverted_commit_and_stopped_session() {
  nextStatus = 0;
  const first = session();
  await first.start(signer, 'reverted', REGISTRATION_DURATION_1Y);
  first.close();
  await expect(first.resume(signer, true)).rejects.toThrow('session changed');
  const reopened = session();
  await reopened.resume(signer, true);
  expect(journal().current()!.status).toBe('reverted');
  expect(sent).toEqual(['commit']);
}

test(
  pns_stale_search_rejects_out_of_order_query_duration_and_clear.name,
  pns_stale_search_rejects_out_of_order_query_duration_and_clear
);
test(
  pns_stale_search_cannot_authorize_a_different_registration_identity.name,
  pns_stale_search_cannot_authorize_a_different_registration_identity
);
test(
  pns_registration_journal_recovers_original_secret_after_component_restart.name,
  pns_registration_journal_recovers_original_secret_after_component_restart
);
test(
  pns_registration_recovers_lost_commit_response_without_rebroadcast.name,
  pns_registration_recovers_lost_commit_response_without_rebroadcast
);
test(
  pns_registration_recovers_lost_reveal_response_without_duplicate_spend.name,
  pns_registration_recovers_lost_reveal_response_without_duplicate_spend
);
test(
  pns_registration_retains_pending_and_expires_only_reconciled_commitment.name,
  pns_registration_retains_pending_and_expires_only_reconciled_commitment
);
test(
  pns_registration_refuses_cross_account_chain_and_undurable_intent.name,
  pns_registration_refuses_cross_account_chain_and_undurable_intent
);
test(
  pns_registration_reconciles_reverted_commit_and_stopped_session.name,
  pns_registration_reconciles_reverted_commit_and_stopped_session
);
