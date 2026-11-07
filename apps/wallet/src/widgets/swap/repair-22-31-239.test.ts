// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import * as workerThreads from 'node:worker_threads';
import { act, createElement, useLayoutEffect, type ReactElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { ethers } from 'ethers';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { NATIVE_PAX } from '@/lib/swap/constants';
import {
  CONTRACTS,
  TOKENS,
  ERC20_ABI,
  PECOR_ABI,
  PECOR_QUOTER_ABI,
  PECOR_ROUTER_ABI,
  PECOR_VAULT_SHARE_ABI,
  getQuote,
} from '@/lib/swap/sdk';
import { RateLimitModal } from '@/components/ui/TransactionModals';
import type { SwapParams, SwapQuote } from '@/lib/swap/types';
import { JournaledSwapExecution, SwapJournal } from './swapJournal';
import { useJournaledSwapExecution, type UseSwapExecutionResult } from './useSwapExecution';
import { SwapRecoveryActions } from './SwapRecoveryActions';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// The peer is a loopback JSON-RPC fixture backed by an in-memory transaction store.
// Transactions are signed/decoded by ethers and all contract results use the SDK ABI.
// No EVM execution or live-chain balance claim is made by this fixture.
const fixtureWallet = new ethers.Wallet(`0x${'01'.padStart(64, '0')}`);
const owner = fixtureWallet.address;
const tokenCodec = new ethers.Interface(ERC20_ABI);
const pecorCodec = new ethers.Interface(PECOR_ABI);
const quoterCodec = new ethers.Interface(PECOR_QUOTER_ABI);
const routerCodec = new ethers.Interface(PECOR_ROUTER_ABI);
const transferCodec = new ethers.Interface(PECOR_VAULT_SHARE_ABI);
const transfer = transferCodec.encodeEventLog(transferCodec.getEvent('Transfer')!, [
  CONTRACTS.PECORVault,
  owner,
  ethers.parseEther('1'),
]);
const blockHash = ethers.id('swap recovery receipt fixture');
const zeroBloom = `0x${'00'.repeat(256)}`;
const transactions = new Map<string, Record<string, unknown>>();
const receipts = new Map<string, Record<string, unknown>>();
let mined = true;
let nextReceiptStatus = 1;
let server: Server;
let provider: ethers.JsonRpcProvider;
let signer: ethers.JsonRpcSigner;
let output: ethers.Contract;
let quote: SwapQuote;
let params: SwapParams;
let root: Root | null = null;
let container: HTMLDivElement;
let latest: UseSwapExecutionResult | null = null;
let currentOperation: Promise<void> | null = null;

function fixtureBlock(pending: boolean) {
  return {
    number: '0x1',
    hash: blockHash,
    parentHash: ethers.ZeroHash,
    timestamp: '0x6553f100',
    nonce: '0x0000000000000000',
    difficulty: '0x0',
    gasLimit: '0x1c9c380',
    gasUsed: '0x30d40',
    miner: ethers.ZeroAddress,
    extraData: '0x',
    transactions: [...transactions.keys()].filter((hash) => pending || receipts.has(hash)),
  };
}

function mineFixtureReceipts(): void {
  for (const [hash, tx] of transactions) {
    if (receipts.has(hash)) continue;
    tx.blockHash = blockHash;
    tx.blockNumber = '0x1';
    tx.transactionIndex = '0x0';
    receipts.set(hash, {
      transactionHash: hash,
      transactionIndex: '0x0',
      blockHash,
      blockNumber: '0x1',
      from: owner,
      to: tx.to,
      contractAddress: null,
      status: ethers.toQuantity(nextReceiptStatus),
      type: '0x0',
      gasUsed: '0x30d40',
      cumulativeGasUsed: '0x30d40',
      effectiveGasPrice: '0x3b9aca00',
      logsBloom: zeroBloom,
      logs:
        nextReceiptStatus === 1
          ? [
              {
                address: TOKENS.USDL.address,
                topics: transfer.topics,
                data: transfer.data,
                blockHash,
                blockNumber: '0x1',
                transactionHash: hash,
                transactionIndex: '0x0',
                logIndex: '0x0',
                removed: false,
              },
            ]
          : [],
    });
  }
}

async function rpcFixture(method: string, args: unknown[]): Promise<unknown> {
  switch (method) {
    case 'eth_chainId':
      return '0x7d';
    case 'eth_accounts':
      return [owner, '0x0000000000000000000000000000000000000002'];
    case 'eth_blockNumber':
      return '0x1';
    case 'eth_estimateGas':
      return '0x30d40';
    case 'eth_gasPrice':
      return '0x3b9aca00';
    case 'eth_getTransactionCount':
      return ethers.toQuantity(args[1] === 'pending' ? transactions.size : receipts.size);
    case 'eth_getBlockByNumber':
      return fixtureBlock(args[0] === 'pending');
    case 'eth_getTransactionByHash':
      return transactions.get(String(args[0])) ?? null;
    case 'eth_getTransactionReceipt': {
      const hash = String(args[0]);
      // The consumer must have persisted broadcast identity BEFORE requesting any receipt.
      expect(
        journal()
          .current()
          ?.transactions.some((tx) => tx.hash === hash)
      ).toBe(true);
      return receipts.get(hash) ?? null;
    }
    case 'eth_call': {
      const call = args[0] as { to: string; data: string };
      if (call.to.toLowerCase() === CONTRACTS.PECORQuoter.toLowerCase()) {
        const decoded = quoterCodec.parseTransaction(call);
        expect(decoded?.name).toBe('quoteExactIn');
        const result = {
          amountIn: ethers.parseEther('1'),
          amountOut: ethers.parseEther('1'),
          grossAmountOut: ethers.parseEther('1'),
          executionPrice: ethers.parseEther('1'),
          spotPriceIn: ethers.parseEther('1'),
          spotPriceOut: ethers.parseEther('1'),
          feeAmount: 0n,
          feeBps: 0n,
          sufficientLiquidity: true,
          availableLiquidity: ethers.parseEther('100'),
          priceTimestampIn: 1700000000n,
          priceTimestampOut: 1700000000n,
          priceStaleIn: false,
          priceStaleOut: false,
          utilizationCapBps: 10000n,
          tradeUtilizationBps: 100n,
          cappedByUtilization: false,
          twapOut: ethers.parseEther('1'),
          twapDeviationBps: 0n,
          effectiveFeeBps: 0n,
          tier: 0n,
          priceImpactBps: 0n,
          priceImpactDeduction: 0n,
        };
        return quoterCodec.encodeFunctionResult('quoteExactIn', [result]);
      }
      const callData = tokenCodec.parseTransaction(call);
      if (callData?.name === 'balanceOf') {
        expect(callData.args[0]).toBe(owner);
        return tokenCodec.encodeFunctionResult('balanceOf', [
          BigInt([...receipts.values()].filter((receipt) => receipt.status === '0x1').length) *
            10n ** 18n,
        ]);
      }
      if (callData?.name === 'allowance') {
        expect(callData.args[0]).toBe(owner);
        return tokenCodec.encodeFunctionResult('allowance', [ethers.MaxUint256]);
      }
      throw new Error(`Unexpected fixture contract call: ${call.data}`);
    }
    case 'eth_sendTransaction': {
      const request = args[0] as {
        from: string;
        to: string;
        data: string;
        value?: string;
        nonce: string;
      };
      expect(request.from.toLowerCase()).toBe(owner.toLowerCase());
      const saved = journal().current()!.transactions.at(-1)!;
      expect(saved).toMatchObject({
        hash: null,
        nonce: Number(BigInt(request.nonce)),
        data: request.data,
      });
      const codec =
        request.to.toLowerCase() === CONTRACTS.PECORRouter.toLowerCase() ? routerCodec : pecorCodec;
      expect(codec.parseTransaction(request)?.name).toBe(
        codec === routerCodec ? 'swapMultiHop' : 'swapExactInNative'
      );
      const signed = await fixtureWallet.signTransaction({
        chainId: 125,
        nonce: Number(BigInt(request.nonce)),
        to: request.to,
        data: request.data,
        value: request.value ?? 0,
        gasLimit: 200000,
        gasPrice: 1000000000,
        type: 0,
      });
      const tx = ethers.Transaction.from(signed);
      const hash = tx.hash!;
      transactions.set(hash, {
        hash,
        from: tx.from,
        to: tx.to,
        input: tx.data,
        value: ethers.toQuantity(tx.value),
        nonce: ethers.toQuantity(tx.nonce),
        chainId: '0x7d',
        type: '0x0',
        gas: '0x30d40',
        gasPrice: '0x3b9aca00',
        v: ethers.toQuantity(tx.signature!.networkV!),
        r: tx.signature!.r,
        s: tx.signature!.s,
        blockNumber: null,
        blockHash: null,
        transactionIndex: null,
      });
      if (mined) mineFixtureReceipts();
      return hash;
    }
    default:
      throw new Error(`Unexpected JSON-RPC fixture method: ${method}`);
  }
}

beforeAll(async () => {
  const locks: unknown = Reflect.get(workerThreads, 'locks');
  if (!locks) throw new Error('The recovery test requires Node Web Locks support.');
  Object.defineProperty(navigator, 'locks', { value: locks, configurable: true });
  server = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString()) as {
        id: number;
        method: string;
        params: unknown[];
      };
      const result = await rpcFixture(body.method, body.params);
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
    } catch (error) {
      response.statusCode = 500;
      response.end(error instanceof Error ? error.message : String(error));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Fixture did not bind a TCP port.');
  provider = new ethers.JsonRpcProvider(`http://127.0.0.1:${address.port}`, 125, {
    staticNetwork: true,
    batchMaxCount: 1,
    cacheTimeout: -1,
  });
  signer = await provider.getSigner();
  output = new ethers.Contract(TOKENS.USDL.address, ERC20_ABI, signer);
});

afterAll(async () => {
  provider?.destroy();
  if (server)
    await new Promise<void>((resolveClose, reject) => {
      server.close((error) => (error ? reject(error) : resolveClose()));
      server.closeAllConnections();
    });
});

beforeEach(async () => {
  transactions.clear();
  receipts.clear();
  mined = true;
  nextReceiptStatus = 1;
  localStorage.clear();
  container = document.createElement('div');
  document.body.appendChild(container);
  quote = {
    protocol: 'pecor',
    protocolLabel: 'PECOR',
    tokenIn: NATIVE_PAX,
    tokenOut: { ...TOKENS.USDL, decimals: 18 },
    amountIn: ethers.parseEther('1').toString(),
    amountOut: ethers.parseEther('1').toString(),
    amountOutDisplay: '1',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
  };
  params = {
    tokenIn: quote.tokenIn,
    tokenOut: quote.tokenOut,
    amountIn: quote.amountIn,
    slippageBps: 1,
    recipient: owner,
  };
  latest = null;
  currentOperation = null;
});

afterEach(async () => {
  await stop();
  await currentOperation;
  container.remove();
});

const getSigner = async () => signer;
const isCurrent = () => true;

function Consumer({ account = owner, chainId = 125 }: { account?: string; chainId?: number }) {
  const execution = useJournaledSwapExecution({ account, chainId, getSigner, isCurrent });
  useLayoutEffect(() => {
    latest = execution;
  });
  return createElement(SwapRecoveryActions, { execution });
}

function current(): UseSwapExecutionResult {
  if (!latest) throw new Error('Swap consumer did not mount.');
  return latest;
}

async function mount(account = owner, chainId = 125): Promise<void> {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(Consumer, { account, chainId }));
  });
  await until(() => !current().executing);
}

async function stop(): Promise<void> {
  await act(async () => {
    root?.unmount();
  });
  root = null;
}

async function until(check: () => boolean): Promise<void> {
  await expect
    .poll(
      async () => {
        await act(async () => {
          await new Promise((resolveWait) => setTimeout(resolveWait, 10));
        });
        return check();
      },
      { timeout: 10_000, interval: 20 }
    )
    .toBe(true);
}

function journal(): SwapJournal {
  return new SwapJournal(localStorage, owner, 125);
}

function startSwap(): void {
  currentOperation = current().execute(
    quote,
    quote.tokenIn,
    quote.tokenOut,
    params.slippageBps,
    owner,
    ethers.parseEther('10')
  );
}

function button(label: string): HTMLButtonElement {
  const element = Array.from(document.querySelectorAll('button')).find(
    (candidate) => candidate.textContent?.trim() === label
  );
  if (!element) throw new Error(`Recovery action was not rendered: ${label}`);
  return element;
}

async function swap_broadcast_recovery_reopens_original_transaction() {
  mined = false;
  const nonce = await provider.getTransactionCount(owner, 'pending');
  await mount();
  await act(async () => {
    startSwap();
  });
  await until(() => !!current().txHash);
  const original = journal().current()!;
  const hash = original.transactions[0].hash!;
  expect(current().progress).toBe('pending');
  expect(current().completedLegs).toBe(0);
  expect(original.transactions[0]).toMatchObject({
    nonce,
    hash,
    receipt: null,
    value: quote.amountIn,
  });
  expect(original.account).toBe(owner.toLowerCase());
  expect(original.chainId).toBe('125');
  expect(await provider.getTransactionReceipt(hash)).toBeNull();
  expect(await output.balanceOf(owner)).toBe(0n);

  await stop();
  await currentOperation;
  await mount();
  expect(current().intent?.id).toBe(original.id);
  expect(current().txHash).toBe(hash);
  expect(current().progress).toBe('pending');
  expect(journal().current()?.transactions[0].nonce).toBe(nonce);
  mineFixtureReceipts();
  await act(async () => {
    button('Check transaction status').click();
  });
  await until(() => current().progress === 'confirmed');
  const receipt = await provider.getTransactionReceipt(hash);
  expect(current().completedLegs).toBe(1);
  expect(journal().current()?.transactions[0].receipt).toEqual({
    status: 1,
    blockNumber: receipt!.blockNumber,
    blockHash: receipt!.blockHash,
    logs: receipt!.logs.map((log) => ({
      address: log.address,
      topics: [...log.topics],
      data: log.data,
    })),
  });
  expect(await output.balanceOf(owner)).toBe(ethers.parseEther('1'));
  expect(await provider.getTransactionCount(owner)).toBe(nonce + 1);
  await stop();
  await mount();
  expect(current().progress).toBe('confirmed');
  expect(current().txHash).toBe(hash);
  await act(async () => current().reset());
  expect(current().intent).toBeNull();
  expect(localStorage.getItem(journal().key)).toContain(hash);
}

async function swap_broadcast_recovery_retains_partial_multihop_progress() {
  // The production encoder must satisfy the actual SDK output decoder (gotcha 2968).
  const decoded = await getQuote(
    TOKENS.WPAX9.address,
    TOKENS.USDL.address,
    ethers.parseEther('1'),
    provider
  );
  expect(decoded.amountIn).toBe(ethers.parseEther('1'));
  expect(decoded.amountOut).toBe(ethers.parseEther('1'));
  expect(decoded.sufficientLiquidity).toBe(true);
  await mount();
  await act(async () => {
    startSwap();
  });
  await until(() => !current().executing);
  expect(current().progress).toBe('confirmed');
  await stop();
  // Retain a pre-atomic route snapshot using the journal producer's real transaction
  // and receipt. Only the legacy route metadata differs from today's direct route.
  const legacy = journal().current()!;
  legacy.quote.protocol = 'pecor-v4-multihop';
  legacy.expectedLegs = 2;
  legacy.error = 'The remaining launchpad leg failed after the first leg confirmed.';
  journal().save(legacy);
  await mount();
  const intent = journal().current()!;
  const swaps = intent.transactions.filter((tx) => tx.kind === 'swap');
  expect(current().execError).not.toBe('');
  expect(current().progress, current().execError).toBe('partial');
  expect(intent.expectedLegs).toBe(2);
  expect(swaps).toHaveLength(1);
  expect(swaps[0].receipt?.status).toBe(1);
  expect(await output.balanceOf(owner)).toBe(ethers.parseEther('1'));
  const nonce = await provider.getTransactionCount(owner, 'pending');
  await stop();
  await mount();
  expect(current().completedLegs).toBe(1);
  expect(current().progress).toBe('partial');
  expect(current().intent?.id).toBe(intent.id);
  expect(current().execError).toBe(intent.error);
  expect(container.textContent).toContain('1 of 2 swap legs confirmed');
  await act(async () => {
    current().reset();
  });
  expect(current().intent?.id).toBe(intent.id);
  await act(async () => {
    await current().execute(
      quote,
      quote.tokenIn,
      quote.tokenOut,
      params.slippageBps,
      owner,
      ethers.parseEther('10')
    );
  });
  expect(await provider.getTransactionCount(owner, 'pending')).toBe(nonce);
  expect(await output.balanceOf(owner)).toBe(ethers.parseEther('1'));
  expect(journal().current()?.transactions).toEqual(intent.transactions);
}

async function swap_speed_up_resubmits_recovery_actions_only_observe_original_nonce() {
  mined = false;
  const nonce = await provider.getTransactionCount(owner, 'pending');
  const boundary = new JournaledSwapExecution(journal(), () => {}, isCurrent, 20);
  await expect(boundary.execute(signer, quote, params)).rejects.toThrow('timed out');
  boundary.close();
  const original = journal().current()!;
  const hash = original.transactions[0].hash!;
  expect(original.transactions[0].nonce).toBe(nonce);
  await mount();
  expect(current().txHash).toBe(hash);
  expect(current().execError).toBe(original.error);
  expect(journal().current()?.error).toBe(original.error);
  expect(document.body.textContent).not.toContain('Speed Up (increase gas)');
  expect(document.body.textContent).toContain('0 of 1 swap legs confirmed');
  expect(document.body.textContent).toContain(hash);
  await act(async () => {
    button('Wait longer').click();
  });
  await until(() => !current().executing);
  await act(async () => {
    button('Check transaction status').click();
  });
  await until(() => !current().executing);

  // Drive the exact retry callback installed on the real rate-limit modal.
  const actions = SwapRecoveryActions({ execution: current() });
  const children = actions.props.children as ReactElement<{ onRetry?: () => void }>[];
  const retry = children.find((child) => child && child.type === RateLimitModal)?.props.onRetry;
  expect(retry).toBeTypeOf('function');
  await act(async () => {
    retry!();
  });
  await until(() => !current().executing);
  await act(async () => {
    startSwap();
  });
  await until(() => !current().executing);
  expect(journal().current()?.id).toBe(original.id);
  expect(journal().current()?.transactions).toHaveLength(1);
  const pending = await provider.getBlock('pending');
  expect(pending).not.toBeNull();
  expect(pending!.transactions).toHaveLength(1);
  expect(pending!.transactions[0]).toBe(hash);
  const pendingTransaction = await provider.getTransaction(pending!.transactions[0]);
  expect(pendingTransaction).not.toBeNull();
  expect(pendingTransaction!.nonce).toBe(nonce);
  mineFixtureReceipts();
  await act(async () => {
    await current().recover();
  });
  expect(current().progress).toBe('confirmed');
  expect(await output.balanceOf(owner)).toBe(ethers.parseEther('1'));
  expect(await provider.getTransactionCount(owner)).toBe(nonce + 1);
}

async function swap_recovery_isolates_account_chain_and_blocks_corrupt_journal() {
  mined = false;
  const boundary = new JournaledSwapExecution(journal(), () => {}, isCurrent, 20);
  await expect(boundary.execute(signer, quote, params)).rejects.toThrow('timed out');
  boundary.close();
  const original = journal().current()!;
  const other = await provider.getSigner(1);
  await mount(await other.getAddress());
  expect(current().intent).toBeNull();
  await stop();
  await mount(owner, 126);
  expect(current().intent).toBeNull();
  await stop();
  await mount();
  expect(current().intent?.id).toBe(original.id);
  const pendingNonce = await provider.getTransactionCount(owner, 'pending');
  await stop();
  const raw = localStorage.getItem(journal().key)!;
  localStorage.setItem(journal().key, raw.slice(0, -1));
  await mount();
  expect(current().execError).not.toBe('');
  await act(async () => {
    startSwap();
  });
  await until(() => !current().executing);
  expect(await provider.getTransactionCount(owner, 'pending')).toBe(pendingNonce);
  expect(localStorage.getItem(journal().key)).toBe(raw.slice(0, -1));
}

async function swap_atomic_routes_recover_one_transaction_and_prebroadcast_errors_remain_failed() {
  quote = {
    ...quote,
    protocol: 'pecor-v4-multihop',
    tokenIn: { ...TOKENS.USDC },
    tokenOut: {
      address: '0x0000000000000000000000000000000000000321',
      name: 'Launchpad',
      symbol: 'LP',
      decimals: 6,
      isLaunchpad: true,
    },
  };
  params = { ...params, tokenIn: quote.tokenIn, tokenOut: quote.tokenOut };
  await mount();
  await act(async () => {
    startSwap();
  });
  await until(() => !current().executing);
  expect(current().progress, current().execError).toBe('confirmed');
  expect(current().intent?.expectedLegs).toBe(1);
  expect(current().completedLegs).toBe(1);
  expect(journal().current()?.transactions).toHaveLength(1);
  const originalHash = current().txHash;
  await stop();
  await mount();
  expect(current().progress).toBe('confirmed');
  expect(current().txHash).toBe(originalHash);
  await act(async () => {
    current().reset();
  });
  quote = { ...quote, tokenIn: NATIVE_PAX };
  await act(async () => {
    startSwap();
  });
  await until(() => !current().executing);
  expect(current().progress).toBe('failed');
  expect(current().execError).toBe('Invalid V4 multihop route');
  expect(journal().current()?.transactions).toHaveLength(0);
  expect(transactions.size).toBe(1);
}

test(
  swap_atomic_routes_recover_one_transaction_and_prebroadcast_errors_remain_failed.name,
  swap_atomic_routes_recover_one_transaction_and_prebroadcast_errors_remain_failed
);

test(
  swap_broadcast_recovery_reopens_original_transaction.name,
  { timeout: 20_000 },
  swap_broadcast_recovery_reopens_original_transaction
);
test(
  swap_broadcast_recovery_retains_partial_multihop_progress.name,
  { timeout: 20_000 },
  swap_broadcast_recovery_retains_partial_multihop_progress
);
test(
  swap_speed_up_resubmits_recovery_actions_only_observe_original_nonce.name,
  { timeout: 20_000 },
  swap_speed_up_resubmits_recovery_actions_only_observe_original_nonce
);
test(
  swap_recovery_isolates_account_chain_and_blocks_corrupt_journal.name,
  { timeout: 20_000 },
  swap_recovery_isolates_account_chain_and_blocks_corrupt_journal
);
