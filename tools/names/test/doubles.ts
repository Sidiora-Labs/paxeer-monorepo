import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer, type IncomingMessage, type Server } from 'node:http';
import type { TestContext } from 'node:test';
import {
  decodeFunctionData,
  encodeAbiParameters,
  keccak256,
  parseTransaction,
  type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';

export const privateKey = `0x${'11'.repeat(32)}` as const;
export const resolver = '0x0000000000000000000000000000000000001234' as const;
const blockHash = `0x${'22'.repeat(32)}` as const;
const account = privateKeyToAccount(privateKey);
export const abi = [
  {
    type: 'function',
    name: 'setContenthash',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'node', type: 'bytes32' },
      { name: 'hash', type: 'bytes' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'contenthash',
    stateMutability: 'view',
    inputs: [{ name: 'node', type: 'bytes32' }],
    outputs: [{ name: '', type: 'bytes' }],
  },
] as const;

async function body(request: IncomingMessage): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks);
}

async function listen(server: Server): Promise<string> {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
}

export async function startS3(t: TestContext) {
  const stored = new Map<string, Buffer>();
  const requests: {
    method: string;
    key: string;
    conditional?: string;
    cache?: string;
    type?: string;
  }[] = [];
  const raceKeys = new Set<string>();
  let headStatus: number | undefined;
  const s3 = createServer((request, response) => {
    void (async () => {
      const url = new URL(request.url!, 'http://localhost');
      if (!url.pathname.startsWith('/sites/')) {
        response.writeHead(404).end();
        return;
      }
      const key = decodeURIComponent(url.pathname.slice('/sites/'.length));
      requests.push({
        method: request.method!,
        key,
        conditional: request.headers['if-none-match'],
        cache: request.headers['cache-control'],
        type: request.headers['content-type'],
      });
      if (request.method === 'HEAD' || request.method === 'GET') {
        const bytes = stored.get(key);
        if (request.method === 'HEAD' && headStatus !== undefined) {
          response.writeHead(headStatus).end();
        } else if (!bytes) {
          response.writeHead(404).end();
        } else {
          response
            .writeHead(200, { 'content-length': bytes.length })
            .end(request.method === 'HEAD' ? undefined : bytes);
        }
      } else if (request.method === 'PUT') {
        const bytes = await body(request);
        if (raceKeys.delete(key)) stored.set(key, bytes);
        if (request.headers['if-none-match'] === '*' && stored.has(key)) {
          response
            .writeHead(412, { 'content-type': 'application/xml' })
            .end('<Error><Code>PreconditionFailed</Code></Error>');
          return;
        }
        if (request.headers['if-none-match'] !== '*') {
          response.writeHead(400).end();
          return;
        }
        stored.set(key, bytes);
        response.writeHead(200, { etag: '"stored"' }).end();
      } else {
        response.writeHead(405).end();
      }
    })().catch((error: unknown) => {
      response.writeHead(500).end(String(error));
    });
  });
  t.after(() => close(s3));
  const endpoint = await listen(s3);
  return {
    endpoint,
    stored,
    requests,
    raceKeys,
    setHeadStatus(status: number) {
      headStatus = status;
    },
  };
}

export type StorageProofVector = {
  name: string;
  address: string;
  slot: string;
  height: number;
  value: Hex;
  app_hash: string;
  proof_ops: { type: string; key: `0x${string}`; data: `0x${string}` }[];
};

export async function startRpc(t: TestContext, vector?: StorageProofVector) {
  const transactions: { to: string; data: Hex; hash: Hex; raw: Hex }[] = [];
  const records = new Map<string, Hex>();
  const calls: string[] = [];
  const headerHeights: bigint[] = [];
  let reverted = false;
  let proofValue: Hex | undefined;
  const rpc = createServer((request, response) => {
    void (async () => {
      if (request.method === 'GET') {
        assert.ok(vector, 'Storage proof vector is required');
        const height = BigInt(vector.height) + 1n;
        assert.equal(request.url, `/commit?height=${height}`);
        headerHeights.push(height);
        response.writeHead(200, { 'content-type': 'application/json' }).end(
          JSON.stringify({
            result: {
              signed_header: {
                header: {
                  height: String(height),
                  app_hash: vector.app_hash.slice(2).toUpperCase(),
                },
              },
            },
          })
        );
        return;
      }
      const payload = JSON.parse((await body(request)).toString()) as {
        id: number;
        method: string;
        params?: unknown[];
      };
      const params = payload.params ?? [];
      calls.push(payload.method);
      let result: unknown;
      switch (payload.method) {
        case 'eth_chainId':
          result = '0x7d';
          break;
        case 'eth_getTransactionCount':
          result = `0x${transactions.length.toString(16)}`;
          break;
        case 'eth_gasPrice':
          result = '0x3b9aca00';
          break;
        case 'eth_estimateGas':
          result = '0x186a0';
          break;
        case 'eth_maxPriorityFeePerGas':
          result = '0x3b9aca00';
          break;
        case 'eth_blockNumber':
          result = vector ? `0x${vector.height.toString(16)}` : '0x1';
          break;
        case 'eth_getProof':
          assert.ok(vector, 'Storage proof vector is required');
          assert.deepEqual(params.slice(0, 2), [vector.address, [vector.slot]]);
          if (params.length !== 3 || params[2] !== `0x${vector.height.toString(16)}`) {
            response.writeHead(200, { 'content-type': 'application/json' }).end(
              JSON.stringify({
                jsonrpc: '2.0',
                id: payload.id,
                error: { code: -32602, message: 'Expected the explicit proof height tag' },
              })
            );
            return;
          }
          result = {
            address: vector.address,
            hexValues: [proofValue ?? vector.value],
            storageProof: [
              {
                ops: vector.proof_ops.map((op) => ({
                  type: op.type,
                  key: Buffer.from(op.key.slice(2), 'hex').toString('base64'),
                  data: Buffer.from(op.data.slice(2), 'hex').toString('base64'),
                })),
              },
            ],
          };
          break;
        case 'eth_getBlockByNumber':
          result = {
            number: '0x1',
            hash: blockHash,
            parentHash: blockHash,
            timestamp: '0x1',
            gasLimit: '0x1c9c380',
            gasUsed: '0x186a0',
            baseFeePerGas: '0x3b9aca00',
            transactions: [],
          };
          break;
        case 'eth_sendRawTransaction': {
          const raw = params[0] as Hex;
          const transaction = parseTransaction(raw);
          assert.equal(transaction.chainId, 125);
          assert.ok(transaction.to);
          assert.ok(transaction.data);
          const decoded = decodeFunctionData({ abi, data: transaction.data });
          assert.equal(decoded.functionName, 'setContenthash');
          if (decoded.functionName !== 'setContenthash') throw new Error('Unexpected transaction');
          const [node, contenthash] = decoded.args;
          if (!reverted) records.set(node, contenthash);
          const hash = keccak256(raw);
          transactions.push({ to: transaction.to, data: transaction.data, hash, raw });
          result = hash;
          break;
        }
        case 'eth_getTransactionReceipt': {
          const transaction = transactions.find((item) => item.hash === params[0]);
          result = transaction
            ? {
                transactionHash: transaction.hash,
                transactionIndex: '0x0',
                blockHash,
                blockNumber: '0x1',
                from: account.address,
                to: transaction.to,
                cumulativeGasUsed: '0x186a0',
                gasUsed: '0x186a0',
                contractAddress: null,
                logs: [],
                logsBloom: `0x${'00'.repeat(256)}`,
                status: reverted ? '0x0' : '0x1',
                effectiveGasPrice: '0x3b9aca00',
                type: '0x2',
              }
            : null;
          break;
        }
        case 'eth_call': {
          const call = params[0] as { to: string; data: Hex };
          assert.equal(call.to.toLowerCase(), resolver.toLowerCase());
          const decoded = decodeFunctionData({ abi, data: call.data });
          assert.equal(decoded.functionName, 'contenthash');
          result = encodeAbiParameters([{ type: 'bytes' }], [records.get(decoded.args[0]) ?? '0x']);
          break;
        }
        default:
          response.writeHead(200, { 'content-type': 'application/json' }).end(
            JSON.stringify({
              jsonrpc: '2.0',
              id: payload.id,
              error: { code: -32601, message: `Unexpected method: ${payload.method}` },
            })
          );
          return;
      }
      response
        .writeHead(200, { 'content-type': 'application/json' })
        .end(JSON.stringify({ jsonrpc: '2.0', id: payload.id, result }));
    })().catch((error: unknown) => {
      response.writeHead(500).end(String(error));
    });
  });
  t.after(() => close(rpc));
  const rpcUrl = await listen(rpc);
  return {
    rpcUrl,
    transactions,
    records,
    calls,
    headerHeights,
    setReverted() {
      reverted = true;
    },
    setProofValue(value: Hex) {
      proofValue = value;
    },
  };
}
