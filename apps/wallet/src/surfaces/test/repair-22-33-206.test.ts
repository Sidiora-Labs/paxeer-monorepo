// @vitest-environment jsdom
import './chain.test';
import { expect, test } from 'vitest';
import { BRIDGE_EVENTS } from '@sidiora/layerx-sdk/browser';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { act, createElement } from 'react';
import {
  JsonRpcClient,
  bridge,
  readSequencerReceipt,
  requireAuthenticatedReceipt,
  sendModuleTransactionReceipted,
  type ModuleProvider,
} from '@paxeer/wallet';
import { mount } from '@/account/test/render';
import { surfaceReceipt, useModuleSend, type ModuleSend } from '../useSurface';
import { SurfaceChain, nodeReceiptCall, nodeReceiptVectors } from './chain';

class RecordedSurfaceNode {
  readonly chain = new SurfaceChain();
  readonly requests: { method: string; params: readonly unknown[] }[] = [];
  fallback = false;
  alter: ((method: string, answer: unknown) => unknown) | null = null;
  private readonly server: Server;
  private url = '';

  constructor() {
    this.server = createServer(async (request, response) => {
      let id: unknown = null;
      try {
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const envelope = JSON.parse(Buffer.concat(chunks).toString()) as {
          id: unknown;
          method: string;
          params: readonly unknown[];
        };
        id = envelope.id;
        this.requests.push(envelope);
        if (this.fallback && envelope.method === 'paxeer_sendTransactionReceipted') {
          throw Object.assign(new Error('unsupported method'), { code: 4200 });
        }
        const answer = await this.chain.request(envelope);
        const result = this.alter ? this.alter(envelope.method, answer) : answer;
        response.writeHead(200, { 'content-type': 'application/json' });
        response.end(JSON.stringify({ jsonrpc: '2.0', id, result }));
      } catch (error) {
        response.writeHead(200, { 'content-type': 'application/json' });
        response.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id,
            error: {
              code:
                typeof (error as { code?: unknown }).code === 'number'
                  ? (error as { code: number }).code
                  : -32603,
              message: error instanceof Error ? error.message : String(error),
            },
          })
        );
      }
    });
  }

  async open(): Promise<ModuleProvider> {
    await new Promise<void>((resolve) => this.server.listen(0, '127.0.0.1', resolve));
    this.url = `http://127.0.0.1:${(this.server.address() as AddressInfo).port}`;
    const client = new JsonRpcClient({ url: this.url });
    return { request: ({ method, params }) => client.call(method, params ?? []) };
  }

  async close(): Promise<void> {
    this.server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      this.server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

async function repair_22_33_206_unsigned_surface_fixture_replays_node_send_and_read() {
  const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-layerxbridge-bridge-out')!;
  const call = nodeReceiptCall(entry);
  const transaction = { to: call.to, data: call.data, value: BigInt(call.value) };
  const peer = new RecordedSurfaceNode();
  const provider = await peer.open();
  const rpc = (method: string, params: readonly unknown[]) => provider.request({ method, params });
  try {
    expect(await readSequencerReceipt(rpc, entry.receiptJson.txHash)).toBeNull();
    for (const fallback of [false, true]) {
      peer.fallback = fallback;
      const start = peer.requests.length;
      const receipt = await sendModuleTransactionReceipted(
        provider,
        peer.chain.address,
        transaction
      );
      requireAuthenticatedReceipt(receipt);
      expect(receipt.receipt).toBe(entry.signedReceipt);
      expect(receipt.authentication.receiptDigest).toBe(entry.receiptDigest);
      const sent = surfaceReceipt(receipt, bridge(provider));
      expect(sent.events[0]).toHaveProperty(
        'precompile',
        BRIDGE_EVENTS.find((event) => event.name === 'BridgeOut')!.precompile
      );
      expect(sent).toEqual({
        hash: entry.receiptJson.txHash,
        rung: 'receipted',
        status: 'confirmed',
        events: [
          {
            event: 'BridgeOut',
            precompile: BRIDGE_EVENTS.find((event) => event.name === 'BridgeOut')!.precompile,
            topic0: entry.receiptJson.logs[0]!.topics[0],
            fields: {
              chain: 1n,
              asset: nodeReceiptVectors().find(({ id }) => id === 'V1-erc20-approve')!.receiptJson
                .to,
              amount: 5_000_000_000_000_000_000n,
              recipient: '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc',
              nonce: 7n,
            },
          },
        ],
      });
      const read = await readSequencerReceipt(rpc, receipt.txHash);
      requireAuthenticatedReceipt(read);
      expect(read).toEqual(receipt);
      expect(surfaceReceipt(read, bridge(provider))).toEqual(sent);
      expect(peer.requests.slice(start).map(({ method }) => method)).toEqual(
        fallback
          ? [
              'paxeer_sendTransactionReceipted',
              'eth_sendTransaction',
              'pax_sequencerReceipt',
              'px_getReceiptKeys',
              'eth_getRawTransactionByHash',
              'px_getReceiptKeys',
              'eth_getRawTransactionByHash',
              'pax_sequencerReceipt',
              'px_getReceiptKeys',
              'eth_getRawTransactionByHash',
            ]
          : [
              'paxeer_sendTransactionReceipted',
              'px_getReceiptKeys',
              'eth_getRawTransactionByHash',
              'pax_sequencerReceipt',
              'px_getReceiptKeys',
              'eth_getRawTransactionByHash',
            ]
      );
      expect(peer.chain.sent).toHaveLength(fallback ? 2 : 1);
      expect(peer.chain.sent.at(-1)).toEqual(call);
    }
  } finally {
    await peer.close();
  }
}

test(
  repair_22_33_206_unsigned_surface_fixture_replays_node_send_and_read.name,
  repair_22_33_206_unsigned_surface_fixture_replays_node_send_and_read
);

async function repair_22_33_206_minimal_receipt_fixture_contract_gates_surface_confirmation() {
  const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-layerxbridge-bridge-out')!;
  const call = nodeReceiptCall(entry);
  const transaction = { to: call.to, data: call.data, value: BigInt(call.value) };
  const peer = new RecordedSurfaceNode();
  const provider = await peer.open();
  let state: ModuleSend | undefined;
  function Harness() {
    state = useModuleSend(bridge(provider), provider, peer.chain.address);
    return createElement(
      'output',
      { 'data-role': 'result' },
      JSON.stringify(state.sent, (_, value) =>
        typeof value === 'bigint' ? value.toString() : value
      )
    );
  }
  const mounted = await mount(createElement(Harness));
  try {
    for (const mode of [
      'missing receipt',
      'signature',
      'execution',
      'logs',
      'governed pin',
      'raw transaction',
    ]) {
      peer.alter = (method, answer) => {
        if (mode === 'governed pin' && method === 'px_getReceiptKeys') {
          return { keys: [{ ...entry.pinnedKey, publicKey: `0x${'42'.repeat(32)}` }] };
        }
        if (mode === 'raw transaction' && method === 'eth_getRawTransactionByHash') {
          return nodeReceiptVectors()[0]!.rawTransaction;
        }
        if (method !== 'paxeer_sendTransactionReceipted') return answer;
        const changed = structuredClone(answer) as Record<string, unknown>;
        if (mode === 'missing receipt') Reflect.deleteProperty(changed, 'receipt');
        if (mode === 'signature')
          changed.receipt = `${entry.signedReceipt.slice(0, -2)}${entry.signedReceipt.endsWith('00') ? '01' : '00'}`;
        if (mode === 'execution') changed.gasUsed = '0x1';
        if (mode === 'logs') changed.logs = [];
        return changed;
      };
      await act(async () => {
        await state!.send(transaction);
      });
      expect(state!.sent, mode).toBeNull();
      expect(state!.error, mode).toBeTypeOf('string');
      expect(state!.error!.length, mode).toBeGreaterThan(0);
      expect(state!.sending).toBe(false);
      expect(mounted.text('[data-role="result"]')).toBe('null');
    }
    peer.alter = null;
    await act(async () => {
      await state!.send(transaction);
    });
    expect(state!.error).toBeNull();
    expect(state!.sending).toBe(false);
    expect(state!.sent?.hash).toBe(entry.receiptJson.txHash);
    expect(state!.sent?.status).toBe('confirmed');
    expect(state!.sent?.rung).toBe('receipted');
    expect(state!.sent?.events[0]?.event).toBe('BridgeOut');
    expect(mounted.text('[data-role="result"]')).toContain('BridgeOut');
  } finally {
    await mounted.unmount();
    await peer.close();
  }
}

test(
  repair_22_33_206_minimal_receipt_fixture_contract_gates_surface_confirmation.name,
  repair_22_33_206_minimal_receipt_fixture_contract_gates_surface_confirmation
);
