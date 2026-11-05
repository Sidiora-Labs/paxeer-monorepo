import { createServer, type Server } from 'node:http';
import { createServer as createTcpServer, type AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { decodeFunctionData, keccak256, parseTransaction, type Hex } from 'viem';
import { RpcPool } from '../src/rpc/pool.js';
import {
  ADDR_PRECOMPILE,
  broadcastOnce,
  readReceipt,
  sponsoredBindAbi,
  waitForReceipt,
} from '../src/provision/bind.js';
import { reservePorts } from './support/ports.js';
import {
  assertSponsoredSubmitEvidence,
  assertWalletSendEvidence,
} from './support/wallet-send-evidence.js';
import {
  authenticateNodeReceipt,
  authenticateSponsoredBindReceipt,
  nodeEvidenceForTransaction,
  nodeReceiptCorpus,
  sponsoredBindRecording,
  type NodeReceiptEvidence,
} from './support/node-receipts.js';

class RecordedNode {
  private server: Server | undefined;
  readonly accepted = new Set<Hex>();
  readonly requests: { method: string; params: unknown[] }[] = [];
  readonly errors: unknown[] = [];
  loseBroadcastResponse = false;
  url = '';

  constructor(readonly evidence: NodeReceiptEvidence) {}

  async start(): Promise<void> {
    this.server = createServer(async (request, response) => {
      try {
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const envelope = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
          id: number;
          method: string;
          params: unknown[];
        };
        this.requests.push(envelope);
        const hash = this.evidence.receiptJson.txHash;
        let result: unknown;
        switch (envelope.method) {
          case 'eth_blockNumber':
            result = this.evidence.receiptJson.height;
            break;
          case 'eth_sendRawTransaction':
            expect(envelope.params).toEqual([this.evidence.rawTransaction]);
            this.accepted.add(hash);
            if (this.loseBroadcastResponse) {
              this.loseBroadcastResponse = false;
              response.destroy();
              return;
            }
            result = hash;
            break;
          case 'eth_getTransactionByHash':
            expect(envelope.params).toEqual([hash]);
            result = this.accepted.has(hash)
              ? { hash, blockNumber: this.evidence.receiptJson.height }
              : null;
            break;
          case 'pax_sequencerReceipt':
            expect(envelope.params).toEqual([hash]);
            result = this.accepted.has(hash) ? this.evidence.receiptJson : null;
            break;
          default:
            throw new Error(`method is absent from the node transcript: ${envelope.method}`);
        }
        response.setHeader('content-type', 'application/json');
        response.end(JSON.stringify({ jsonrpc: '2.0', id: envelope.id, result }));
      } catch (error) {
        this.errors.push(error);
        response.writeHead(500);
        response.end();
      }
    });
    await new Promise<void>((resolve, reject) => {
      this.server!.once('error', reject);
      this.server!.listen(0, '127.0.0.1', resolve);
    });
    this.url = `http://127.0.0.1:${(this.server.address() as AddressInfo).port}`;
  }

  pool(): RpcPool {
    return new RpcPool({
      urls: [this.url],
      sequencerUrl: this.url,
      chainId: 125,
      timeoutMs: 2_000,
      healthIntervalMs: 60_000,
      lagThresholdBlocks: 2,
    });
  }

  async stop(): Promise<void> {
    if (!this.server) return;
    await new Promise<void>((resolve, reject) => {
      this.server!.close((error) => (error ? reject(error) : resolve()));
      this.server!.closeAllConnections();
    });
    this.server = undefined;
  }
}

async function stationPortReservationRetainsAllTenListeners(): Promise<void> {
  const reservations = await reservePorts(10);
  const released = new Set<number>();
  const consumers: ReturnType<typeof createTcpServer>[] = [];
  try {
    expect(new Set(reservations.map(({ port }) => port)).size).toBe(10);
    for (const reservation of reservations) {
      const contender = createTcpServer();
      try {
        await expect(
          new Promise<void>((resolve, reject) => {
            contender.once('error', reject);
            contender.listen(reservation.port, '127.0.0.1', resolve);
          })
        ).rejects.toMatchObject({ code: 'EADDRINUSE' });
      } finally {
        if (contender.listening)
          await new Promise<void>((resolve) => contender.close(() => resolve()));
      }
    }
    for (let member = 0; member < 5; member += 1) {
      const pair = [reservations[member]!, reservations[member + 5]!];
      await Promise.all(
        pair.map(async (reservation) => {
          await reservation.release();
          released.add(reservation.port);
        })
      );
      for (const reservation of pair) {
        const consumer = createTcpServer();
        consumers.push(consumer);
        await new Promise<void>((resolve, reject) => {
          consumer.once('error', reject);
          consumer.listen(reservation.port, '127.0.0.1', resolve);
        });
        expect((consumer.address() as AddressInfo).port).toBe(reservation.port);
      }
    }
    expect(consumers).toHaveLength(10);
  } finally {
    await Promise.all(
      consumers
        .filter((server) => server.listening)
        .map(
          (server) =>
            new Promise<void>((resolve, reject) =>
              server.close((error) => (error ? reject(error) : resolve()))
            )
        )
    );
    await Promise.all(
      reservations
        .filter(({ port }) => !released.has(port))
        .map((reservation) => reservation.release())
    );
  }
}

function transferEvidence(): NodeReceiptEvidence {
  const evidence = nodeReceiptCorpus.cases.find((entry) => entry.id === 'V1-pax-transfer');
  if (!evidence) throw new Error('missing node transfer transcript');
  return nodeEvidenceForTransaction(evidence.rawTransaction);
}

async function walletSendReceiptAuthenticatesCanonicalNodeEvidence(): Promise<void> {
  expect(nodeReceiptCorpus.provenance.generator).toBe('rpc/sequencer_receipt_vectors_test.go');
  const evidence = transferEvidence();
  const node = new RecordedNode(evidence);
  await node.start();
  const rpc = node.pool();
  try {
    const hash = await rpc.sendRawTransaction(evidence.rawTransaction);
    expect(hash).toBe(keccak256(evidence.rawTransaction));
    const answer = await rpc.sequencerReceipt(hash);
    expect(answer).toEqual(evidence.receiptJson);
    expect(await authenticateNodeReceipt(answer, evidence)).toEqual(answer);
    for (const change of [
      { status: '0x0' },
      { feeAmount: '0x0' },
      { nonce: '0x1' },
      { paymentDenom: 'SID' },
      { from: `0x${'ff'.repeat(20)}` },
      { receipt: '0x' },
      { receipt: `0x${'00'.repeat(64)}` },
    ]) {
      await expect(
        authenticateNodeReceipt({ ...answer, ...change }, evidence)
      ).rejects.toThrow();
    }
    await expect(
      authenticateNodeReceipt(answer, {
        ...evidence,
        pinnedKey: { ...evidence.pinnedKey, publicKey: `0x${'ff'.repeat(32)}` },
      })
    ).rejects.toThrow();
    expect(node.errors).toEqual([]);
  } finally {
    rpc.stop();
    await node.stop();
  }
  for (const variant of ['receipted', null, 'submitted', 'committed'] as const) {
    await assertWalletSendEvidence(variant);
  }
}

async function provisioningReceiptTransportRecoversWithoutResubmission(): Promise<void> {
  const evidence = nodeEvidenceForTransaction(sponsoredBindRecording.positive.rawTransaction);
  const node = new RecordedNode(evidence);
  await node.start();
  let rpc = node.pool();
  try {
    const hash = evidence.receiptJson.txHash;
    expect(await readReceipt(rpc, hash)).toBeNull();
    node.loseBroadcastResponse = true;
    expect(await broadcastOnce(rpc, evidence.rawTransaction)).toBe(hash);
    expect(node.accepted.has(hash)).toBe(true);
    rpc.stop();
    await node.stop();
    await node.start();
    rpc = node.pool();
    expect(await broadcastOnce(rpc, evidence.rawTransaction)).toBe(hash);
    expect(await waitForReceipt(rpc, hash, 0, 1)).toEqual({
      status: 'success',
      gasUsed: BigInt(evidence.receiptJson.gasUsed),
      blockNumber: BigInt(evidence.receiptJson.height),
    });
    expect(await authenticateNodeReceipt(await rpc.sequencerReceipt(hash), evidence)).toEqual(
      evidence.receiptJson
    );
    expect(
      await authenticateSponsoredBindReceipt(await rpc.sequencerReceipt(hash), evidence)
    ).toEqual(evidence.receiptJson);
    expect(
      node.requests.filter((request) => request.method === 'eth_sendRawTransaction')
    ).toHaveLength(1);
    expect(node.errors).toEqual([]);
  } finally {
    rpc.stop();
    await node.stop();
  }
}

async function sponsoredSubmitReceiptRequiresCanonicalSponsoredExecution(): Promise<void> {
  const evidence = nodeReceiptCorpus.cases.find(
    (entry) =>
      entry.expected.verifier === 'accepts' &&
      entry.rawTransaction.startsWith('0x04') &&
      BigInt(entry.receiptJson.sponsor) !== 0n
  );
  expect(
    evidence,
    'node owner must supply canonical sponsored EIP-7702 execution evidence; an EIP-1559 transfer is not this proof'
  ).toBeDefined();
  if (!evidence) throw new Error('missing sponsored execution transcript');
  const node = new RecordedNode(evidence);
  await node.start();
  const rpc = node.pool();
  try {
    const hash = await rpc.sendRawTransaction(evidence.rawTransaction);
    const answer = await rpc.sequencerReceipt(hash);
    expect(await authenticateNodeReceipt(answer, evidence)).toEqual(evidence.receiptJson);
    expect(await authenticateSponsoredBindReceipt(answer, evidence)).toEqual(
      evidence.receiptJson
    );
    expect(answer).toMatchObject({
      status: '0x1',
      nonce: evidence.receiptJson.nonce,
      sponsor: evidence.receiptJson.sponsor,
    });
    expect(node.errors).toEqual([]);
  } finally {
    rpc.stop();
    await node.stop();
  }
  const refusal = nodeEvidenceForTransaction(sponsoredBindRecording.refusal.rawTransaction);
  const ordinaryNode = new RecordedNode(refusal);
  await ordinaryNode.start();
  const ordinaryRpc = ordinaryNode.pool();
  try {
    const hash = await ordinaryRpc.sendRawTransaction(refusal.rawTransaction);
    const answer = await ordinaryRpc.sequencerReceipt(hash);
    expect(await authenticateNodeReceipt(answer, refusal)).toEqual(refusal.receiptJson);
    await expect(authenticateSponsoredBindReceipt(answer, refusal)).rejects.toThrow(
      'sponsored set-code execution required'
    );
    await expect(authenticateNodeReceipt(answer, evidence)).rejects.toThrow();
    expect(ordinaryNode.errors).toEqual([]);
  } finally {
    ordinaryRpc.stop();
    await ordinaryNode.stop();
  }
  await assertSponsoredSubmitEvidence();
}

async function provisioningRequiresNodeExecutedSponsoredBind(): Promise<void> {
  const candidates = nodeReceiptCorpus.cases.filter(
    (entry) => entry.expected.verifier === 'accepts' && entry.rawTransaction.startsWith('0x04')
  );
  expect(
    candidates.length,
    'node owner must record sponsored binding execution and resulting account state; fixture-side applyBind cannot prove it'
  ).toBeGreaterThan(0);
  for (const evidence of candidates) {
    const transaction = parseTransaction(evidence.rawTransaction);
    if (!transaction.data) throw new Error('sponsored bind transcript has no calldata');
    const execution = decodeFunctionData({ abi: sponsoredBindAbi, data: transaction.data });
    expect(execution.functionName).toBe('executeSponsored');
    const calls = execution.args![0];
    expect(calls).toHaveLength(1);
    expect(calls[0]!.to.toLowerCase()).toBe(ADDR_PRECOMPILE);
    expect(calls[0]!.data.slice(0, 10)).toBe('0xdd9aa628');
    expect(evidence.receiptJson.status).toBe('0x1');
    expect(await authenticateNodeReceipt(evidence.receiptJson, evidence)).toEqual(
      evidence.receiptJson
    );
    expect(await authenticateSponsoredBindReceipt(evidence.receiptJson, evidence)).toEqual(
      evidence.receiptJson
    );
  }
  const refusal = nodeEvidenceForTransaction(sponsoredBindRecording.refusal.rawTransaction);
  await expect(authenticateSponsoredBindReceipt(refusal.receiptJson, refusal)).rejects.toThrow(
    'sponsored set-code execution required'
  );
}

test(
  stationPortReservationRetainsAllTenListeners,
  stationPortReservationRetainsAllTenListeners
);
test(
  walletSendReceiptAuthenticatesCanonicalNodeEvidence,
  walletSendReceiptAuthenticatesCanonicalNodeEvidence,
  30_000
);
test(
  provisioningReceiptTransportRecoversWithoutResubmission,
  provisioningReceiptTransportRecoversWithoutResubmission
);
test(
  sponsoredSubmitReceiptRequiresCanonicalSponsoredExecution,
  sponsoredSubmitReceiptRequiresCanonicalSponsoredExecution
);
test(
  provisioningRequiresNodeExecutedSponsoredBind,
  provisioningRequiresNodeExecutedSponsoredBind
);
