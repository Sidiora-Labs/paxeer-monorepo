import assert from 'node:assert/strict';
import { test } from 'node:test';
import { performance } from 'node:perf_hooks';
import { confirmedReceipt } from './local-chain.mjs';

async function confirmedReturnsTheReceiptOnceTheChainHasIt() {
  const hash = `0x${'1'.repeat(64)}`;
  const receipt = { hash, status: 1, blockNumber: 42 };
  let reads = 0;
  const blocks = [];
  const transactions = [];
  const provider = {
    async getTransactionReceipt(requested) {
      assert.equal(requested, hash);
      return ++reads === 3 ? receipt : null;
    },
    async getBlockNumber() {
      blocks.push(42);
      return 42;
    },
    async getTransaction(requested) {
      transactions.push(requested);
      return { hash };
    },
  };
  const started = performance.now();
  assert.equal(await confirmedReceipt(provider, Promise.resolve({ hash }), 500, 500), receipt);
  assert.equal(reads, 3);
  assert.deepEqual(blocks, [42, 42]);
  assert.deepEqual(transactions, [hash, hash]);
  assert.ok(performance.now() - started < 5000);
  assert.equal(
    await confirmedReceipt(
      {
        ...provider,
        async getTransactionReceipt() {
          return receipt;
        },
      },
      { hash },
      500,
      500,
    ),
    receipt,
  );
  const failure = new Error('Receipt read refused');
  await assert.rejects(
    confirmedReceipt(
      {
        ...provider,
        async getTransactionReceipt() {
          throw failure;
        },
      },
      { hash },
      500,
      500,
    ),
    (error) => error === failure,
  );
}

async function confirmedEndsAtItsLimitWithWhatTheChainKnows() {
  const hash = `0x${'2'.repeat(64)}`;
  for (const transaction of [{ hash }, null]) {
    let reads = 0;
    const provider = {
      async getTransactionReceipt(requested) {
        assert.equal(requested, hash);
        reads++;
        return null;
      },
      async getBlockNumber() {
        return 73;
      },
      async getTransaction(requested) {
        assert.equal(requested, hash);
        return transaction;
      },
    };
    const started = performance.now();
    await assert.rejects(confirmedReceipt(provider, { hash }, 500, 160), {
      message: `Receipt timed out for ${hash}: block 73, transaction known: ${transaction !== null}`,
    });
    assert.ok(reads >= 2);
    assert.ok(performance.now() - started >= 160);
    assert.ok(performance.now() - started < 5000);
  }
  for (const stalledRead of ['receipt', 'diagnostics']) {
    const provider = {
      getTransactionReceipt: () =>
        stalledRead === 'receipt' ? new Promise(() => {}) : Promise.resolve(null),
      getBlockNumber: () => new Promise(() => {}),
      getTransaction: () => new Promise(() => {}),
    };
    const started = performance.now();
    await assert.rejects(confirmedReceipt(provider, { hash }, 500, 100), {
      message: `Receipt timed out for ${hash}: block unknown, transaction known: unknown`,
    });
    assert.ok(performance.now() - started < 5000);
  }
}

async function confirmedEndsAtItsLimitWhenTheSendDoesNotAnswer() {
  let reads = 0;
  const provider = {
    async getTransactionReceipt() {
      reads++;
      return null;
    },
    async getBlockNumber() {
      reads++;
      return 91;
    },
    async getTransaction() {
      reads++;
      return null;
    },
  };
  const started = performance.now();
  await assert.rejects(confirmedReceipt(provider, new Promise(() => {}), 100, 500), {
    message: 'Transaction send timed out after 100ms before a hash was available',
  });
  assert.equal(reads, 0);
  assert.ok(performance.now() - started >= 100);
  assert.ok(performance.now() - started < 5000);
  const failure = new Error('Send refused');
  await assert.rejects(
    confirmedReceipt(provider, Promise.reject(failure), 500, 500),
    (error) => error === failure,
  );
  const controller = new AbortController();
  const pending = confirmedReceipt(
    provider,
    new Promise(() => {}),
    500,
    500,
    controller.signal,
  );
  controller.abort(new Error('Case ended'));
  await assert.rejects(pending, { message: 'Case ended' });
  assert.equal(reads, 0);
}

test(
  confirmedReturnsTheReceiptOnceTheChainHasIt.name,
  { timeout: 5000 },
  confirmedReturnsTheReceiptOnceTheChainHasIt,
);
test(
  confirmedEndsAtItsLimitWithWhatTheChainKnows.name,
  { timeout: 5000 },
  confirmedEndsAtItsLimitWithWhatTheChainKnows,
);
test(
  confirmedEndsAtItsLimitWhenTheSendDoesNotAnswer.name,
  { timeout: 5000 },
  confirmedEndsAtItsLimitWhenTheSendDoesNotAnswer,
);
