import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  decodeSequencerReceipt,
  JsonRpcClient,
  JsonRpcError,
  ReceiptTimeoutError,
  readSequencerReceipt,
  waitForSequencerReceipt,
  type SequencerRung,
  type TransferRpc,
} from '../src/index.js';

const recorded = JSON.parse(
  readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8')
) as Record<'submitted' | 'receipted' | 'committed', Record<string, unknown>>;
const transactionHash = String(recorded.submitted.txHash);
const foreignHash = `0x${'ff'.repeat(32)}`;
const answers: Record<string, unknown> = {
  submitted: recorded.submitted,
  receipted: recorded.receipted,
  committed: recorded.committed,
  missing: null,
  foreign: { ...recorded.submitted, txHash: foreignHash },
  unknown: { ...recorded.submitted, rung: 'final' },
};
const requests: { method: string; params: unknown[] }[] = [];
const sequences = new Map<
  string,
  { answers: readonly unknown[]; offset: number; onRequest?: () => void }
>();
let base: string;
const server = createServer(async (req, res) => {
  let body = '';
  for await (const chunk of req) body += chunk;
  const request = JSON.parse(body);
  requests.push({ method: request.method, params: request.params });
  const name = req.url?.slice(1) ?? '';
  const sequence = sequences.get(name);
  res.writeHead(200, { 'content-type': 'application/json' });
  if (
    request.method !== 'pax_sequencerReceipt' ||
    request.params?.[0] !== transactionHash ||
    request.params.length !== 1 ||
    (!(name in answers) && !sequence) ||
    (sequence && sequence.offset >= sequence.answers.length)
  ) {
    res.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: request.id,
        error: { code: -32602, message: 'invalid receipt request' },
      })
    );
    return;
  }
  const result = sequence ? sequence.answers[sequence.offset++] : answers[name];
  sequence?.onRequest?.();
  res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }));
});

beforeAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) =>
    server.close((err) => (err ? reject(err) : resolve()))
  );
});

function receiptRpc(name: string): TransferRpc {
  const client = new JsonRpcClient({ url: `${base}/${name}` });
  return (method, params) => client.call(method, params);
}

function receiptSequenceRpc(values: readonly unknown[], onRequest?: () => void): TransferRpc {
  const name = `sequence-${sequences.size}`;
  sequences.set(name, { answers: values, offset: 0, onRequest });
  return receiptRpc(name);
}

async function read_sequencer_receipt_calls_pax_sequencer_receipt_and_decodes_each_rung() {
  const start = requests.length;
  for (const rung of ['submitted', 'receipted', 'committed'] as const) {
    const answer = await readSequencerReceipt(receiptRpc(rung), transactionHash);
    expect(answer).toEqual(answers[rung]);
    expect(answer?.rung).toBe(rung);
    expect(decodeSequencerReceipt(answers[rung])).toEqual(answer);
    if (rung !== 'committed') {
      expect(answer).not.toHaveProperty('blockNumber');
      expect(answer).not.toHaveProperty('blockHash');
    }
  }
  expect(await readSequencerReceipt(receiptRpc('missing'), transactionHash)).toBeNull();
  expect(requests.slice(start)).toEqual(
    Array.from({ length: 4 }, () => ({
      method: 'pax_sequencerReceipt',
      params: [transactionHash],
    }))
  );
  const beforeInvalidHash = requests.length;
  await expect(readSequencerReceipt(receiptRpc('submitted'), 'invalid')).rejects.toThrow(
    'transaction hash'
  );
  expect(requests).toHaveLength(beforeInvalidHash);
  await expect(
    readSequencerReceipt(receiptRpc('unavailable'), transactionHash)
  ).rejects.toBeInstanceOf(JsonRpcError);
  const largeNonce = { ...recorded.submitted, nonce: '0xffffffffffffffff' };
  expect(decodeSequencerReceipt(largeNonce).nonce).toBe(largeNonce.nonce);
}

async function decode_sequencer_receipt_refuses_an_unknown_rung_or_a_foreign_hash() {
  for (const rung of ['final', 'instant', '', null, 1, undefined]) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, rung }, transactionHash)).toThrow(
      'rung'
    );
  }
  expect(() => decodeSequencerReceipt(answers.foreign, transactionHash)).toThrow('asked hash');
  await expect(readSequencerReceipt(receiptRpc('foreign'), transactionHash)).rejects.toThrow(
    'asked hash'
  );
  await expect(readSequencerReceipt(receiptRpc('unknown'), transactionHash)).rejects.toThrow(
    'rung'
  );
  for (const field of ['blockNumber', 'blockHash'] as const) {
    for (const invalid of [undefined, null, '', 1, '0x', '0x00']) {
      expect(() =>
        decodeSequencerReceipt({ ...recorded.committed, [field]: invalid })
      ).toThrow(field);
    }
  }
  for (const invalid of [null, undefined, [], true, 'submitted']) {
    expect(() => decodeSequencerReceipt(invalid)).toThrow(TypeError);
  }
  for (const txHash of [undefined, null, '0x12', foreignHash.slice(2)]) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, txHash })).toThrow('txHash');
  }
  for (const nonce of [undefined, null, 0, '0x00', '0x', '1']) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, nonce })).toThrow('nonce');
  }
  expect(decodeSequencerReceipt(recorded.receipted)).toEqual(recorded.receipted);
  expect(() => decodeSequencerReceipt({ ...recorded.committed, logs: [null] })).toThrow('log');
  expect(() => decodeSequencerReceipt({ ...recorded.committed, receipt: '0x1' })).toThrow(
    'receipt'
  );
}

function decode_sequencer_receipt_preserves_strict_validation_before_commit() {
  for (const rung of ['submitted', 'receipted', 'committed'] as const) {
    for (const field of [
      'blockNumber',
      'seq',
      'height',
      'index',
      'timestamp',
      'status',
      'gasUsed',
      'cumulativeGasUsed',
      'effectiveGasPrice',
      'feeAmount',
    ] as const) {
      for (const invalid of [null, 1, '0x', '0x00', '0x-1']) {
        expect(() => decodeSequencerReceipt({ ...recorded[rung], [field]: invalid })).toThrow(
          field
        );
      }
    }
    for (const field of ['blockHash', 'keyId'] as const) {
      for (const invalid of [null, 1, '0x', `0x${'aa'.repeat(31)}`, `0x${'aa'.repeat(33)}`]) {
        expect(() => decodeSequencerReceipt({ ...recorded[rung], [field]: invalid })).toThrow(
          field
        );
      }
    }
    for (const field of ['returnData', 'receipt'] as const) {
      for (const invalid of [null, 1, '0x1', '0xgg']) {
        expect(() => decodeSequencerReceipt({ ...recorded[rung], [field]: invalid })).toThrow(
          field
        );
      }
    }
    for (const field of ['sponsor', 'from', 'to', 'contractAddress'] as const) {
      for (const invalid of [null, 1, '0x', `0x${'aa'.repeat(19)}`, `0x${'aa'.repeat(21)}`]) {
        expect(() => decodeSequencerReceipt({ ...recorded[rung], [field]: invalid })).toThrow(
          field
        );
      }
    }
    for (const field of ['feeDenom', 'vmError'] as const) {
      for (const invalid of [null, 1, []]) {
        expect(() => decodeSequencerReceipt({ ...recorded[rung], [field]: invalid })).toThrow(
          field
        );
      }
    }
    for (const logs of [null, {}, [null], [{ topics: [] }]]) {
      expect(() => decodeSequencerReceipt({ ...recorded[rung], logs })).toThrow('log');
    }
  }
}

async function wait_for_sequencer_receipt_polling_continues_through_the_unchanged_receipted_answer() {
  const start = requests.length;
  const answer = await waitForSequencerReceipt(
    receiptSequenceRpc([recorded.submitted, recorded.receipted, recorded.committed]),
    transactionHash,
    { rung: 'committed', attempts: 3, intervalMs: 0 }
  );
  expect(answer).toEqual(recorded.committed);
  expect(requests.slice(start)).toEqual(
    Array.from({ length: 3 }, () => ({
      method: 'pax_sequencerReceipt',
      params: [transactionHash],
    }))
  );
}

async function wait_for_sequencer_receipt_refuses_malformed_bounds_before_polling() {
  const start = requests.length;
  const rpc = receiptRpc('committed');
  for (const attempts of [-1, 0.5, Number.MAX_SAFE_INTEGER + 1, NaN, Infinity, -Infinity]) {
    await expect(
      waitForSequencerReceipt(rpc, transactionHash, { rung: 'committed', attempts })
    ).rejects.toThrow('attempts');
  }
  for (const intervalMs of [-1, NaN, Infinity, -Infinity, 2147483648]) {
    await expect(
      waitForSequencerReceipt(rpc, transactionHash, { rung: 'committed', intervalMs })
    ).rejects.toThrow('intervalMs');
  }
  await expect(
    waitForSequencerReceipt(rpc, transactionHash, { rung: 'final' as SequencerRung })
  ).rejects.toThrow('unknown sequencer receipt rung');
  await expect(
    waitForSequencerReceipt(rpc, 'invalid', { rung: 'committed' })
  ).rejects.toThrow('transaction hash');
  expect(requests).toHaveLength(start);
}

async function wait_for_sequencer_receipt_returns_once_the_requested_rung_is_reached() {
  for (const [rung, values, expected] of [
    ['submitted', [null, answers.submitted, answers.unknown], answers.submitted],
    ['submitted', [answers.receipted, answers.unknown], answers.receipted],
    ['submitted', [answers.committed, answers.unknown], answers.committed],
    ['receipted', [null, answers.submitted, answers.receipted, answers.unknown], answers.receipted],
    ['receipted', [answers.committed, answers.unknown], answers.committed],
    [
      'committed',
      [null, answers.submitted, answers.receipted, answers.committed, answers.unknown],
      answers.committed,
    ],
  ] as const satisfies readonly (readonly [SequencerRung, readonly unknown[], unknown])[]) {
    const start = requests.length;
    const result = await waitForSequencerReceipt(receiptSequenceRpc(values), transactionHash, {
      rung,
      attempts: values.length,
      intervalMs: 0,
    });
    expect(result).toEqual(expected);
    expect(requests.slice(start)).toEqual(
      Array.from({ length: values.length - 1 }, () => ({
        method: 'pax_sequencerReceipt',
        params: [transactionHash],
      }))
    );
  }
  const start = requests.length;
  expect(
    await waitForSequencerReceipt(receiptRpc('committed'), transactionHash, {
      rung: 'committed',
    })
  ).toEqual(answers.committed);
  expect(requests).toHaveLength(start + 1);
  await expect(
    waitForSequencerReceipt(receiptRpc('foreign'), transactionHash, {
      rung: 'committed',
      attempts: 3,
      intervalMs: 0,
    })
  ).rejects.toThrow('asked hash');
  await expect(
    waitForSequencerReceipt(receiptRpc('unavailable'), transactionHash, {
      rung: 'committed',
      attempts: 3,
      intervalMs: 0,
    })
  ).rejects.toBeInstanceOf(JsonRpcError);
  expect(requests).toHaveLength(start + 3);
}

async function wait_for_sequencer_receipt_times_out_with_the_last_answer() {
  for (const values of [
    [null, answers.submitted, answers.receipted],
    [answers.receipted, answers.submitted],
    [answers.submitted, null],
    [null],
    [],
  ]) {
    const start = requests.length;
    const error = await waitForSequencerReceipt(receiptSequenceRpc(values), transactionHash, {
      rung: 'committed',
      attempts: values.length,
      intervalMs: 0,
    }).catch((error: unknown) => error);
    expect(error).toBeInstanceOf(ReceiptTimeoutError);
    expect(error).toMatchObject({ name: 'ReceiptTimeoutError', lastAnswer: values.at(-1) ?? null });
    expect(requests).toHaveLength(start + values.length);
  }
  const defaultStart = requests.length;
  await expect(
    waitForSequencerReceipt(receiptRpc('missing'), transactionHash, {
      rung: 'committed',
      intervalMs: 0,
    })
  ).rejects.toMatchObject({ name: 'ReceiptTimeoutError', lastAnswer: null });
  expect(requests).toHaveLength(defaultStart + 30);
  const finalStart = requests.length;
  await expect(
    waitForSequencerReceipt(receiptRpc('submitted'), transactionHash, {
      rung: 'committed',
      attempts: 1,
      intervalMs: 60_000,
    })
  ).rejects.toBeInstanceOf(ReceiptTimeoutError);
  expect(requests).toHaveLength(finalStart + 1);

  const preAborted = new AbortController();
  preAborted.abort(new Error('cancelled'));
  const abortStart = requests.length;
  await expect(
    waitForSequencerReceipt(receiptRpc('submitted'), transactionHash, {
      rung: 'committed',
      signal: preAborted.signal,
    })
  ).rejects.toMatchObject({ name: 'AbortError' });
  expect(requests).toHaveLength(abortStart);

  for (const phase of ['request', 'interval'] as const) {
    const controller = new AbortController();
    const start = requests.length;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let watchdog: ReturnType<typeof setTimeout> | undefined;
    let didAbort = false;
    const rpc = receiptSequenceRpc([answers.submitted], () => {
      if (phase === 'request') {
        didAbort = true;
        controller.abort();
      } else {
        timer = setTimeout(() => {
          didAbort = true;
          controller.abort();
        }, 50);
      }
    });
    try {
      const pending = waitForSequencerReceipt(rpc, transactionHash, {
        rung: 'committed',
        attempts: 3,
        intervalMs: 60_000,
        signal: controller.signal,
      });
      await expect(
        Promise.race([
          pending,
          new Promise<never>((_, reject) => {
            watchdog = setTimeout(
              () => reject(new Error('abort did not stop receipt polling')),
              1000
            );
          }),
        ])
      ).rejects.toMatchObject({ name: 'AbortError' });
      expect(didAbort).toBe(true);
      expect(requests).toHaveLength(start + 1);
    } finally {
      controller.abort();
      clearTimeout(timer);
      clearTimeout(watchdog);
    }
  }
}

for (const run of [
  wait_for_sequencer_receipt_returns_once_the_requested_rung_is_reached,
  wait_for_sequencer_receipt_times_out_with_the_last_answer,
  read_sequencer_receipt_calls_pax_sequencer_receipt_and_decodes_each_rung,
  decode_sequencer_receipt_refuses_an_unknown_rung_or_a_foreign_hash,
  decode_sequencer_receipt_preserves_strict_validation_before_commit,
  wait_for_sequencer_receipt_polling_continues_through_the_unchanged_receipted_answer,
  wait_for_sequencer_receipt_refuses_malformed_bounds_before_polling,
]) {
  test(run.name, run);
}
