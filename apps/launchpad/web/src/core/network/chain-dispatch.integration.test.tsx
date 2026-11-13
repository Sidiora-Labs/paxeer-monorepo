import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { EventEmitter } from 'node:events';
import { afterAll, expect, test } from 'vitest';
import React, { act } from 'react';
import ts from 'typescript';
import {
  createPublicClient,
  encodeDeployData,
  http as viemHttp,
  type EIP1193Provider,
  type Address,
  type Abi,
  type Hex,
} from 'viem';
const url = process.env.DISPATCH_TEST_RPC_URL ?? 'http://127.0.0.1:18531';
if (!['127.0.0.1', 'localhost', '[::1]'].includes(new URL(url).hostname))
  throw new Error('Owned loopback EVM required');
const rpc = createPublicClient({ transport: viemHttp(url, { retryCount: 0 }) });
async function nodeRpc(method: string, params: unknown[] = []): Promise<unknown> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 2, method, params }),
  });
  const body = (await response.json()) as { result?: unknown; error?: { message: string } };
  if (body.error) throw new Error(body.error.message);
  return body.result;
}

const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (
    html: string,
    options: { url: string }
  ) => { window: Window & typeof globalThis & { close(): void } };
};
const dom = new JSDOM('<html><body><main id="root"></main></body></html>', {
  url: 'http://localhost:3050',
});
const browserGlobals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'] as const;
const oldGlobals = new Map(
  browserGlobals.map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
);
Object.defineProperty(globalThis, 'window', { value: dom.window, configurable: true });
Object.defineProperty(globalThis, 'document', {
  value: dom.window.document,
  configurable: true,
});
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
afterAll(() => {
  dom.window.close();
  for (const key of browserGlobals) {
    const descriptor = oldGlobals.get(key);
    if (descriptor) Object.defineProperty(globalThis, key, descriptor);
    else Reflect.deleteProperty(globalThis, key);
  }
});
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { createRoot } = await import('react-dom/client');
const { createConfig, http, WagmiProvider } = await import('wagmi');
const { connect, disconnect } = await import('wagmi/actions');
const { injected } = await import('wagmi/connectors');
const { erc20ApprovalRequest, useWriteErc20Approve } = await import('./contracts');
const { CORE_DEPLOYMENT } = await import('./deployment');
const { useWriteMetaAGRouterSwapBestRoute } = await import('../clients/meta-ag');
const { getWalletClient } = await import('../wallet-sdk/client');
const chain = {
  id: 125,
  name: 'Paxeer',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: [url] } },
} as const;
test('all19active raw contract dispatches pin chain125', () => {
  let count = 0;
  let approvalHelpers = 0;
  let rewardDispatches = 0;
  for (const path of [
    'contracts.ts',
    '../clients/meta-ag.ts',
    '../../widgets/rewards/rewards-view.tsx',
    '../../widgets/rewards/reward-claim-recovery.ts',
  ]) {
    const src = ts.createSourceFile(
      path,
      readFileSync(new URL(path, import.meta.url), 'utf8'),
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TSX
    );
    const visit = (n: ts.Node) => {
      if (
        ts.isCallExpression(n) &&
        /(?:^|\.)writeContract$/.test(n.expression.getText(src))
      ) {
        count++;
        let arg = n.arguments[0];
        let expectedPin = '125';
        if (
          path === 'contracts.ts' &&
          arg &&
          ts.isCallExpression(arg) &&
          ts.isIdentifier(arg.expression) &&
          arg.expression.text === 'erc20ApprovalRequest'
        ) {
          approvalHelpers++;
          const helper = src.statements.find(
            (statement): statement is ts.FunctionDeclaration =>
              ts.isFunctionDeclaration(statement) &&
              statement.name?.text === 'erc20ApprovalRequest'
          );
          const returns = helper?.body?.statements.filter(ts.isReturnStatement);
          expect(returns).toHaveLength(1);
          const expression = returns![0].expression;
          if (!expression || !ts.isAsExpression(expression))
            throw new Error('Concrete approval request return required');
          arg = expression.expression;
          expectedPin = 'CORE_DEPLOYMENT.chainId';
          expect(CORE_DEPLOYMENT.chainId).toBe(125);
          expect(
            erc20ApprovalRequest({
              token: '0x0000000000000000000000000000000000000001',
              spender: CORE_DEPLOYMENT.router,
              amount: 1n,
            }).chainId
          ).toBe(125);
        }
        if (!arg || !ts.isObjectLiteralExpression(arg))
          throw new Error('Concrete dispatch options required');
        const pin = arg.properties.find(
          (p) => ts.isPropertyAssignment(p) && p.name.getText(src) === 'chainId'
        );
        expect(
          pin && ts.isPropertyAssignment(pin) ? pin.initializer.getText(src) : undefined,
          path
        ).toBe(expectedPin);
      }
      if (
        ts.isCallExpression(n) &&
        ts.isPropertyAccessExpression(n.expression) &&
        n.expression.name.text === 'sendTransaction'
      ) {
        count++;
        rewardDispatches++;
        expect(path).toBe('../../widgets/rewards/reward-claim-recovery.ts');
        const receiver = n.expression.expression;
        expect(ts.isCallExpression(receiver) && receiver.expression.getText(src)).toBe(
          'getWalletClient'
        );
        const importsWalletClient = src.statements.some(
          (statement) =>
            ts.isImportDeclaration(statement) &&
            ts.isStringLiteral(statement.moduleSpecifier) &&
            statement.moduleSpecifier.text === '@/core/wallet-sdk/client' &&
            statement.importClause?.namedBindings &&
            ts.isNamedImports(statement.importClause.namedBindings) &&
            statement.importClause.namedBindings.elements.some(
              (element) => element.name.text === 'getWalletClient' && !element.propertyName
            )
        );
        expect(importsWalletClient).toBe(true);
        const options = n.arguments[0];
        if (!options || !ts.isObjectLiteralExpression(options))
          throw new Error('Concrete reward transaction required');
        const propertyNames = (object: ts.ObjectLiteralExpression) =>
          object.properties.map((property) => {
            if (!ts.isPropertyAssignment(property) || !ts.isIdentifier(property.name))
              throw new Error('Explicit transaction fields required');
            return property.name.text;
          });
        const fields = options.properties.flatMap((property) => {
          if (ts.isPropertyAssignment(property) && ts.isIdentifier(property.name))
            return [property.name.text];
          if (
            ts.isSpreadAssignment(property) &&
            ts.isParenthesizedExpression(property.expression) &&
            ts.isConditionalExpression(property.expression.expression)
          ) {
            const conditional = property.expression.expression;
            if (
              ts.isObjectLiteralExpression(conditional.whenTrue) &&
              ts.isObjectLiteralExpression(conditional.whenFalse)
            )
              return [
                ...propertyNames(conditional.whenTrue),
                ...propertyNames(conditional.whenFalse),
              ];
          }
          throw new Error(
            'Reward transaction fields must not override the pinned client chain'
          );
        });
        expect(fields.sort()).toEqual([
          'account',
          'data',
          'gas',
          'maxFeePerGas',
          'maxPriorityFeePerGas',
          'to',
          'value',
        ]);
        expect(getWalletClient({ request: rpc.request } as EIP1193Provider).chain.id).toBe(
          125
        );
      }
      ts.forEachChild(n, visit);
    };
    visit(src);
  }
  expect(count).toBe(19);
  expect(approvalHelpers).toBe(1);
  expect(rewardDispatches).toBe(1);
});
test('actual approval hook rejects actual chain1 with zero transaction dispatch', async () => {
  expect(await rpc.getChainId()).toBe(1);
  const accounts = (await nodeRpc('eth_accounts')) as Address[];
  const events = new EventEmitter();
  let sends = 0;
  // Transparent relay forwards every request to the actual owned EVM, including chain/account lookups.
  const provider = {
    on: events.on.bind(events),
    removeListener: events.removeListener.bind(events),
    async request(args: { method: string; params?: unknown }) {
      if (/^eth_send(?:Raw)?Transaction$/.test(args.method)) sends++;
      const response = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, ...args }),
      });
      const body = (await response.json()) as {
        result?: unknown;
        error?: { code: number; message: string };
      };
      if (body.error)
        throw Object.assign(new Error(body.error.message), { code: body.error.code });
      return body.result;
    },
  } as EIP1193Provider;
  const config = createConfig({
    chains: [chain],
    connectors: [injected({ target: { id: 'owned-anvil', name: 'Owned Anvil', provider } })],
    multiInjectedProviderDiscovery: false,
    transports: { 125: http(url, { retryCount: 0 }) },
  });
  const nonce = await rpc.getTransactionCount({ address: accounts[0]!, blockTag: 'pending' });
  const query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const root = createRoot(document.getElementById('root')!);
  let current: ReturnType<typeof useWriteErc20Approve> | undefined;
  let optional: ReturnType<typeof useWriteMetaAGRouterSwapBestRoute> | undefined;
  function Probe() {
    current = useWriteErc20Approve();
    optional = useWriteMetaAGRouterSwapBestRoute();
    return null;
  }
  async function waitForObservedMutation(predicate: () => boolean) {
    for (let i = 0; i < 100; i++) {
      if (predicate()) return;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
    }
    throw new Error('Actual mutation observer did not reach the expected state');
  }
  async function resetApproval() {
    await act(async () => {
      current!.reset();
    });
    // reset() notifies asynchronously; the preceding attempt's error must clear
    // before observing the next attempt, even under the full suite's scheduler.
    await waitForObservedMutation(
      () => current?.isIdle === true && current.error === null && current.data === undefined
    );
  }

  try {
    await act(async () => {
      root.render(
        <WagmiProvider config={config} reconnectOnMount={false}>
          <QueryClientProvider client={query}>
            <Probe />
          </QueryClientProvider>
        </WagmiProvider>
      );
    });
    await act(async () => {
      await connect(config, { connector: config.connectors[0]! });
    });
    await act(async () => {
      current!.write({ token: accounts[1]!, spender: CORE_DEPLOYMENT.router, amount: 1n });
    });
    await waitForObservedMutation(() => current?.isError === true);
    expect(current?.error?.message).toMatch(/chain|network/i);
    expect(current?.data).toBeUndefined();
    expect(sends).toBe(0);
    await resetApproval();
    expect(() =>
      current!.write({ token: accounts[1]!, spender: accounts[2]!, amount: 1n })
    ).toThrow(/spender|deployment/i);
    expect(current?.isIdle).toBe(true);
    expect(current?.error).toBeNull();
    expect(() =>
      optional!.write({
        tokenIn: accounts[1]!,
        tokenOut: accounts[2]!,
        amountIn: 1n,
        amountOutMin: 1n,
        deadline: 1n,
      })
    ).toThrow(/unavailable|manifest/i);
    expect(optional?.isIdle).toBe(true);
    expect(optional?.error).toBeNull();
    expect(optional?.data).toBeUndefined();
    expect(sends).toBe(0);
    expect(
      await rpc.getTransactionCount({ address: accounts[0]!, blockTag: 'pending' })
    ).toBe(nonce);
    // Positive control executes actual ERC20 approve bytecode and reads on-chain allowance.
    await nodeRpc('anvil_setChainId', [125]);
    await act(async () => {
      await disconnect(config);
      await connect(config, { connector: config.connectors[0]! });
    });
    expect(await rpc.getChainId()).toBe(125);
    const artifactPath =
      process.env.DISPATCH_TEST_ERC20_ARTIFACT ??
      new URL(
        '../../../../../../contracts/launchpad/artifacts/contracts/core/SidioraERC20.sol/SidioraERC20.json',
        import.meta.url
      );
    const artifact = JSON.parse(readFileSync(artifactPath, 'utf8')) as {
      abi: Abi;
      bytecode: Hex;
    };
    const creation = encodeDeployData({
      abi: artifact.abi,
      bytecode: artifact.bytecode,
      args: ['Owned fixture', 'FIX', 1000000n, accounts[0]!],
    });
    const deploymentHash = (await nodeRpc('eth_sendTransaction', [
      { from: accounts[0]!, data: creation, gas: '0x4c4b40' },
    ])) as Hex;
    const deploymentReceipt = await rpc.waitForTransactionReceipt({ hash: deploymentHash });
    expect(deploymentReceipt.status).toBe('success');
    const token = deploymentReceipt.contractAddress!;
    await resetApproval();
    await act(async () => {
      current!.write({ token, spender: CORE_DEPLOYMENT.router, amount: 12345n });
    });
    await waitForObservedMutation(
      () => current?.isSuccess === true || current?.isError === true
    );
    expect(current?.error).toBeNull();
    expect(current?.data).toBeDefined();
    expect((await rpc.waitForTransactionReceipt({ hash: current!.data! })).status).toBe(
      'success'
    );
    expect(
      await rpc.readContract({
        address: token,
        abi: artifact.abi,
        functionName: 'allowance',
        args: [accounts[0]!, CORE_DEPLOYMENT.router],
      })
    ).toBe(12345n);
    expect(sends).toBe(1);
  } finally {
    try {
      await nodeRpc('anvil_setChainId', [1]);
    } finally {
      await act(async () => root.unmount());
      query.clear();
      await disconnect(config);
    }
  }
}, 15000);
