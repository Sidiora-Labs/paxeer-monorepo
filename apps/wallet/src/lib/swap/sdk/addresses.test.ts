import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';
import { SIDIORA_CONTRACTS } from './addresses';

function sidiora_contracts_are_the_generation_2_surfaces() {
  const source = readFileSync(
    resolve(__dirname, '../../../../../../modules/launchpad/keeper/surfaces.go'),
    'utf8'
  );
  const surfaces = /^var Generation2Surfaces = map\[string\]common\.Address\{([\s\S]*?)^\}/m.exec(
    source
  );
  expect(surfaces).not.toBeNull();

  for (const role of ['Router', 'Quoter', 'PoolRegistry'] as const) {
    const entry = new RegExp(
      `^\\s*types\\.Role${role}:\\s*common\\.HexToAddress\\("(0x[0-9a-fA-F]{40})"\\),\\s*$`,
      'm'
    ).exec(surfaces![1]);
    expect(entry, `Generation2Surfaces.Role${role}`).not.toBeNull();
    expect(SIDIORA_CONTRACTS[role].toLowerCase()).toBe(entry![1].toLowerCase());
  }
}

test(sidiora_contracts_are_the_generation_2_surfaces);
