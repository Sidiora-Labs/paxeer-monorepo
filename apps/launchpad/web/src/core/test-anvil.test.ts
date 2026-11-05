import { chmodSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';
import { expect, test } from 'vitest';
import { resolveAnvilBinary } from './test-anvil';

function resolveAnvilBinaryPrefersAnvilBin() {
  const root = mkdtempSync(join(tmpdir(), 'test-anvil-'));
  try {
    const explicit = join(root, 'configured-anvil');
    const blockedDirectory = join(root, 'blocked');
    const firstDirectory = join(root, 'first');
    const secondDirectory = join(root, 'second');
    for (const directory of [blockedDirectory, firstDirectory, secondDirectory]) mkdirSync(directory);
    const blocked = join(blockedDirectory, 'anvil');
    const first = join(firstDirectory, 'anvil');
    const second = join(secondDirectory, 'anvil');
    for (const binary of [explicit, blocked, first, second]) {
      writeFileSync(binary, '#!/bin/sh\nexit 0\n', { mode: 0o700 });
    }
    chmodSync(blocked, 0o600);
    const PATH = [join(root, 'missing'), blockedDirectory, firstDirectory, secondDirectory].join(delimiter);

    expect(resolveAnvilBinary({ ANVIL_BIN: explicit, PATH })).toBe(explicit);
    expect(resolveAnvilBinary({ ANVIL_BIN: explicit })).toBe(explicit);
    expect(resolveAnvilBinary({ PATH })).toBe(first);
    expect(resolveAnvilBinary({ ANVIL_BIN: join(root, 'missing-anvil'), PATH })).toBe(first);
    expect(resolveAnvilBinary({ ANVIL_BIN: blocked, PATH })).toBe(first);
    expect(resolveAnvilBinary({ ANVIL_BIN: firstDirectory, PATH })).toBe(first);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

function resolveAnvilBinaryFailsWithoutCandidate() {
  const root = mkdtempSync(join(tmpdir(), 'test-anvil-'));
  try {
    const blockedDirectory = join(root, 'blocked');
    const directoryCandidate = join(root, 'directory');
    mkdirSync(blockedDirectory);
    mkdirSync(directoryCandidate);
    const blocked = join(blockedDirectory, 'anvil');
    writeFileSync(blocked, '#!/bin/sh\nexit 0\n', { mode: 0o600 });
    mkdirSync(join(directoryCandidate, 'anvil'));
    const PATH = [join(root, 'missing'), blockedDirectory, directoryCandidate].join(delimiter);
    const unavailable = new Error('anvil binary unavailable: set ANVIL_BIN or PATH');

    expect(() => resolveAnvilBinary({})).toThrow(unavailable);
    expect(() => resolveAnvilBinary({ ANVIL_BIN: '', PATH: '' })).toThrow(unavailable);
    expect(() => resolveAnvilBinary({ ANVIL_BIN: join(root, 'missing-anvil'), PATH })).toThrow(unavailable);
    expect(() => resolveAnvilBinary({ ANVIL_BIN: blocked, PATH })).toThrow(unavailable);
    expect(() => resolveAnvilBinary({ ANVIL_BIN: directoryCandidate, PATH })).toThrow(unavailable);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test(resolveAnvilBinaryPrefersAnvilBin);
test(resolveAnvilBinaryFailsWithoutCandidate);
