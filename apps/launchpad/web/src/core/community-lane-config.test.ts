import { globSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';
import config from '../widgets/community-interactions/vitest.config.mjs';

function communityLaneIncludeMatchesSuites() {
  const suites = globSync(config.test.include, { cwd: resolve(__dirname, '../..') });

  expect(suites).toEqual(['src/widgets/community-interactions/interaction-controls.test.tsx']);
}

test(communityLaneIncludeMatchesSuites);
