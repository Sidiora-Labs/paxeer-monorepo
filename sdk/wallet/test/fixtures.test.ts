import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';

function sdk_fixtures_carry_one_answer_per_sequencer_and_explorer_rung(): void {
  const sequencer = JSON.parse(
    readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8')
  ) as Record<string, Record<string, unknown>>;
  const explorer = JSON.parse(
    readFileSync(new URL('./fixtures/explorer-status.json', import.meta.url), 'utf8')
  ) as Record<string, Record<string, unknown>>;
  const rungs = ['submitted', 'receipted', 'committed'];
  const hash = /^0x[0-9a-f]{64}$/;
  const address = /^0x[0-9a-f]{40}$/;
  const quantity = /^0x(?:0|[1-9a-f][0-9a-f]*)$/;
  const bytes = /^0x(?:[0-9a-f]{2})*$/;
  const receiptFields = [
    'seq',
    'height',
    'index',
    'timestamp',
    'status',
    'returnData',
    'logs',
    'gasUsed',
    'cumulativeGasUsed',
    'effectiveGasPrice',
    'feeDenom',
    'feeAmount',
    'sponsor',
    'from',
    'to',
    'contractAddress',
    'vmError',
    'keyId',
    'receipt',
  ];

  expect(Object.keys(sequencer).sort()).toEqual([...rungs].sort());
  expect(Object.keys(explorer).sort()).toEqual([...rungs].sort());

  for (const rung of rungs) {
    const answer = sequencer[rung];
    const status = explorer[rung];
    expect(answer.rung).toBe(rung);
    expect(answer.txHash).toMatch(hash);
    expect(answer.nonce).toMatch(quantity);
    expect(status.rung).toBe(rung);
    expect(Object.keys(status).sort()).toEqual(['block_number', 'committed_height', 'rung']);
    for (const field of ['block_number', 'committed_height']) {
      if (status[field] !== null) {
        expect(Number.isSafeInteger(status[field])).toBe(true);
        expect(status[field]).toBeGreaterThanOrEqual(0);
      }
    }

    const fields = ['rung', 'txHash', 'nonce'];
    if (rung !== 'submitted') {
      fields.push(...receiptFields);
      for (const field of [
        'seq',
        'height',
        'index',
        'timestamp',
        'status',
        'gasUsed',
        'cumulativeGasUsed',
        'effectiveGasPrice',
        'feeAmount',
      ]) {
        expect(answer[field]).toMatch(quantity);
      }
      for (const field of ['sponsor', 'from', 'to', 'contractAddress']) {
        expect(answer[field]).toMatch(address);
      }
      expect(answer.status).toBe('0x1');
      expect(answer.returnData).toMatch(bytes);
      expect(answer.logs).toEqual([]);
      expect(answer.feeDenom).toBe('uhpx');
      expect(answer.vmError).toBe('');
      expect(answer.keyId).toMatch(hash);
      expect(answer.receipt).toMatch(/^0x(?:[0-9a-f]{2})+$/);
    }
    if (rung === 'committed') {
      fields.push('blockNumber', 'blockHash');
      expect(answer.blockNumber).toMatch(quantity);
      expect(answer.blockHash).toMatch(hash);
      expect(BigInt(answer.blockNumber as string) >= BigInt(answer.height as string)).toBe(true);
      expect(Number.isSafeInteger(status.block_number)).toBe(true);
      expect(Number.isSafeInteger(status.committed_height)).toBe(true);
      expect(status.committed_height).toBeGreaterThanOrEqual(status.block_number as number);
      expect(BigInt(answer.blockNumber as string)).toBe(BigInt(status.block_number as number));
    }
    expect(Object.keys(answer).sort()).toEqual(fields.sort());
  }
}

test(sdk_fixtures_carry_one_answer_per_sequencer_and_explorer_rung);
