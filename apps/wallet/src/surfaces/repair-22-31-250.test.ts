// @vitest-environment jsdom
import { expect, test } from 'vitest';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { webcrypto } from 'node:crypto';
import { act, createElement } from 'react';
import { parseTransaction, keccak256, bytesToHex, hexToBytes, type Hex } from 'viem';
import {
  JsonRpcClient,
  PaxeerProvider,
  bridge,
  decodeSequencerReceipt,
  encodeKernelReceipt,
  readSequencerReceipt,
  requireAuthenticatedReceipt,
  sendModuleTransactionReceipted,
  waitForSequencerReceipt,
  type ModuleProvider,
  type ModuleTransaction,
  type SequencerReceiptAnswer,
} from '@paxeer/wallet';
import { verifySequencerReceipt } from '@sidiora/layerx-sdk/browser';
import { mount } from '@/account/test/render';
import {
  authenticatedSponsoredPhase,
  receiptEvents,
  receiptStatus,
  surfaceReceipt,
  useModuleSend,
  type ModuleSend,
} from './useSurface';

Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true });

interface ProducerVector {
  id: string;
  rawTransaction: Hex;
  signedReceipt: Hex;
  receiptDigest: Hex;
  receiptJson: Exclude<SequencerReceiptAnswer, { rung: 'submitted' }>;
  pinnedKey: { keyId: Hex; publicKey: Hex; firstHeight: number; lastHeight: number };
}

function repositoryRoot(testUrl: string): string {
  let directory = dirname(fileURLToPath(testUrl));
  while (
    !existsSync(resolve(directory, 'pnpm-workspace.yaml')) ||
    !existsSync(resolve(directory, 'sdk/wallet/src/receipt.ts'))
  ) {
    const parent = dirname(directory);
    if (parent === directory) throw new Error('cannot locate repository workspace from test file');
    directory = parent;
  }
  return directory;
}

const corpus: {
  provenance: { generator: string; encoder: string; sourceGenerator: string };
  cases: ProducerVector[];
} = JSON.parse(
  readFileSync(
    resolve(
      repositoryRoot(import.meta.url),
      'sdk/platform/conformance/fixtures/kernel-receipt-v1.json'
    ),
    'utf8'
  )
);

function vector(id: string): ProducerVector {
  const result = corpus.cases.find((entry) => entry.id === id);
  if (!result) throw new Error(`missing real producer vector ${id}`);
  return result;
}

const bridgeVector = vector('V1-layerxbridge-bridge-out');
const approvalVector = vector('V1-erc20-approve');
const revertedVector = vector('V2-layerxledger-revert');

function call(entry: ProducerVector): ModuleTransaction {
  const transaction = parseTransaction(entry.rawTransaction);
  if (!transaction.to || !transaction.data) throw new Error('producer vector carries no call');
  return { to: transaction.to, data: transaction.data, value: transaction.value ?? 0n };
}

// Replay the committed kernel encoder corpus over real HTTP and the SDK codec.
// Only negative cases alter the recorded producer messages or inject a transport failure.
class RecordedReceiptEndpoint {
  private server: Server;
  private url = '';
  selected = bridgeVector;
  answer: unknown = structuredClone(bridgeVector.receiptJson);
  pins: unknown = { keys: [bridgeVector.pinnedKey] };
  raw: unknown = bridgeVector.rawTransaction;
  fallback = false;
  requests: { method: string; params: unknown[] }[] = [];
  sends: unknown[] = [];
  sequence: ProducerVector[] = [];

  constructor() {
    this.server = createServer(async (request, response) => {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const envelope = JSON.parse(Buffer.concat(chunks).toString()) as {
        id: number;
        method: string;
        params: unknown[];
      };
      this.requests.push(envelope);
      if (
        this.sequence.length &&
        (envelope.method === 'eth_sendTransaction' ||
          (envelope.method === 'paxeer_sendTransactionReceipted' && !this.fallback))
      ) {
        this.select(this.sequence.shift()!);
      }
      let result: unknown;
      let error: { code: number; message: string } | undefined;
      switch (envelope.method) {
        case 'paxeer_sendTransactionReceipted':
          if (this.fallback) error = { code: 4200, message: 'unsupported method' };
          else {
            this.sends.push(envelope.params[0]);
            result = this.answer;
          }
          break;
        case 'eth_sendTransaction':
          this.sends.push(envelope.params[0]);
          result = this.selected.receiptJson.txHash;
          break;
        case 'pax_sequencerReceipt':
          result = this.answer;
          break;
        case 'px_getReceiptKeys':
          result = this.pins;
          break;
        case 'eth_getRawTransactionByHash':
          result = this.raw;
          break;
        default:
          error = { code: -32601, message: `unrecorded method ${envelope.method}` };
      }
      response.writeHead(200, { 'content-type': 'application/json' });
      const payload = JSON.stringify({
        jsonrpc: '2.0',
        id: envelope.id,
        ...(error ? { error } : { result }),
      });
      // Integer tokens are losslessly preserved by JsonRpcClient. Use exponent notation
      // for unsafe numeric pins so this negative case reaches the numeric pin validator.
      response.end(
        envelope.method === 'px_getReceiptKeys'
          ? payload.replace(/("(?:firstHeight|lastHeight)":)([0-9]+)/g, (token, field, height) =>
              Number.isSafeInteger(Number(height))
                ? token
                : `${field}${Number(height).toExponential()}`
            )
          : payload
      );
    });
  }

  select(entry: ProducerVector) {
    this.selected = entry;
    this.answer = structuredClone(entry.receiptJson);
    this.pins = { keys: [entry.pinnedKey] };
    this.raw = entry.rawTransaction;
  }

  async open() {
    await new Promise<void>((resolve) => this.server.listen(0, '127.0.0.1', resolve));
    this.url = `http://127.0.0.1:${(this.server.address() as AddressInfo).port}`;
  }

  provider(): ModuleProvider {
    const client = new JsonRpcClient({ url: this.url });
    return { request: ({ method, params }) => client.call(method, params ?? []) };
  }

  embeddedProvider(): PaxeerProvider {
    return new PaxeerProvider({ gatewayUrl: this.url, rpcUrl: this.url, token: () => null });
  }

  async close() {
    this.server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      this.server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

async function receipt_authentication_preserves_real_producer_evidence_and_events() {
  const root = repositoryRoot(import.meta.url);
  expect(repositoryRoot(pathToFileURL(resolve(root, 'apps/wallet/package.json')).href)).toBe(root);
  expect(root).toBe(resolve(dirname(fileURLToPath(import.meta.url)), '../../../..'));
  expect(corpus.provenance.generator).toBe(
    'sdk/platform/conformance/fixtures/generate_kernel_receipt_fixture.mjs'
  );
  expect(corpus.provenance.encoder).toBe('sdk/wallet/src/receipt.ts#encodeKernelReceipt');
  expect(corpus.provenance.sourceGenerator).toBe('rpc/sequencer_receipt_vectors_test.go');
  expect(corpus.cases.filter((entry) => /^V[12]-/.test(entry.id))).toHaveLength(6);
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const provider = endpoint.embeddedProvider();
    for (const entry of corpus.cases.filter((entry) => /^V[12]-/.test(entry.id))) {
      endpoint.select(entry);
      const receipt = await waitForSequencerReceipt(
        (method, params) => provider.request({ method, params }),
        entry.receiptJson.txHash,
        { rung: 'receipted', attempts: 1, intervalMs: 0 }
      );
      requireAuthenticatedReceipt(receipt);
      const verified = await verifySequencerReceipt(
        hexToBytes(entry.signedReceipt),
        hexToBytes(entry.rawTransaction),
        [
          {
            keyId: hexToBytes(entry.pinnedKey.keyId),
            publicKey: hexToBytes(entry.pinnedKey.publicKey),
            first_height: BigInt(entry.pinnedKey.firstHeight),
            last_height: BigInt(entry.pinnedKey.lastHeight),
          },
        ]
      );
      expect(bytesToHex(encodeKernelReceipt(verified.receipt))).toBe(entry.signedReceipt);
      expect(receipt.authentication).toEqual({
        level: 'sequencer-signed',
        keyId: entry.pinnedKey.keyId,
        signingKey: entry.pinnedKey.publicKey,
        height: BigInt(entry.receiptJson.height!),
        receiptDigest: entry.receiptDigest,
        rawTransaction: entry.rawTransaction,
      });
      expect(receipt.signingKey).toBe(entry.receiptJson.signingKey);
      expect(receipt.receipt).toBe(entry.signedReceipt);
      expect(receipt.txHash).toBe(keccak256(entry.rawTransaction));
      expect(receiptStatus(receipt)).toBe(
        entry.receiptJson.status === '0x1' ? 'confirmed' : 'reverted'
      );
      expect(Object.isFrozen(receipt)).toBe(true);
      expect(Object.isFrozen(receipt.logs)).toBe(true);
      if (entry.id === bridgeVector.id) {
        const result = surfaceReceipt(receipt, bridge(provider));
        expect(result.status).toBe('confirmed');
        expect(result.events).toEqual([
          {
            event: 'BridgeOut',
            precompile: bridgeVector.receiptJson.to,
            topic0: bridgeVector.receiptJson.logs![0]!.topics[0],
            fields: {
              chain: 1n,
              asset: approvalVector.receiptJson.to,
              amount: 5_000_000_000_000_000_000n,
              recipient: '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc',
              nonce: 7n,
            },
          },
        ]);
        expect(() => requireAuthenticatedReceipt({ ...receipt })).toThrow(
          'authentication required'
        );
        expect(() => Object.assign(receipt.logs![0]!, { data: '0x' })).toThrow();
      }
    }
    expect(endpoint.requests.some(({ method }) => method === 'px_getReceiptKeys')).toBe(true);
    expect(endpoint.requests.some(({ method }) => method === 'eth_getRawTransactionByHash')).toBe(
      true
    );
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_preserves_real_producer_evidence_and_events.name,
  receipt_authentication_preserves_real_producer_evidence_and_events
);

async function receipt_authentication_rejects_real_producer_negative_vectors() {
  expect(corpus.cases.filter((entry) => entry.id.startsWith('V3-'))).toHaveLength(6);
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const provider = endpoint.provider();
    for (const entry of corpus.cases.filter((entry) => entry.id.startsWith('V3-'))) {
      endpoint.select(entry);
      await expect(
        waitForSequencerReceipt(
          (method, params) => provider.request({ method, params }),
          entry.receiptJson.txHash,
          { rung: 'receipted', attempts: 1, intervalMs: 0 }
        ),
        entry.id
      ).rejects.toThrow();
    }
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_rejects_real_producer_negative_vectors.name,
  receipt_authentication_rejects_real_producer_negative_vectors
);

async function receipt_authentication_rejects_unsigned_fields_signatures_keys_and_logs() {
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const provider = endpoint.provider();
    const mutations: [string, (answer: ProducerVector['receiptJson']) => void][] = [
      [
        'missing canonical bytes',
        (answer) => {
          Reflect.deleteProperty(answer, 'receipt');
        },
      ],
      [
        'arbitrary canonical bytes',
        (answer) => {
          answer.receipt = '0x01';
        },
      ],
      [
        'missing signing key',
        (answer) => {
          Reflect.deleteProperty(answer, 'signingKey');
        },
      ],
      [
        'self asserted signing key',
        (answer) => {
          answer.signingKey = `0x${'42'.repeat(32)}`;
        },
      ],
      [
        'signature corruption',
        (answer) => {
          answer.receipt = `0x${answer.receipt!.slice(2, -2)}ff`;
        },
      ],
      [
        'wrong key id',
        (answer) => {
          answer.keyId = `0x${'42'.repeat(32)}`;
        },
      ],
      [
        'height',
        (answer) => {
          answer.height = '0x1';
        },
      ],
      [
        'sequence',
        (answer) => {
          answer.seq = '0xff';
        },
      ],
      [
        'nonce',
        (answer) => {
          answer.nonce = '0xff';
        },
      ],
      [
        'index',
        (answer) => {
          answer.index = '0xff';
        },
      ],
      [
        'timestamp',
        (answer) => {
          answer.timestamp = '0x1';
        },
      ],
      [
        'status',
        (answer) => {
          answer.status = '0x0';
        },
      ],
      [
        'sender',
        (answer) => {
          answer.from = `0x${'42'.repeat(20)}`;
        },
      ],
      [
        'recipient',
        (answer) => {
          answer.to = `0x${'42'.repeat(20)}`;
        },
      ],
      [
        'gas used',
        (answer) => {
          answer.gasUsed = '0x1';
        },
      ],
      [
        'cumulative gas',
        (answer) => {
          answer.cumulativeGasUsed = '0x1';
        },
      ],
      [
        'gas price',
        (answer) => {
          answer.effectiveGasPrice = '0x1';
        },
      ],
      [
        'fee amount',
        (answer) => {
          answer.feeAmount = '0x1';
        },
      ],
      [
        'fee denom',
        (answer) => {
          answer.feeDenom = 'usid';
        },
      ],
      [
        'payment denom',
        (answer) => {
          answer.paymentDenom = 'usid';
        },
      ],
      [
        'sponsor',
        (answer) => {
          answer.sponsor = `0x${'42'.repeat(20)}`;
        },
      ],
      [
        'contract address',
        (answer) => {
          answer.contractAddress = `0x${'42'.repeat(20)}`;
        },
      ],
      [
        'return data',
        (answer) => {
          answer.returnData = '0x00';
        },
      ],
      [
        'vm error',
        (answer) => {
          answer.vmError = 'invented';
        },
      ],
      [
        'missing logs',
        (answer) => {
          Reflect.deleteProperty(answer, 'logs');
        },
      ],
      [
        'log count',
        (answer) => {
          answer.logs = [];
        },
      ],
      [
        'log address',
        (answer) => {
          answer.logs![0]!.address = `0x${'42'.repeat(20)}`;
        },
      ],
      [
        'log topic',
        (answer) => {
          answer.logs![0]!.topics[0] = `0x${'42'.repeat(32)}`;
        },
      ],
      [
        'log amount',
        (answer) => {
          answer.logs![0]!.data = `0x${'00'.repeat(64)}`;
        },
      ],
      [
        'log index',
        (answer) => {
          answer.logs![0]!.logIndex = '0x1';
        },
      ],
      [
        'uncommitted block',
        (answer) => {
          answer.blockNumber = '0x3e8';
        },
      ],
    ];
    for (const [label, mutate] of mutations) {
      endpoint.select(bridgeVector);
      const answer = structuredClone(bridgeVector.receiptJson);
      mutate(answer);
      endpoint.answer = answer;
      await expect(
        sendModuleTransactionReceipted(
          provider,
          bridgeVector.receiptJson.from!,
          call(bridgeVector)
        ),
        label
      ).rejects.toThrow();
    }
    for (const pins of [
      { keys: [] },
      { keys: [{ ...bridgeVector.pinnedKey, keyId: `0x${'42'.repeat(32)}` }] },
      { keys: [{ ...bridgeVector.pinnedKey, publicKey: `0x${'42'.repeat(32)}` }] },
      { keys: [{ ...bridgeVector.pinnedKey, firstHeight: 1002 }] },
      { keys: [{ ...bridgeVector.pinnedKey, lastHeight: 1000 }] },
      { keys: [{ ...bridgeVector.pinnedKey, lastHeight: Number.MAX_SAFE_INTEGER + 1 }] },
      { keys: [bridgeVector.pinnedKey, bridgeVector.pinnedKey] },
    ]) {
      endpoint.select(bridgeVector);
      endpoint.pins = pins;
      await expect(
        sendModuleTransactionReceipted(provider, bridgeVector.receiptJson.from!, call(bridgeVector))
      ).rejects.toThrow();
    }
    endpoint.select(bridgeVector);
    endpoint.raw = approvalVector.rawTransaction;
    await expect(
      sendModuleTransactionReceipted(provider, bridgeVector.receiptJson.from!, call(bridgeVector))
    ).rejects.toThrow();
    endpoint.raw = null;
    await expect(
      sendModuleTransactionReceipted(provider, bridgeVector.receiptJson.from!, call(bridgeVector))
    ).rejects.toThrow();
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_rejects_unsigned_fields_signatures_keys_and_logs.name,
  receipt_authentication_rejects_unsigned_fields_signatures_keys_and_logs
);

async function receipt_authentication_binds_direct_and_fallback_sends_to_the_requested_call() {
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const provider = endpoint.provider();
    const from = bridgeVector.receiptJson.from!;
    for (const fallback of [false, true]) {
      endpoint.fallback = fallback;
      const receipt = await sendModuleTransactionReceipted(provider, from, call(bridgeVector));
      expect(surfaceReceipt(receipt, bridge(provider)).events[0]?.event).toBe('BridgeOut');
      for (const tx of [
        { ...call(bridgeVector), to: approvalVector.receiptJson.to! },
        { ...call(bridgeVector), data: `${call(bridgeVector).data}00` },
        { ...call(bridgeVector), value: 1n },
      ]) {
        await expect(sendModuleTransactionReceipted(provider, from, tx)).rejects.toThrow(
          'requested transaction'
        );
      }
      await expect(
        sendModuleTransactionReceipted(provider, `0x${'42'.repeat(20)}`, call(bridgeVector))
      ).rejects.toThrow('requested transaction');
      endpoint.answer = {
        rung: 'submitted',
        txHash: bridgeVector.receiptJson.txHash,
        nonce: bridgeVector.receiptJson.nonce,
      };
      if (!fallback) {
        await expect(
          sendModuleTransactionReceipted(provider, from, call(bridgeVector))
        ).rejects.toThrow('receipt rung');
      }
      endpoint.select(bridgeVector);
    }
    endpoint.answer = { ...bridgeVector.receiptJson, txHash: approvalVector.receiptJson.txHash };
    await expect(
      waitForSequencerReceipt(
        (method, params) => provider.request({ method, params }),
        bridgeVector.receiptJson.txHash,
        { rung: 'receipted', attempts: 1 }
      )
    ).rejects.toThrow('asked hash');
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_binds_direct_and_fallback_sends_to_the_requested_call.name,
  receipt_authentication_binds_direct_and_fallback_sends_to_the_requested_call
);

async function receipt_authentication_is_required_at_each_surface_consumer() {
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const decoded = decodeSequencerReceipt(bridgeVector.receiptJson, undefined, [
      {
        keyId: bridgeVector.pinnedKey.keyId,
        publicKey: bridgeVector.pinnedKey.publicKey,
        firstHeight: BigInt(bridgeVector.pinnedKey.firstHeight),
        lastHeight: BigInt(bridgeVector.pinnedKey.lastHeight),
      },
    ]);
    const surface = bridge(endpoint.provider());
    expect(() => receiptStatus(decoded)).toThrow('authentication required');
    expect(() => receiptEvents(decoded, surface)).toThrow('authentication required');
    expect(() => surfaceReceipt(decoded, surface)).toThrow('authentication required');
    expect(() =>
      receiptStatus({ ...decoded, authentication: { level: 'sequencer-signed' } })
    ).toThrow('authentication required');
    expect(receiptStatus(null)).toBe('pending');
    expect(endpoint.requests).toHaveLength(0);
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_is_required_at_each_surface_consumer.name,
  receipt_authentication_is_required_at_each_surface_consumer
);

async function receipt_authentication_stops_unverified_approvals_and_hook_success() {
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  const provider = endpoint.provider();
  let state: ModuleSend | undefined;
  let approval: ModuleTransaction | undefined;
  function Harness() {
    const result = useModuleSend(
      bridge(provider),
      provider,
      bridgeVector.receiptJson.from!,
      undefined,
      async () => approval
    );
    state = result;
    return createElement(
      'output',
      { 'data-role': 'result' },
      JSON.stringify(result.sent, (_, value) =>
        typeof value === 'bigint' ? value.toString() : value
      )
    );
  }
  const mounted = await mount(createElement(Harness));
  try {
    approval = call(approvalVector);
    endpoint.select(approvalVector);
    const unsigned = structuredClone(approvalVector.receiptJson);
    Reflect.deleteProperty(unsigned, 'receipt');
    endpoint.answer = unsigned;
    await act(async () => {
      await state!.send(call(bridgeVector));
    });
    expect(state!.sent).toBeNull();
    expect(state!.error).toContain('receipt');
    expect(endpoint.sends).toHaveLength(1);
    expect(endpoint.sends[0]).toMatchObject({ to: approvalVector.receiptJson.to });
    expect(state!.approving).toBe(false);
    expect(state!.sending).toBe(false);

    approval = undefined;
    endpoint.select(bridgeVector);
    endpoint.answer = { ...bridgeVector.receiptJson, logs: [] };
    await act(async () => {
      await state!.send(call(bridgeVector));
    });
    expect(state!.sent).toBeNull();
    expect(state!.error).toContain('log count');
    expect(mounted.text('[data-role="result"]')).toBe('null');

    endpoint.select(bridgeVector);
    await act(async () => {
      await state!.send(call(bridgeVector));
    });
    expect(state!.error).toBeNull();
    expect(state!.sent?.status).toBe('confirmed');
    expect(state!.sent?.events[0]?.event).toBe('BridgeOut');
    expect(mounted.text('[data-role="result"]')).toContain('BridgeOut');

    approval = call(approvalVector);
    endpoint.sequence = [approvalVector, bridgeVector];
    const sendsBefore = endpoint.sends.length;
    await act(async () => {
      await state!.send(call(bridgeVector));
    });
    expect(endpoint.sends).toHaveLength(sendsBefore + 2);
    expect(endpoint.sends[sendsBefore]).toMatchObject({ to: approvalVector.receiptJson.to });
    expect(endpoint.sends[sendsBefore + 1]).toMatchObject({ to: bridgeVector.receiptJson.to });
    expect(state!.error).toBeNull();
    expect(state!.sent?.status).toBe('confirmed');
    expect(state!.sent?.events[0]?.event).toBe('BridgeOut');
  } finally {
    await mounted.unmount();
    await endpoint.close();
  }
}

test(
  receipt_authentication_stops_unverified_approvals_and_hook_success.name,
  receipt_authentication_stops_unverified_approvals_and_hook_success
);

async function receipt_authentication_gates_sponsored_settlement_and_recovery() {
  const endpoint = new RecordedReceiptEndpoint();
  await endpoint.open();
  try {
    const provider = endpoint.embeddedProvider();
    const report = { status: 'confirmed' as const, tx_hash: bridgeVector.receiptJson.txHash };
    endpoint.answer = { ...bridgeVector.receiptJson, receipt: '0x' };
    await expect(authenticatedSponsoredPhase(provider, report)).rejects.toThrow();
    endpoint.select(bridgeVector);
    expect(await authenticatedSponsoredPhase(provider, report)).toBe('confirmed');
    await expect(
      authenticatedSponsoredPhase(provider, { ...report, status: 'reverted' })
    ).rejects.toThrow('differs from the authenticated receipt');
    await expect(
      authenticatedSponsoredPhase(provider, { status: 'confirmed', tx_hash: null })
    ).rejects.toThrow('no transaction');
    endpoint.select(revertedVector);
    expect(
      await authenticatedSponsoredPhase(provider, {
        status: 'reverted',
        tx_hash: revertedVector.receiptJson.txHash,
      })
    ).toBe('reverted');
    endpoint.answer = { ...revertedVector.receiptJson, status: '0x1' };
    await expect(
      authenticatedSponsoredPhase(provider, {
        status: 'confirmed',
        tx_hash: revertedVector.receiptJson.txHash,
      })
    ).rejects.toThrow('status');
    const reads = endpoint.requests.length;
    expect(await authenticatedSponsoredPhase(provider, { status: 'pending', tx_hash: null })).toBe(
      'submitted'
    );
    expect(
      await authenticatedSponsoredPhase(provider, { status: 'cancelled', tx_hash: null })
    ).toBe('cancelled');
    expect(endpoint.requests).toHaveLength(reads);

    endpoint.select(bridgeVector);
    endpoint.answer = {
      rung: 'submitted',
      txHash: bridgeVector.receiptJson.txHash,
      nonce: bridgeVector.receiptJson.nonce,
    };
    const pending = await readSequencerReceipt(
      (method, params) => provider.request({ method, params }),
      bridgeVector.receiptJson.txHash
    );
    expect(pending?.rung).toBe('submitted');
    expect(() => requireAuthenticatedReceipt(pending)).toThrow();
  } finally {
    await endpoint.close();
  }
}

test(
  receipt_authentication_gates_sponsored_settlement_and_recovery.name,
  receipt_authentication_gates_sponsored_settlement_and_recovery
);
