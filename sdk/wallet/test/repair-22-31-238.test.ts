import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, parseTransaction, recoverMessageAddress, recoverTypedDataAddress } from 'viem';
import { EndpointClient, DEFAULT_ENDPOINT_URL } from '../src/endpoint.js';
import { JsonRpcClient } from '../src/rpc.js';
import { PaxeerProvider } from '../src/provider.js';
import {
  decodeSequencerReceipt,
  readSequencerReceipt,
  readReceiptKeyPins,
  waitForSequencerReceipt,
} from '../src/receipt.js';
import { statusLadder, recoverTransferObservation, submittedTransfer } from '../src/ladder.js';
import { sendModuleTransactionReceipted } from '../src/modules/index.js';
import type { Hex, ReceiptKeyPin } from '../src/types.js';

function fixture(path: string): any {
  return JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'));
}
const vectors = fixture('../../platform/conformance/fixtures/sequencer-receipt-v1.json');
const positive = vectors.cases.filter(
  (entry: any) => entry.vector === 'V1' || entry.vector === 'V2'
);
const gateway = Object.fromEntries(
  ['wallet-me', 'wallet-send', 'wallet-sign-message', 'wallet-sign-typed-data'].map((name) => [
    name,
    fixture(`./fixtures/gateway/${name}.json`),
  ])
);
const constructions = fixture('./fixtures/gateway/constructions.json');
const endpointExchanges = fixture('./fixtures/endpoint/available.json');
const accountExchange = endpointExchanges.find(
  (entry: any) => entry.request.method === 'px_resolveAccount'
);
const legacyNetworkExchange = endpointExchanges.find(
  (entry: any) => entry.request.method === 'px_getNetwork'
);
const chainExchange = fixture('./fixtures/gateway/rpc-chain-id.json');
const committedRecording = fixture(
  '../../../daemon/committed-fixture/vectors/vector-v4-committed.json'
);
const committedVector = fixture(
  '../../platform/conformance/fixtures/vector-v4-committed.json'
).positive;
const seen: { path: string; authorization?: string; body: any }[] = [];
let base: string;
let activeReceipt: any = positive[0].receiptJson;
let activePin: any = positive[0].pinnedKey;
let swapSignatures = false;

async function readBody(req: IncomingMessage): Promise<any> {
  let text = '';
  for await (const chunk of req) text += chunk;
  return text ? JSON.parse(text) : undefined;
}
// Replay producer-emitted node and wallet-gateway captures over real HTTP. Only
// adversarial cases below alter captured answers; no replacement signer is used.
const server = createServer(async (req, res) => {
  try {
    const path = req.url!;
    const body = await readBody(req);
    seen.push({ path, authorization: req.headers.authorization, body });
    res.setHeader('content-type', 'application/json');
    const name =
      path === '/v1/wallet/me'
        ? 'wallet-me'
        : path === '/v1/wallet/send'
          ? 'wallet-send'
          : path === '/v1/wallet/sign-message'
            ? swapSignatures
              ? 'wallet-sign-typed-data'
              : 'wallet-sign-message'
            : path === '/v1/wallet/sign-typed-data'
              ? swapSignatures
                ? 'wallet-sign-message'
                : 'wallet-sign-typed-data'
              : null;
    if (name) {
      res.statusCode = gateway[name].status;
      res.end(JSON.stringify(gateway[name].body));
      return;
    }
    const answer = (call: any): any => {
      let result: any;
      if (path === '/node') {
        if (
          call.method === 'pax_sequencerReceipt' ||
          call.method === 'paxeer_sendTransactionReceipted'
        )
          result = activeReceipt;
        else if (call.method === 'px_getReceiptKeys') result = { keys: [activePin] };
        else if (call.method === 'eth_chainId') return { ...chainExchange.body, id: call.id };
        else throw new Error(`unrecorded node method ${call.method}`);
      } else if (path === '/hosted') {
        const exchange = [accountExchange, legacyNetworkExchange].find(
          (entry) => entry.request.method === call.method
        );
        if (!exchange) throw new Error(`unrecorded hosted method ${call.method}`);
        result = exchange.response.result;
      } else throw new Error(`unrecorded path ${path}`);
      return { jsonrpc: '2.0', id: call.id, result };
    };
    res.end(
      JSON.stringify(Array.isArray(body) ? body.map(answer) : answer(body)).replaceAll(
        '\"18446744073709551615\"',
        '18446744073709551615'
      )
    );
  } catch (error) {
    res.statusCode = 500;
    res.end(JSON.stringify({ error: String(error) }));
  }
});
beforeAll(async () => {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});
afterAll(async () => {
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
});
function rpc() {
  const client = new JsonRpcClient({ url: `${base}/node` });
  return (method: string, params: readonly unknown[]) => client.call(method, params);
}
function pins(entry: any): readonly ReceiptKeyPin[] {
  return [
    {
      ...entry.pinnedKey,
      firstHeight: BigInt(entry.pinnedKey.firstHeight),
      lastHeight: BigInt(entry.pinnedKey.lastHeight),
    },
  ];
}

async function receipt_fields_lost_preserves_complete_node_views(): Promise<void> {
  expect(vectors.provenance.generator).toBe('rpc/sequencer_receipt_vectors_test.go');
  for (const entry of positive) {
    activeReceipt = entry.receiptJson;
    activePin = entry.pinnedKey;
    expect(keccak256(entry.rawTransaction)).toBe(activeReceipt.txHash);
    const answer = await readSequencerReceipt(rpc(), activeReceipt.txHash);
    expect(answer).toEqual(activeReceipt);
    expect(answer!.paymentDenom).toBe(activeReceipt.paymentDenom);
    expect(answer!.signingKey).toBe(activePin.publicKey);
    expect(
      decodeSequencerReceipt(JSON.parse(JSON.stringify(answer)), activeReceipt.txHash, pins(entry))
    ).toEqual(activeReceipt);
    expect(Object.isFrozen(answer)).toBe(true);
  }
  activeReceipt = committedRecording.settlement.receipt;
  activePin = committedVector.pinnedKey;
  expect(activeReceipt.rung).toBe('committed');
  expect(await readSequencerReceipt(rpc(), activeReceipt.txHash)).toEqual(activeReceipt);
}

async function receipt_authentication_rejects_tampered_node_outputs(): Promise<void> {
  for (const entry of positive) {
    const original = entry.receiptJson;
    const trust = pins(entry);
    const changes: Record<string, unknown> = {
      txHash: `0x${'ab'.repeat(32)}`,
      nonce: '0x99',
      seq: '0x99',
      height: '0x99',
      index: '0x99',
      timestamp: '0x99',
      status: original.status === '0x1' ? '0x0' : '0x1',
      returnData: '0x00',
      logs: [],
      gasUsed: '0x99',
      cumulativeGasUsed: '0x99',
      effectiveGasPrice: '0x99',
      paymentDenom: 'other',
      feeDenom: 'other',
      feeAmount: '0x99',
      sponsor: `0x${'ab'.repeat(20)}`,
      from: `0x${'ab'.repeat(20)}`,
      to: `0x${'ab'.repeat(20)}`,
      contractAddress: `0x${'ab'.repeat(20)}`,
      vmError: 'tampered',
      keyId: `0x${'ab'.repeat(32)}`,
      signingKey: `0x${'ab'.repeat(32)}`,
      receipt: `${original.receipt.slice(0, -2)}${original.receipt.endsWith('00') ? '01' : '00'}`,
    };
    for (const [field, value] of Object.entries(changes)) {
      if (field === 'logs' && original.logs.length === 0) continue;
      expect(
        () => decodeSequencerReceipt({ ...original, [field]: value }, original.txHash, trust),
        `${entry.id}: ${field}`
      ).toThrow();
    }
    for (const field of Object.keys(original).filter(
      (field) => !['rung', 'txHash', 'nonce'].includes(field)
    )) {
      const missing = { ...original };
      delete missing[field];
      expect(
        () => decodeSequencerReceipt(missing, original.txHash, trust),
        `missing ${field}`
      ).toThrow();
    }
    expect(() => decodeSequencerReceipt(original)).toThrow(/pin/);
    expect(() =>
      decodeSequencerReceipt(
        { ...original, signingKey: committedVector.pinnedKey.publicKey },
        original.txHash,
        [{ ...trust[0]!, publicKey: committedVector.pinnedKey.publicKey }]
      )
    ).toThrow(/signature/);
    expect(() =>
      decodeSequencerReceipt(original, original.txHash, [
        { ...trust[0]!, lastHeight: BigInt(original.height) - 1n },
      ])
    ).toThrow(/range/);
    if (original.logs.length) {
      const changed = structuredClone(original);
      changed.logs[0].data = `${changed.logs[0].data}00`;
      expect(() => decodeSequencerReceipt(changed, original.txHash, trust)).toThrow(/digest/);
      changed.logs[0] = { ...original.logs[0], topics: [`0x${'ff'.repeat(32)}`] };
      expect(() => decodeSequencerReceipt(changed, original.txHash, trust)).toThrow(/topic/);
      for (const [field, value] of [
        ['address', `0x${'ab'.repeat(20)}`],
        ['logIndex', '0x99'],
      ]) {
        const changed = structuredClone(original);
        changed.logs[0][field] = value;
        expect(() => decodeSequencerReceipt(changed, original.txHash, trust)).toThrow();
      }
    }
    activePin = entry.pinnedKey;
    activeReceipt = { ...original, returnData: `${original.returnData}00` };
    await expect(readSequencerReceipt(rpc(), original.txHash)).rejects.toThrow(/digest/);
    await expect(
      waitForSequencerReceipt(rpc(), original.txHash, { rung: 'receipted', attempts: 1 })
    ).rejects.toThrow(/digest/);
  }
  for (const id of [
    'V3-key-not-pinned',
    'V3-height-outside-key-range',
    'V3-activity-id-not-raw-hash',
    'V3-tampered-log',
    'V3-non-canonical-bytes',
  ]) {
    const entry = vectors.cases.find((entry: any) => entry.id === id);
    expect(() =>
      decodeSequencerReceipt(entry.receiptJson, keccak256(entry.rawTransaction), pins(entry))
    ).toThrow();
  }
  const entry = positive[0];
  activePin = entry.pinnedKey;
  activeReceipt = {
    rung: 'receipted',
    txHash: entry.receiptJson.txHash,
    nonce: entry.receiptJson.nonce,
  };
  await expect(
    waitForSequencerReceipt(rpc(), activeReceipt.txHash, {
      rung: 'receipted',
      attempts: 1,
    })
  ).rejects.toThrow();
  const committed = committedRecording.settlement.receipt;
  activeReceipt = committed;
  activePin = committedVector.pinnedKey;
  expect(
    await waitForSequencerReceipt(rpc(), committed.txHash, {
      rung: 'committed',
      attempts: 1,
    })
  ).toEqual(committed);
  expect(() =>
    decodeSequencerReceipt(
      { ...committed, blockNumber: '0x0' },
      committed.txHash,
      pins(committedVector)
    )
  ).toThrow(/height/);
  expect(() =>
    decodeSequencerReceipt(
      { ...entry.receiptJson, rung: 'submitted' },
      entry.receiptJson.txHash,
      pins(entry)
    )
  ).toThrow(/submitted/);
  expect(() =>
    decodeSequencerReceipt(
      { ...entry.receiptJson, blockNumber: '0x999', blockHash: committed.blockHash },
      entry.receiptJson.txHash,
      pins(entry)
    )
  ).toThrow(/receipted/);
}

async function receipt_authentication_protects_module_provider_and_ladder_consumers(): Promise<void> {
  const entry = positive.find((entry: any) => entry.id === 'V1-layerxledger-pay-exact');
  activeReceipt = entry.receiptJson;
  activePin = entry.pinnedKey;
  const client = new JsonRpcClient({ url: `${base}/node` });
  const provider = {
    request: ({ method, params }: { method: string; params?: readonly unknown[] }) =>
      client.call(method, params ?? []),
  };
  const transaction = parseTransaction(entry.rawTransaction);
  expect(transaction.to).toBe(activeReceipt.to);
  const tx = { to: activeReceipt.to, data: transaction.data!, value: 0n };
  expect(await sendModuleTransactionReceipted(provider, activeReceipt.from, tx)).toEqual(
    activeReceipt
  );
  expect(
    seen.filter(({ body }) => body?.method === 'paxeer_sendTransactionReceipted').at(-1)!.body
      .params[0]
  ).toMatchObject({ from: activeReceipt.from, to: transaction.to, data: transaction.data });
  expect(
    statusLadder.highest(statusLadder.fromSequencerReceipt(activeReceipt, pins(entry)))?.rung
  ).toBe('receipted');
  expect(() => statusLadder.fromSequencerReceipt(activeReceipt)).toThrow(/pin/);
  activeReceipt = { ...entry.receiptJson, returnData: '0xab' };
  await expect(
    sendModuleTransactionReceipted(provider, entry.receiptJson.from, tx)
  ).rejects.toThrow(/digest/);
  const embedded = new PaxeerProvider({
    gatewayUrl: base,
    rpcUrl: `${base}/node`,
    token: () => 'session-a',
  });
  await expect(
    embedded.request({
      method: 'pax_sequencerReceipt',
      params: [entry.receiptJson.txHash],
    })
  ).rejects.toThrow(/digest/);
  const identity = { hash: entry.receiptJson.txHash, chainId: 125 };
  const persisted = JSON.parse(
    JSON.stringify({
      ...submittedTransfer(identity),
      lastVerified: {
        blockNumber: 1000,
        blockHash: committedRecording.settlement.receipt.blockHash,
        steps: [{ rung: 'receipted', source: 'receipt', state: 'receipted' }],
        sequencer: activeReceipt,
      },
    })
  );
  expect(recoverTransferObservation(identity, persisted, pins(entry)).lastVerified).toBeUndefined();
  delete persisted.lastVerified.sequencer;
  expect(recoverTransferObservation(identity, persisted, pins(entry)).lastVerified).toBeUndefined();
  persisted.lastVerified.sequencer = entry.receiptJson;
  expect(
    recoverTransferObservation(identity, persisted, pins(entry)).lastVerified?.sequencer
  ).toEqual(entry.receiptJson);
  const failed = positive.find((entry: any) => entry.receiptJson.status === '0x0');
  const failedIdentity = { hash: failed.receiptJson.txHash, chainId: 125 };
  const reopened = JSON.parse(
    JSON.stringify({
      ...submittedTransfer(failedIdentity),
      lastVerified: { ...persisted.lastVerified, sequencer: failed.receiptJson },
    })
  );
  expect(
    recoverTransferObservation(failedIdentity, reopened, pins(failed)).lastVerified
  ).toBeUndefined();
  expect(
    statusLadder.highest(statusLadder.fromSequencerReceipt(failed.receiptJson, pins(failed)))?.rung
  ).toBe('submitted');
}

async function receipt_authentication_keeps_uint64_key_ranges_lossless(): Promise<void> {
  activePin = { ...positive[0].pinnedKey, lastHeight: '18446744073709551615' };
  // JSON-RPC emits uint64 as a JSON number, including values outside JS's safe range.
  const raw = JSON.stringify({ keys: [activePin] }).replace(
    '"18446744073709551615"',
    '18446744073709551615'
  );
  const { parseReceiptKeyResponse } = await import('../src/rpc.js');
  const decoded = parseReceiptKeyResponse(raw) as any;
  expect(decoded.keys[0].lastHeight).toBe('18446744073709551615');
  const result = await readReceiptKeyPins(rpc());
  expect(result[0]!.lastHeight).toBe(18446744073709551615n);
  const embedded = new PaxeerProvider({
    gatewayUrl: base,
    rpcUrl: `${base}/node`,
    token: () => 'session-a',
  });
  expect(await embedded.request({ method: 'px_getReceiptKeys' })).toEqual(decoded);
  activeReceipt = positive[0].receiptJson;
  expect(decodeSequencerReceipt(activeReceipt, activeReceipt.txHash, result)).toEqual(
    activeReceipt
  );
}

function providerConfig(
  token: () => string,
  confirm?: (request: any) => boolean | Promise<boolean>
) {
  return { gatewayUrl: base, rpcUrl: `${base}/node`, token, confirm };
}
async function provider_session_binding_invalidates_approval_before_writes(): Promise<void> {
  for (const method of [
    'eth_sendTransaction',
    'personal_sign',
    'eth_signTypedData_v4',
    'eth_sign',
  ]) {
    let token = 'session-a';
    const provider = new PaxeerProvider(
      providerConfig(
        () => token,
        () => {
          token = 'session-b';
          return true;
        }
      )
    );
    await provider.request({ method: 'eth_requestAccounts' });
    const start = seen.length;
    const params =
      method === 'eth_sendTransaction'
        ? [{ to: constructions.address }]
        : method === 'personal_sign'
          ? [constructions.message, constructions.address]
          : method === 'eth_signTypedData_v4'
            ? [constructions.address, constructions.typedData]
            : [
                constructions.address,
                constructions.eip7702AuthorizationDigest,
                constructions.eip7702Authorization,
              ];
    await expect(provider.request({ method, params })).rejects.toMatchObject({
      code: 4100,
    });
    expect(seen.slice(start).filter((entry) => entry.path.startsWith('/v1/wallet/'))).toEqual([]);
  }
  let provider: PaxeerProvider;
  provider = new PaxeerProvider(
    providerConfig(
      () => 'session-a',
      () => {
        provider.invalidateCapsSession();
        return true;
      }
    )
  );
  await provider.request({ method: 'eth_requestAccounts' });
  const start = seen.length;
  await expect(
    provider.request({
      method: 'eth_sendTransaction',
      params: [{ to: constructions.address }],
    })
  ).rejects.toMatchObject({ code: 4100 });
  expect(seen.slice(start)).toEqual([]);
  let current = 'session-a';
  const changed = new PaxeerProvider(providerConfig(() => current));
  await changed.request({ method: 'eth_requestAccounts' });
  current = 'session-b';
  await expect(
    changed.request({
      method: 'eth_sendTransaction',
      params: [{ to: constructions.address }],
    })
  ).rejects.toMatchObject({ code: 4100 });
}

async function provider_session_binding_checks_final_token_await_before_dispatch(): Promise<void> {
  let signing = false;
  let reads = 0;
  const provider = new PaxeerProvider(
    providerConfig(() => (signing && ++reads >= 4 ? 'session-b' : 'session-a'))
  );
  await provider.request({ method: 'eth_requestAccounts' });
  signing = true;
  const start = seen.length;
  await expect(
    provider.request({
      method: 'eth_sendTransaction',
      params: [{ to: constructions.address }],
    })
  ).rejects.toMatchObject({ code: 4100 });
  expect(seen.slice(start)).toEqual([]);
  expect(reads).toBe(4);
}

async function provider_session_binding_preserves_approved_sender_token_and_payload(): Promise<void> {
  let release!: () => void;
  let entered!: () => void;
  const confirming = new Promise<void>((resolve) => {
    entered = resolve;
  });
  const approval = new Promise<void>((resolve) => {
    release = resolve;
  });
  const provider = new PaxeerProvider(
    providerConfig(
      () => 'session-a',
      async ({ params }) => {
        expect(Object.isFrozen(params[0])).toBe(true);
        entered();
        await approval;
        return true;
      }
    )
  );
  await provider.request({ method: 'eth_requestAccounts' });
  const tx = {
    from: constructions.address,
    to: constructions.address,
    value: '0x1',
    chainId: '0x7d',
  };
  const pending = provider.request({ method: 'eth_sendTransaction', params: [tx] });
  await confirming;
  tx.to = `0x${'ab'.repeat(20)}`;
  tx.value = '0x999';
  release();
  expect(await pending).toBe(gateway['wallet-send'].body.tx_hash);
  const sent = seen.filter((entry) => entry.path === '/v1/wallet/send').at(-1)!;
  expect(sent.authorization).toBe('Bearer session-a');
  expect(sent.body).toEqual({
    tx: {
      from: constructions.address,
      to: constructions.address,
      value: '1',
      chainId: 125,
    },
  });
}

async function provider_session_binding_recovers_message_and_typed_data_signers(): Promise<void> {
  const provider = new PaxeerProvider(providerConfig(() => 'session-a'));
  await provider.request({ method: 'eth_requestAccounts' });
  const messageSignature = (await provider.request({
    method: 'personal_sign',
    params: [constructions.message, constructions.address],
  })) as Hex;
  expect(
    (
      await recoverMessageAddress({
        message: constructions.message,
        signature: messageSignature,
      })
    ).toLowerCase()
  ).toBe(constructions.address.toLowerCase());
  const typedSignature = (await provider.request({
    method: 'eth_signTypedData_v4',
    params: [constructions.address, constructions.typedData],
  })) as Hex;
  expect(
    (
      await recoverTypedDataAddress({
        ...constructions.typedData,
        signature: typedSignature,
      })
    ).toLowerCase()
  ).toBe(constructions.address.toLowerCase());
  swapSignatures = true;
  try {
    await expect(
      provider.request({
        method: 'personal_sign',
        params: [constructions.message, constructions.address],
      })
    ).rejects.toMatchObject({ code: 4100 });
    await expect(
      provider.request({
        method: 'eth_signTypedData_v4',
        params: [constructions.address, constructions.typedData],
      })
    ).rejects.toMatchObject({ code: 4100 });
  } finally {
    swapSignatures = false;
  }
}

async function default_node_hosted_methods_require_hosted_configuration_and_use_producer_namespaces(): Promise<void> {
  const defaultClient = new EndpointClient();
  expect(defaultClient.rpc.url).toBe(DEFAULT_ENDPOINT_URL);
  await expect(defaultClient.resolveAccount(constructions.address)).rejects.toThrow(/hostedUrl/);
  await expect(defaultClient.getNetwork()).rejects.toThrow(/hostedUrl/);
  const openrpc = fixture('../../../services/platform/hosted/gateway/openrpc.json');
  const methods = openrpc.methods.map((entry: any) => entry.name);
  expect(methods).toContain('px_resolveAccount');
  expect(methods).toContain('px_getNetwork');
  const nodeSource = readFileSync(new URL('../../../rpc/server.go', import.meta.url), 'utf8');
  expect(nodeSource).not.toContain('px_getNetwork');
  expect(nodeSource).not.toContain('px_resolveAccount');
  const client = new EndpointClient({
    nodeUrl: `${base}/node`,
    hostedUrl: `${base}/hosted`,
  });
  expect(await client.resolveAccount(accountExchange.request.params[0])).toEqual(
    accountExchange.response.result
  );
  // The committed legacy capture lacks the merged consensus view. Reaching the
  // hosted producer must not silently accept its retired network interface.
  const networkStart = seen.length;
  await expect(client.getNetwork()).rejects.toThrow(/consensus head/);
  expect(seen.slice(networkStart).map(({ path, body }) => ({ path, method: body.method }))).toEqual(
    [{ path: '/hosted', method: 'px_getNetwork' }]
  );
  expect(await client.rpc.call('eth_chainId', [])).toBe(chainExchange.body.result);
  expect(
    await client.batch([
      { method: 'eth_chainId', params: [] },
      accountExchange.request,
      { method: 'eth_chainId', params: [] },
    ])
  ).toEqual([
    { ok: true, result: chainExchange.body.result },
    { ok: true, result: accountExchange.response.result },
    { ok: true, result: chainExchange.body.result },
  ]);
  const noHosted = new EndpointClient({ nodeUrl: `${base}/node` });
  const start = seen.length;
  await expect(
    noHosted.batch([
      { method: 'eth_chainId', params: [] },
      { method: 'px_getNetwork', params: [] },
    ])
  ).rejects.toThrow(/hostedUrl/);
  expect(seen.slice(start)).toEqual([]);
}

test(
  receipt_fields_lost_preserves_complete_node_views.name,
  receipt_fields_lost_preserves_complete_node_views
);
test(
  receipt_authentication_rejects_tampered_node_outputs.name,
  receipt_authentication_rejects_tampered_node_outputs
);
test(
  receipt_authentication_protects_module_provider_and_ladder_consumers.name,
  receipt_authentication_protects_module_provider_and_ladder_consumers
);
test(
  receipt_authentication_keeps_uint64_key_ranges_lossless.name,
  receipt_authentication_keeps_uint64_key_ranges_lossless
);
test(
  provider_session_binding_invalidates_approval_before_writes.name,
  provider_session_binding_invalidates_approval_before_writes
);
test(
  provider_session_binding_checks_final_token_await_before_dispatch.name,
  provider_session_binding_checks_final_token_await_before_dispatch
);
test(
  provider_session_binding_preserves_approved_sender_token_and_payload.name,
  provider_session_binding_preserves_approved_sender_token_and_payload
);
test(
  provider_session_binding_recovers_message_and_typed_data_signers.name,
  provider_session_binding_recovers_message_and_typed_data_signers
);
test(
  default_node_hosted_methods_require_hosted_configuration_and_use_producer_namespaces.name,
  default_node_hosted_methods_require_hosted_configuration_and_use_producer_namespaces
);
