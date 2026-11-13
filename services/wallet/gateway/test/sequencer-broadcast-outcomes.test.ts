import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { keccak256, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import {
  BroadcastOutcomeUnknownError,
  outcomeUnknownBody,
  RpcPool,
  RpcUnavailableError,
  TransactionRejectedError,
} from '../src/rpc/pool.js';
import { TestChain } from './support/chain.js';

const CHAIN_ID = 125;
const RECIPIENT = '0x000000000000000000000000000000000000c0de';
const account = privateKeyToAccount(`0x${'12'.repeat(32)}`);
type Failure = 'disconnect' | 'http' | 'invalid-json' | 'truncated-body' | 'timeout' | 'body-timeout' | 'none';

async function signedTransaction(nonce: number): Promise<Hex> {
  return account.signTransaction({
    type: 'eip1559', chainId: CHAIN_ID, nonce, to: RECIPIENT, value: 17n,
    gas: 21_000n, maxFeePerGas: 2_000_000_000n, maxPriorityFeePerGas: 1_000_000_000n,
  });
}

async function withTransport(
  failure: Failure,
  run: (rpc: RpcPool, chain: TestChain, requests: Hex[], sequencerUrl: string) => Promise<void>,
): Promise<void> {
  const chain = new TestChain(CHAIN_ID);
  const requests: Hex[] = [];
  const transportErrors: unknown[] = [];
  let proxy: Server | undefined;
  let rpc: RpcPool | undefined;
  try {
    await chain.start();
    proxy = createServer(async (req, res) => {
      try {
        const chunks: Buffer[] = [];
        for await (const chunk of req) chunks.push(Buffer.from(chunk));
        const raw = Buffer.concat(chunks).toString('utf8');
        const request = JSON.parse(raw) as { method: string; params: Hex[] };
        if (request.method === 'eth_sendRawTransaction') requests.push(request.params[0]!);
        const upstream = await fetch(chain.url, {
          method: 'POST', headers: { 'content-type': 'application/json' }, body: raw,
        });
        const answer = await upstream.text();
        // Lose the answer only after the receiving node has applied the signed transaction.
        if (request.method === 'eth_sendRawTransaction' && chain.sent.length > 0) {
          switch (failure) {
            case 'disconnect':
              req.socket.destroy();
              return;
            case 'http':
              res.writeHead(503);
              res.end('upstream connection lost');
              return;
            case 'invalid-json':
              res.end('<html>upstream connection lost</html>');
              return;
            case 'truncated-body':
              res.writeHead(200, { 'content-type': 'application/json', 'content-length': Buffer.byteLength(answer) });
              res.write(answer.slice(0, 10));
              setImmediate(() => res.destroy());
              return;
            case 'timeout':
              return;
            case 'body-timeout':
              res.writeHead(200, { 'content-type': 'application/json' });
              res.write(answer.slice(0, 10));
              return;
            case 'none':
              break;
          }
        }
        res.writeHead(upstream.status, { 'content-type': 'application/json' });
        res.end(answer);
      } catch (error) {
        transportErrors.push(error);
        res.destroy();
      }
    });
    await new Promise<void>((resolve, reject) => {
      proxy!.once('error', reject);
      proxy!.listen(0, '127.0.0.1', resolve);
    });
    const sequencerUrl = `http://127.0.0.1:${(proxy.address() as AddressInfo).port}`;
    rpc = new RpcPool({
      urls: [chain.url], sequencerUrl, chainId: CHAIN_ID, lagThresholdBlocks: 20,
      timeoutMs: 1_000, healthIntervalMs: 60_000,
    });
    await run(rpc, chain, requests, sequencerUrl);
    expect(transportErrors).toEqual([]);
  } finally {
    rpc?.stop();
    if (proxy) {
      proxy.closeAllConnections();
      await new Promise<void>((resolve, reject) => proxy!.close((error) => error ? reject(error) : resolve()));
    }
    await chain.stop();
  }
}

async function assertUncertainBroadcast(failure: Failure, nonce: number): Promise<void> {
  await withTransport(failure, async (rpc, chain, requests, sequencerUrl) => {
    chain.balances.set(account.address.toLowerCase(), 10n ** 18n);
    chain.nonces.set(account.address.toLowerCase(), nonce);
    const signed = await signedTransaction(nonce);
    const hash = keccak256(signed);
    const error: unknown = await rpc.sendRawTransactionVia(signed).catch((caught: unknown) => caught);
    expect(chain.sent).toHaveLength(1);
    expect(chain.sent[0]).toMatchObject({ hash, nonce, from: account.address.toLowerCase(), to: RECIPIENT, value: 17n });
    expect(chain.balances.get(RECIPIENT)).toBe(17n);
    expect(error).toBeInstanceOf(BroadcastOutcomeUnknownError);
    // broadcastReserved releases a reservation only for this definitive-refusal type.
    expect(error).not.toBeInstanceOf(TransactionRejectedError);
    if (!(error instanceof BroadcastOutcomeUnknownError)) throw error;
    expect(error.hash).toBe(hash);
    expect(error.nonce).toBe(nonce);
    expect(error.member).toBe(1);
    expect(error.members).toBe(2);
    expect(error.message).not.toContain(sequencerUrl);

    // This is the response serializer consumed by the human send route.
    const response = outcomeUnknownBody(error);
    expect(response).toEqual({
      error: 'outcome_unknown', tx_hash: hash, nonce, retry: 'poll_receipt',
      poll: 'eth_getTransactionReceipt', must_not_resubmit: true, member: 1, members: 2,
    });
    const receipt = await rpc.request(response.poll as string, [response.tx_hash]);
    expect(receipt).toEqual(chain.receipts.get(hash));
    expect(receipt).toMatchObject({ transactionHash: hash, status: '0x1' });
    expect(requests).toEqual([signed]);
    expect(chain.sent).toHaveLength(1);
  });
}

async function sendTransportUncertaintyAfterNodeAcceptance(): Promise<void> {
  await assertUncertainBroadcast('disconnect', 7);
}

async function sendHttpFailureRetainsCanonicalHashAndNonce(): Promise<void> {
  await assertUncertainBroadcast('http', 8);
}

async function sendLostJsonBodyRequiresReceiptPolling(): Promise<void> {
  await assertUncertainBroadcast('invalid-json', 9);
  await assertUncertainBroadcast('truncated-body', 10);
}

async function sendTimeoutPreservesUncertainOutcome(): Promise<void> {
  await assertUncertainBroadcast('timeout', 0);
}

async function sendBodyTimeoutPreservesUncertainOutcome(): Promise<void> {
  await assertUncertainBroadcast('body-timeout', 11);
}

async function definitiveNodeRefusalRemainsRejected(): Promise<void> {
  await withTransport('none', async (rpc, chain, requests) => {
    const signed = await signedTransaction(0);
    const error: unknown = await rpc.sendRawTransactionVia(signed).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(TransactionRejectedError);
    expect(error).not.toBeInstanceOf(BroadcastOutcomeUnknownError);
    expect(error).toMatchObject({ code: -32000, message: 'insufficient funds for gas * price + value' });
    expect(chain.sent).toHaveLength(0);
    expect(requests).toEqual([signed]);
  });
}

async function missingSequencerRemainsUnavailableBeforeSend(): Promise<void> {
  await withTransport('none', async (_rpc, chain, requests) => {
    const rpc = new RpcPool({
      urls: [chain.url], chainId: CHAIN_ID, lagThresholdBlocks: 20,
      timeoutMs: 1_000, healthIntervalMs: 60_000,
    });
    await expect(rpc.sendRawTransactionVia(await signedTransaction(0))).rejects.toBeInstanceOf(RpcUnavailableError);
    expect(requests).toEqual([]);
    expect(chain.sent).toHaveLength(0);
  });
}

test(sendTransportUncertaintyAfterNodeAcceptance.name, sendTransportUncertaintyAfterNodeAcceptance);
test(sendHttpFailureRetainsCanonicalHashAndNonce.name, sendHttpFailureRetainsCanonicalHashAndNonce);
test(sendLostJsonBodyRequiresReceiptPolling.name, sendLostJsonBodyRequiresReceiptPolling);
test(sendTimeoutPreservesUncertainOutcome.name, sendTimeoutPreservesUncertainOutcome);
test(sendBodyTimeoutPreservesUncertainOutcome.name, sendBodyTimeoutPreservesUncertainOutcome);
test(definitiveNodeRefusalRemainsRejected.name, definitiveNodeRefusalRemainsRejected);
test(missingSequencerRemainsUnavailableBeforeSend.name, missingSequencerRemainsUnavailableBeforeSend);
