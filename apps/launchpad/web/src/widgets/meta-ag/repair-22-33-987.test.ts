import { afterEach, expect, test } from 'vitest';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, writeFileSync, rmSync, symlinkSync, statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import {
  BaseError,
  InvalidParamsRpcError,
  UserRejectedRequestError,
  decodeFunctionData,
  encodeFunctionData,
} from 'viem';
import type { AdvancedAction } from './action-recovery';
import {
  createAdvancedProvider,
  advancedOrdersAbi,
  type AdvancedProvider,
} from './advanced-provider';
import { PendingAttemptStore } from '../trading/transaction-controls/recovery';
import { prepareConnectedChainInputs, digest } from './connected-chain-inputs.mjs';
import {
  openRpcTranscript,
  owner,
  other,
  usdl,
  token,
  adapterId,
  adapter,
  routeAbi,
  router,
  blockHash,
} from './repair-22-33-987.fixtures';

const cleanup: Array<() => void | Promise<void>> = [];
afterEach(async () => {
  for (const close of cleanup.splice(0).reverse()) await close();
});

async function setup(history = false) {
  const peer = await openRpcTranscript(history);
  cleanup.push(peer.close);
  const manifestJson = JSON.stringify(peer.manifest);
  const options = {
    publicClient: peer.publicClient,
    walletClient: peer.walletClient,
    manifestJson,
    manifestSha256: createHash('sha256').update(manifestJson).digest('hex'),
    attempts: peer.storage,
  };
  async function reopen() {
    const provider = await createAdvancedProvider(options);
    cleanup.push(() => provider.close());
    return provider;
  }
  return { peer, options, provider: await reopen(), reopen };
}

async function orderReview(provider: AdvancedProvider) {
  return provider.prepareOrder(
    {
      owner,
      kind: 'limit-buy',
      token,
      settlementAsset: usdl,
      amount: '1',
      limitPrice: '1',
      expiresAt: BigInt(Math.floor(Date.now() / 1000)) + 1800n,
    },
    700000n,
  );
}
async function routeReview(provider: AdvancedProvider) {
  const quote = await provider.quote(usdl, token, '1');
  expect(quote).not.toBeNull();
  if (!quote) throw new Error('Exact fixture quote required');
  return quote;
}
async function customReview(provider: AdvancedProvider) {
  return provider.prepareCustomRoute(
    [{ adapterId, adapterAddress: adapter, tokenIn: usdl, tokenOut: token, minAmountOut: 700000n }],
    1000000n,
    700000n,
    BigInt(Math.floor(Date.now() / 1000)) + 300n,
  );
}

async function advanced_transcript_estimates_only_recorded_actions() {
  const { peer, provider } = await setup();
  await provider.placeOrder(await orderReview(provider));
  await provider.executeRoute(
    await routeReview(provider),
    700000n,
    BigInt(Math.floor(Date.now() / 1000)) + 300n,
  );
  await provider.executeCustomRoute(await customReview(provider));
  expect(peer.writes).toHaveLength(3);
  for (const [hash, tx] of peer.transactions) {
    const estimate = await peer.publicClient.request({
      method: 'eth_estimateGas',
      params: [{ from: tx.from, to: tx.to, data: tx.data }],
    });
    expect(estimate).toBe(peer.receipts.get(hash)!.gasUsed);
  }
  const unrecordedCall = encodeFunctionData({ abi: routeAbi, functionName: 'getAdapters' });
  for (const call of [
    { from: owner, to: router, data: unrecordedCall },
    { ...peer.writes[1], to: token },
    { from: owner, to: router, data: '0xdeadbeef' as const },
  ]) {
    await expect(peer.publicClient.request({ method: 'eth_estimateGas', params: [call] })).rejects.toThrow();
    await expect(peer.publicClient.request({ method: 'eth_sendTransaction', params: [call] })).rejects.toThrow();
  }
  expect(peer.writes).toHaveLength(3);
  expect(peer.control.sendRequests).toBe(3);
}
test(advanced_transcript_estimates_only_recorded_actions);

async function advanced_broadcast_recovery_placement() {
  const { peer, provider, reopen } = await setup();
  const prepared = await orderReview(provider);
  peer.control.dropReceipts = true;
  await expect(provider.placeOrder(prepared)).rejects.toThrow();
  expect(peer.writes).toHaveLength(1);
  const pending = await provider.pendingAction('placeOrder');
  expect(pending?.hash).toMatch(/^0x[0-9a-f]{64}$/);
  expect(pending?.calldata).toBe(peer.writes[0].data);
  expect(new PendingAttemptStore(peer.storage, owner).load().records).toMatchObject([
    { txHash: pending!.hash, action: 'advanced:placeOrder', actionFingerprint: pending!.intentId },
  ]);
  expect(peer.control.receiptReads).toBeGreaterThan(0);
  expect(
    peer.receiptStorageSnapshots.every(
      (snapshot) =>
        snapshot.some((value) => value.includes(`\"txHash\":\"${pending!.hash}\"`)) &&
        snapshot.some((value) => value.includes(`\"hash\":\"${pending!.hash}\"`)),
    ),
  ).toBe(true);
  provider.close();
  const reopened = await reopen();
  expect(await reopened.pendingAction('placeOrder')).toEqual(pending);
  await expect(reopened.placeOrder(await orderReview(reopened))).rejects.toThrow(
    'resume existing hash',
  );
  expect(peer.writes).toHaveLength(1);
  peer.control.dropReceipts = false;
  const recovered = await reopened.recoverAction('placeOrder');
  expect(recovered).toMatchObject({
    action: 'placeOrder',
    status: 'success',
    created: { hash: pending!.hash, orderId: 1n, creationBlock: 100n, family: 'limit' },
  });
  expect(await reopened.pendingAction('placeOrder')).toBeNull();
  expect(new PendingAttemptStore(peer.storage, owner).load().records).toHaveLength(0);
  expect(peer.writes).toHaveLength(1);
  const decoded = decodeFunctionData({ abi: advancedOrdersAbi, data: peer.writes[0].data });
  expect(decoded.functionName).toBe('placeOrder');
  if (decoded.functionName !== 'placeOrder') throw new Error('Real placement calldata required');
  expect(decoded.args[0]).toMatchObject({
    amount: 1000000n,
    token,
    stablecoin: usdl,
    minimumOutput: 700000n,
  });
}
test(advanced_broadcast_recovery_placement.name, advanced_broadcast_recovery_placement);

async function advanced_broadcast_recovery_best_route() {
  const { peer, provider, reopen } = await setup();
  const quote = await routeReview(provider);
  peer.control.dropReceipts = true;
  await expect(
    provider.executeRoute(quote, 700000n, BigInt(Math.floor(Date.now() / 1000)) + 300n),
  ).rejects.toThrow();
  const pending = await provider.pendingAction('swapViaAdapter');
  expect(pending?.hash).toMatch(/^0x[0-9a-f]{64}$/);
  expect(pending?.calldata).toBe(peer.writes[0].data);
  provider.close();
  const reopened = await reopen();
  const fresh = await routeReview(reopened);
  await expect(
    reopened.executeRoute(fresh, 700000n, BigInt(Math.floor(Date.now() / 1000)) + 300n),
  ).rejects.toThrow('resume existing hash');
  peer.control.dropReceipts = false;
  peer.control.corruptSettlement = true;
  await expect(reopened.recoverAction('swapViaAdapter')).rejects.toThrow(
    'Exact reviewed route settlement',
  );
  expect((await reopened.pendingAction('swapViaAdapter'))?.hash).toBe(pending!.hash);
  peer.control.corruptSettlement = false;
  expect(await reopened.recoverAction('swapViaAdapter')).toMatchObject({
    action: 'swapViaAdapter',
    status: 'success',
    receipt: { transactionHash: pending!.hash },
  });
  expect(await reopened.pendingAction('swapViaAdapter')).toBeNull();
  expect(peer.writes).toHaveLength(1);
  expect(decodeFunctionData({ abi: routeAbi, data: peer.writes[0].data })).toMatchObject({
    functionName: 'swapViaAdapter',
    args: [adapterId, usdl, token, 1000000n, 700000n, expect.anything()],
  });
}
test(advanced_broadcast_recovery_best_route.name, advanced_broadcast_recovery_best_route);

async function advanced_broadcast_recovery_custom_route() {
  const { peer, provider, reopen } = await setup();
  peer.control.dropReceipts = true;
  await expect(provider.executeCustomRoute(await customReview(provider))).rejects.toThrow();
  const pending = await provider.pendingAction('swapMultiHop');
  expect(pending?.hash).toMatch(/^0x[0-9a-f]{64}$/);
  provider.close();
  const reopened = await reopen();
  await expect(reopened.executeCustomRoute(await customReview(reopened))).rejects.toThrow(
    'resume existing hash',
  );
  peer.control.dropReceipts = false;
  const recovered = await reopened.recoverAction('swapMultiHop');
  expect(recovered).toMatchObject({
    action: 'swapMultiHop',
    status: 'success',
    receipt: { transactionHash: pending!.hash },
  });
  expect(await reopened.pendingAction('swapMultiHop')).toBeNull();
  expect(peer.writes).toHaveLength(1);
  expect(decodeFunctionData({ abi: routeAbi, data: peer.writes[0].data })).toMatchObject({
    functionName: 'swapMultiHop',
    args: [
      [{ tokenIn: usdl, tokenOut: token, adapterId, minAmountOut: 700000n }],
      1000000n,
      700000n,
      expect.anything(),
    ],
  });
}
test(advanced_broadcast_recovery_custom_route.name, advanced_broadcast_recovery_custom_route);

async function advanced_broadcast_recovery_concurrent_and_exact_evidence() {
  const { peer, provider, reopen } = await setup();
  const [first, second] = await Promise.all([orderReview(provider), orderReview(provider)]);
  peer.control.corruptSettlement = true;
  const results = await Promise.allSettled([
    provider.placeOrder(first),
    provider.placeOrder(second),
  ]);
  expect(results.every((result) => result.status === 'rejected')).toBe(true);
  expect(peer.writes).toHaveLength(1);
  const hash = (await provider.pendingAction('placeOrder'))!.hash;
  provider.close();
  const reopened = await reopen();
  await expect(reopened.recoverAction('placeOrder')).rejects.toThrow('Exact order placement');
  expect((await reopened.pendingAction('placeOrder'))?.hash).toBe(hash);
  peer.control.corruptSettlement = false;
  peer.control.noncanonical = true;
  await expect(reopened.recoverAction('placeOrder')).rejects.toThrow('Receipt no longer canonical');
  peer.control.noncanonical = false;
  expect(await reopened.recoverAction('placeOrder')).toMatchObject({
    status: 'success',
    created: { orderId: 1n },
  });
  expect(peer.writes).toHaveLength(1);
}
test(
  advanced_broadcast_recovery_concurrent_and_exact_evidence.name,
  advanced_broadcast_recovery_concurrent_and_exact_evidence,
);

async function advanced_signature_failure_recovery() {
  const actions: Array<{
    action: AdvancedAction;
    review: (provider: AdvancedProvider) => Promise<() => Promise<unknown>>;
  }> = [
    {
      action: 'placeOrder',
      review: async (provider) => {
        const prepared = await orderReview(provider);
        return () => provider.placeOrder(prepared);
      },
    },
    {
      action: 'swapViaAdapter',
      review: async (provider) => {
        const quote = await routeReview(provider);
        const deadline = BigInt(Math.floor(Date.now() / 1000)) + 300n;
        return () => provider.executeRoute(quote, 700000n, deadline);
      },
    },
    {
      action: 'swapMultiHop',
      review: async (provider) => {
        const prepared = await customReview(provider);
        return () => provider.executeCustomRoute(prepared);
      },
    },
  ];
  for (const { action, review } of actions) {
    for (const outcome of ['reject', 'invalid'] as const) {
      const { peer, provider, reopen } = await setup();
      const submit = await review(provider);
      peer.control.walletOutcome = outcome;
      const failure: unknown = await submit().catch((error: unknown) => error);
      expect(failure).toBeInstanceOf(BaseError);
      if (!(failure instanceof BaseError)) throw new Error('Typed wallet failure required');
      expect(
        failure.walk(
          (cause) =>
            cause instanceof
            (outcome === 'reject' ? UserRejectedRequestError : InvalidParamsRpcError),
        ),
      ).toBeInstanceOf(outcome === 'reject' ? UserRejectedRequestError : InvalidParamsRpcError);
      expect(failure.message).toContain(
        outcome === 'reject'
          ? 'Signature rejected by wallet user'
          : 'Invalid transaction parameters; transaction not accepted',
      );
      expect(peer.control.sendRequests).toBe(1);
      expect(peer.writes).toHaveLength(0);
      expect(peer.control.receiptReads).toBe(0);
      expect(await provider.pendingAction(action)).toBeNull();
      expect(new PendingAttemptStore(peer.storage, owner).load()).toEqual({
        records: [],
        corrupt: false,
      });
      await expect(submit()).rejects.toThrow();
      expect(peer.control.sendRequests).toBe(1);
      provider.close();
      const reopened = await reopen();
      expect(await reopened.pendingAction(action)).toBeNull();
      peer.control.walletOutcome = 'accept';
      await (
        await review(reopened)
      )();
      expect(peer.writes).toHaveLength(1);
      expect(peer.control.sendRequests).toBe(2);
      expect(await reopened.pendingAction(action)).toBeNull();
      expect(new PendingAttemptStore(peer.storage, owner).load().records).toHaveLength(0);
    }

    for (const outcome of ['unknown', 'timeout'] as const) {
      const { peer, provider, reopen } = await setup();
      peer.control.walletOutcome = outcome;
      await expect((await review(provider))()).rejects.toThrow();
      const pending = await provider.pendingAction(action);
      expect(pending).toMatchObject({ action, account: owner, chainId: 125 });
      expect(pending?.hash).toBeUndefined();
      const snapshot = [...peer.values];
      provider.close();
      const reopened = await reopen();
      expect(await reopened.pendingAction(action)).toEqual(pending);
      await expect(reopened.recoverAction(action)).rejects.toThrow('Signature outcome unknown');
      await expect((await review(reopened))()).rejects.toThrow('resume existing hash');
      expect([...peer.values]).toEqual(snapshot);
      expect(peer.writes).toHaveLength(0);
      expect(peer.control.sendRequests).toBe(1);
      expect(peer.control.receiptReads).toBe(0);
    }

    {
      const { peer, provider, reopen } = await setup();
      peer.control.walletOutcome = 'lost-hash';
      await expect((await review(provider))()).rejects.toThrow();
      expect(peer.writes).toHaveLength(1);
      expect(peer.control.sendRequests).toBe(1);
      const pending = await provider.pendingAction(action);
      expect(pending?.hash).toBeUndefined();
      expect(pending?.calldata).toBe(peer.writes[0].data);
      expect(peer.control.receiptReads).toBe(0);
      const snapshot = [...peer.values];
      const [hash, transaction] = [...peer.transactions][0];
      provider.close();
      const reopened = await reopen();
      expect(await reopened.pendingAction(action)).toEqual(pending);
      await expect(reopened.recoverAction(action, blockHash(999n))).rejects.toThrow();
      expect([...peer.values]).toEqual(snapshot);
      for (const change of [
        { from: other },
        { to: other },
        { data: '0x' as const },
        { chainId: 1 },
      ]) {
        peer.transactions.set(hash, { ...transaction, ...change });
        await expect(reopened.recoverAction(action, hash)).rejects.toThrow(
          'Advanced transaction intent mismatch',
        );
        expect([...peer.values]).toEqual(snapshot);
        expect(await reopened.pendingAction(action)).toEqual(pending);
      }
      peer.transactions.set(hash, transaction);
      peer.control.chainId = 1;
      await expect(reopened.recoverAction(action, hash)).rejects.toThrow('chain125');
      peer.control.chainId = 125;
      peer.control.account = other;
      await expect(reopened.recoverAction(action, hash)).rejects.toThrow('No advanced action');
      peer.control.account = owner;
      expect([...peer.values]).toEqual(snapshot);
      expect(peer.control.receiptReads).toBe(0);
      await expect((await review(reopened))()).rejects.toThrow('resume existing hash');

      peer.control.dropReceipts = true;
      await expect(reopened.recoverAction(action, hash)).rejects.toThrow();
      const adopted = await reopened.pendingAction(action);
      expect(adopted).toEqual({ ...pending, hash });
      expect(new PendingAttemptStore(peer.storage, owner).load().records).toMatchObject([
        { txHash: hash, action: `advanced:${action}`, actionFingerprint: pending!.intentId },
      ]);
      expect(peer.receiptStorageSnapshots.length).toBeGreaterThan(0);
      expect(
        peer.receiptStorageSnapshots.every(
          (values) =>
            values.some((value) => value.includes(`"txHash":"${hash}"`)) &&
            values.some((value) => value.includes(`"hash":"${hash}"`)),
        ),
      ).toBe(true);
      reopened.close();
      const resumed = await reopen();
      expect(await resumed.pendingAction(action)).toEqual(adopted);
      const adoptedSnapshot = [...peer.values];
      const receiptReads = peer.control.receiptReads;
      await expect(resumed.recoverAction(action, blockHash(999n))).rejects.toThrow(
        'Conflicting advanced transaction hash',
      );
      expect([...peer.values]).toEqual(adoptedSnapshot);
      expect(peer.control.receiptReads).toBe(receiptReads);
      peer.control.dropReceipts = false;
      peer.control.corruptSettlement = true;
      await expect(resumed.recoverAction(action, hash)).rejects.toThrow(
        action === 'placeOrder' ? 'Exact order placement' : 'Exact reviewed route settlement',
      );
      expect(await resumed.pendingAction(action)).toEqual(adopted);
      peer.control.corruptSettlement = false;
      peer.control.noncanonical = true;
      await expect(resumed.recoverAction(action)).rejects.toThrow('Receipt no longer canonical');
      expect(await resumed.pendingAction(action)).toEqual(adopted);
      peer.control.noncanonical = false;
      expect(await resumed.recoverAction(action, hash)).toMatchObject({
        action,
        status: 'success',
        receipt: { transactionHash: hash },
      });
      expect(await resumed.pendingAction(action)).toBeNull();
      expect(new PendingAttemptStore(peer.storage, owner).load().records).toHaveLength(0);
      expect(peer.writes).toHaveLength(1);
      expect(peer.control.sendRequests).toBe(1);
    }

    {
      const { peer, provider, reopen } = await setup();
      const submit = await review(provider);
      const paused = peer.pauseSignature();
      cleanup.push(paused.release);
      peer.control.walletOutcome = 'reject';
      const sending = submit().catch((error: unknown) => error);
      await paused.entered;
      try {
        const pending = await provider.pendingAction(action);
        expect(pending?.hash).toBeUndefined();
        expect(pending?.action).toBe(action);
        const snapshot = [...peer.values];
        const reopened = await reopen();
        await expect(reopened.recoverAction(action, blockHash(999n))).rejects.toThrow(
          'already signing',
        );
        await expect(reopened.recoverAction(action)).rejects.toThrow('already signing');
        await expect((await review(reopened))()).rejects.toThrow('resume existing hash');
        expect([...peer.values]).toEqual(snapshot);
        expect(await reopened.pendingAction(action)).toEqual(pending);
        expect(peer.control.receiptReads).toBe(0);
        expect(peer.control.sendRequests).toBe(1);
        expect(peer.writes).toHaveLength(0);
        provider.close();
        paused.release();
        expect(await sending).toBeInstanceOf(BaseError);
        expect(await reopened.pendingAction(action)).toBeNull();
        peer.control.walletOutcome = 'accept';
        await (
          await review(reopened)
        )();
        expect(peer.writes).toHaveLength(1);
        expect(peer.control.sendRequests).toBe(2);
        expect(await reopened.pendingAction(action)).toBeNull();
      } finally {
        paused.release();
        await sending;
      }
    }
  }
}
test(advanced_signature_failure_recovery.name, advanced_signature_failure_recovery, 30_000);

async function old_orders_unreadable_paginated_exact_history() {
  const { peer, provider } = await setup(true);
  const filled = await provider.order(1n, 'limit', 10n);
  expect(filled).toMatchObject({
    state: 'filled',
    filledRaw: 750000n,
    refundRaw: 0n,
    executionTxHash: blockHash(100021n),
  });
  const cancelled = await provider.order(2n, 'limit', 20n);
  expect(cancelled).toMatchObject({ state: 'cancelled', filledRaw: 0n, refundRaw: 1000000n });
  const expired = await provider.order(3n, 'limit', 30n);
  expect(expired).toMatchObject({ state: 'expired', filledRaw: 0n, refundRaw: 1000000n });
  const activated = await provider.order(4n, 'stop', 40n);
  expect(activated).toMatchObject({
    state: 'triggered',
    filledRaw: 0n,
    refundRaw: 0n,
    eventRevision: 5004n << 32n,
  });
  expect(peer.ranges.length).toBeGreaterThan(8);
  expect(
    peer.ranges.every(
      (range) => range.to - range.from < 2000n && range.from >= 10n && range.to <= 6010n,
    ),
  ).toBe(true);
  expect(peer.ranges.some((range) => range.from > 4000n)).toBe(true);
  peer.control.noncanonical = true;
  await expect(provider.order(1n, 'limit', 10n)).rejects.toThrow('Noncanonical order history');
}
test(
  old_orders_unreadable_paginated_exact_history.name,
  old_orders_unreadable_paginated_exact_history,
);

async function old_orders_unreadable_mixed_account_pages() {
  const { peer, provider } = await setup(true);
  const first = await provider.orderPage(null, 2);
  expect(
    first.items.map((item) => [item.orderId, item.state, item.filledRaw, item.refundRaw]),
  ).toEqual([
    ['1', 'filled', 750000n, 0n],
    ['2', 'cancelled', 0n, 1000000n],
  ]);
  expect(first.nextCursor).not.toBeNull();
  const second = await provider.orderPage(first.nextCursor, 2);
  expect(second.items.map((item) => [item.orderId, item.state, item.refundRaw])).toEqual([
    ['3', 'expired', 1000000n],
    ['4', 'triggered', 0n],
  ]);
  expect([...first.items, ...second.items].every((item) => item.owner === owner)).toBe(true);
  expect([...first.items, ...second.items].map((item) => item.owner)).not.toContain(other);
  expect(peer.ranges.every((range) => range.to - range.from < 2000n)).toBe(true);
  expect(second.nextCursor).not.toBeNull();
  const end = await provider.orderPage(second.nextCursor, 2);
  expect(end.items).toEqual([]);
  expect(end.nextCursor).not.toBeNull();
}
test(old_orders_unreadable_mixed_account_pages.name, old_orders_unreadable_mixed_account_pages);

function qualificationFixture() {
  const directory = mkdtempSync(resolve(tmpdir(), 'meta-ag-repair-'));
  cleanup.push(() => rmSync(directory, { recursive: true, force: true }));
  const manifestFile = resolve(directory, 'manifest.json');
  writeFileSync(
    manifestFile,
    JSON.stringify({ version: 2, chainId: 125, revision: 'input-admission-only' }),
  );
  const fixtureFile = resolve(directory, 'fixture.json');
  writeFileSync(
    fixtureFile,
    JSON.stringify({
      realm: 'private_fixture',
      manifestFile,
      manifestSha256: digest(manifestFile),
    }),
  );
  return {
    directory,
    manifestFile,
    fixtureFile,
    fixtureSha256: digest(fixtureFile),
    output: resolve(directory, 'private-evidence'),
  };
}

function advanced_qualification_paths_admitted_native_plan() {
  const fixture = qualificationFixture();
  const plan = prepareConnectedChainInputs(fixture);
  const manifest = JSON.parse(readFileSync(plan.sourceManifest, 'utf8'));
  expect(manifest.files['apps/launchpad/web/src/widgets/meta-ag/advanced-provider.ts']).toBe(
    digest(resolve(plan.root, 'apps/launchpad/web/src/widgets/meta-ag/advanced-provider.ts')),
  );
  expect(manifest.files['apps/launchpad/pnpm-lock.yaml']).toBe(
    digest(resolve(plan.root, 'apps/launchpad/pnpm-lock.yaml')),
  );
  expect(
    Object.keys(manifest.files).some(
      (path) => path.endsWith('/custom-route.tsx') || path === 'client/pnpm-lock.yaml',
    ),
  ).toBe(false);
  for (const [path, hash] of Object.entries(manifest.files))
    expect(hash).toBe(digest(resolve(plan.root, path)));
  expect(plan.sourceManifestSha).toBe(digest(plan.sourceManifest));
  expect(plan.command).toEqual([
    'pnpm',
    'exec',
    'vitest',
    'run',
    'src/widgets/meta-ag/connected-journeys.test.tsx',
    'src/widgets/meta-ag/domain.test.ts',
    'src/widgets/meta-ag/write-gates.test.tsx',
    'src/widgets/meta-ag/capability-gates.test.tsx',
    'src/widgets/meta-ag/measurement-harness.test.ts',
    '--config',
    'src/widgets/meta-ag/vitest.l28.config.ts',
    '--reporter=json',
    '--outputFile',
    resolve(plan.run, 'native.json'),
  ]);
  expect(plan.environment).toEqual({
    L28_CHAIN_FIXTURE_JSON: fixture.fixtureFile,
    L28_OBSERVATION_DIRECTORY: plan.run,
    L28_SOURCE_MANIFEST_JSON: plan.sourceManifest,
    L28_MEASURE: '0',
    L28_PRIVATE_CACHE_DIRECTORY: resolve(plan.run, 'cache'),
  });
  expect(statSync(plan.run).mode & 0o777).toBe(0o700);
  expect(statSync(plan.sourceManifest).mode & 0o777).toBe(0o600);
}
test(
  advanced_qualification_paths_admitted_native_plan.name,
  advanced_qualification_paths_admitted_native_plan,
);

function advanced_qualification_paths_reject_changed_or_nonregular_inputs() {
  const fixture = qualificationFixture();
  expect(() => prepareConnectedChainInputs({ ...fixture, fixtureSha256: '0'.repeat(64) })).toThrow(
    'descriptor digest mismatch',
  );
  const linkedDescriptor = resolve(fixture.directory, 'linked-descriptor.json');
  symlinkSync(fixture.fixtureFile, linkedDescriptor);
  expect(() => prepareConnectedChainInputs({ ...fixture, fixtureFile: linkedDescriptor })).toThrow(
    'Missing regular source/input',
  );
  writeFileSync(fixture.manifestFile, 'changed authority');
  expect(() => prepareConnectedChainInputs(fixture)).toThrow('deployment manifest digest mismatch');
  const link = resolve(fixture.directory, 'linked-input');
  symlinkSync(fixture.manifestFile, link);
  expect(() => digest(link)).toThrow('Missing regular source/input');
  expect(() => digest(fixture.directory)).toThrow('Missing regular source/input');
  expect(() => digest(resolve(fixture.directory, 'missing'))).toThrow();
  expect(() => prepareConnectedChainInputs({ ...fixture, timeoutSeconds: 1801 })).toThrow(
    'execution cap',
  );
}
test(
  advanced_qualification_paths_reject_changed_or_nonregular_inputs.name,
  advanced_qualification_paths_reject_changed_or_nonregular_inputs,
);
