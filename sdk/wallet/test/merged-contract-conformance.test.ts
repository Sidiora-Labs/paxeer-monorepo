import { expect, test } from 'vitest';
import { keccak256, parseTransaction } from 'viem';
import { EndpointClient } from '../src/endpoint.js';
import { KernelClient } from '../src/kernel.js';
import { PaxeerProvider } from '../src/provider.js';
import { WalletInterface } from '../src/wallet.js';
import { JsonRpcClient } from '../src/rpc.js';
import { decodeSequencerReceipt, readSequencerReceipt } from '../src/receipt.js';
import { bridge } from '../src/modules/bridge.js';
import { sendModuleTransactionReceipted } from '../src/modules/index.js';
import type { AssetPage, SequencerReceiptAnswer } from '../src/types.js';
import {
  authenticatedReceipt,
  receiptRecording,
  committedReceipt,
  nodeVector,
  recording,
  receiptPins,
  receiptWireKeys,
} from './support/receipt-recordings.js';
import { replay, rpcExchange } from './support/replay.js';
import './fixtures.test.js';
import './gas-station.test.js';
import './merged-types.test.js';
import './proof_wallet_sdk.test.js';

const merged = recording<any>('../fixtures/endpoint/merged-contract.json');

async function endpoint_tests_retired_shapes_use_merged_transport() {
  const source = nodeVector('V1-pax-transfer');
  const address = merged.account.account.evm_address;
  const transport = await replay([
    rpcExchange('px_getNetwork', [], merged.network),
    rpcExchange('px_getAccount', [address], merged.account),
    rpcExchange('px_getBalances', [address], merged.balances),
    rpcExchange('eth_getTransactionCount', [address, 'pending'], '0x2a'),
    rpcExchange('pax_sequencerReceipt', [source.receiptJson.txHash], source.receiptJson),
    rpcExchange('px_getReceiptKeys', [], receiptWireKeys(source.receiptJson)),
    rpcExchange('eth_getRawTransactionByHash', [source.receiptJson.txHash], source.rawTransaction),
  ]);
  try {
    const endpoint = new EndpointClient({
      nodeUrl: `${transport.url}/node`,
      hostedUrl: `${transport.url}/hosted`,
    });
    const network = await endpoint.getNetwork();
    expect(network.consensus).toEqual(merged.network.consensus);
    expect(network.consensus_reason).toBeNull();
    for (const field of ['kernel', 'anchor', 'layerx']) expect(network).not.toHaveProperty(field);
    const kernel = new KernelClient(endpoint);
    expect(await kernel.getAccount(address)).toEqual({
      available: true,
      result: { ...merged.account, paxeer: { ...merged.account.paxeer, height: '16' } },
    });
    expect(await kernel.getBalances(address)).toEqual({
      available: true,
      result: { ...merged.balances, height: '16' },
    });
    expect(await kernel.getSequence(address)).toEqual({ available: true, result: '42' });
    expect(await kernel.getReceipt(source.receiptJson.txHash)).toEqual({
      available: true,
      result: authenticatedReceipt(source.receiptJson),
    });
    expect(Reflect.get(kernel, 'getActivityStatus')).toBeUndefined();
    expect(() => kernel.getAccount('7c'.repeat(32))).toThrow(/address/);
    expect(
      transport.seen.map(({ path, body }) => [path, (body as { method: string }).method])
    ).toEqual([
      ['/node', 'px_getNetwork'],
      ['/node', 'px_getNetwork'],
      ['/node', 'px_getAccount'],
      ['/node', 'px_getBalances'],
      ['/node', 'eth_getTransactionCount'],
      ['/node', 'pax_sequencerReceipt'],
      ['/node', 'px_getReceiptKeys'],
      ['/node', 'eth_getRawTransactionByHash'],
    ]);
    expect(transport.errors).toEqual([]);
  } finally {
    await transport.close();
  }
}

async function receipt_fixture_third_shape_authenticates_every_rung() {
  const fixtures = recording<Record<string, SequencerReceiptAnswer>>(
    '../fixtures/sequencer-receipt.json'
  );
  for (const answer of Object.values(fixtures)) {
    const transport = await replay([
      rpcExchange('pax_sequencerReceipt', [answer.txHash], answer),
      rpcExchange('px_getReceiptKeys', [], receiptWireKeys(answer)),
      rpcExchange(
        'eth_getRawTransactionByHash',
        [answer.txHash],
        receiptRecording(answer).rawTransaction
      ),
    ]);
    try {
      const rpc = new JsonRpcClient({ url: `${transport.url}/node` });
      expect(
        await readSequencerReceipt((method, params) => rpc.call(method, params), answer.txHash)
      ).toEqual(authenticatedReceipt(answer));
      if (answer.rung !== 'submitted') {
        expect(answer.paymentDenom).toBe('uhpx');
        expect(answer.signingKey).toBe(receiptPins(answer)[0]!.publicKey);
        for (const field of Object.keys(answer).filter(
          (field) => !['blockNumber', 'blockHash', 'rung', 'txHash', 'nonce'].includes(field)
        )) {
          const missing: Record<string, unknown> = { ...answer };
          delete missing[field];
          expect(
            () => decodeSequencerReceipt(missing, answer.txHash, receiptPins(answer)),
            `missing ${field}`
          ).toThrow();
        }
        expect(() =>
          decodeSequencerReceipt(
            { ...answer, returnData: `${answer.returnData}00` },
            answer.txHash,
            receiptPins(answer)
          )
        ).toThrow(/digest/);
        expect(() =>
          decodeSequencerReceipt(
            { ...answer, status: '0x0', vmError: 'execution reverted' },
            answer.txHash,
            receiptPins(answer)
          )
        ).toThrow();
      }
      expect(transport.errors).toEqual([]);
    } finally {
      await transport.close();
    }
  }
}

async function bank_types_test_nullability_keeps_zero_and_rejects_invalid_metadata() {
  const empty = merged.assets.assets[0];
  // Committed metadata-missing producer example plus the existing zero-decimal
  // bank consumer boundary. Neither represents a fabricated signed receipt.
  const zero = {
    asset_id: 'ibc/ABCDEF',
    denom: 'ibc/ABCDEF',
    kind: 'ibc',
    name: 'IBC Asset',
    symbol: 'IBC',
    decimals: 0,
    supply: '0',
  };
  const page: AssetPage = { height: '16', joined_limit: 16, assets: [empty, zero] };
  const transport = await replay([rpcExchange('px_listAssets', [], page)]);
  try {
    const client = new EndpointClient({ nodeUrl: `${transport.url}/node` });
    expect(await client.listBankAssets()).toEqual(page);
    const decoded = await client.listBankAssets();
    expect(decoded.assets[0]).toEqual({
      asset_id: empty.asset_id,
      denom: empty.denom,
      kind: empty.kind,
      name: null,
      symbol: null,
      decimals: null,
      supply: null,
    });
    expect(decoded.assets[1]!.decimals).toBe(0);
    expect(decoded.assets[1]!.supply).toBe('0');
    expect(transport.errors).toEqual([]);
  } finally {
    await transport.close();
  }
  for (const [field, value] of [
    ['asset_id', null],
    ['kind', null],
    ['decimals', -1],
    ['decimals', 256],
    ['decimals', '0'],
    ['decimals', 0.5],
    ['supply', '-1'],
    ['supply', 0],
    ['supply', '01'],
    ['name', 42],
    ['symbol', false],
    ['denom', undefined],
  ] as const) {
    const invalid = await replay([
      rpcExchange('px_listAssets', [], { ...page, assets: [{ ...zero, [field]: value }] }),
    ]);
    try {
      await expect(
        new EndpointClient({ nodeUrl: `${invalid.url}/node` }).listBankAssets()
      ).rejects.toThrow(/malformed bank asset/);
      expect(invalid.errors).toEqual([]);
    } finally {
      await invalid.close();
    }
  }
}

async function module_fixture_invalid_signed_view_uses_captured_bridge_outputs() {
  const source = nodeVector('V1-layerxbridge-bridge-out');
  const answer = source.receiptJson;
  const raw = parseTransaction(source.rawTransaction);
  expect(answer.txHash).toBe(keccak256(source.rawTransaction));
  const transport = await replay([
    rpcExchange(
      'paxeer_sendTransactionReceipted',
      [
        {
          from: answer.from,
          to: raw.to,
          data: raw.data,
          value: '0x0',
          type: '0x2',
          chainId: '0x7d',
        },
      ],
      answer
    ),
    rpcExchange('px_getReceiptKeys', [], receiptWireKeys(answer)),
    rpcExchange(
      'eth_getRawTransactionByHash',
      [answer.txHash],
      receiptRecording(answer).rawTransaction
    ),
  ]);
  try {
    const rpc = new JsonRpcClient({ url: `${transport.url}/node` });
    const provider = {
      request: ({ method, params }: { method: string; params?: readonly unknown[] }) =>
        rpc.call(method, params ?? []),
    };
    const module = bridge(provider);
    const tx = module.bridgeOut(
      1n,
      '0x5fbdb2315678afecb367f032d93f642f64180aa3',
      5_000_000_000_000_000_000n,
      '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc'
    );
    expect(tx.data).toBe(raw.data);
    const receipt = await sendModuleTransactionReceipted(provider, answer.from!, tx);
    expect(receipt).toEqual(authenticatedReceipt(answer));
    expect(receipt).not.toHaveProperty('blockNumber');
    expect(receipt).not.toHaveProperty('blockHash');
    expect(receipt.logs).toHaveLength(1);
    expect(module.decodeEvent(receipt.logs![0]!)).toMatchObject({
      event: 'BridgeOut',
      fields: { chain: 1n, nonce: 7n, amount: 5_000_000_000_000_000_000n },
    });
    expect(transport.errors).toEqual([]);
  } finally {
    await transport.close();
  }
  for (const tampered of [
    { ...answer, logs: [] },
    { ...answer, logs: answer.logs!.map((log) => ({ ...log, data: `${log.data}00` })) },
    { ...answer, blockNumber: committedReceipt.blockNumber, blockHash: committedReceipt.blockHash },
    { ...answer, rung: 'committed' },
  ]) {
    const invalid = await replay([
      rpcExchange(
        'paxeer_sendTransactionReceipted',
        [
          {
            from: answer.from,
            to: raw.to,
            data: raw.data,
            value: '0x0',
            type: '0x2',
            chainId: '0x7d',
          },
        ],
        tampered
      ),
      rpcExchange('px_getReceiptKeys', [], receiptWireKeys(answer)),
      rpcExchange(
        'eth_getRawTransactionByHash',
        [answer.txHash],
        receiptRecording(answer).rawTransaction
      ),
    ]);
    try {
      const rpc = new JsonRpcClient({ url: `${invalid.url}/node` });
      await expect(
        sendModuleTransactionReceipted(
          { request: ({ method, params }) => rpc.call(method, params ?? []) },
          answer.from!,
          { to: raw.to!, data: raw.data!, value: 0n }
        )
      ).rejects.toThrow(/malformed or unauthenticated sequencer receipt/);
      expect(invalid.errors).toEqual([]);
    } finally {
      await invalid.close();
    }
  }
}

async function provider_tests_removed_custody_has_no_surface_or_transport() {
  const transport = await replay([]);
  try {
    const provider = new PaxeerProvider({
      gatewayUrl: transport.url,
      rpcUrl: `${transport.url}/node`,
      token: () => null,
    });
    const wallet = new WalletInterface(provider);
    expect(Reflect.get(wallet, 'signCustody')).toBeUndefined();
    expect('signCustody' in wallet).toBe(false);
    for (const method of ['paxeer_signCustody', 'paxeer_prepareCustody']) {
      await expect(provider.request({ method, params: [] })).rejects.toMatchObject({ code: 4200 });
    }
    expect(transport.seen).toEqual([]);
    expect(transport.errors).toEqual([]);
  } finally {
    await transport.close();
  }
}

test(endpoint_tests_retired_shapes_use_merged_transport);
test(receipt_fixture_third_shape_authenticates_every_rung);
test(bank_types_test_nullability_keeps_zero_and_rejects_invalid_metadata);
test(module_fixture_invalid_signed_view_uses_captured_bridge_outputs);
test(provider_tests_removed_custody_has_no_surface_or_transport);
