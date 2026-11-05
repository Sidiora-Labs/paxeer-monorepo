import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { contenthashSlot, MANIFEST_DIGEST_SLOT } from '../src/index.js';

function slot_constant_matches_the_resolver_layout_vector() {
  const layout = JSON.parse(
    readFileSync(
      new URL('../../../tests/vectors/names/resolver_layout_v1.json', import.meta.url),
      'utf8'
    )
  );
  const storage = JSON.parse(
    readFileSync(
      new URL('../../../tests/vectors/names/storage_proof_v1.json', import.meta.url),
      'utf8'
    )
  );
  expect(MANIFEST_DIGEST_SLOT).toBe(layout.manifest_digest_slot);
  expect(layout.slot_formula).toBe('keccak256(abi.encode(node, manifest_digest_slot))');
  expect(contenthashSlot(storage.node)).toBe(storage.slot);
  for (const node of [
    '0x',
    `0x${'00'.repeat(31)}`,
    `0x${'00'.repeat(33)}`,
    `0x${'gg'.repeat(32)}`,
  ]) {
    expect(() => contenthashSlot(node)).toThrow(TypeError);
  }
}

test(
  slot_constant_matches_the_resolver_layout_vector,
  slot_constant_matches_the_resolver_layout_vector
);
