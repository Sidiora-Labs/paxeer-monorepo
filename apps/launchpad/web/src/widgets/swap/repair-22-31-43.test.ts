import { createRequire } from 'node:module';
import React, { act } from 'react';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { custom, decodeFunctionData, type Address, type Hex } from 'viem';
import RouterAbi from '@/core/network/abis/Router.json';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import {
  deploySwapChain,
  swapTestChain,
  type SwapChain,
} from './swap-chain.fixture';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (
    html: string,
    options: object,
  ) => { window: Window & typeof globalThis & { close(): void } };
};
const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'http://localhost:3050',
  pretendToBeVisual: true,
});
const installed: string[] = [];
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globals || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] =
    typeof value === 'function' && !/^[A-Z]/.test(key)
      ? value.bind(dom.window)
      : value;
  installed.push(key);
}
globals.IS_REACT_ACT_ENVIRONMENT = true;
const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import(
  '@tanstack/react-query'
);
const { createConfig, WagmiProvider } = await import('wagmi');
const { reconnect, disconnect } = await import('wagmi/actions');
const { injected } = await import('wagmi/connectors');
const { useSwapQuoteExecution } = await import('./swap-panel');
const { HighImpactWarning } = await import('@/widgets/trade/slippage-selector');
let chain: SwapChain;

beforeAll(async () => {
  chain = await deploySwapChain();
}, 180000);
afterAll(async () => {
  expect(chain?.provider.errors).toEqual([]);
  await act(async () => {
    await chain?.provider.disconnect();
    await new Promise<void>((resolve) => setImmediate(resolve));
  });
  dom.window.close();
  for (const key of installed) delete globals[key];
  delete globals.IS_REACT_ACT_ENVIRONMENT;
});

async function until(check: () => boolean) {
  const deadline = Date.now() + 10000;
  while (!check() && Date.now() < deadline) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 15));
    });
  }
  expect(check()).toBe(true);
}

async function mountTrade(amountIn: bigint) {
  const config = createConfig({
    chains: [swapTestChain],
    connectors: [
      injected({
        shimDisconnect: false,
        target: { id: 'swap-evm', name: 'Swap EVM', provider: chain.provider },
      }),
    ],
    transports: { 125: custom(chain.provider, { retryCount: 0 }) },
    multiInjectedProviderDiscovery: false,
    storage: null,
    batch: { multicall: false },
  });
  const query = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  let input = {
    tokenIn: chain.tokens[0],
    tokenOut: chain.tokens[1],
    amountIn,
    enabled: true,
    slippageBps: 100,
  };
  let current: ReturnType<typeof useSwapQuoteExecution>;
  function Trade() {
    current = useSwapQuoteExecution(input);
    return React.createElement(
      React.Fragment,
      null,
      React.createElement(
        'button',
        { disabled: !current.quote, onClick: current.requestSwap },
        'Swap',
      ),
      current.review
        ? React.createElement(HighImpactWarning, {
            priceImpactBps: current.review.quote.combinedPriceImpactBps,
            onConfirm: current.confirmSwap,
            onCancel: current.cancelReview,
          })
        : null,
    );
  }
  async function render(patch: Partial<typeof input> = {}) {
    input = { ...input, ...patch };
    await act(async () =>
      root.render(
        React.createElement(
          WagmiProvider,
          { config, reconnectOnMount: false },
          React.createElement(
            QueryClientProvider,
            { client: query },
            React.createElement(Trade),
          ),
        ),
      ),
    );
  }
  async function close() {
    await act(async () => root.unmount());
    query.clear();
    await disconnect(config);
    container.remove();
  }
  try {
    await act(async () => {
      const connected = await reconnect(config, {
        connectors: [config.connectors[0]],
      });
      expect(connected).toHaveLength(1);
    });
    await render();
    await until(() => !!current?.quote);
    return {
      get current() {
        return current;
      },
      container,
      query,
      render,
      disconnect: () =>
        act(async () => {
          await disconnect(config);
        }),
      close,
    };
  } catch (error) {
    await close();
    throw error;
  }
}

async function expectNoTransaction(nonce: number) {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 40));
  });
  expect(
    await chain.client.getTransactionCount({
      address: chain.account,
      blockTag: 'pending',
    }),
  ).toBe(nonce);
}

async function assertSwap(
  hash: Hex,
  tokenIn: Address,
  tokenOut: Address,
  amountIn: bigint,
  minAmountOut: bigint,
) {
  const receipt = await chain.client.waitForTransactionReceipt({ hash });
  const transaction = await chain.client.getTransaction({ hash });
  expect(
    receipt.status,
    `Swap ${hash}: used ${receipt.gasUsed} of ${transaction.gas} gas`,
  ).toBe('success');
  expect(transaction.to?.toLowerCase()).toBe(
    CORE_DEPLOYMENT.router.toLowerCase(),
  );
  const decoded = decodeFunctionData({
    abi: RouterAbi,
    data: transaction.input,
  });
  expect(decoded.functionName).toBe('swapTokenForToken');
  expect(decoded.args?.slice(0, 4)).toEqual([
    tokenIn,
    tokenOut,
    amountIn,
    minAmountOut,
  ]);
}

async function swapQuoteAmountEditsInvalidateDuringDebounce() {
  const trade = await mountTrade(1000000n);
  try {
    const previous = trade.current.quote!;
    expect(previous.identity).toEqual({
      chainId: 125,
      tokenIn: chain.tokens[0],
      tokenOut: chain.tokens[1],
      amountIn: 1000000n,
    });
    expect(previous.amountOut).toBe(
      (await chain.quote(chain.tokens[0], chain.tokens[1], 1000000n)).amountOut,
    );
    const nonce = await chain.client.getTransactionCount({
      address: chain.account,
      blockTag: 'pending',
    });
    const oldRequest = trade.current.requestSwap;
    await trade.render({ amountIn: 10000000n });
    expect(trade.current.quote).toBeNull();
    expect(trade.current.minAmountOut).toBe(0n);
    expect(trade.container.querySelector('button')?.disabled).toBe(true);
    await act(async () => oldRequest());
    await expectNoTransaction(nonce);
    await until(() => trade.current.quote?.identity.amountIn === 10000000n);
    const quoted = trade.current.quote!;
    expect(quoted.amountOut).toBe(
      (await chain.quote(chain.tokens[0], chain.tokens[1], 10000000n))
        .amountOut,
    );
    const minimum = (quoted.amountOut * 9900n) / 10000n;
    expect(trade.current.minAmountOut).toBe(minimum);
    const beforeIn = await chain.balance(chain.tokens[0]);
    const beforeOut = await chain.balance(chain.tokens[1]);
    await act(async () => trade.container.querySelector('button')!.click());
    await until(() => !!trade.current.txHash);
    await assertSwap(
      trade.current.txHash!,
      chain.tokens[0],
      chain.tokens[1],
      10000000n,
      minimum,
    );
    expect(beforeIn - (await chain.balance(chain.tokens[0]))).toBe(10000000n);
    expect(
      (await chain.balance(chain.tokens[1])) - beforeOut,
    ).toBeGreaterThanOrEqual(minimum);
  } finally {
    await trade.close();
  }
}

async function swapHighImpactConfirmationBindsExactQuote() {
  const amount = (await chain.balance(chain.tokens[0])) / 2n;
  const trade = await mountTrade(amount);
  try {
    expect(trade.current.quote!.combinedPriceImpactBps).toBeGreaterThan(500);
    const nonce = await chain.client.getTransactionCount({
      address: chain.account,
      blockTag: 'pending',
    });
    await act(async () => trade.current.requestSwap());
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain(
      `${(trade.current.quote!.combinedPriceImpactBps / 100).toFixed(2)}%`,
    );
    const oldConfirm = trade.current.confirmSwap;
    await trade.render({ amountIn: amount / 2n });
    expect(trade.current.quote).toBeNull();
    expect(trade.current.review).toBeNull();
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => oldConfirm());
    await expectNoTransaction(nonce);
    await until(() => trade.current.quote?.identity.amountIn === amount / 2n);
    await act(async () => oldConfirm());
    await expectNoTransaction(nonce);
    expect(trade.current.review).toBeNull();
    await act(async () => trade.current.requestSwap());
    expect(trade.current.review).not.toBeNull();
    const oldSlippageConfirm = trade.current.confirmSwap;
    await trade.render({ slippageBps: 300 });
    expect(trade.current.review).toBeNull();
    await act(async () => oldSlippageConfirm());
    await expectNoTransaction(nonce);
    await act(async () => trade.current.requestSwap());
    const approved = trade.current.review!;
    expect(approved.quote.combinedPriceImpactBps).toBeGreaterThan(500);
    expect(approved.minAmountOut).toBe(
      (approved.quote.amountOut * 9700n) / 10000n,
    );
    const beforeOut = await chain.balance(chain.tokens[1]);
    const confirm = Array.from(document.querySelectorAll('button')).find(
      (button) => button.textContent === 'Trade anyway',
    );
    expect(confirm).toBeDefined();
    await act(async () => confirm!.click());
    await until(() => !!trade.current.txHash);
    await assertSwap(
      trade.current.txHash!,
      chain.tokens[0],
      chain.tokens[1],
      amount / 2n,
      approved.minAmountOut,
    );
    expect(
      (await chain.balance(chain.tokens[1])) - beforeOut,
    ).toBeGreaterThanOrEqual(approved.minAmountOut);
  } finally {
    await trade.close();
  }
}

async function swapConfirmationRejectsPairRefetchAndWalletChanges() {
  const amount = (await chain.balance(chain.tokens[0])) / 2n;
  const trade = await mountTrade(amount);
  try {
    expect(trade.current.quote!.combinedPriceImpactBps).toBeGreaterThan(500);
    const nonce = await chain.client.getTransactionCount({
      address: chain.account,
      blockTag: 'pending',
    });
    await act(async () => trade.current.requestSwap());
    const oldPairConfirm = trade.current.confirmSwap;
    await trade.render({ tokenOut: chain.tokens[2] });
    expect(trade.current.review).toBeNull();
    await until(
      () => trade.current.quote?.identity.tokenOut === chain.tokens[2],
    );
    await act(async () => oldPairConfirm());
    await expectNoTransaction(nonce);
    expect(trade.current.quote!.amountOut).toBe(
      (await chain.quote(chain.tokens[0], chain.tokens[2], amount)).amountOut,
    );
    await act(async () => trade.current.requestSwap());
    expect(trade.current.review).not.toBeNull();
    const oldRefetchConfirm = trade.current.confirmSwap;
    const pending = chain.holdCalls();
    try {
      let refetch: Promise<void> | undefined;
      await act(async () => {
        refetch = trade.query.invalidateQueries();
        await pending.started;
      });
      await until(() => trade.current.quote === null);
      expect(trade.current.review).toBeNull();
      await act(async () => {
        pending.release();
        await refetch;
      });
    } finally {
      pending.release();
    }
    await until(() => !!trade.current.quote);
    expect(trade.current.review).toBeNull();
    await act(async () => oldRefetchConfirm());
    await expectNoTransaction(nonce);
    await act(async () => trade.current.requestSwap());
    expect(trade.current.review).not.toBeNull();
    const disconnectedConfirm = trade.current.confirmSwap;
    await trade.disconnect();
    expect(trade.current.quote).toBeNull();
    expect(trade.current.review).toBeNull();
    await act(async () => disconnectedConfirm());
    await expectNoTransaction(nonce);
  } finally {
    await trade.close();
  }
}

test(
  swapQuoteAmountEditsInvalidateDuringDebounce,
  { timeout: 30000 },
  swapQuoteAmountEditsInvalidateDuringDebounce,
);
test(
  swapHighImpactConfirmationBindsExactQuote,
  { timeout: 30000 },
  swapHighImpactConfirmationBindsExactQuote,
);
test(
  swapConfirmationRejectsPairRefetchAndWalletChanges,
  { timeout: 30000 },
  swapConfirmationRejectsPairRefetchAndWalletChanges,
);
