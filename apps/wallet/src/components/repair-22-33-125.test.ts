// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { act, createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MotionConfig } from 'framer-motion';
import { ethers } from 'ethers';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { mount } from '@/account/test/render';
import { submittedTransfer } from '@paxeer/wallet';
import { SwapSuccess } from './SwapSuccess';
import { resolveSwapReceiptResult } from './swapReceiptResult';
import type { SwapIntent } from '@/widgets/swap/swapJournal';
import type { SwapQuote, SwapParams } from '@/lib/swap/types';

// Loopback JSON-RPC peer with signed transaction identities and ABI-encoded receipt logs.
// It holds the pending/mined state in memory; production ethers and SDK codecs read its HTTP bytes.
const wallet = new ethers.Wallet(`0x${'01'.padStart(64, '0')}`);
const router = '0x0000000000000000000000000000000000000010';
const outputToken = '0x0000000000000000000000000000000000000020';
const otherToken = '0x0000000000000000000000000000000000000030';
const otherRecipient = '0x0000000000000000000000000000000000000040';
const blockHash = ethers.id('22.33.125 canonical receipt block');
const transferCodec = new ethers.Interface([
  'event Transfer(address indexed from, address indexed to, uint256 value)',
]);
const actualRaw = 9007199254740993123457n;
let server: Server;
let base: string;
let provider: ethers.JsonRpcProvider;
let swapTransaction: ethers.Transaction;
let sendTransaction: ethers.Transaction;
let swapMined = true;
let transferMined = false;
let receiptStatus = 1;
let canonicalHash = blockHash;
let reportedChainId = 125;
let includeOutput = true;
let unavailable = false;
let seen: Array<{ path: string; method: string; params: unknown[] }> = [];
let unexpected: string[] = [];
let transferModule: typeof import('./TransferSuccess');
let networkModule: typeof import('./NetworkSwitcher');
let repositories: typeof import('@/platform/storage/repositories');
let journalModule: typeof import('@/widgets/swap/swapJournal');
let originModule: typeof import('./transferOrigin');
const configuredKeys = [
  'NEXT_PUBLIC_PAXEER_RPC_URL',
  'NEXT_PUBLIC_EXPLORER_BASE',
  'NEXT_PUBLIC_PAXEER_EXPLORER_URL',
  'NEXT_PUBLIC_CHAIN_ID',
] as const;
const previousEnv = new Map<string, string | undefined>();

function transactionWire(tx: ethers.Transaction, mined: boolean) {
  return {
    hash: tx.hash,
    from: tx.from,
    to: tx.to,
    input: tx.data,
    value: ethers.toQuantity(tx.value),
    nonce: ethers.toQuantity(tx.nonce),
    chainId: ethers.toQuantity(tx.chainId),
    gas: ethers.toQuantity(tx.gasLimit),
    gasPrice: ethers.toQuantity(tx.gasPrice ?? 0),
    type: '0x0',
    v: ethers.toQuantity(tx.signature!.networkV!),
    r: tx.signature!.r,
    s: tx.signature!.s,
    blockNumber: mined ? '0x1' : null,
    blockHash: mined ? blockHash : null,
    transactionIndex: mined ? '0x0' : null,
  };
}

function logWire(
  tx: ethers.Transaction,
  token: string,
  from: string,
  to: string,
  amount: bigint,
  index: number
) {
  const event = transferCodec.encodeEventLog(transferCodec.getEvent('Transfer')!, [
    from,
    to,
    amount,
  ]);
  return {
    address: token,
    topics: event.topics,
    data: event.data,
    transactionHash: tx.hash,
    transactionIndex: '0x0',
    blockHash,
    blockNumber: '0x1',
    logIndex: ethers.toQuantity(index),
    removed: false,
  };
}

function receiptWire(tx: ethers.Transaction) {
  const logs =
    tx.hash === swapTransaction.hash && includeOutput
      ? [
          logWire(tx, outputToken, router, wallet.address, actualRaw - 2n, 0),
          logWire(tx, outputToken, router, wallet.address, 7n, 1),
          logWire(tx, outputToken, wallet.address, router, 5n, 2),
          logWire(tx, otherToken, router, wallet.address, 999n, 3),
          logWire(tx, outputToken, router, otherRecipient, 888n, 4),
        ]
      : [];
  return {
    transactionHash: tx.hash,
    transactionIndex: '0x0',
    blockHash,
    blockNumber: '0x1',
    from: tx.from,
    to: tx.to,
    contractAddress: null,
    status: ethers.toQuantity(receiptStatus),
    type: '0x0',
    gasUsed: '0x5208',
    cumulativeGasUsed: '0x5208',
    effectiveGasPrice: '0x1',
    logsBloom: `0x${'00'.repeat(256)}`,
    logs,
  };
}

function blockWire() {
  return {
    number: '0x1',
    hash: canonicalHash,
    parentHash: ethers.ZeroHash,
    timestamp: '0x6553f100',
    nonce: '0x0000000000000000',
    difficulty: '0x0',
    gasLimit: '0x1c9c380',
    gasUsed: '0x5208',
    miner: ethers.ZeroAddress,
    extraData: '0x',
    transactions: [
      swapMined ? swapTransaction.hash : null,
      transferMined ? sendTransaction.hash : null,
    ].filter(Boolean),
  };
}

function rpcResult(method: string, params: unknown[]) {
  if (method === 'eth_chainId') return ethers.toQuantity(reportedChainId);
  if (method === 'eth_blockNumber') return '0x1';
  if (method === 'eth_getTransactionCount') return '0x0';
  if (method === 'pax_sequencerReceipt') return null;
  if (method === 'eth_getBlockByNumber') return blockWire();
  const tx =
    params[0] === swapTransaction.hash
      ? swapTransaction
      : params[0] === sendTransaction.hash
        ? sendTransaction
        : null;
  if (!tx) throw new Error(`Unexpected transaction: ${String(params[0])}`);
  const mined = tx === swapTransaction ? swapMined : transferMined;
  if (method === 'eth_getTransactionByHash') return transactionWire(tx, mined);
  if (method === 'eth_getTransactionReceipt') return mined ? receiptWire(tx) : null;
  throw new Error(`Unexpected method ${method}`);
}

beforeAll(async () => {
  swapTransaction = ethers.Transaction.from(
    await wallet.signTransaction({
      to: router,
      data: '0x12345678',
      value: 0n,
      chainId: 125,
      nonce: 0,
      gasLimit: 21000,
      gasPrice: 1,
      type: 0,
    })
  );
  sendTransaction = ethers.Transaction.from(
    await wallet.signTransaction({
      to: otherRecipient,
      value: 1n,
      chainId: 125,
      nonce: 1,
      gasLimit: 21000,
      gasPrice: 1,
      type: 0,
    })
  );
  server = createServer(async (request, response) => {
    response.setHeader('content-type', 'application/json');
    const path = request.url ?? '';
    try {
      if (
        request.method === 'GET' &&
        path === `/explorer/api/v2/transactions/${sendTransaction.hash}/status`
      ) {
        seen.push({ path, method: 'GET', params: [] });
        response.end(JSON.stringify({ rung: 'committed', block_number: 1, committed_height: 1 }));
        return;
      }
      if (unavailable) {
        response.statusCode = 503;
        response.end(JSON.stringify({ error: 'Recorded offline interval' }));
        return;
      }
      if (request.method !== 'POST' || path !== '/rpc') throw new Error(`Unrecorded path ${path}`);
      let body = '';
      for await (const chunk of request) body += chunk;
      const input = JSON.parse(body);
      const reply = (call: { id: number; method: string; params: unknown[] }) => {
        seen.push({ path, method: call.method, params: call.params });
        return { jsonrpc: '2.0', id: call.id, result: rpcResult(call.method, call.params) };
      };
      response.end(JSON.stringify(Array.isArray(input) ? input.map(reply) : reply(input)));
    } catch (error) {
      unexpected.push(String(error));
      response.statusCode = 500;
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback port unavailable');
  base = `http://127.0.0.1:${address.port}`;
  const values = [`${base}/rpc`, `${base}/explorer`, `${base}/explorer`, '125'];
  configuredKeys.forEach((key, index) => {
    previousEnv.set(key, process.env[key]);
    process.env[key] = values[index];
  });
  // Configure only the fixture, before importing the production network resolver.
  transferModule = await import('./TransferSuccess');
  networkModule = await import('./NetworkSwitcher');
  repositories = await import('@/platform/storage/repositories');
  journalModule = await import('@/widgets/swap/swapJournal');
  originModule = await import('./transferOrigin');
});

beforeEach(() => {
  localStorage.clear();
  swapMined = true;
  transferMined = false;
  receiptStatus = 1;
  canonicalHash = blockHash;
  reportedChainId = 125;
  includeOutput = true;
  unavailable = false;
  seen = [];
  unexpected = [];
  // Disable ethers' short request cache so each assertion sees the current fixture revision.
  provider = new ethers.JsonRpcProvider(`${base}/rpc`, undefined, {
    cacheTimeout: -1,
    batchMaxCount: 1,
  });
});

afterEach(() => {
  provider.destroy();
});

afterAll(async () => {
  for (const key of configuredKeys) {
    const value = previousEnv.get(key);
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
});

async function approvedIntent(decimals = 6): Promise<SwapIntent> {
  const tokenIn = { address: otherToken, symbol: 'IN', name: 'Input', decimals: 6 };
  const tokenOut = { address: outputToken, symbol: 'OUT', name: 'Output', decimals };
  const quote: SwapQuote = {
    protocol: 'pecor',
    protocolLabel: 'PECOR',
    tokenIn,
    tokenOut,
    amountIn: '1000000',
    amountOut: (actualRaw + 100n).toString(),
    amountOutDisplay: '99999999999999999',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
  };
  const params: SwapParams = {
    tokenIn,
    tokenOut,
    amountIn: quote.amountIn,
    slippageBps: 100,
    recipient: wallet.address,
  };
  const journal = new journalModule.SwapJournal(localStorage, wallet.address, 125);
  const intent = journal.begin(quote, params);
  // Receipt metadata is transported from the real ethers provider into the production journal.
  const receipt = await provider.getTransactionReceipt(swapTransaction.hash!);
  if (!receipt) throw new Error('Fixture swap receipt missing');
  intent.transactions.push({
    kind: 'swap',
    leg: 0,
    nonce: swapTransaction.nonce,
    to: router,
    data: swapTransaction.data,
    value: '0',
    startBlock: 0,
    scannedThrough: 1,
    hash: swapTransaction.hash,
    receipt: {
      status: receipt.status!,
      blockNumber: receipt.blockNumber,
      blockHash: receipt.blockHash,
      logs: receipt.logs.map((log) => ({
        address: log.address,
        topics: [...log.topics],
        data: log.data,
      })),
    },
  });
  journal.save(intent);
  quote.tokenOut.symbol = 'REFRESHED';
  quote.amountOutDisplay = '123456789';
  const reopened = new journalModule.SwapJournal(localStorage, wallet.address, 125).current();
  if (!reopened) throw new Error('Executed quote not recovered');
  return reopened;
}

function swapMarkup(
  result: Awaited<ReturnType<typeof resolveSwapReceiptResult>> | undefined,
  txHash = swapTransaction.hash!
) {
  return renderToStaticMarkup(
    createElement(
      MotionConfig,
      { isStatic: true },
      createElement(SwapSuccess, {
        fromAmount: '999',
        fromSymbol: 'REFRESHED-IN',
        toAmount: '777777',
        toSymbol: 'REFRESHED-OUT',
        txHash,
        result,
        onDone() {},
      })
    )
  );
}

async function swap_receipt_output_preserves_exact_units_and_executed_quote() {
  const intent = await approvedIntent();
  const pending = resolveSwapReceiptResult(intent, provider);
  // A refresh during receipt I/O cannot change the approved identity or output metadata.
  const quoteId = intent.id;
  intent.id = 'refreshed-quote';
  intent.quote.tokenOut.symbol = 'REFRESHED';
  const result = await pending;
  expect(Object.isFrozen(result)).toBe(true);
  expect(result.quoteId).toBe(quoteId);
  expect(result.toAmountRaw).toBe(actualRaw.toString());
  expect(result.toAmount).toBe('9007199254740993.123457');
  const html = swapMarkup(result);
  expect(html).toContain('You received');
  expect(html).toContain('9007199254740993.123457 OUT');
  expect(html).toContain(`data-swap-quote="${quoteId}"`);
  expect(html).toContain('data-swap-chain="125"');
  expect(html).not.toContain('777777');
  expect(html).not.toContain('REFRESHED');
  const completed = await mount(
    createElement(SwapSuccess, {
      fromAmount: '999',
      fromSymbol: 'REFRESHED-IN',
      toAmount: '777777',
      toSymbol: 'REFRESHED-OUT',
      txHash: result.txHash,
      result,
      onDone() {},
    })
  );
  try {
    await completed.until(
      () => completed.text('h2') === 'Swap Completed',
      'verified completed swap'
    );
    expect(completed.container.textContent).toContain('9007199254740993.123457 OUT');
    expect(completed.container.textContent).not.toContain('777777');
  } finally {
    await completed.unmount();
  }
  expect(seen.some((call) => call.method === 'eth_getBlockByNumber')).toBe(true);
  expect(unexpected).toEqual([]);
  provider.destroy();
}

async function swap_receipt_zero_decimals_and_unverified_estimates_stay_distinct() {
  const intent = await approvedIntent(0);
  const result = await resolveSwapReceiptResult(intent, provider);
  expect(result.toAmount).toBe(actualRaw.toString());
  expect(swapMarkup(result)).toContain(`${actualRaw} OUT`);
  for (const html of [swapMarkup(undefined), swapMarkup(result, sendTransaction.hash!)]) {
    expect(html).toContain('Estimated output');
    expect(html).toContain('Received amount awaits verified execution evidence.');
    expect(html).not.toContain('You received');
    expect(html).not.toContain('Swap Completed');
  }
  provider.destroy();
}

async function swap_receipt_rejects_foreign_chain_reorg_revert_and_missing_output() {
  const intent = await approvedIntent();
  reportedChainId = 126;
  await expect(resolveSwapReceiptResult(intent, provider)).rejects.toThrow(
    /different chain|network changed/
  );
  provider.destroy();
  reportedChainId = 125;
  provider = new ethers.JsonRpcProvider(`${base}/rpc`, undefined, {
    cacheTimeout: -1,
    batchMaxCount: 1,
  });
  canonicalHash = ethers.id('replacement canonical block');
  await expect(resolveSwapReceiptResult(intent, provider)).rejects.toThrow('canonical block');
  canonicalHash = blockHash;
  receiptStatus = 0;
  await expect(resolveSwapReceiptResult(intent, provider)).rejects.toThrow('executed transaction');
  receiptStatus = 1;
  includeOutput = false;
  await expect(resolveSwapReceiptResult(intent, provider)).rejects.toThrow(
    'matching received output'
  );
  expect(unexpected).toEqual([]);
  provider.destroy();
}

function transferView(chainId = 125) {
  return createElement(transferModule.TransferSuccess, {
    fromLabel: 'Sender',
    fromAmount: '1',
    fromSymbol: 'PAX',
    toLabel: 'Recipient',
    transfer: { chainId, hash: sendTransaction.hash! },
    explorerUrl: `${base}/wrong-explorer/tx/${sendTransaction.hash}`,
    onExplorerView() {
      throw new Error('Active-network explorer callback must not be used');
    },
    onDone() {},
  });
}

async function transfer_pending_reopens_on_its_original_network_after_switch() {
  const identity = { chainId: 125, hash: sendTransaction.hash! };
  let view = await mount(transferView());
  try {
    await view.until(() => view.text('h2') === 'Transfer Pending', 'original pending transfer');
    const originalHref = `${base}/explorer/tx/${identity.hash}`;
    expect(view.query('a')?.getAttribute('href')).toBe(originalHref);
    const saved =
      repositories.transferObservationsRepository.read()[originModule.transferStorageKey(identity)];
    expect(saved).toMatchObject({
      origin: {
        chainId: 125,
        rpcUrl: `${base}/rpc`,
        explorerUrl: `${base}/explorer`,
        statusUrl: `${base}/explorer`,
      },
    });

    const port = new URL(base).port;
    repositories.preferencesRepository.update((preferences) => ({
      ...preferences,
      networks: [
        {
          chainId: 125000001,
          kind: 'virtual',
          name: 'Other network',
          rpcUrl: `https://127.0.0.1:${port}/wrong-rpc`,
          explorerUrl: `https://127.0.0.1:${port}/other-explorer`,
        },
      ],
    }));
    // Switch through the production network selector, including its RPC_CHANGED_EVENT.
    const updatedSwitcher = await mount(createElement(networkModule.NetworkSwitcher));
    try {
      const reads = seen.filter((call) => call.method === 'eth_getTransactionReceipt').length;
      await updatedSwitcher.click('[data-chain-id="125000001"] button');
      await view.until(
        () => seen.filter((call) => call.method === 'eth_getTransactionReceipt').length > reads,
        'pending poll after switch'
      );
      expect(view.text('h2')).toBe('Transfer Pending');
      expect(view.query('a')?.getAttribute('href')).toBe(originalHref);
    } finally {
      await updatedSwitcher.unmount();
    }

    await view.unmount();
    // Network removal and root RPC changes must not rewrite the stored transfer origin.
    repositories.preferencesRepository.update((preferences) => ({
      ...preferences,
      networks: [],
      customRpc: `https://127.0.0.1:${port}/changed-rpc`,
    }));
    view = await mount(transferView());
    await view.until(() => view.text('h2') === 'Transfer Pending', 'recovered pending transfer');
    expect(view.query('a')?.getAttribute('href')).toBe(originalHref);
    transferMined = true;
    await act(async () => {
      window.dispatchEvent(new Event('online'));
    });
    await view.until(() => view.text('h2') === 'Transfer Completed', 'original chain commitment');
    expect(view.query('a')?.getAttribute('href')).toBe(originalHref);
    expect(
      seen.some((call) => call.path === `/explorer/api/v2/transactions/${identity.hash}/status`)
    ).toBe(true);
    expect(seen.every((call) => call.path === '/rpc' || call.path.startsWith('/explorer/'))).toBe(
      true
    );
    expect(unexpected).toEqual([]);

    await view.unmount();
    view = await mount(transferView());
    await view.until(
      () => view.text('h2') === 'Transfer Completed',
      'revalidated committed transfer after reopen'
    );
    expect(view.query('a')?.getAttribute('href')).toBe(originalHref);
  } finally {
    await view.unmount();
    provider.destroy();
  }
}

async function transfer_unavailable_origin_never_uses_the_active_chain() {
  const before = seen.length;
  const view = await mount(transferView(999));
  try {
    await view.until(() => view.query('[role="alert"]') !== null, 'unavailable origin error');
    expect(view.text('[role="alert"]')).toContain('submitted chain 999');
    expect(view.text('h2')).toBe('Transfer Status Unknown');
    expect(view.query('a')).toBeNull();
    expect(seen.length).toBe(before);
  } finally {
    await view.unmount();
    provider.destroy();
  }
}

async function transfer_origin_refuses_a_wrong_chain_after_recovery() {
  const identity = { chainId: 125000002, hash: sendTransaction.hash! };
  const origin = Object.freeze({
    chainId: identity.chainId,
    rpcUrl: `${base}/rpc`,
    explorerUrl: `${base}/original-virtual`,
    statusUrl: `${base}/original-virtual`,
  });
  originModule.saveTransferObservation(submittedTransfer(identity), origin);
  const recovered = originModule.resolveTransferOrigin(identity);
  expect(recovered).toEqual(origin);
  expect(Object.isFrozen(recovered)).toBe(true);
  // The real observer refuses the wrong chain while preserving its original link and origin.
  const view = await mount(transferView(identity.chainId));
  try {
    await view.until(() => view.query('[role="alert"]') !== null, 'wrong-chain receipt refusal');
    expect(view.text('[role="alert"]')).toContain('transfer was submitted on chain 125000002');
    expect(view.query('a')?.getAttribute('href')).toBe(
      `${base}/original-virtual/tx/${identity.hash}`
    );
    expect(originModule.resolveTransferOrigin(identity)).toEqual(origin);
    expect(unexpected).toEqual([]);
  } finally {
    await view.unmount();
    provider.destroy();
  }
}

async function transfer_origin_is_saved_before_an_offline_observation_and_recovered() {
  const identity = { chainId: 125, hash: sendTransaction.hash! };
  unavailable = true;
  let view = await mount(transferView());
  try {
    await view.until(() => view.query('[role="alert"]') !== null, 'offline origin persisted');
    expect(view.text('[role="alert"]')).toContain('HTTP 503');
    const original = originModule.resolveTransferOrigin(identity);
    expect(original.rpcUrl).toBe(`${base}/rpc`);
    expect(
      repositories.transferObservationsRepository.read()[originModule.transferStorageKey(identity)]
    ).toMatchObject({ origin: original });
    await view.unmount();
    repositories.preferencesRepository.update((preferences) => ({
      ...preferences,
      customRpc: `https://127.0.0.1:${new URL(base).port}/changed-rpc`,
    }));
    unavailable = false;
    view = await mount(transferView());
    await view.until(
      () => view.text('h2') === 'Transfer Pending',
      'offline submission reopened on original RPC'
    );
    expect(originModule.resolveTransferOrigin(identity)).toEqual(original);
    expect(view.query('a')?.getAttribute('href')).toBe(`${base}/explorer/tx/${identity.hash}`);
    expect(unexpected).toEqual([]);
  } finally {
    await view.unmount();
  }
}

for (const proof of [
  swap_receipt_output_preserves_exact_units_and_executed_quote,
  swap_receipt_zero_decimals_and_unverified_estimates_stay_distinct,
  swap_receipt_rejects_foreign_chain_reorg_revert_and_missing_output,
  transfer_pending_reopens_on_its_original_network_after_switch,
  transfer_unavailable_origin_never_uses_the_active_chain,
  transfer_origin_is_saved_before_an_offline_observation_and_recovered,
  transfer_origin_refuses_a_wrong_chain_after_recovery,
])
  test(proof.name, proof, 15000);
