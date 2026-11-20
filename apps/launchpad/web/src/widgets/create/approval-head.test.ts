import { EventEmitter, once } from 'node:events';
import { expect, test } from 'vitest';
import type { PublicClient, WalletClient } from 'viem';
import {
  ApprovalHeadWaitError,
  KnownCapabilityAdmissionError,
  runtimeApprovalHeadWait,
  runtimeCreationAdmission,
  type ApprovalHeadWait,
} from './cap-admission';
import { ChainCreationAdapter } from './creation-adapters';
import { CreationController } from './creation-controller';
import type { CreationStorage } from './creation-storage';
import {
  creationWireDraft,
  creationWireFixture,
  deployment,
  owner,
} from './creation-wire.fixture';
import { CREATION_PAUSED, plainLaunchError, plainLaunchStatus } from './launch-copy';

type Fixture = Awaited<ReturnType<typeof creationWireFixture>>;

function observeHeads(fixture: Fixture) {
  const original = fixture.authority.network.committedHead;
  const reads: { started: number; finished?: number; error?: unknown; signal: AbortSignal }[] = [];
  const events = new EventEmitter();
  fixture.authority.network.committedHead = async (signal) => {
    const read: (typeof reads)[number] = { started: Date.now(), signal };
    reads.push(read);
    events.emit('reading');
    try {
      return await original(signal);
    } catch (error) {
      read.error = error;
      throw error;
    } finally {
      read.finished = Date.now();
      events.emit('read');
    }
  };
  return { reads, events };
}

function headController(fixture: Fixture, wait: ApprovalHeadWait) {
  const adapter = new ChainCreationAdapter(
    fixture.client as PublicClient,
    fixture.wallet as WalletClient,
    deployment,
    () => fixture.scope,
    runtimeCreationAdmission(fixture.authority, deployment),
    wait,
  );
  return new CreationController(
    adapter,
    fixture.storage as CreationStorage,
    (message) => fixture.wallet.signMessage({ account: owner, message }),
  );
}

async function approvalHeadReturnsImmediately() {
  const fixture = await creationWireFixture();
  try {
    const { reads } = observeHeads(fixture);
    const wait = runtimeApprovalHeadWait(fixture.authority);
    for (const target of [0n, fixture.peer.height]) {
      const before = reads.length;
      await wait(target);
      expect(reads).toHaveLength(before + 1);
    }
    expect(reads.every((read) => read.finished !== undefined)).toBe(true);
    await fixture.adapter.waitForApprovalHead(fixture.peer.height + 1n);
    expect(reads).toHaveLength(2);
  } finally {
    await fixture.close();
  }
}

async function approvalHeadPollsAfterTwoSeconds() {
  const fixture = await creationWireFixture();
  const abort = new AbortController();
  try {
    const { reads, events } = observeHeads(fixture);
    const first = once(events, 'read');
    let reached = false;
    const waiting = runtimeApprovalHeadWait(fixture.authority)(2n, abort.signal).then(() => { reached = true; });
    await first;
    expect(reached).toBe(false);
    const tx = await fixture.adapter.approve('1000000', fixture.scope);
    await fixture.adapter.approvalProof(tx, '1000000', fixture.scope);
    expect(fixture.peer.height).toBe(2n);
    await waiting;
    expect(reads).toHaveLength(2);
    expect(reads[1].started - reads[0].finished!).toBeGreaterThanOrEqual(2000);
    expect(reached).toBe(true);
  } finally {
    abort.abort();
    await fixture.close();
  }
}

async function approvalHeadContinuesAfterReadError() {
  const fixture = await creationWireFixture();
  const abort = new AbortController();
  try {
    const { reads, events } = observeHeads(fixture);
    const block = fixture.peer.blocks.get(1n)!;
    fixture.peer.blocks.delete(1n);
    const first = once(events, 'read');
    const waiting = runtimeApprovalHeadWait(fixture.authority)(1n, abort.signal);
    await first;
    expect(reads[0].error).toBeInstanceOf(Error);
    fixture.peer.blocks.set(1n, block);
    await waiting;
    expect(reads).toHaveLength(2);
    expect(reads[1].started - reads[0].finished!).toBeGreaterThanOrEqual(2000);
    expect(reads[1].error).toBeUndefined();
  } finally {
    abort.abort();
    await fixture.close();
  }
}

async function approvalHeadExpiresWithLastReadError() {
  const fixture = await creationWireFixture();
  try {
    const { reads } = observeHeads(fixture);
    fixture.peer.blocks.delete(1n);
    const started = Date.now();
    const error = await runtimeApprovalHeadWait(fixture.authority)(2n).catch((error: unknown) => error);
    expect(Date.now() - started).toBeGreaterThanOrEqual(60000);
    expect(error).toBeInstanceOf(ApprovalHeadWaitError);
    expect(error).not.toBeInstanceOf(KnownCapabilityAdmissionError);
    expect(reads.length).toBeGreaterThan(1);
    expect(reads.at(-1)!.error).toBeInstanceOf(Error);
    expect(error).toHaveProperty('cause', reads.at(-1)!.error);
  } finally {
    await fixture.close();
  }
}

async function approvalHeadAbortsImmediately() {
  const fixture = await creationWireFixture();
  try {
    const { reads, events } = observeHeads(fixture);
    const wait = runtimeApprovalHeadWait(fixture.authority);
    const reason = new Error('Head observation stopped.');
    await expect(wait(2n, AbortSignal.abort(reason))).rejects.toBe(reason);
    expect(reads).toHaveLength(0);
    const abort = new AbortController();
    const first = once(events, 'read');
    const outcome = wait(2n, abort.signal).catch((error: unknown) => error);
    await first;
    const started = Date.now();
    abort.abort(reason);
    expect(await outcome).toBe(reason);
    expect(Date.now() - started).toBeLessThan(2000);
    expect(reads).toHaveLength(1);
    const inFlight = new AbortController();
    const reading = once(events, 'reading');
    const inFlightOutcome = wait(2n, inFlight.signal).catch((error: unknown) => error);
    await reading;
    inFlight.abort(reason);
    expect(await inFlightOutcome).toBe(reason);
    expect(reads).toHaveLength(2);
    expect(reads[1].signal.aborted).toBe(true);
  } finally {
    await fixture.close();
  }
}

async function approvalHeadCreationWaitsForReceiptBlock() {
  const fixture = await creationWireFixture();
  const entered = new EventEmitter();
  let receiptBlock: bigint | undefined;
  const controller = headController(fixture, async (block, signal) => {
    receiptBlock = block;
    fixture.peer.height = block - 1n;
    const waiting = runtimeApprovalHeadWait(fixture.authority)(block, signal);
    entered.emit('waiting');
    await waiting;
  });
  try {
    const draft = creationWireDraft();
    const reviewed = await controller.reviewDraft(draft);
    const waiting = once(entered, 'waiting');
    const approval = controller.approve();
    await waiting;
    expect(receiptBlock).toBe(2n);
    expect([...fixture.peer.receipts.values()][0].blockNumber).toBe('0x2');
    expect(controller.state.busy).toBe(true);
    expect(controller.state.reviewed).toBe(reviewed);
    expect(controller.state.phase).toBe('approving');
    expect(plainLaunchStatus(controller.state.message)).toBe('Fee approved. Preparing your coin...');
    fixture.peer.height = receiptBlock!;
    await approval;
    expect(controller.state.reviewed).toBeUndefined();
    expect(controller.state.phase).toBe('draft');
    expect(plainLaunchStatus(controller.state.message)).toBe('Fee approved. Now create your coin');
    expect(fixture.peer.transactions.size).toBe(1);
    const fresh = await controller.reviewDraft(draft);
    expect(fresh.read.gasStage).toBe('creation');
    expect((await controller.create()).state).toBe('submitted');
  } finally {
    controller.dispose();
    await fixture.close();
  }
}

async function approvalHeadCreationResetsReviewAfterWaitExpires() {
  const fixture = await creationWireFixture();
  const controller = headController(fixture, async (block, signal) => {
    fixture.peer.height = block - 1n;
    await runtimeApprovalHeadWait(fixture.authority)(block, signal);
  });
  try {
    await controller.reviewDraft(creationWireDraft());
    const error = await controller.approve().catch((error: unknown) => error);
    expect(error).toBeInstanceOf(ApprovalHeadWaitError);
    expect(error).toHaveProperty('cause', undefined);
    expect(controller.state.reviewed).toBeUndefined();
    expect(controller.state.phase).toBe('draft');
    expect(controller.state.busy).toBe(false);
    expect(fixture.peer.transactions.size).toBe(1);
    const copy = 'Fee approved. Press Create coin again in a moment';
    expect(plainLaunchError(error)).toBe(copy);
    expect(plainLaunchStatus(controller.state.message)).toBe(copy);
    expect(plainLaunchError(error)).not.toBe(CREATION_PAUSED);
    await expect(controller.create()).rejects.toThrow('Review this launch');
    expect(fixture.peer.transactions.size).toBe(1);
  } finally {
    controller.dispose();
    await fixture.close();
  }
}

function approvalHeadErrorCopyPrecedesOtherRules() {
  const error = new ApprovalHeadWaitError(new Error('Network revision changed.'));
  const copy = 'Fee approved. Press Create coin again in a moment';
  for (const message of [
    'Creation admission failed before wallet dispatch.',
    'insufficient_usdl: image metadata purchase expired',
    'Something went wrong, try again',
    'Launch action failed.',
  ]) {
    const wrapped = new KnownCapabilityAdmissionError(message, error);
    expect(plainLaunchError(wrapped)).toBe(copy);
    expect(plainLaunchError(new Error(message, { cause: wrapped }))).toBe(copy);
  }
  expect(plainLaunchError(Object.assign(new Error('Approval stopped.', { cause: error }), { code: 4001 }))).toBe('You cancelled in your wallet');
  expect(plainLaunchError(new Error('User rejected the request.', { cause: error }))).toBe('You cancelled in your wallet');
}

async function approvalHeadPurchaseReportsApprovalInEitherCase() {
  for (const reached of [true, false]) {
    const fixture = await creationWireFixture();
    let receiptBlock: bigint | undefined;
    const controller = headController(fixture, async (block, signal) => {
      receiptBlock = block;
      if (!reached) fixture.peer.height = block - 1n;
      await runtimeApprovalHeadWait(fixture.authority)(block, signal);
    });
    try {
      const draft = creationWireDraft();
      await fixture.controller.reviewDraft(draft);
      await fixture.controller.approve();
      await fixture.controller.reviewDraft(draft);
      const attempt = await fixture.controller.create();
      const resolved = await fixture.adapter.resolve(attempt.createTxHash!, owner, undefined, draft);
      expect(resolved.state).toBe('resolved');
      await fixture.storage.put({
        ...attempt,
        poolAddress: resolved.poolAddress,
        tokenAddress: resolved.tokenAddress,
        state: 'resolved',
      });
      await controller.approvePurchase(attempt.intentId);
      expect(receiptBlock).toBe(4n);
      expect([...fixture.peer.receipts.values()].at(-1)!.blockNumber).toBe('0x4');
      expect(controller.state.quote).toBeUndefined();
      expect(controller.state.busy).toBe(false);
      expect(controller.state.message).toBe('First purchase approval confirmed. Request and review a fresh quote.');
      expect(plainLaunchStatus(controller.state.message)).toBe('Approved. Get a quote to buy');
      expect(fixture.peer.transactions.size).toBe(3);
      fixture.peer.height = receiptBlock!;
      expect(await fixture.adapter.purchaseAllowance(owner)).toBe(draft.firstBuyRaw);
    } finally {
      controller.dispose();
      await fixture.close();
    }
  }
}

test('approval head returns immediately at or above the receipt block', approvalHeadReturnsImmediately);
test('approval head polls no sooner than two seconds after a read', approvalHeadPollsAfterTwoSeconds);
test('approval head continues after a read error', approvalHeadContinuesAfterReadError);
test('approval head expires after sixty seconds with the last read error', { timeout: 90000 }, approvalHeadExpiresWithLastReadError);
test('approval head aborts immediately', approvalHeadAbortsImmediately);
test('approval head creation waits for the receipt block before refreshing the review', { timeout: 30000 }, approvalHeadCreationWaitsForReceiptBlock);
test('approval head creation resets the review when the wait expires', { timeout: 90000 }, approvalHeadCreationResetsReviewAfterWaitExpires);
test('approval head error copy precedes other rules and preserves wallet rejection', approvalHeadErrorCopyPrecedesOtherRules);
test('approval head purchase reports approval when the head arrives or the wait expires', { timeout: 90000 }, approvalHeadPurchaseReportsApprovalInEitherCase);
