import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { decodeContract, decodeV2Contract } from '@kindle/sdk';
import {
  actionFailure,
  approvalChainId,
  networkFailure,
  notificationLinkPath,
  PageHistory,
  pageResourceKey,
  withinDeadline,
  requireRootQuoteChain,
} from '../src/runtime/flow-state.ts';
import { directMessageFrame } from '../src/runtime/chat-frame.ts';
import {
  replacementOutcome,
  walletSubmissionState,
} from '../src/runtime/wallet-flow.ts';
import { parseNativeRoute } from '../src/contracts.ts';
import {
  SnapshotCache,
  validatePublicSnapshot,
} from '../src/runtime/offline-cache.ts';
import { PublicSnapshotDisk } from './support/offline-cache-proof.ts';
import type { TradeIdentity } from '../src/runtime/trade-recovery.ts';

async function marketRecording() {
  const recording = JSON.parse(
    await readFile(
      new URL('./vitest/fixtures/market-route-v2.json', import.meta.url),
      'utf8',
    ),
  );
  return {
    list: decodeContract('PoolListResponse', recording.poolList),
    detail: decodeContract('PoolDetailResponse', recording.poolDetail),
  };
}

async function mobileMarketSnapshotsRejectLegacyProducerContracts() {
  const { list, detail } = await marketRecording();
  const directory = await mkdtemp(
    join(tmpdir(), 'mobile-market-snapshots-'),
  );
  const disk = await PublicSnapshotDisk.open(directory);
  const storedAt = Date.parse(list.generatedAt);
  try {
    const cache = new SnapshotCache(undefined, undefined, disk);
    const poolKey = `pool:${detail.data.pool.poolAddress}`;
    for (const [key, value] of [
      ['pools', list],
      [poolKey, detail],
    ] as const) {
      assert.throws(() => validatePublicSnapshot(key, value));
      assert.throws(() =>
        cache.put(key, value, null, storedAt + 300000, storedAt),
      );
      await disk.write(
        key,
        JSON.stringify({
          formatVersion: 2,
          contractVersion: 1,
          value,
          storedAt,
          expiresAt: storedAt + 300000,
          account: null,
          bytes: Buffer.byteLength(JSON.stringify(value)),
        }),
      );
    }
    await disk.close();
    const reopened = await PublicSnapshotDisk.open(directory);
    try {
      const consumer = new SnapshotCache(undefined, undefined, reopened);
      for (const key of ['pools', poolKey])
        await assert.rejects(
          consumer.read(key, null, storedAt),
          /Unsupported public snapshot version/,
        );
    } finally {
      await reopened.close();
    }
  } finally {
    await disk.close();
    await rm(directory, { recursive: true, force: true });
  }
}
test(mobileMarketSnapshotsRejectLegacyProducerContracts);

async function mobilePagesAdvanceReturnAndRejectRepeatedCursors() {
  const recording = JSON.parse(
    await readFile(
      new URL('./vitest/fixtures/market-route-v2.json', import.meta.url),
      'utf8',
    ),
  );
  const cursor = recording.snapshot.data.snapshot.resumeCursor as string;
  const history = new PageHistory();
  assert.equal(history.cursor, undefined);
  history.next(cursor);
  assert.equal(history.cursor, cursor);
  assert.equal(pageResourceKey('pools', cursor), `pools:page:${cursor}`);
  assert.throws(() => history.next(cursor), /did not advance/);
  history.previous();
  assert.equal(history.cursor, undefined);
  history.next(cursor);
  history.reset();
  assert.equal(history.cursor, undefined);
  for (const invalid of ['', 'x'.repeat(2049)])
    assert.throws(() => history.next(invalid));
  const live = decodeV2Contract(
    'StreamListResponse',
    JSON.parse(
      await readFile(
        new URL('./fixtures/live-v2-response.json', import.meta.url),
        'utf8',
      ),
    ),
  );
  assert.deepEqual(
    validatePublicSnapshot(pageResourceKey('live', cursor), live),
    live,
  );
}
test(mobilePagesAdvanceReturnAndRejectRepeatedCursors);

function mobileServiceErrorsKeepAuthorizationAndContractErrorsDistinctFromOffline() {
  for (const status of [401, 403, 404, 409, 412, 422, 429, 501]) {
    assert.equal(networkFailure({ status }), false);
    assert.ok(actionFailure({ status }).length > 0);
  }
  assert.match(actionFailure({ status: 412 }), /Reload current data/);
  assert.match(actionFailure({ status: 401 }), /Sign out and sign in/);
  assert.match(
    actionFailure({ status: 501 }),
    /unavailable in this deployment/,
  );
  assert.equal(
    networkFailure(new TypeError('Invalid pool address')),
    false,
  );
  assert.equal(
    networkFailure(new TypeError('Network request failed')),
    true,
  );
  assert.equal(networkFailure({ status: 503 }), true);
  assert.equal(networkFailure({ status: 0 }), true);
  assert.doesNotMatch(
    actionFailure(
      new Error('https://private.invalid/signature?token=value'),
    ),
    /private|signature|token=/,
  );
}
test(
  mobileServiceErrorsKeepAuthorizationAndContractErrorsDistinctFromOffline,
);

async function mobileServiceDeadlineSettlesAndAbortsOutstandingReads() {
  const controller = new AbortController();
  let bounded: AbortSignal | undefined;
  await assert.rejects(
    withinDeadline(
      controller.signal,
      (signal) => {
        bounded = signal;
        return new Promise<never>(() => {});
      },
      5,
    ),
    { name: 'TimeoutError' },
  );
  assert.equal(bounded?.aborted, true);
  const cancelled = new AbortController();
  const request = withinDeadline(cancelled.signal, (signal) => {
    bounded = signal;
    return new Promise<never>(() => {});
  });
  cancelled.abort(new Error('Read cancelled'));
  await assert.rejects(request, /Read cancelled/);
  assert.equal(bounded?.aborted, true);
  assert.equal(
    await withinDeadline(controller.signal, async () => 'content'),
    'content',
  );
}
test(mobileServiceDeadlineSettlesAndAbortsOutstandingReads);

function mobileWalletRefusalsAndInterruptionsHaveDistinctRecoveryStates() {
  for (const code of [4001, 5000])
    assert.equal(walletSubmissionState({ code }, true), 'ready');
  assert.equal(
    walletSubmissionState(new Error('Wallet unavailable'), false),
    'ready',
  );
  assert.equal(
    walletSubmissionState(new Error('Wallet connection closed'), true),
    'unknown',
  );
  assert.equal(walletSubmissionState({ code: -32603 }, true), 'unknown');
  assert.doesNotThrow(() => requireRootQuoteChain(125));
  assert.throws(
    () => requireRootQuoteChain(125000001),
    /Open this market in the web app/,
  );
  assert.equal(approvalChainId(undefined), 125);
  assert.equal(approvalChainId(125000001), 125000001);
  assert.equal(approvalChainId(4419967295), 4419967295);
  for (const chain of [1, 125000000, 4419967296, NaN, '125', null])
    assert.throws(() => approvalChainId(chain));
}
test(mobileWalletRefusalsAndInterruptionsHaveDistinctRecoveryStates);

async function mobileReplacementIdentityRequiresTheSameAccountChainAndNonce() {
  const { detail } = await marketRecording();
  const account = detail.data.pool.poolAddress;
  const identity: TradeIdentity = {
    version: 1,
    quoteId: 'claim_revenue',
    nonce: 7,
    request: {
      chainId: '125',
      from: account,
      to: detail.data.token.address,
      data: '0x1234',
      valueRaw: '0',
      gasLimitRaw: null,
      maxFeePerGasRaw: null,
      maxPriorityFeePerGasRaw: null,
    },
  };
  const original = {
    from: account,
    to: identity.request.to,
    chainId: 125,
    nonce: 7,
    input: identity.request.data,
    value: 0n,
  };
  assert.equal(replacementOutcome(identity, original), 'same_action');
  assert.equal(
    replacementOutcome(identity, {
      ...original,
      to: account,
      input: '0x',
    }),
    'cancelled',
  );
  assert.equal(
    replacementOutcome(identity, { ...original, input: '0x5678' }),
    'replaced_action',
  );
  for (const altered of [
    { ...original, chainId: 1 },
    { ...original, nonce: 8 },
    { ...original, from: detail.data.token.address },
  ])
    assert.throws(
      () => replacementOutcome(identity, altered),
      /same account, chain and nonce/,
    );
}
test(mobileReplacementIdentityRequiresTheSameAccountChainAndNonce);

async function mobileDirectMessagesUseDurableSenderFieldsAndServiceLength() {
  const { list, detail } = await marketRecording();
  const frame = directMessageFrame(
    detail.data.token.address,
    'Hello 🌍',
    list.requestId,
  );
  assert.deepEqual(frame, {
    type: 'dm',
    to: detail.data.token.address.toLowerCase(),
    content: 'Hello 🌍',
    client_message_id: list.requestId,
  });
  assert.equal(
    directMessageFrame(
      frame.to,
      '🌍'.repeat(500),
      frame.client_message_id,
    ).content,
    '🌍'.repeat(500),
  );
  for (const body of ['', '   ', '🌍'.repeat(501)])
    assert.throws(() =>
      directMessageFrame(frame.to, body, frame.client_message_id),
    );
  assert.throws(() =>
    directMessageFrame('invalid', 'Hello', frame.client_message_id),
  );
  assert.throws(() => directMessageFrame(frame.to, 'Hello', 'invalid'));
}
test(mobileDirectMessagesUseDurableSenderFieldsAndServiceLength);

function mobileNotificationLinksOpenOnlyTheAuthorizedInboxDestination() {
  const id = 'ab'.repeat(32);
  assert.deepEqual(
    parseNativeRoute(notificationLinkPath(`/notifications/${id}`)),
    { name: 'notifications' },
  );
  for (const path of [
    `/notifications/${id}?token=x`,
    '/notifications/short',
    '/notifications/%61' + id.slice(1),
    `https://external.invalid/notifications/${id}`,
  ])
    assert.throws(() => parseNativeRoute(notificationLinkPath(path)));
}
test(mobileNotificationLinksOpenOnlyTheAuthorizedInboxDestination);
