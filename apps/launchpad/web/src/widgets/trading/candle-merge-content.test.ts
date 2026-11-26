import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { decodeV2Contract, type V2ContractTypes } from '@kindle/sdk';
import { CanonicalCandleRevisionMerge, CHART_WINDOW_CANDLES } from './udf-datafeed';
import { assertHistoryAndTopicKeepFeedDrawingNewerRevisions } from './candle-merge-content.support';

type Candle = V2ContractTypes['Candle'];
const recorded = JSON.parse(readFileSync(new URL('../../core/test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8'));
const prices = ['open', 'high', 'low', 'close'] as const;
const volumes = ['volumeUsdlRaw', 'volumeTokenRaw', 'buyVolumeUsdlRaw', 'sellVolumeUsdlRaw'] as const;

function candleAnswers() {
  const history: Candle = {
    ...structuredClone(decodeV2Contract('Candle', recorded.candles.body.data.items[0])),
    revision: '1',
    seriesModelVersion: 'accounting-market-v1',
    coverage: 'partial',
    uniqueTraderCount: null,
  };
  const topic = {
    ...structuredClone(history),
    seriesModelVersion: 'qualified-exact-v2',
    coverage: 'complete' as const,
    uniqueTraderCount: '0',
    actorSemantics: 'recipient_or_sender',
    algorithmVersion: 'qualified-exact-v2',
    completenessStatus: 'complete_through_watermark',
    correctionReason: 'source_correction',
    displayRevision: '1',
    intervalEnd: new Date(Date.parse(history.start) + 60_000).toISOString(),
    provenance: 'actual_trade',
    sourceWatermarkHeight: history.lastPosition.height,
    unknownActorTradeCount: '0',
  };
  return { history, topic };
}

function equalRevisionMetadataKeepsCandleContent() {
  const { history, topic } = candleAnswers();
  const changed = Object.keys(topic).filter(key => JSON.stringify(topic[key as keyof typeof topic]) !== JSON.stringify(history[key as keyof Candle]));
  expect(changed.sort()).toEqual([
    'actorSemantics', 'algorithmVersion', 'completenessStatus', 'correctionReason',
    'coverage', 'displayRevision', 'intervalEnd', 'provenance', 'seriesModelVersion',
    'sourceWatermarkHeight', 'uniqueTraderCount', 'unknownActorTradeCount',
  ].sort());
  for (const answers of [[history, topic], [topic, history]]) {
    const merge = new CanonicalCandleRevisionMerge();
    expect(merge.accept(answers[0], history.poolAddress, history.interval)).toBe('apply');
    expect(merge.accept(answers[1], history.poolAddress, history.interval)).toBe('ignore');
    expect(merge.size).toBe(1);
  }
}
test(equalRevisionMetadataKeepsCandleContent, equalRevisionMetadataKeepsCandleContent);

function equalRevisionDrawnContentRequiresResync() {
  const { history } = candleAnswers();
  const changes: Candle[] = [];
  for (const name of prices) {
    for (const component of ['numerator', 'denominator'] as const) {
      const next = structuredClone(history);
      next[name][component] = (BigInt(next[name][component]) + 1n).toString();
      changes.push(next);
    }
  }
  for (const name of [...volumes, 'trades'] as const) {
    changes.push({ ...history, [name]: (BigInt(history[name]) + 1n).toString() });
  }
  for (const next of changes) {
    const merge = new CanonicalCandleRevisionMerge();
    expect(merge.accept(history, history.poolAddress, history.interval)).toBe('apply');
    expect(merge.accept(next, history.poolAddress, history.interval)).toBe('resync');
    expect(merge.accept(history, history.poolAddress, history.interval)).toBe('ignore');
  }
}
test(equalRevisionDrawnContentRequiresResync, equalRevisionDrawnContentRequiresResync);

function topicCandleAdditionalMembersPassContract() {
  const { topic } = candleAnswers();
  expect(decodeV2Contract('Candle', topic)).toEqual(topic);
}
test(topicCandleAdditionalMembersPassContract, topicCandleAdditionalMembersPassContract);

function candleMergePreservesRevisionScopeAndRetention() {
  const { history, topic } = candleAnswers();
  const evicted: string[] = [];
  const merge = new CanonicalCandleRevisionMerge(key => evicted.push(key));
  expect(merge.accept(history, history.poolAddress, history.interval)).toBe('apply');
  const newer = { ...topic, revision: '2', volumeUsdlRaw: (BigInt(history.volumeUsdlRaw) + 1n).toString() };
  expect(merge.accept(newer, history.poolAddress, history.interval)).toBe('apply');
  expect(merge.accept(history, history.poolAddress, history.interval)).toBe('ignore');
  expect(merge.accept(newer, history.poolAddress, history.interval)).toBe('ignore');
  expect(merge.accept(history, history.poolAddress, '5m')).toBe('resync');
  expect(merge.accept(history, history.open.baseAsset.address!, history.interval)).toBe('resync');
  expect(merge.accept({ ...history, revision: '0' }, history.poolAddress, history.interval)).toBe('resync');
  expect(merge.accept({ ...history, close: { ...history.close, denominator: '0' } }, history.poolAddress, history.interval)).toBe('resync');
  expect(merge.accept({ ...history, coverage: 'unknown' }, history.poolAddress, history.interval)).toBe('resync');
  for (let index = 1; index <= CHART_WINDOW_CANDLES; index++) {
    const candle = { ...history, start: new Date(Date.parse(history.start) + index * 60_000).toISOString() };
    expect(merge.accept(candle, history.poolAddress, history.interval)).toBe('apply');
  }
  expect(merge.size).toBe(CHART_WINDOW_CANDLES);
  expect(evicted).toEqual([`${history.poolAddress.toLowerCase()}:${history.interval}:${history.start}`]);
  merge.clear();
  expect(merge.size).toBe(0);
}
test(candleMergePreservesRevisionScopeAndRetention, candleMergePreservesRevisionScopeAndRetention);

async function historyAndTopicKeepFeedDrawingNewerRevisions() {
  await assertHistoryAndTopicKeepFeedDrawingNewerRevisions(false);
}
test(historyAndTopicKeepFeedDrawingNewerRevisions, historyAndTopicKeepFeedDrawingNewerRevisions, 150_000);
