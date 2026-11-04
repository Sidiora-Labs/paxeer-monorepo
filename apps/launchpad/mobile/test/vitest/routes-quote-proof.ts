import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { expect } from 'vitest';
import {
  decodeFunctionData,
  decodeFunctionResult,
  encodeFunctionData,
  encodeFunctionResult,
  getAddress,
  keccak256,
  parseAbi,
  toHex,
} from 'viem';
import { decodeContract } from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { quote } from '../../src/runtime/routes/activity.ts';
import type { PublicConfiguration } from '../../src/runtime/service-client.ts';

const quoteAbi = parseAbi([
  'function quoteExactInput(address pool,uint256 amountIn,bool isBuy) view returns((uint256 amountOut,uint256 feeAmount,uint256 priceImpactBps))',
]);

export async function routeQuoteUsesChain125Interface() {
  const recorded = JSON.parse(
    readFileSync(
      new URL('./fixtures/repair-22-33-929-v2.json', import.meta.url),
      'utf8',
    ),
  ) as {
    chainQuote: {
      chainId: string;
      pool: string;
      inputRaw: string;
      side: 'buy';
      result: [string, string, string];
    };
  };
  const vector = recorded.chainQuote;
  const pool = getAddress(vector.pool);
  const quoter = getAddress('0x793006f4F321b8863F26DBD75b11bAa01DA181dc');
  const block = {
    number: '0x2380000',
    hash: `0x${'ab'.repeat(32)}`,
    parentHash: `0x${'cd'.repeat(32)}`,
    timestamp: '0x6955b900',
    transactions: [],
  };
  const produced = {
    amountOut: BigInt(vector.result[0]),
    feeAmount: BigInt(vector.result[1]),
    priceImpactBps: BigInt(vector.result[2]),
  };
  const encoded = encodeFunctionResult({
    abi: quoteAbi,
    functionName: 'quoteExactInput',
    result: produced,
  });
  expect(
    decodeFunctionResult({
      abi: quoteAbi,
      functionName: 'quoteExactInput',
      data: encoded,
    }),
  ).toEqual(produced);
  const calldata = encodeFunctionData({
    abi: quoteAbi,
    functionName: 'quoteExactInput',
    args: [pool, BigInt(vector.inputRaw), true],
  });
  const observed: { method: string; params: unknown[] }[] = [];
  const violations: string[] = [];
  let chainId = vector.chainId;
  let output = encoded;
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      response.setHeader('Content-Type', 'application/json');
      if (request.method !== 'POST' || request.url !== '/rpc') {
        violations.push(`${request.method} ${request.url}`);
        response.writeHead(404);
        response.end();
        return;
      }
      const rpc = JSON.parse(body) as {
        jsonrpc: string;
        id: number;
        method: string;
        params?: unknown[];
      };
      const params = rpc.params ?? [];
      observed.push({ method: rpc.method, params });
      let result: unknown;
      if (
        rpc.jsonrpc === '2.0' &&
        rpc.method === 'eth_chainId' &&
        params.length === 0
      ) {
        result = chainId;
      } else if (
        rpc.jsonrpc === '2.0' &&
        rpc.method === 'eth_getBlockByNumber' &&
        JSON.stringify(params) === JSON.stringify(['latest', false])
      ) {
        result = block;
      } else if (rpc.jsonrpc === '2.0' && rpc.method === 'eth_call') {
        const call = params[0] as
          | { to?: string; data?: string }
          | undefined;
        if (
          params.length === 2 &&
          params[1] === block.number &&
          call?.to?.toLowerCase() === quoter.toLowerCase() &&
          call.data === calldata
        ) {
          result = output;
        }
      }
      if (result === undefined) {
        violations.push(body);
        response.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: rpc.id,
            error: { code: -32602, message: 'Unrecorded chain125 request' },
          }),
        );
      } else {
        response.end(
          JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }),
        );
      }
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = origin.replace('http:', 'ws:');
    const config: PublicConfiguration = {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
      canonicalOrigin: origin,
      identityURI: `${origin}/identity`,
      manifestDigest: 'ab'.repeat(32),
      manifestURL: `${origin}/manifest.json`,
      optionalFeatures: { chat: false, live: false },
      approvedPlaybackOrigins: [],
      walletConnect: {
        projectId: 'ab'.repeat(16),
        walletLink: 'https://wallet.example.com',
        rpcURL: `${origin}/rpc`,
        iconURL: 'https://wallet.example.com/icon.png',
      },
    };
    const services = { config };
    const signal = new AbortController().signal;
    const body = {
      poolAddress: pool,
      side: vector.side,
      inputRaw: vector.inputRaw,
      slippageBps: 50,
    };
    for (const slippageBps of [1, 73, 1000]) {
      const before = observed.length;
      const result = await quote(
        services,
        { ...body, slippageBps },
        signal,
      );
      const timestamp = BigInt(block.timestamp);
      const id = keccak256(
        toHex(
          JSON.stringify([
            quoter.toLowerCase(),
            pool.toLowerCase(),
            body.side,
            body.inputRaw,
            slippageBps,
            block.hash,
          ]),
        ),
      ).slice(2);
      expect(decodeContract('Quote', result)).toEqual({
        quoteId: `${id.slice(0, 8)}-${id.slice(8, 12)}-8${id.slice(13, 16)}-8${id.slice(17, 20)}-${id.slice(20, 32)}`,
        poolAddress: pool,
        side: body.side,
        inputRaw: body.inputRaw,
        outputRaw: vector.result[0],
        feeRaw: vector.result[1],
        minOutputRaw: (
          (produced.amountOut * BigInt(10000 - slippageBps)) /
          10000n
        ).toString(),
        validUntil: new Date(Number(timestamp + 30n) * 1000).toISOString(),
        watermark: {
          chainId: '125',
          height: BigInt(block.number).toString(),
          blockHash: block.hash,
          integrity: 'healthy',
          observedAt: new Date(Number(timestamp) * 1000).toISOString(),
          projectionRevision: '1',
        },
      });
      expect(observed.slice(before)).toEqual([
        { method: 'eth_chainId', params: [] },
        { method: 'eth_getBlockByNumber', params: ['latest', false] },
        {
          method: 'eth_call',
          params: [{ data: calldata, to: quoter }, block.number],
        },
      ]);
      expect(decodeFunctionData({ abi: quoteAbi, data: calldata })).toEqual(
        {
          functionName: 'quoteExactInput',
          args: [pool, BigInt(body.inputRaw), true],
        },
      );
    }
    const before = observed.length;
    await expect(
      quote(services, body, AbortSignal.abort()),
    ).rejects.toThrow();
    await expect(
      quote(
        { config: { ...config, walletConnect: undefined } },
        body,
        signal,
      ),
    ).rejects.toThrow('Chain125 native RPC authority required');
    expect(observed).toHaveLength(before);
    chainId = '0x1';
    await expect(quote(services, body, signal)).rejects.toThrow(
      'Chain 125 is required for mobile quotes',
    );
    expect(observed.slice(before)).toEqual([
      { method: 'eth_chainId', params: [] },
    ]);
    chainId = vector.chainId;
    output = '0x';
    await expect(quote(services, body, signal)).rejects.toThrow();
    expect(violations).toEqual([]);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}
