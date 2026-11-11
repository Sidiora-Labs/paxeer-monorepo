import { strict as assert } from 'node:assert';
import { test } from 'vitest';
import type { RevisionMessage } from './revision-projection';

import { projectMessage, orderedMessages } from './revision-projection';
test('out-of-order creation and duplicate delivery cannot resurrect deleted text', () => {
  let rows: ReadonlyMap<string, RevisionMessage> = new Map();
  const base = { id: 'one', sequence: '1', revision: '2', content: 'edited', deleted: false };
  rows = projectMessage(rows, base);
  rows = projectMessage(rows, { ...base, revision: '3', deleted: true });
  const removed = rows;
  for (let i = 0; i < 10; i++)
    rows = projectMessage(rows, { ...base, revision: '1', content: 'old' });
  assert.equal(rows, removed);
  assert.equal(rows.get('one')?.content, '');
  assert.equal(rows.size, 1);
  rows = projectMessage(rows, { ...base, id: 'two', sequence: '2', revision: '1' });
  assert.deepEqual(
    orderedMessages(rows).map((x) => x.id),
    ['one', 'two']
  );
  assert.throws(() => projectMessage(rows, { ...base, sequence: 'bad' }));
});

test('duplicate deletion cannot be replaced by conflicting content at the same revision', () => {
  const tombstone = { id: 'terminal', sequence: '7', revision: '3', content: '', deleted: true };
  const rows = projectMessage(new Map<string, RevisionMessage>(), tombstone);
  assert.equal(
    projectMessage(rows, { ...tombstone, revision: '3', deleted: false, content: 'resurrect' }),
    rows
  );
  assert.equal(rows.get('terminal')?.content, '');
});
