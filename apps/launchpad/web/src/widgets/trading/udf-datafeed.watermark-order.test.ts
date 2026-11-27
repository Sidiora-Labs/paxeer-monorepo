import { expect, test } from 'vitest';
import type { V2ContractTypes } from '@kindle/sdk';
import { classifyChartWatermark } from './udf-datafeed';

function watermark(height: string, blockHash = `0x${'ab'.repeat(32)}`): V2ContractTypes['Watermark'] {
  return {
    chainId: '125',
    height,
    blockHash,
    integrity: 'healthy',
    observedAt: '2026-11-27T00:00:00Z',
    projectionRevision: '1',
  };
}

function olderHistoryAnswerIsAnOlderObservation() {
  const kept = Object.freeze(watermark('100'));
  const incoming = Object.freeze(watermark('98', `0x${'cd'.repeat(32)}`));
  expect(classifyChartWatermark(kept, incoming, true)).toBe('older');
  expect(kept).toEqual(watermark('100'));
  expect(incoming).toEqual(watermark('98', `0x${'cd'.repeat(32)}`));
}
test(olderHistoryAnswerIsAnOlderObservation, olderHistoryAnswerIsAnOlderObservation);

function sameHeightWithAnotherHashIsAConflict() {
  const kept = watermark('100');
  for (const fromHistory of [true, false]) {
    expect(
      classifyChartWatermark(kept, watermark('100', `0x${'cd'.repeat(32)}`), fromHistory)
    ).toBe('conflict');
    expect(
      classifyChartWatermark(kept, watermark('100', `0x${'AB'.repeat(32)}`), fromHistory)
    ).toBe('advance');
    expect(classifyChartWatermark(kept, watermark('100'), fromHistory)).toBe('advance');
  }
}
test(sameHeightWithAnotherHashIsAConflict, sameHeightWithAnotherHashIsAConflict);

function lowerFrameOfTheStreamIsAConflict() {
  expect(classifyChartWatermark(watermark('100'), watermark('98'), false)).toBe('conflict');
}
test(lowerFrameOfTheStreamIsAConflict, lowerFrameOfTheStreamIsAConflict);

function higherWatermarkAdvances() {
  for (const fromHistory of [true, false]) {
    expect(classifyChartWatermark(watermark('100'), watermark('101'), fromHistory)).toBe('advance');
    for (const height of ['98', '100', '101']) {
      expect(classifyChartWatermark(null, watermark(height), fromHistory)).toBe('advance');
    }
    expect(
      classifyChartWatermark(watermark('9007199254740992'), watermark('9007199254740993'), fromHistory)
    ).toBe('advance');
  }
}
test(higherWatermarkAdvances, higherWatermarkAdvances);
