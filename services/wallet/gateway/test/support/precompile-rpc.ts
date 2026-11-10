import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import {
  concatHex,
  decodeFunctionData,
  encodeFunctionResult,
  hashDomain,
  keccak256,
  recoverAddress,
  zeroAddress,
  type Abi,
  type Hex
} from 'viem';
import { TestChain, repoRoot } from './chain.js';

export type FixturePrecompile = 'scheduler' | 'streams' | 'eip712' | 'teeattestor';
const addresses: Record<FixturePrecompile, Hex> = {
  scheduler: '0x0000000000000000000000000000000000000905',
  streams: '0x0000000000000000000000000000000000000906',
  teeattestor: '0x0000000000000000000000000000000000000907',
  eip712: '0x0000000000000000000000000000000000000908'
};

export interface FixtureStream {
  id: bigint;
  payer: Hex;
  payee: Hex;
  token: Hex;
  ratePerSecond: bigint;
  cap: bigint;
  startTime: bigint;
  stopTime: bigint;
  settled: bigint;
  active: boolean;
}

/** Route conformance peer. ABI and empty-state/error answers follow the published
 * Go precompiles, not the gateway's encoders. Only the exercised methods exist.
 * This does not certify node execution; node-owned receipt transcripts cover it.
 */
export async function startPrecompileRpc(kind: FixturePrecompile) {
  const source = readFileSync(
    `${repoRoot}/precompiles/layerxagent/${kind}/${kind}.go`,
    'utf8'
  );
  const match = /\babiJSON\s*=\s*`([\s\S]*?)`/u.exec(source);
  assert(match, `published ${kind} ABI missing`);
  const abi = JSON.parse(match[1]!) as Abi;
  const chain = new TestChain(125);
  await chain.start();
  const failures: unknown[] = [];
  const streamStates = new Map<bigint, FixtureStream>();
  const baseMethods = new Set([
    'eth_chainId',
    'eth_blockNumber',
    'eth_getBalance',
    'eth_getTransactionCount',
    'eth_getBlockByNumber',
    'eth_maxPriorityFeePerGas',
    'eth_gasPrice',
    'eth_estimateGas',
    'eth_sendRawTransaction',
    'eth_getTransactionByHash'
  ]);
  const server = createServer(async (req, res) => {
    let id: unknown = null;
    try {
      assert.equal(req.method, 'POST');
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const request = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number;
        method: string;
        params: unknown[];
      };
      id = request.id;
      let result: unknown;
      if (request.method === 'eth_call') {
        const call = request.params[0] as { to: Hex; data: Hex };
        assert.equal(call.to.toLowerCase(), addresses[kind]);
        assert.equal(request.params[1], 'latest');
        if (call.data === '0xffffffff') {
          assert(source.includes('ErrUnknownMethod'));
          res.setHeader('content-type', 'application/json');
          res.end(
            JSON.stringify({
              jsonrpc: '2.0',
              id,
              error: { code: -32000, message: 'LXP_PRECOMPILE_UNKNOWN_METHOD' }
            })
          );
          return;
        }
        const { functionName, args = [] } = decodeFunctionData({ abi, data: call.data });
        if (kind === 'scheduler') {
          if (functionName === 'getJob') {
            assert.equal(args[0], (1n << 256n) - 1n);
            result = [0n, zeroAddress, zeroAddress, '0x', 0n, 0n, 0n, false];
          } else {
            assert.equal(functionName, 'pending');
            result = [];
          }
        } else if (kind === 'streams') {
          if (functionName === 'accrued') {
            assert.equal(args[0], 0n);
            result = 0n;
          } else {
            assert.equal(functionName, 'getStream');
            const id = args[0] as bigint;
            assert(id === 0n || streamStates.has(id), `unrecorded stream ${id}`);
            result = streamStates.get(id) ?? {
              id: 0n,
              payer: zeroAddress,
              payee: zeroAddress,
              token: zeroAddress,
              ratePerSecond: 0n,
              cap: 0n,
              startTime: 0n,
              stopTime: 0n,
              settled: 0n,
              active: false
            };
          }
        } else if (kind === 'eip712') {
          if (functionName === 'domainSeparator') {
            const [name, version, chainId, verifyingContract] = args as [
              string,
              string,
              bigint,
              Hex
            ];
            result = hashDomain({
              domain: { name, version, chainId, verifyingContract },
              types: {
                EIP712Domain: [
                  { name: 'name', type: 'string' },
                  { name: 'version', type: 'string' },
                  { name: 'chainId', type: 'uint256' },
                  { name: 'verifyingContract', type: 'address' }
                ]
              }
            });
          } else {
            assert(['hashTypedData', 'recoverTypedSigner'].includes(functionName));
            const [separator, structHash, signature] = args as [Hex, Hex, Hex];
            const hash = keccak256(concatHex(['0x1901', separator, structHash]));
            result =
              functionName === 'hashTypedData'
                ? hash
                : await recoverAddress({ hash, signature });
          }
        } else if (functionName === 'verify' || functionName === 'verifyAndExpect') {
          assert.equal(args[1], '0x0102');
          assert(source.includes('ErrTEEInvalidQuote'));
          res.setHeader('content-type', 'application/json');
          res.end(
            JSON.stringify({
              jsonrpc: '2.0',
              id,
              error: {
                code: -32000,
                message: 'LXP_AGENT_TEE_INVALID_QUOTE: invalid quote'
              }
            })
          );
          return;
        } else if (functionName === 'rootCount') {
          assert.equal(args[0], 2);
          result = 0n;
        } else {
          assert.equal(functionName, 'rootOf');
          assert.deepEqual(args, [3, 1n << 255n]);
          result = '0x';
        }
        result = encodeFunctionResult({ abi, functionName, result });
      } else {
        assert(baseMethods.has(request.method), `unexpected ${kind} RPC ${request.method}`);
        const upstream = await fetch(chain.url, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(request)
        });
        res.setHeader('content-type', 'application/json');
        res.end(await upstream.text());
        return;
      }
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ jsonrpc: '2.0', id, result }));
    } catch (error) {
      failures.push(error);
      res.setHeader('content-type', 'application/json');
      res.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id,
          error: { code: -32601, message: String(error) }
        })
      );
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  assert(address && typeof address !== 'string');
  return {
    url: `http://127.0.0.1:${address.port}`,
    streamStates,
    async stop() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve()))
      );
      await chain.stop();
      assert.deepEqual(failures, [], `${kind} fixture received an unexpected request`);
    }
  };
}
