import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';
import { createVitest, type TestSpecification } from 'vitest/node';
import { chainClockAdvancingTestFiles } from './chain-clock-sequencer';

async function chainClockAdvancingFilesRunLastWithoutReorderingOtherFiles() {
  const root = fileURLToPath(new URL('../../../', import.meta.url));
  const cacheDirectory = await mkdtemp(join(tmpdir(), 'chain-clock-sequence-'));
  const runner = await createVitest('test', { root, watch: false, reporters: [], cache: false });
  try {
    const sequencer = new runner.config.sequence.sequencer(runner);
    const all = await runner.globTestSpecifications();
    const trading = await runner.globTestSpecifications(['src/widgets/trading']);
    const named = await runner.globTestSpecifications([
      'src/widgets/trading/candle-merge-content.test.ts',
      'src/widgets/trading/transaction-controls/trade-submit-recovery.test.tsx',
      ...chainClockAdvancingTestFiles,
    ]);
    expect(named).toHaveLength(3);
    expect(all.length).toBeGreaterThan(trading.length);
    expect(trading.length).toBeGreaterThan(named.length);
    expect(await sequencer.sort([])).toEqual([]);

    const assertOrder = async (input: TestSpecification[]) => {
      const original = [...input];
      const ordinary = input.filter(file => !chainClockAdvancingTestFiles.has(file.moduleId));
      const advancing = input.filter(file => chainClockAdvancingTestFiles.has(file.moduleId));
      expect(advancing.map(file => file.moduleId).sort()).toEqual([...chainClockAdvancingTestFiles].sort());
      const sorted = await sequencer.sort(input);
      expect(sorted).toEqual([...ordinary, ...advancing]);
      expect(input).toEqual(original);
      expect(await sequencer.sort(sorted)).toEqual(sorted);
      expect(await sequencer.sort(ordinary)).toEqual(ordinary);
      expect(await sequencer.sort(advancing)).toEqual(advancing);
    };
    const assertSelections = async () => {
      for (const files of [all, trading, named]) {
        const ordinary = files.filter(file => !chainClockAdvancingTestFiles.has(file.moduleId));
        const advancing = files.filter(file => chainClockAdvancingTestFiles.has(file.moduleId));
        for (const order of [ordinary, [...ordinary].reverse()]) {
          for (const position of [0, Math.floor(order.length / 2), order.length]) {
            await assertOrder([...order.slice(0, position), ...advancing, ...order.slice(position)]);
          }
        }
      }
    };
    await assertSelections();
    await runner.cache.stats.populateStats(root, all);
    await assertSelections();

    // A previous slow or failed day-boundary run must still follow every other file.
    runner.cache.results.setConfig(root, { dir: cacheDirectory });
    const results = all.map((file, index) => [
      `${file.project.name}:${relative(root, file.moduleId)}`,
      { failed: chainClockAdvancingTestFiles.has(file.moduleId), duration: index + 1 },
    ]);
    await writeFile(join(cacheDirectory, 'results.json'), JSON.stringify({ version: runner.version, results }));
    await runner.cache.results.readFromCache();
    for (const [key, result] of results) {
      expect(runner.cache.getFileTestResults(String(key))).toEqual(result);
    }
    await assertSelections();
  } finally {
    await runner.close();
    await rm(cacheDirectory, { recursive: true, force: true });
  }
}
test(chainClockAdvancingFilesRunLastWithoutReorderingOtherFiles, chainClockAdvancingFilesRunLastWithoutReorderingOtherFiles);
