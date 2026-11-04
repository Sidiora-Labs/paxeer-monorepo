import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import receiptRecording from '../../../../../services/interop/crates/layerx-gas-station/tests/fixtures/sequencer_receipts.json';
import nodeRecordings from '../../../../../sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json';
import { decodeSequencerReceipt, type ReceiptKeyPin } from '@paxeer/wallet';

const nodeRecording = nodeRecordings.cases.find((entry) => entry.id === 'V1-erc20-approve');
if (!nodeRecording) throw new Error('the node recordings carry no ERC-20 approval transaction');
const rawTransaction = nodeRecording.rawTransaction;

export const receiptPin = nodeRecording.pinnedKey;
export const receiptPins: readonly ReceiptKeyPin[] = [
  {
    ...receiptPin,
    keyId: receiptPin.keyId as `0x${string}`,
    publicKey: receiptPin.publicKey as `0x${string}`,
    firstHeight: BigInt(receiptPin.firstHeight),
    lastHeight: BigInt(receiptPin.lastHeight),
  },
];
export const receiptedAnswer = nodeRecording.receiptJson;
export const committedAnswer = receiptRecording.committed;
export const receiptHash = committedAnswer.txHash;
export const submittedAnswer = receiptRecording.submitted;
export const matchingExplorer = {
  rung: 'committed',
  block_number: Number(BigInt(committedAnswer.blockNumber)),
  committed_height: Number(BigInt(committedAnswer.blockNumber)),
};

export function recordedReceipt(rung: 'receipted' | 'committed') {
  return decodeSequencerReceipt(
    rung === 'receipted' ? receiptedAnswer : committedAnswer,
    receiptHash,
    receiptPins
  );
}

export async function startReceiptEndpoint() {
  const seen: { method: string; path: string; params?: unknown[] }[] = [];
  const unexpected: string[] = [];
  const state = {
    receipt: null as unknown,
    sequence: [] as unknown[],
    explorer: { rung: 'submitted', block_number: null, committed_height: null } as unknown,
    explorerStatus: 200,
    holdExplorer: false,
    holdReceipt: false,
    abortedReceipts: 0,
    releaseExplorer: undefined as (() => void) | undefined,
    releaseReceipt: undefined as (() => void) | undefined,
  };
  const server = createServer(async (req, res) => {
    res.setHeader('content-type', 'application/json');
    const route = req.url ?? '';
    if (req.method === 'GET' && route === `/api/v2/transactions/${receiptHash}/status`) {
      seen.push({ method: 'GET', path: route });
      const answer = () => {
        res.statusCode = state.explorerStatus;
        res.end(JSON.stringify(state.explorer));
      };
      if (state.holdExplorer) state.releaseExplorer = answer;
      else answer();
      return;
    }
    if (req.method === 'POST' && route === '/') {
      let text = '';
      for await (const chunk of req) text += chunk;
      const call = JSON.parse(text) as { id: number; method: string; params: unknown[] };
      seen.push({ method: call.method, path: route, params: call.params });
      if (
        call.method === 'pax_sequencerReceipt' &&
        JSON.stringify(call.params) === JSON.stringify([receiptHash])
      ) {
        res.once('close', () => {
          if (!res.writableEnded) state.abortedReceipts += 1;
        });
        const answer = () => {
          const result = state.sequence.length > 0 ? state.sequence.shift() : state.receipt;
          res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result }));
        };
        if (state.holdReceipt) state.releaseReceipt = answer;
        else answer();
        return;
      }
      if (call.method === 'px_getReceiptKeys' && call.params.length === 0) {
        res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result: { keys: [receiptPin] } }));
        return;
      }
      if (
        call.method === 'eth_getRawTransactionByHash' &&
        JSON.stringify(call.params) === JSON.stringify([receiptHash])
      ) {
        res.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: call.id,
            result: rawTransaction,
          })
        );
        return;
      }
    }
    unexpected.push(`${req.method} ${route}`);
    res.statusCode = 404;
    res.end(JSON.stringify({ message: 'unrecorded request' }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  return {
    base,
    seen,
    state,
    unexpected,
    config: { endpointUrl: base, explorerUrl: base, humanUrl: base },
    close: () =>
      new Promise<void>((resolve, reject) => {
        server.closeAllConnections();
        server.close((error) => (error ? reject(error) : resolve()));
      }),
  };
}
