import assert from "node:assert/strict";
import { test } from "node:test";

import {
  decodeHistoryPage,
  historyCursor,
  historyParams,
  type HistoryRow,
} from "../src/history.js";

const ACCOUNT = `0x${"12".repeat(20)}`;
const ACCOUNT_PATTERN = /^0x[0-9a-f]{40}$/u;
const HASH = `0x${"ab".repeat(32)}`;

function historyRow() {
  return {
    id: "1",
    height_or_seq: "42",
    chain: "paxeer",
    kind: "erc20_transfer",
    direction: "out",
    account: ACCOUNT,
    counterparty: `0x${"34".repeat(20)}`,
    asset: `0x${"56".repeat(20)}`,
    amount: "100",
    tx_hash: HASH,
    ordinal: "3",
    final: true,
    decoded: { amount: "100" },
    asset_metadata: null,
    blockHeight: 42,
    txIndex: 2,
    logIndex: 3,
  };
}

function history_cursor_encodes_block_height_tx_index_and_log_index(): void {
  const vectors: readonly (readonly [bigint, number, number, string])[] = [
    [0n, 0, 0, "00000000000000000000000000000000"],
    [42n, 2, 3, "000000000000002a0000000200000003"],
    [
      0x0123456789abcdefn,
      0x12345678,
      0x9abcdef0,
      "0123456789abcdef123456789abcdef0",
    ],
    [
      (1n << 64n) - 1n,
      0xffffffff,
      0xffffffff,
      "ffffffffffffffffffffffffffffffff",
    ],
  ];
  for (const [height, txIndex, logIndex, expected] of vectors) {
    assert.equal(historyCursor(height, txIndex, logIndex), expected);
    assert.deepEqual(historyParams(ACCOUNT, { cursor: expected }), [
      ACCOUNT,
      expected,
      null,
      null,
    ]);
  }
  assert.deepEqual(historyParams(ACCOUNT), [ACCOUNT, null, null, null]);
  const position = historyCursor(42n, 2, 3);
  for (const cursor of [
    "",
    position.slice(1),
    `${position}0`,
    `0x${position}`,
    position.toUpperCase(),
    `${position}\n`,
    ` ${position}`,
    "g".repeat(32),
  ]) {
    assert.throws(
      () => historyParams(ACCOUNT, { cursor }),
      /Invalid history cursor/,
    );
  }
  for (const height of [-1n, 1n << 64n]) {
    assert.throws(
      () => historyCursor(height, 0, 0),
      /Invalid history position/,
    );
  }
  for (const index of [-1, 0x100000000, 0.5, NaN, Infinity]) {
    assert.throws(
      () => historyCursor(0n, index, 0),
      /Invalid history position/,
    );
    assert.throws(
      () => historyCursor(0n, 0, index),
      /Invalid history position/,
    );
  }

  const document = {
    account: ACCOUNT,
    items: [historyRow()],
    next_cursor: null,
  };
  for (const cursor of [
    historyCursor(41n, 0xffffffff, 0xffffffff),
    historyCursor(42n, 1, 0xffffffff),
    historyCursor(42n, 2, 2),
    position,
  ]) {
    assert.throws(
      () =>
        decodeHistoryPage(
          { ...document, next_cursor: cursor },
          ACCOUNT_PATTERN,
        ),
      /Invalid history cursor/,
    );
  }
  for (const cursor of [
    historyCursor(42n, 2, 4),
    historyCursor(42n, 3, 0),
    historyCursor(43n, 0, 0),
    vectors[3]![3],
  ]) {
    assert.equal(
      decodeHistoryPage({ ...document, next_cursor: cursor }, ACCOUNT_PATTERN)
        .nextCursor,
      cursor,
    );
  }
  assert.equal(decodeHistoryPage(document, ACCOUNT_PATTERN).nextCursor, null);
  assert.throws(
    () =>
      decodeHistoryPage(
        { ...document, items: [], next_cursor: position },
        ACCOUNT_PATTERN,
      ),
    /Invalid history cursor/,
  );
  assert.throws(
    () =>
      decodeHistoryPage(
        { ...document, next_cursor: `${historyCursor(43n, 0, 0)}\n` },
        ACCOUNT_PATTERN,
      ),
    /Invalid history cursor/,
  );
}

function history_rows_carry_the_transaction_hash(): void {
  const first = historyRow();
  const second = {
    ...historyRow(),
    id: "2",
    tx_hash: `0x${"CD".repeat(32)}`,
    logIndex: 4,
  };
  const document = {
    account: ACCOUNT,
    items: [first, second],
    next_cursor: null,
  };
  const page = decodeHistoryPage(document, ACCOUNT_PATTERN);
  const rows: readonly HistoryRow[] = page.items;
  assert.deepEqual(
    rows.map((row) => row.txHash),
    [first.tx_hash, second.tx_hash],
  );
  assert.equal(rows[0]!.heightOrSeq, 42n);
  assert.equal(rows[0]!.amount, 100n);
  assert.deepEqual(rows[0]!.decoded, first.decoded);
  assert.equal("txId" in rows[0]!, false);
  assert(Object.isFrozen(page));
  assert(Object.isFrozen(rows));
  assert(Object.isFrozen(rows[0]));

  const { tx_hash: _hash, ...withoutHash } = first;
  assert.throws(
    () =>
      decodeHistoryPage({ ...document, items: [withoutHash] }, ACCOUNT_PATTERN),
    /Incomplete history document/,
  );
  assert.throws(
    () =>
      decodeHistoryPage(
        { ...document, items: [{ ...withoutHash, tx_id: HASH }] },
        ACCOUNT_PATTERN,
      ),
    /Incomplete history document/,
  );
  for (const tx_hash of [
    null,
    undefined,
    1,
    "",
    "ab".repeat(32),
    `0x${"ab".repeat(31)}`,
    `0x${"ab".repeat(33)}`,
    `0x${"gg".repeat(32)}`,
    `${HASH}\n`,
  ]) {
    assert.throws(
      () =>
        decodeHistoryPage(
          { ...document, items: [{ ...first, tx_hash }] },
          ACCOUNT_PATTERN,
        ),
      /Invalid history transaction hash/,
    );
  }
}

test(history_cursor_encodes_block_height_tx_index_and_log_index);
test(history_rows_carry_the_transaction_hash);
