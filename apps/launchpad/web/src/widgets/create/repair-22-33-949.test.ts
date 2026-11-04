import { createHash } from 'node:crypto';
import {
  readFileSync,
  realpathSync,
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  symlinkSync,
  rmSync,
} from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join, relative } from 'node:path';
import { createElement, act } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { builtinEnvironments } from 'vitest/environments';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createConfig, WagmiProvider, http } from 'wagmi';
import { poolPath } from '@/core/app-foundation/navigation';
import { CreationController } from './creation-controller';
import { ChainCreationAdapter } from './creation-adapters';
import {
  type CreationStorage,
  validateCreationDraft,
} from './creation-storage';
import {
  verifyCreationApproval,
  type ApprovalContext,
} from './creation-domain';
import { PurchaseRecoveryControl } from './purchase-recovery-control';
import { OpticalPresetPicker } from './optical-preset-picker';
import {
  MemoryCreationStore,
  creationWireFixture,
  creationWireDraft,
  a,
  h,
  owner,
  native,
  pool,
  deployment,
  abi,
  chain,
} from './creation-wire.fixture';
import {
  decodeFunctionData,
  type Hex,
  type Address,
  type PublicClient,
  type WalletClient,
} from 'viem';

export const repair2233949Record = {
  findings: {
    'purchase-recovery-control': 'purchaseRecoveryControlReopensAndVerifies',
    'chain-config-layout': 'chainConfigUsesTrackedProjectAndPrivateCache',
    'fixture-source-layout': 'sourceAuthorityBindsCandidateAndRejectsDrift',
    'fixture-current-controls':
      'fixtureConsumesRenderedTradingRulesAndPoolPath',
    'purchase-hash-recovery':
      'purchaseRecoveryPersistsHashAcrossInterruptedReconciliation',
    'domain-approval-fixture':
      'nativeApprovalRejectsRouterAndConsumesExactAllowance',
    'journey-bypasses-controller':
      'controllerJourneyUsesRuntimeAuthorityAndCanonicalApprovals',
    'chain-runner-layout': 'runnerSharesRootsAcrossEveryPhase',
  },
  gotchas: [
    'pnpm dlx resolves file arguments from the command working directory; use the web workspace as the explicit working directory.',
  ],
  baseline:
    'No preliminary verify ran: the explicit implementation-first order and bounded hf-build/hf-done gates prohibit an unchanged-tree verify. No pre-change failure is claimed.',
  wiring: [
    'modules/launchpad/keeper/custody.go — native launchpad domain: publish an owned chain-125 fixture with the native 0x1017 USDL spender, zero initial allowance, and runtime authority profile for creation-journey.integration.test.tsx; the legacy Solidity/Anvil deployment is retained as contract evidence and is not native qualification.',
    'sdk/launchpad/shared/config/capabilities/preflight.go — shared configuration domain: qualify the canonical approval and creation admission stages against the native fixture without replacing runtime authority.',
    'tools/launchpad/delivery — delivery domain: regenerate the phase-4 source manifest and scenario actions for this exact candidate and current accessible controls, then run the complete owned-chain/browser journey; the declared Node gate requires no native artifact or browser binary.',
  ],
} as const;

const require = createRequire(import.meta.url);
const { creationChainRoots } =
  require('./creation-chain-layout.cjs') as typeof import('./creation-chain-layout.cjs');
const { creationSourceAuthority, creationEthersEntry, creationPresetControls } =
  require('./creation-chain.fixture.cjs') as typeof import('./creation-chain.fixture.cjs');
const roots = creationChainRoots();
const hashBytes = (bytes: Buffer) =>
  createHash('sha256').update(bytes).digest('hex');

async function createdFixture() {
  const f = await creationWireFixture();
  try {
    const draft = creationWireDraft();
    await f.controller.reviewDraft(draft);
    await f.controller.approve();
    await f.controller.reviewDraft(draft);
    const attempt = await f.controller.create();
    await f.controller.resume(attempt.intentId);
    return { ...f, draft, intentId: attempt.intentId };
  } catch (error) {
    await f.close();
    throw error;
  }
}

async function purchaseReadyFixture() {
  const f = await createdFixture();
  try {
    await f.controller.approvePurchase(f.intentId);
    const quote = await f.controller.reviewPurchase(f.intentId, 100);
    return { ...f, quote };
  } catch (error) {
    await f.close();
    throw error;
  }
}

function purchaseHashes(f: Awaited<ReturnType<typeof creationWireFixture>>) {
  return [...f.peer.transactions.entries()]
    .filter(
      ([, value]) =>
        decodeFunctionData({ abi, data: value.input as Hex }).functionName ===
        'buy',
    )
    .map(([hash]) => hash);
}

function reopen(f: Awaited<ReturnType<typeof creationWireFixture>>) {
  f.controller.dispose();
  const storage: Pick<CreationStorage, keyof CreationStorage> =
    new MemoryCreationStore(owner, f.memoryStorage.database);
  return new CreationController(
    f.adapter,
    storage as CreationStorage,
    (message) => f.wallet.signMessage({ account: owner, message }),
  );
}

async function controllerJourneyUsesRuntimeAuthorityAndCanonicalApprovals() {
  const f = await purchaseReadyFixture();
  try {
    expect(f.peer.stages[0]).toMatchObject({
      stage: 'approval',
      allowance: 0n,
    });
    expect(
      f.peer.stages.some(
        (row) => row.stage === 'creation' && row.allowance === 1_000_000n,
      ),
    ).toBe(true);
    const created = (await f.storage.history())[0];
    expect(created.state).toBe('resolved');
    expect(created.poolAddress).toBe(pool);
    expect(f.controller.state.resolutionVerified).toBe(true);
    expect(f.quote.minOutRaw).toBe(
      ((BigInt(f.quote.amountOutRaw) * 9900n) / 10000n).toString(),
    );
    await f.controller.buy(f.intentId, false, false);
    await f.controller.resumePurchase(f.intentId);
    expect(f.controller.state.phase).toBe('purchase_confirmed');
    expect(f.peer.ledger.credits).toBe(3_000_000n);
    expect(f.peer.ledger.allowance(owner, native)).toBe(0n);
    expect(f.peer.ledger.allowance(owner, deployment.router)).toBe(0n);
    expect(purchaseHashes(f)).toHaveLength(1);
    await expect(f.controller.reviewPurchase(f.intentId, 100)).rejects.toThrow(
      'Recover the existing purchase',
    );
  } finally {
    await f.close();
  }
}

async function nativeApprovalRejectsRouterAndConsumesExactAllowance() {
  const f = await creationWireFixture();
  try {
    expect(deployment.router).not.toBe(native);
    const tx = await f.adapter.approve('1000000', f.scope);
    const proof = await f.adapter.approvalProof(tx, '1000000', f.scope);
    const context: ApprovalContext = {
      chainId: 125,
      token: deployment.usdl,
      spender: native,
      account: owner,
      manifestRevision: deployment.revision,
      accountEpoch: 1,
      targetRaw: '1000000',
      transactionHash: tx,
      blockHeight: String(proof.receipt.blockNumber),
      blockHash: proof.receipt.blockHash,
    };
    const allowance = {
      ...context,
      allowanceRaw: proof.allowanceRaw,
      observedAt: Date.now(),
    };
    expect(
      verifyCreationApproval(
        context,
        { ...context, status: 'success' },
        allowance,
        deployment,
        Date.now(),
        1,
      ),
    ).toBe(true);
    for (const patch of [
      { spender: deployment.router },
      { account: a(99) },
      { accountEpoch: 2 },
      { targetRaw: '2000000' },
      { transactionHash: h(99) },
      { blockHash: h(99) },
      { blockHeight: '99' },
      { status: 'reverted' as const },
    ]) {
      expect(() =>
        verifyCreationApproval(
          context,
          { ...context, status: 'success', ...patch },
          allowance,
          deployment,
          Date.now(),
          1,
        ),
      ).toThrow();
    }
    const legacy = { ...context, spender: deployment.router };
    expect(() =>
      verifyCreationApproval(
        legacy,
        { ...legacy, status: 'success' },
        { ...allowance, spender: deployment.router },
        deployment,
        Date.now(),
        1,
      ),
    ).toThrow('approval_context_invalid');
    const draft = creationWireDraft();
    await f.controller.reviewDraft(draft);
    await f.controller.create();
    expect(f.peer.ledger.credits).toBe(1_000_000n);
    expect(f.peer.ledger.allowance(owner, native)).toBe(0n);
    expect(f.peer.ledger.balances.get(owner)).toBe(19_000_000n);
    const wrong = await f.wallet.writeContract({
      address: deployment.usdl as Address,
      abi,
      functionName: 'approve',
      args: [deployment.router as Address, 1_000_000n],
    });
    await expect(
      f.adapter.approvalProof(wrong, '1000000', f.scope),
    ).rejects.toThrow('reviewed amount and spender');
  } finally {
    await f.close();
  }
}

async function purchaseRecoveryPersistsHashAcrossInterruptedReconciliation() {
  const f = await purchaseReadyFixture();
  let resumed: CreationController | undefined;
  try {
    f.peer.interruptBuyReply = true;
    await expect(f.controller.buy(f.intentId, false, false)).rejects.toThrow();
    expect(purchaseHashes(f)).toHaveLength(1);
    const tx = purchaseHashes(f)[0];
    expect((await f.storage.history())[0]).toMatchObject({
      purchaseState: 'outcome_unknown',
      purchaseReview: JSON.stringify(f.quote),
    });
    expect((await f.storage.history())[0].purchaseTxHash).toBeUndefined();
    resumed = reopen(f);
    await resumed.resume(f.intentId);
    await expect(resumed.reviewPurchase(f.intentId, 100)).rejects.toThrow(
      'Recover the existing purchase',
    );
    await expect(resumed.approvePurchase(f.intentId)).rejects.toThrow();
    await expect(resumed.skipPurchase(f.intentId)).rejects.toThrow();
    await expect(
      resumed.attachRecoveredPurchaseHash(
        f.intentId,
        (await f.storage.history())[0].createTxHash!,
      ),
    ).rejects.toThrow();
    expect((await f.storage.history())[0].purchaseTxHash).toBeUndefined();
    f.memoryStorage.database.failPurchaseConfirmationWrites = 1;
    await expect(
      resumed.attachRecoveredPurchaseHash(f.intentId, tx),
    ).rejects.toThrow('reconciliation storage interrupted');
    expect((await f.storage.history())[0]).toMatchObject({
      purchaseTxHash: tx,
      purchaseState: 'submitted',
    });
    await expect(
      resumed.attachRecoveredPurchaseHash(
        f.intentId,
        (await f.storage.history())[0].createTxHash!,
      ),
    ).rejects.toThrow('cannot be replaced');
    resumed.dispose();
    resumed = reopen(f);
    await resumed.resumePurchase(f.intentId);
    expect(resumed.state.attempt?.purchaseState).toBe('confirmed');
    expect(resumed.state.attempt?.purchaseTxHash).toBe(tx);
    expect(purchaseHashes(f)).toEqual([tx]);
    expect(f.peer.ledger.credits).toBe(3_000_000n);
    expect(
      f.memoryStorage.database.writes
        .filter((row) => row.purchaseTxHash === tx)
        .map((row) => row.purchaseState),
    ).toEqual(['submitted', 'confirmed']);
  } finally {
    resumed?.dispose();
    await f.close();
  }
}

async function purchaseHashPersistenceFailureKeepsRestartFence() {
  const f = await purchaseReadyFixture();
  let resumed: CreationController | undefined;
  try {
    f.memoryStorage.database.failPurchaseHashWrites = 1;
    await expect(f.controller.buy(f.intentId, false, false)).rejects.toThrow(
      'do not resubmit',
    );
    const tx = purchaseHashes(f)[0];
    expect((await f.storage.history())[0].purchaseState).toBe('signing_locked');
    expect((await f.storage.history())[0].purchaseTxHash).toBeUndefined();
    resumed = reopen(f);
    f.scope.epoch++;
    await resumed.resume(f.intentId);
    await expect(resumed.buy(f.intentId, false, false)).rejects.toThrow();
    await resumed.attachRecoveredPurchaseHash(f.intentId, tx);
    expect(resumed.state.attempt?.purchaseState).toBe('confirmed');
    expect(purchaseHashes(f)).toEqual([tx]);
  } finally {
    resumed?.dispose();
    await f.close();
  }
}

async function purchaseRecoveryRejectsHostileTransactionAndScope() {
  const f = await purchaseReadyFixture();
  let resumed: CreationController | undefined;
  try {
    f.peer.interruptBuyReply = true;
    await expect(f.controller.buy(f.intentId, false, false)).rejects.toThrow();
    const original = purchaseHashes(f)[0];
    resumed = reopen(f);
    for (const patch of [
      {
        amount: 1_000_000n,
        minimum: 1n,
        deadline: BigInt(Math.floor(f.quote.deadline / 1000)),
      },
      {
        amount: BigInt(f.quote.amountInRaw),
        minimum: 1n,
        deadline: BigInt(Math.floor(f.quote.deadline / 1000)),
      },
      {
        amount: BigInt(f.quote.amountInRaw),
        minimum: BigInt(f.quote.minOutRaw),
        deadline: BigInt(Math.floor(f.quote.deadline / 1000)) + 1n,
      },
    ]) {
      const approval = await f.adapter.approve(
        patch.amount.toString(),
        f.scope,
      );
      await f.adapter.approvalProof(approval, patch.amount.toString(), f.scope);
      const hostile = await f.wallet.writeContract({
        address: deployment.router as Address,
        abi,
        functionName: 'buy',
        args: [pool, patch.amount, patch.minimum, patch.deadline],
      });
      const count = f.peer.transactions.size;
      await expect(
        resumed.attachRecoveredPurchaseHash(f.intentId, hostile),
      ).rejects.toThrow('receipt_action_mismatch');
      expect(f.peer.transactions.size).toBe(count);
      expect((await f.storage.history())[0].purchaseTxHash).toBeUndefined();
    }
    f.scope.chainId = 1;
    await expect(
      resumed.attachRecoveredPurchaseHash(f.intentId, original),
    ).rejects.toThrow('Wallet changed');
    f.scope.chainId = 125;
    f.scope.creator = a(99);
    await expect(
      resumed.attachRecoveredPurchaseHash(f.intentId, original),
    ).rejects.toThrow('Wallet changed');
    f.scope.creator = owner;
    const changed = {
      ...deployment,
      revision: 'changed',
      tradeDeployment: { ...deployment.tradeDeployment!, revision: 'changed' },
    };
    const otherStorage: Pick<CreationStorage, keyof CreationStorage> =
      new MemoryCreationStore(owner, f.memoryStorage.database);
    const other = new CreationController(
      new ChainCreationAdapter(
        f.client as PublicClient,
        f.wallet as WalletClient,
        changed,
        () => f.scope,
      ),
      otherStorage as CreationStorage,
      (message) => f.wallet.signMessage({ account: owner, message }),
    );
    try {
      await expect(
        other.attachRecoveredPurchaseHash(f.intentId, original),
      ).rejects.toThrow('original creator and deployment');
    } finally {
      other.dispose();
    }
    const count = f.peer.transactions.size;
    await resumed.attachRecoveredPurchaseHash(f.intentId, original);
    expect(resumed.state.attempt?.purchaseTxHash).toBe(original);
    expect(resumed.state.attempt?.purchaseState).toBe('confirmed');
    expect(f.peer.transactions.size).toBe(count);
  } finally {
    resumed?.dispose();
    await f.close();
  }
}

function chainConfigUsesTrackedProjectAndPrivateCache() {
  const config = require('./creation-chain.config.cjs') as {
    paths: { artifacts: string; cache: string };
    networks: { lane: { url: string; chainId: number } };
  };
  expect(config.paths.artifacts).toBe(
    join(roots.source, 'contracts/launchpad/artifacts'),
  );
  expect(config.paths.cache).toBe(join(__dirname, '.private-cache'));
  expect(config.networks.lane).toEqual({
    url: 'http://127.0.0.1:22700',
    chainId: 125,
  });
  const explicit = creationChainRoots(
    roots.source,
    join(roots.source, 'contracts/launchpad'),
  );
  expect(explicit.contracts).toBe(roots.contracts);
  expect(creationEthersEntry(explicit.contracts)).toBe(
    join(
      roots.source,
      'contracts/launchpad/node_modules/ethers/lib.esm/index.js',
    ),
  );
}

function runnerSharesRootsAcrossEveryPhase() {
  const layout = JSON.parse(
    readFileSync(join(__dirname, 'creation-chain-layout.json'), 'utf8'),
  ) as { repositoryFromWidget: string; contracts: string; web: string };
  expect(realpathSync(join(__dirname, layout.repositoryFromWidget))).toBe(
    roots.source,
  );
  expect(roots.web).toBe(join(roots.source, 'apps/launchpad/web'));
  expect(realpathSync(join(roots.source, layout.contracts))).toBe(
    roots.contracts,
  );
  expect(
    JSON.parse(readFileSync(join(roots.contracts, 'package.json'), 'utf8'))
      .name,
  ).toBe('kindle-contracts');
  expect(
    JSON.parse(readFileSync(join(roots.web, 'package.json'), 'utf8')).name,
  ).toBe('@sidiora/web');
  const runner = readFileSync(join(__dirname, 'run-creation-chain.py'), 'utf8');
  expect(runner).toContain('**creation_environment(os.environ)');
  expect(runner.match(/\*\*creation_environment\(os.environ\)/g)).toHaveLength(
    2,
  );
  expect(runner).toContain('cwd=web_root');
  expect(runner).not.toContain('client/apps/web');
  expect(runner).not.toContain("root.parent/'contracts'");
}

function sourceAuthorityBindsCandidateAndRejectsDrift() {
  const owned = [
    'create-wizard.tsx',
    'creation-adapters.ts',
    'creation-chain.config.cjs',
    'creation-chain.fixture.cjs',
    'creation-chain-layout.cjs',
    'creation-chain-layout.json',
    'creation-controller.ts',
    'creation-domain.test.mjs',
    'creation-domain.ts',
    'creation-journey.integration.test.tsx',
    'creation-storage.ts',
    'run-creation-chain.py',
    'purchase-recovery-control.tsx',
    'optical-preset-picker.tsx',
    'cap-admission.ts',
  ];
  const paths = [
    ...owned.map((name) => join(__dirname, name)),
    join(roots.web, 'src/core/runtime-authority.ts'),
    join(roots.web, 'src/widgets/trading/transaction-controls/providers.ts'),
  ];
  const files = paths.map((path) => ({
    path: relative(roots.source, path),
    sha256: hashBytes(readFileSync(path)),
  }));
  const revision = '0e1d5eb94cabe89e2260279a849b7154911ef8ee';
  const manifest = { revision, files };
  expect(
    creationSourceAuthority(roots.source, manifest, revision),
  ).toHaveLength(files.length);
  expect(() =>
    creationSourceAuthority(roots.source, manifest, 'f'.repeat(40)),
  ).toThrow();
  expect(() =>
    creationSourceAuthority(
      roots.source,
      { ...manifest, files: files.slice(1) },
      revision,
    ),
  ).toThrow('Unbound actual source');
  expect(() =>
    creationSourceAuthority(
      roots.source,
      {
        ...manifest,
        files: [{ ...files[0], sha256: 'f'.repeat(64) }, ...files.slice(1)],
      },
      revision,
    ),
  ).toThrow('Current source drift');
  const temporary = mkdtempSync(join(tmpdir(), 'creation-authority-'));
  try {
    mkdirSync(join(temporary, 'candidate'));
    writeFileSync(
      join(temporary, 'outside.ts'),
      'export const outside = true;',
    );
    symlinkSync(
      join(temporary, 'outside.ts'),
      join(temporary, 'candidate/escape.ts'),
    );
    expect(() =>
      creationSourceAuthority(
        join(temporary, 'candidate'),
        {
          revision,
          files: [
            {
              path: 'escape.ts',
              sha256: hashBytes(readFileSync(join(temporary, 'outside.ts'))),
            },
          ],
        },
        revision,
      ),
    ).toThrow('escaped candidate root');
  } finally {
    rmSync(temporary, { recursive: true });
  }
}

async function fixtureConsumesRenderedTradingRulesAndPoolPath() {
  const config = createConfig({
    chains: [chain],
    transports: { [chain.id]: http('http://127.0.0.1:1', { retryCount: 0 }) },
    ssr: true,
  });
  const queryClient = new QueryClient();
  const teardown = await builtinEnvironments.jsdom.setup(globalThis, {
    jsdom: { url: 'http://localhost/' },
  });
  try {
    const draft = creationWireDraft();
    const registered = [a(201), a(202)];
    const html = renderToStaticMarkup(
      createElement(
        WagmiProvider,
        { config },
        createElement(
          QueryClientProvider,
          { client: queryClient },
          createElement(OpticalPresetPicker, {
            value: registered[0],
            registered,
            registry: { state: 'ready' },
            reasonFor: (preset) =>
              validateCreationDraft({ ...draft, opticalSelection: preset })
                .opticalSelection ?? null,
            onChange: () => {
              throw new Error('Blocked presets must not select');
            },
          }),
        ),
      ),
    );
    const host = document.createElement('div');
    host.innerHTML = html;
    const group = host.querySelector('[role="radiogroup"]')!;
    expect(group.getAttribute('aria-labelledby')).toBeTruthy();
    expect(host.querySelector('[aria-expanded="true"]')?.textContent).toContain(
      'Custom rules',
    );
    const snapshot = {
      role: group.getAttribute('role'),
      cards: [...group.querySelectorAll('[role="radio"]')].map((card) => ({
        text: card.textContent!,
        addresses: [...card.querySelectorAll('[title]')].map((node) =>
          node.getAttribute('title'),
        ),
      })),
    };
    expect(creationPresetControls(snapshot)).toEqual(registered);
    expect(
      [...group.querySelectorAll('[role="radio"]')]
        .slice(1)
        .every((node) => node.getAttribute('aria-disabled') === 'true'),
    ).toBe(true);
    expect(() =>
      creationPresetControls({
        role: 'radiogroup',
        cards: [
          { text: 'Standard', addresses: [] },
          ...Array.from({ length: 51 }, (_, i) => ({
            text: 'Custom rules',
            addresses: [a(i + 300)],
          })),
        ],
      }),
    ).toThrow('exceeded50');
    expect(poolPath(125, pool.toUpperCase().replace('0X', '0x'))).toBe(
      `/coin/${pool}`,
    );
    const fixture = readFileSync(
      join(__dirname, 'creation-chain.fixture.cjs'),
      'utf8',
    );
    expect(fixture).toMatch(/poolPath\(125,\s*poolA\.pool\)/);
    expect(fixture).toMatch(/name:\s*'Create coin'/);
    expect(fixture).not.toContain("getByLabel('Optical preset')");
    expect(fixture).not.toContain('Open live pool');
  } finally {
    queryClient.clear();
    await teardown.teardown(globalThis);
  }
}

async function purchaseRecoveryControlReopensAndVerifies() {
  const f = await purchaseReadyFixture();
  let resumed: CreationController | undefined;
  const runtime = {
    crypto: globalThis.crypto,
    AbortSignal: globalThis.AbortSignal,
    AbortController: globalThis.AbortController,
  };
  let teardown:
    | Awaited<ReturnType<typeof builtinEnvironments.jsdom.setup>>
    | undefined;
  let root: import('react-dom/client').Root | undefined;
  try {
    f.memoryStorage.database.failPurchaseHashWrites = 1;
    await expect(f.controller.buy(f.intentId, false, false)).rejects.toThrow();
    const tx = purchaseHashes(f)[0];
    resumed = reopen(f);
    await resumed.resume(f.intentId);
    teardown = await builtinEnvironments.jsdom.setup(globalThis, {
      jsdom: { url: 'http://localhost/' },
    });
    for (const [name, value] of Object.entries(runtime))
      Object.defineProperty(globalThis, name, {
        value,
        configurable: true,
        writable: true,
      });
    Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
    const { createRoot } = await import('react-dom/client');
    const host = document.createElement('div');
    document.body.append(host);
    root = createRoot(host);
    const errors: unknown[] = [];
    const perform = async (action: () => Promise<unknown>) => {
      try {
        await action();
      } catch (error) {
        errors.push(error);
      }
    };
    const render = () =>
      root!.render(
        createElement(PurchaseRecoveryControl, {
          controller: resumed!,
          intentId: f.intentId,
          busy: resumed!.state.busy,
          perform,
        }),
      );
    const unsubscribe = resumed.subscribe(() => render());
    try {
      await act(async () => render());
      expect(host.textContent).toContain('wallet transaction history');
      expect(host.querySelector('label')?.textContent).toContain(
        'Purchase transaction hash',
      );
      expect(host.querySelector('button')?.textContent).toBe(
        'Verify purchase hash',
      );
      expect(host.querySelector('button')?.disabled).toBe(true);
      // Stop and reopen the recovery UI as well as the controller.
      await act(async () => root!.unmount());
      root = createRoot(host);
      await act(async () => render());
      const input = host.querySelector('input')!;
      await act(async () => {
        Object.getOwnPropertyDescriptor(
          window.HTMLInputElement.prototype,
          'value',
        )!.set!.call(input, tx);
        input.dispatchEvent(new window.Event('input', { bubbles: true }));
      });
      expect(host.querySelector('button')?.disabled).toBe(false);
      await act(async () => {
        host.querySelector('button')!.click();
        await new Promise<void>((resolve, reject) => {
          const timer = setTimeout(() => {
            stop();
            reject(new Error('Purchase recovery did not settle'));
          }, 10000);
          const stop = resumed!.subscribe(() => {
            if (!resumed!.state.busy) {
              clearTimeout(timer);
              stop();
              resolve();
            }
          });
        });
      });
      expect(errors).toEqual([]);
      expect(resumed.state.attempt?.purchaseState).toBe('confirmed');
      expect((await f.storage.history())[0].purchaseTxHash).toBe(tx);
      expect(purchaseHashes(f)).toEqual([tx]);
    } finally {
      unsubscribe();
    }
  } finally {
    if (root) await act(async () => root!.unmount());
    resumed?.dispose();
    await teardown?.teardown(globalThis);
    for (const [name, value] of Object.entries(runtime))
      Object.defineProperty(globalThis, name, {
        value,
        configurable: true,
        writable: true,
      });
    await f.close();
  }
}

test(chainConfigUsesTrackedProjectAndPrivateCache);
test(runnerSharesRootsAcrossEveryPhase);
test(sourceAuthorityBindsCandidateAndRejectsDrift);
test(
  controllerJourneyUsesRuntimeAuthorityAndCanonicalApprovals,
  { timeout: 30000 },
  controllerJourneyUsesRuntimeAuthorityAndCanonicalApprovals,
);
test(
  nativeApprovalRejectsRouterAndConsumesExactAllowance,
  { timeout: 30000 },
  nativeApprovalRejectsRouterAndConsumesExactAllowance,
);
test(
  purchaseRecoveryPersistsHashAcrossInterruptedReconciliation,
  { timeout: 30000 },
  purchaseRecoveryPersistsHashAcrossInterruptedReconciliation,
);
test(
  purchaseRecoveryRejectsHostileTransactionAndScope,
  { timeout: 30000 },
  purchaseRecoveryRejectsHostileTransactionAndScope,
);
test(
  purchaseHashPersistenceFailureKeepsRestartFence,
  { timeout: 30000 },
  purchaseHashPersistenceFailureKeepsRestartFence,
);
test(fixtureConsumesRenderedTradingRulesAndPoolPath);
test(
  purchaseRecoveryControlReopensAndVerifies,
  { timeout: 30000 },
  purchaseRecoveryControlReopensAndVerifies,
);
