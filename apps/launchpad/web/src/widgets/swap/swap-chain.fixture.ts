import { EventEmitter } from 'node:events';
import { strict as assert } from 'node:assert';
import {
  createPublicClient,
  custom,
  erc20Abi,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  toHex,
  type Address,
  type EIP1193Provider,
  type Hex,
  type TransactionSerialized,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import QuoterAbi from '@/core/network/abis/Quoter.json';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import recording from './swap-chain.recording.json';

export const swapTestChain = {
  id: 125,
  name: 'Recorded chain 125',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['http://127.0.0.1:1'] } },
} as const;

export interface RecordedCall {
  to: string;
  data: string;
  result: string;
}
export interface RpcRequest {
  method: string;
  params?: readonly unknown[];
}

export function recordedCall(
  calls: readonly RecordedCall[],
  params: readonly unknown[],
) {
  const call = params[0] as { to: string; data: string };
  assert.ok(
    params[1] === 'latest' || params[1] === undefined,
    'Recorded calls use latest state',
  );
  const record = calls.find(
    (row) =>
      row.to === call.to.toLowerCase() && row.data === call.data.toLowerCase(),
  );
  assert.ok(record, `Unrecorded eth_call: ${JSON.stringify(params)}`);
  return record.result;
}

export class RecordedProvider extends EventEmitter {
  readonly requests: RpcRequest[] = [];
  readonly errors: unknown[] = [];
  private closed = false;
  constructor(
    private readonly respond: (
      request: RpcRequest,
    ) => unknown | Promise<unknown>,
  ) {
    super();
  }
  request: EIP1193Provider['request'] = (async (request: RpcRequest) => {
    assert.equal(this.closed, false, 'RPC fixture is closed');
    this.requests.push(request);
    try {
      if (request.method === 'wallet_revokePermissions') {
        assert.deepEqual(request.params, [{ eth_accounts: {} }]);
        return null;
      }
      return await this.respond(request);
    } catch (error) {
      this.errors.push(error);
      throw error;
    }
  }) as EIP1193Provider['request'];
  async disconnect() {
    this.closed = true;
    this.removeAllListeners();
  }
}

export async function deploySwapChain() {
  const account = recording.account as Address;
  const tokens = recording.tokens as Address[];
  const pools = recording.pools as Address[];
  const signer = privateKeyToAccount(recording.privateKey as Hex);
  assert.equal(signer.address.toLowerCase(), account.toLowerCase());
  let stage = 0;
  let callGate: { started(): void; resumed: Promise<void> } | undefined;
  function holdCalls() {
    assert.equal(callGate, undefined, 'Only one RPC response gate may be held');
    let started!: () => void;
    let resume!: () => void;
    const observed = new Promise<void>((resolve) => {
      started = resolve;
    });
    const resumed = new Promise<void>((resolve) => {
      resume = resolve;
    });
    callGate = { started, resumed };
    return {
      started: observed,
      release() {
        callGate = undefined;
        resume();
      },
    };
  }
  const submitted = new Map<Hex, { receipt: object; transaction: object }>();

  function expectedSwap(data: Hex, to: string) {
    const expected = recording.swaps[stage];
    assert.ok(expected, 'No more recorded swaps');
    assert.equal(to.toLowerCase(), CORE_DEPLOYMENT.router.toLowerCase());
    // The production hook supplies a wall-clock deadline; every other calldata byte is recorded.
    assert.equal(
      data.slice(0, -64).toLowerCase(),
      expected.calldata.slice(0, -64).toLowerCase(),
    );
    const deadline = BigInt(`0x${data.slice(-64)}`);
    const now = BigInt(Math.floor(Date.now() / 1000));
    assert.ok(
      deadline > now && deadline <= now + 300n,
      'Swap deadline must be within 300 seconds',
    );
    return expected;
  }

  const provider: RecordedProvider = new RecordedProvider(
    async ({ method, params = [] }) => {
      switch (method) {
        case 'eth_chainId':
          return recording.chainId;
        case 'eth_accounts':
        case 'eth_requestAccounts':
          return [account];
        case 'eth_getTransactionCount':
          assert.equal(String(params[0]).toLowerCase(), account.toLowerCase());
          return toHex(recording.initialNonce + stage);
        case 'eth_call': {
          const pending = callGate;
          if (pending) {
            pending.started();
            await pending.resumed;
          }
          return recordedCall(recording.stages[stage].calls, params);
        }
        case 'eth_estimateGas': {
          const tx = params[0] as { to: string; data: Hex };
          return expectedSwap(tx.data, tx.to).estimate;
        }
        case 'eth_sendTransaction': {
          const tx = params[0] as {
            from: Address;
            to: Address;
            data: Hex;
            value?: Hex;
          };
          assert.equal(tx.from.toLowerCase(), account.toLowerCase());
          assert.equal(BigInt(tx.value ?? '0x0'), 0n);
          const expected = expectedSwap(tx.data, tx.to);
          const gas = await provider.request({
            method: 'eth_estimateGas',
            params: [tx],
          });
          assert.equal(gas, expected.estimate);
          const raw = await signer.signTransaction({
            type: 'legacy',
            chainId: 125,
            nonce: recording.initialNonce + stage,
            to: tx.to,
            data: tx.data,
            gas: BigInt(expected.gas),
            gasPrice: BigInt(expected.gasPrice),
          });
          return provider.request({
            method: 'eth_sendRawTransaction',
            params: [raw],
          });
        }
        case 'eth_sendRawTransaction': {
          const raw = params[0] as TransactionSerialized;
          const tx = parseTransaction(raw);
          assert.ok(tx.to && tx.data);
          const expected = expectedSwap(tx.data, tx.to);
          assert.equal(tx.chainId, 125);
          assert.equal(tx.nonce, recording.initialNonce + stage);
          assert.equal(tx.gas, BigInt(expected.gas));
          assert.equal(tx.gasPrice, BigInt(expected.gasPrice));
          assert.equal(tx.value ?? 0n, 0n);
          const from = await recoverTransactionAddress({
            serializedTransaction: raw,
          });
          assert.equal(from.toLowerCase(), account.toLowerCase());
          const hash = keccak256(raw);
          submitted.set(hash, {
            receipt: {
              ...expected.receipt,
              transactionHash: hash,
              logs: expected.receipt.logs.map((log) => ({
                ...log,
                transactionHash: hash,
              })),
            },
            transaction: {
              ...expected.transaction,
              hash,
              input: tx.data,
              r: tx.r,
              s: tx.s,
              v: toHex(tx.v!),
            },
          });
          stage++;
          return hash;
        }
        case 'eth_getTransactionReceipt': {
          const result = submitted.get(params[0] as Hex);
          assert.ok(result, 'Receipt requested for an unsubmitted transaction');
          return result.receipt;
        }
        case 'eth_getTransactionByHash': {
          const result = submitted.get(params[0] as Hex);
          assert.ok(result, 'Transaction requested for an unsubmitted hash');
          return result.transaction;
        }
        case 'eth_blockNumber':
          return recording.swaps[Math.max(0, stage - 1)].receipt.blockNumber;
        default:
          throw new Error(`Unrecorded swap RPC method: ${method}`);
      }
    },
  );
  const client = createPublicClient({
    chain: swapTestChain,
    transport: custom(provider, { retryCount: 0 }),
    pollingInterval: 10,
  });
  async function balance(token: Address) {
    return client.readContract({
      address: token,
      abi: erc20Abi,
      functionName: 'balanceOf',
      args: [account],
    });
  }
  async function quote(tokenIn: Address, tokenOut: Address, amountIn: bigint) {
    return client.readContract({
      address: CORE_DEPLOYMENT.quoter,
      abi: QuoterAbi,
      functionName: 'quoteMultihop',
      args: [tokenIn, tokenOut, amountIn],
    }) as Promise<{ amountOut: bigint; combinedPriceImpactBps: bigint }>;
  }
  return {
    provider,
    client,
    account,
    tokens,
    pools,
    balance,
    quote,
    holdCalls,
  };
}

export type SwapChain = Awaited<ReturnType<typeof deploySwapChain>>;
