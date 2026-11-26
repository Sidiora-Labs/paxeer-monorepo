import { test } from 'vitest';
import { assertHistoryAndTopicKeepFeedDrawingNewerRevisions } from './candle-merge-content.support';

async function historyAndTopicKeepFeedDrawingNewerRevisionsAcrossDay() {
  await assertHistoryAndTopicKeepFeedDrawingNewerRevisions(true);
}
test(historyAndTopicKeepFeedDrawingNewerRevisionsAcrossDay, historyAndTopicKeepFeedDrawingNewerRevisionsAcrossDay, 150_000);
