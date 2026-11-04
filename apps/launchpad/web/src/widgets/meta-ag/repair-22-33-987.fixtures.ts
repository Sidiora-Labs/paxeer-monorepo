import { createServer, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import {
  createPublicClient,
  createWalletClient,
  defineChain,
  http,
  parseAbi,
  decodeFunctionData,
  encodeFunctionResult,
  encodeAbiParameters,
  encodeEventTopics,
  erc20Abi,
  keccak256,
  numberToHex,
  type Abi,
  type Hex,
} from 'viem';
import { advancedOrdersAbi, type AdvancedManifest } from './advanced-provider';
import type { Address } from './domain';

export const owner = '0x1111111111111111111111111111111111111111' as const;
export const other = '0x2222222222222222222222222222222222222222' as const;
export const usdl = '0x3333333333333333333333333333333333333333' as const;
export const token = '0x4444444444444444444444444444444444444444' as const;
export const orders = '0x5555555555555555555555555555555555555555' as const;
export const router = '0x6666666666666666666666666666666666666666' as const;
export const oracle = '0x7777777777777777777777777777777777777777' as const;
export const vault = '0x8888888888888888888888888888888888888888' as const;
export const adapter = '0x9999999999999999999999999999999999999999' as const;
export const adapterId: Hex = `0x${'ab'.repeat(32)}`;
const code = '0x60006000f3' as const;
const price = 10n ** 18n;
export const routeAbi = parseAbi([
  'event BestRouteSwap(address indexed user,address indexed tokenIn,address indexed tokenOut,uint256 amountIn,uint256 amountOut,bytes32 adapterId)',
  'event MultiHopSwap(address indexed user,address indexed tokenIn,address indexed tokenOut,uint256 amountIn,uint256 amountOut,uint256 hops)',
  'function getAdapters() view returns ((bytes32 adapterId,address adapter,bool active,string name)[])',
  'function swapViaAdapter(bytes32 adapterId,address tokenIn,address tokenOut,uint256 amountIn,uint256 amountOutMin,uint256 deadline) returns (uint256)',
  'function swapMultiHop((address tokenIn,address tokenOut,bytes32 adapterId,uint256 minAmountOut)[] hops,uint256 amountIn,uint256 amountOutMin,uint256 deadline) returns (uint256)',
]);
const adapterAbi = parseAbi([
  'function getQuote(address tokenIn,address tokenOut,uint256 amountIn) view returns ((uint256 amountOut,uint256 priceImpactBps,uint256 feeBps,uint256 feeAmount,bool available,bytes adapterData))',
  'function supportsSwap(address tokenIn,address tokenOut) view returns (bool)',
]);
const oracleAbi = parseAbi([
  'function getTokenPrice(address token) view returns (uint256 price,uint256 timestamp,bool isStale)',
]);
export function blockHash(height: bigint): Hex {
  return `0x${height.toString(16).padStart(64, '0')}`;
}
function txHash(index: number): Hex {
  return blockHash(100000n + BigInt(index));
}

export interface OrderTranscript {
  id: bigint;
  user: Address;
  stablecoin: Address;
  token: Address;
  amount: bigint;
  targetPrice: bigint;
  stopPrice: bigint;
  limitPrice: bigint;
  orderType: number;
  status: number;
  createdAt: bigint;
  expiresAt: bigint;
}
export function recordedOrder(
  id: bigint,
  status: number,
  orderType = 0,
  user: Address = owner,
): OrderTranscript {
  return {
    id,
    user,
    stablecoin: usdl,
    token,
    amount: 1000000n,
    targetPrice: price,
    stopPrice: price,
    limitPrice: price,
    orderType,
    status,
    createdAt: 1700000000n,
    expiresAt: 1700001800n,
  };
}

// An isolated JSON-RPC transcript peer. The wallet/provider and every ABI codec
// run unchanged; this peer owns only deterministic chain responses and storage.
export async function openRpcTranscript(history = false) {
  const now = BigInt(Math.floor(Date.now() / 1000));
  const head = history ? 6010n : 100n;
  const values = new Map<string, string>();
  const receiptStorageSnapshots: string[][] = [];
  const writes: Array<{ from: Address; to: Address; data: Hex }> = [];
  const orderRecords = new Map<bigint, OrderTranscript>();
  const transactions = new Map<Hex, { from: Address; to: Address; data: Hex }>();
  const receipts = new Map<Hex, Record<string, unknown>>();
  const logs: Array<Record<string, unknown>> = [];
  const ranges: Array<{ from: bigint; to: bigint; topics: unknown }> = [];
  const control = {
    dropReceipts: false,
    corruptSettlement: false,
    noncanonical: false,
    receiptReads: 0,
  };
  function log(
    abi: Abi,
    eventName: string,
    args: Record<string, unknown>,
    target: Address,
    height: bigint,
    hash: Hex,
    index = 0,
  ) {
    const event = abi.find((item) => item.type === 'event' && item.name === eventName);
    if (!event || event.type !== 'event') throw new Error('Transcript event missing');
    const unindexed = event.inputs.filter((item) => !item.indexed);
    return {
      address: target,
      topics: encodeEventTopics({ abi: [event], eventName, args }),
      data: encodeAbiParameters(
        unindexed,
        unindexed.map((item) => args[item.name!]),
      ),
      blockNumber: numberToHex(height),
      blockHash: blockHash(height),
      transactionHash: hash,
      transactionIndex: '0x0',
      logIndex: numberToHex(index),
      removed: false,
    };
  }
  function receipt(
    hash: Hex,
    target: Address,
    height: bigint,
    entries: Array<Record<string, unknown>>,
  ) {
    return {
      transactionHash: hash,
      transactionIndex: '0x0',
      from: owner,
      to: target,
      blockNumber: numberToHex(height),
      blockHash: blockHash(height),
      cumulativeGasUsed: '0x5208',
      gasUsed: '0x5208',
      effectiveGasPrice: '0x1',
      contractAddress: null,
      logs: entries,
      logsBloom: `0x${'00'.repeat(256)}`,
      status: '0x1',
      type: '0x0',
    };
  }
  if (history) {
    for (const [id, status, kind, user] of [
      [1n, 2, 0, owner],
      [2n, 3, 1, owner],
      [3n, 4, 0, owner],
      [4n, 1, 3, owner],
      [5n, 2, 0, other],
    ] as const) {
      const order = recordedOrder(id, status, kind, user);
      orderRecords.set(id, order);
      const createdHeight = id * 10n;
      logs.push(
        log(
          advancedOrdersAbi,
          kind === 3 ? 'StopLimitOrderCreated' : 'LimitOrderCreated',
          {
            orderId: id,
            user,
            orderType: kind,
            amount: order.amount,
            targetPrice: price,
            stopPrice: price,
            limitPrice: price,
          },
          orders,
          createdHeight,
          txHash(Number(id) + 10),
        ),
      );
      const settledHeight = 5000n + id;
      const hash = txHash(Number(id) + 20);
      const terminal =
        status === 2
          ? 'LimitOrderExecuted'
          : status === 3
            ? 'LimitOrderCancelled'
            : status === 4
              ? 'OrderExpiredAndRefunded'
              : 'StopLimitActivated';
      const event = log(
        advancedOrdersAbi,
        terminal,
        { orderId: id, user, amount: order.amount, executionPrice: price, activationPrice: price },
        orders,
        settledHeight,
        hash,
      );
      const asset = status === 2 || kind === 1 ? token : usdl;
      const transfer = log(
        erc20Abi,
        'Transfer',
        { from: vault, to: user, value: status === 2 ? 750000n : order.amount },
        asset,
        settledHeight,
        hash,
        1,
      );
      logs.push(event);
      receipts.set(hash, receipt(hash, orders, settledHeight, [event, transfer]));
    }
  }
  const resultFor = (abi: Abi, functionName: string, result: unknown) =>
    encodeFunctionResult({ abi, functionName, result });
  const server = createServer(async (request, response) => {
    try {
      let body = '';
      for await (const chunk of request) body += chunk;
      if (request.url === '/health') {
        response.setHeader('Content-Type', 'application/json');
        response.end(
          JSON.stringify({
            chainId: 125,
            manifestRevision: 'repair-transcript-v1',
            healthy: true,
            checkedAt: Date.now(),
          }),
        );
        return;
      }
      const input = JSON.parse(body) as { id: number; method: string; params: unknown[] };
      const params = input.params;
      const send = (result: unknown) => {
        response.setHeader('Content-Type', 'application/json');
        response.end(JSON.stringify({ jsonrpc: '2.0', id: input.id, result }));
      };
      if (input.method === 'eth_chainId') return send('0x7d');
      if (input.method === 'eth_accounts') return send([owner]);
      if (input.method === 'eth_getCode') return send(code);
      if (input.method === 'eth_blockNumber') return send(numberToHex(head));
      if (input.method === 'eth_getBlockByNumber') {
        const height = params[0] === 'latest' ? head : BigInt(params[0] as string);
        return send({
          number: numberToHex(height),
          hash: control.noncanonical ? blockHash(height + 1n) : blockHash(height),
          parentHash: blockHash(height - 1n),
          timestamp: numberToHex(now),
          gasLimit: '0x1c9c380',
          gasUsed: '0x5208',
          transactions: [],
          baseFeePerGas: '0x1',
          difficulty: '0x0',
          totalDifficulty: '0x0',
          extraData: '0x',
          miner: owner,
          nonce: '0x0000000000000000',
          receiptsRoot: blockHash(1n),
          sha3Uncles: blockHash(2n),
          size: '0x1',
          stateRoot: blockHash(3n),
          transactionsRoot: blockHash(4n),
          logsBloom: `0x${'00'.repeat(256)}`,
          uncles: [],
        });
      }
      if (input.method === 'eth_getTransactionByHash') {
        const hash = params[0] as Hex,
          tx = transactions.get(hash);
        if (!tx) throw new Error('Transcript transaction not found');
        return send({
          hash,
          from: tx.from,
          to: tx.to,
          input: tx.data,
          blockNumber: numberToHex(head),
          blockHash: blockHash(head),
          transactionIndex: '0x0',
          nonce: '0x0',
          value: '0x0',
          gas: '0x100000',
          gasPrice: '0x1',
          type: '0x0',
          v: '0x1d',
          r: blockHash(1n),
          s: blockHash(2n),
        });
      }
      if (input.method === 'eth_getTransactionReceipt') {
        control.receiptReads++;
        receiptStorageSnapshots.push([...values.values()]);
        if (control.dropReceipts) {
          request.socket.destroy();
          return;
        }
        const original = receipts.get(params[0] as Hex);
        if (!original) throw new Error('Transcript receipt not found');
        if (!control.corruptSettlement) return send(original);
        return send({ ...original, logs: [] });
      }
      if (input.method === 'eth_getLogs') {
        const filter = params[0] as {
          fromBlock: string;
          toBlock: string;
          address: Address;
          topics?: Array<string | string[] | null>;
        };
        const from = BigInt(filter.fromBlock),
          to = BigInt(filter.toBlock);
        ranges.push({ from, to, topics: filter.topics });
        if (to - from >= 2000n) throw new Error('Transcript rejects unbounded history range');
        return send(
          logs.filter(
            (item) =>
              item.address === filter.address &&
              BigInt(item.blockNumber as string) >= from &&
              BigInt(item.blockNumber as string) <= to &&
              (filter.topics ?? []).every(
                (topic, index) =>
                  topic === null ||
                  (Array.isArray(topic)
                    ? topic.includes((item.topics as string[])[index])
                    : topic.toLowerCase() === (item.topics as string[])[index]?.toLowerCase()),
              ),
          ),
        );
      }
      if (input.method === 'eth_call') {
        const call = params[0] as { to: Address; data: Hex };
        const abi =
          call.to.toLowerCase() === orders
            ? advancedOrdersAbi
            : call.to.toLowerCase() === router
              ? routeAbi
              : call.to.toLowerCase() === oracle
                ? oracleAbi
                : call.to.toLowerCase() === adapter
                  ? adapterAbi
                  : erc20Abi;
        const decoded = decodeFunctionData({ abi, data: call.data });
        const args = decoded.args ?? [];
        switch (decoded.functionName) {
          case 'decimals':
            return send(resultFor(abi, decoded.functionName, 6));
          case 'balanceOf':
          case 'allowance':
            return send(resultFor(abi, decoded.functionName, 1000000000000n));
          case 'getTokenPrice':
            return send(resultFor(abi, decoded.functionName, [price, now, false]));
          case 'getPricingConfig':
            return send(
              resultFor(abi, decoded.functionName, {
                fee: 10n,
                tier1: 0n,
                tier2: 0n,
                scalar: 1n,
                impact: false,
              }),
            );
          case 'getAdapters':
            return send(
              resultFor(abi, decoded.functionName, [
                { adapterId, adapter, active: true, name: 'Committed transcript adapter' },
              ]),
            );
          case 'supportsSwap':
            return send(resultFor(abi, decoded.functionName, true));
          case 'getQuote':
            return send(
              resultFor(abi, decoded.functionName, {
                amountOut: 750000n,
                priceImpactBps: 0n,
                feeBps: 10n,
                feeAmount: 750n,
                available: true,
                adapterData: encodeAbiParameters(
                  [{ type: 'address' }, { type: 'address' }],
                  [args[0] as Address, args[1] as Address],
                ),
              }),
            );
          case 'getLimitOrder':
          case 'getStopLimitOrder': {
            const record = orderRecords.get(args[0] as bigint);
            if (!record) throw new Error('Transcript order not found');
            return send(resultFor(abi, decoded.functionName, record));
          }
          case 'placeOrder':
            return send(resultFor(abi, decoded.functionName, 1n));
          case 'swapViaAdapter':
          case 'swapMultiHop':
            return send(resultFor(abi, decoded.functionName, 750000n));
          default:
            throw new Error(`Unrecorded contract call: ${decoded.functionName}`);
        }
      }
      if (input.method === 'eth_sendTransaction') {
        const tx = params[0] as { from: Address; to: Address; data: Hex };
        writes.push(tx);
        const hash = txHash(writes.length);
        transactions.set(hash, tx);
        let event: Record<string, unknown>;
        if (tx.to.toLowerCase() === orders) {
          const decoded = decodeFunctionData({ abi: advancedOrdersAbi, data: tx.data });
          if (decoded.functionName !== 'placeOrder') throw new Error('Unexpected economic action');
          const draft = decoded.args[0];
          const order = {
            ...recordedOrder(1n, 0, draft.orderType),
            ...draft,
            user: tx.from,
            targetPrice: draft.triggerPrice,
            stopPrice: draft.triggerPrice,
            createdAt: now,
          };
          orderRecords.set(1n, order);
          event = log(
            advancedOrdersAbi,
            draft.orderType >= 3 ? 'StopLimitOrderCreated' : 'LimitOrderCreated',
            {
              orderId: 1n,
              user: tx.from,
              orderType: draft.orderType,
              amount: draft.amount,
              targetPrice: draft.triggerPrice,
              stopPrice: draft.triggerPrice,
              limitPrice: draft.limitPrice,
            },
            orders,
            head,
            hash,
          );
        } else {
          const decoded = decodeFunctionData({ abi: routeAbi, data: tx.data });
          if (decoded.functionName === 'swapViaAdapter') {
            const [id, from, to, amount] = decoded.args;
            event = log(
              routeAbi,
              'BestRouteSwap',
              {
                user: tx.from,
                tokenIn: from,
                tokenOut: to,
                amountIn: amount,
                amountOut: 750000n,
                adapterId: id,
              },
              router,
              head,
              hash,
            );
          } else if (decoded.functionName === 'swapMultiHop') {
            const [hops, amount] = decoded.args;
            event = log(
              routeAbi,
              'MultiHopSwap',
              {
                user: tx.from,
                tokenIn: hops[0].tokenIn,
                tokenOut: hops.at(-1)!.tokenOut,
                amountIn: amount,
                amountOut: 750000n,
                hops: BigInt(hops.length),
              },
              router,
              head,
              hash,
            );
          } else throw new Error('Unexpected route action');
        }
        logs.push(event);
        receipts.set(hash, receipt(hash, tx.to, head, [event]));
        return send(hash);
      }
      throw new Error(`Unrecorded RPC: ${input.method}`);
    } catch (error) {
      fail(response, error);
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const endpoint = server.address();
  if (!endpoint || typeof endpoint === 'string') throw new Error('Loopback listener unavailable');
  const url = `http://127.0.0.1:${endpoint.port}`;
  const chain = defineChain({
    id: 125,
    name: 'Committed RPC transcript',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [url] } },
  });
  const binding = (address: Address) => ({ address, runtimeCodeHash: keccak256(code) });
  const manifest: AdvancedManifest = {
    version: 2,
    chainId: 125,
    revision: 'repair-transcript-v1',
    deploymentBlock: '1',
    confirmations: 1,
    usdl,
    orders: binding(orders),
    router: binding(router),
    quoter: binding(oracle),
    vault: binding(vault),
    adapters: [{ ...binding(adapter), id: adapterId, adapterData: 'vault-v2' }],
    priceScale: 18,
    priceDenomination: usdl,
    supportedKinds: ['limit-buy', 'stop-limit-buy'],
    maximumOrderLifetimeSeconds: '2592000',
    keeperHealthUrl: `${url}/health`,
  };
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
    removeItem: (key: string) => {
      values.delete(key);
    },
  };
  return {
    publicClient: createPublicClient({
      chain,
      cacheTime: 0,
      pollingInterval: 10,
      transport: http(url, { retryCount: 0, timeout: 2000 }),
    }),
    walletClient: createWalletClient({
      chain,
      account: owner,
      transport: http(url, { retryCount: 0, timeout: 2000 }),
    }),
    manifest,
    storage,
    values,
    writes,
    ranges,
    control,
    receipts,
    logs,
    receiptStorageSnapshots,
    close: async () => {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}
function fail(response: ServerResponse, error: unknown) {
  response.statusCode = 500;
  response.end(String(error));
}
