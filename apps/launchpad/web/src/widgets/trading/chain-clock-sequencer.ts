import { fileURLToPath } from 'node:url';
import { BaseSequencer, type TestSpecification } from 'vitest/node';

// These files advance the shared chain beyond wall time, expiring later swaps' deadlines.
export const chainClockAdvancingTestFiles = new Set([
  fileURLToPath(new URL('./candle-merge-across-day.test.ts', import.meta.url)),
]);

export class ChainClockSequencer extends BaseSequencer {
  async sort(files: TestSpecification[]): Promise<TestSpecification[]> {
    const ordinary: TestSpecification[] = [];
    const advancing: TestSpecification[] = [];
    for (const file of files) {
      (chainClockAdvancingTestFiles.has(file.moduleId) ? advancing : ordinary).push(file);
    }
    return [...ordinary, ...advancing];
  }
}
