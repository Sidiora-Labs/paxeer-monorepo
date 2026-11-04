import type { Dependency, FirstLevelDependency } from '@yarnpkg/lockfile';
import { parse } from '@yarnpkg/lockfile';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { gte, satisfies } from 'semver';
import * as v from 'valibot';

import { expect, test } from 'vitest';

// Task record: GitHub advisory records read with gh api during implementation.
// Neither unpatched upstream is retained: aliases remove those package paths
// while preserving the requiring packages' public APIs.
const ADVISORY_REPAIRS = {
  bigintBuffer: {
    alert: 3217,
    ghsa: 'GHSA-3gc7-fjrx-p6mg',
    vulnerableRange: '<=1.1.5',
    firstPatchedVersion: null,
    requiringPackages: [ '@solana/buffer-layout-utils' ],
    replacement: '@exodus/bigint-buffer',
    version: '1.1.5-exodus.1',
    remediation: 'Remove the upstream dependency path with the maintained JavaScript-only replacement; no vulnerable native converter remains.',
  },
  glob: {
    alert: 4583,
    ghsa: 'GHSA-3g3m-2fjr-6pw4',
    firstPatchedVersion: '14.0.2',
  },
  oracledb: {
    alert: 4736,
    ghsa: 'GHSA-qqmp-wf37-98f9',
    firstPatchedVersion: '0.46.0',
  },
  deepmerge: {
    alert: 4529,
    ghsa: 'GHSA-gcqx-w48m-w2h4',
    vulnerableRange: '>=4.2.0 <=4.3.1',
    firstPatchedVersion: null,
    requiringPackages: [ '@svgr/plugin-svgo', 'formik', 'swagger-client' ],
    replacement: '@stackline/deepmerge',
    version: '1.1.0',
    remediation: 'Remove all upstream dependency paths with the maintained API-compatible replacement that fixes prototype poisoning.',
  },
} as const;

const explorerRoot = path.resolve(__dirname, '..');
const requireInstalled = createRequire(path.join(explorerRoot, 'package.json'));

interface LockDependency extends FirstLevelDependency {
  readonly integrity?: string;
  readonly optionalDependencies?: Dependency;
}

function readLockfile(): Record<string, LockDependency> {
  const parsed = parse(readFileSync(path.join(explorerRoot, 'yarn.lock'), 'utf8'));
  expect(parsed.type).toBe('success');
  expect(Object.keys(parsed.object).length).toBeGreaterThan(0);
  return parsed.object;
}

function assertMinimumResolution(name: string, minimum: string): void {
  const entries = Object.entries(readLockfile()).filter(([ selector ]) => selector.startsWith(`${ name }@`));
  expect(entries.length, `${ name } must remain covered`).toBeGreaterThan(0);
  for (const [ selector, entry ] of entries) {
    expect(gte(entry.version, minimum), `${ selector } resolves to ${ entry.version }`).toBe(true);
  }
}

function assertReplacement(name: string, replacement: string, version: string, requiringPackages: ReadonlyArray<string>): void {
  const lock = readLockfile();
  const entries = Object.entries(lock).filter(([ selector ]) => selector.startsWith(`${ name }@`));
  expect(entries.length).toBeGreaterThan(0);
  for (const [ selector, entry ] of entries) {
    expect(entry.version, selector).toBe(version);
    const tarball = v.parse(v.string(), entry.resolved);
    expect(decodeURIComponent(new URL(tarball).pathname), selector).toBe(
      `/${ replacement }/-/${ name }-${ version }.tgz`,
    );
    expect(entry.integrity, selector).toMatch(/^sha512-/);
  }
  for (const [ selector, entry ] of Object.entries(lock)) {
    if (entry.resolved) {
      expect(new URL(entry.resolved).pathname.startsWith(`/${ name }/`), selector).toBe(false);
    }
    for (const dependencies of [ entry.dependencies, entry.optionalDependencies ]) {
      const requested = dependencies?.[name];
      if (requested) {
        const resolved = lock[`${ name }@${ requested }`];
        expect(resolved, `${ selector } requests ${ name }@${ requested }`).toBeDefined();
        expect(resolved.version, selector).toBe(version);
      }
    }
  }
  for (const requiring of requiringPackages) {
    const consumers = Object.entries(lock).filter(([ selector ]) => selector.startsWith(`${ requiring }@`));
    expect(consumers.length, requiring).toBeGreaterThan(0);
    for (const [ selector, entry ] of consumers) {
      expect(entry.dependencies, selector).toHaveProperty(name);
    }
    const requireConsumer = createRequire(requireInstalled.resolve(requiring));
    const manifest = v.parse(v.object({ name: v.string(), version: v.string() }),
      requireConsumer(`${ name }/package.json`));
    expect(manifest.name, requiring).toBe(replacement);
    expect(manifest.version, requiring).toBe(version);
  }
}

function testExplorerBigintBufferAdvisory(): void {
  const repair = ADVISORY_REPAIRS.bigintBuffer;
  expect(repair.firstPatchedVersion).toBeNull();
  assertReplacement('bigint-buffer', repair.replacement, repair.version, repair.requiringPackages);
  for (const [ selector, entry ] of Object.entries(readLockfile())) {
    if (selector.startsWith('bigint-buffer@')) {
      expect(satisfies(entry.version, repair.vulnerableRange), selector).toBe(false);
    }
  }
}

function testExplorerGlobAdvisory(): void {
  assertMinimumResolution('glob', ADVISORY_REPAIRS.glob.firstPatchedVersion);
}

function testExplorerOracledbAdvisory(): void {
  assertMinimumResolution('@opentelemetry/instrumentation-oracledb', ADVISORY_REPAIRS.oracledb.firstPatchedVersion);
}

function testExplorerDeepmergeAdvisory(): void {
  const repair = ADVISORY_REPAIRS.deepmerge;
  expect(repair.firstPatchedVersion).toBeNull();
  assertReplacement('deepmerge', repair.replacement, repair.version, repair.requiringPackages);
  for (const [ selector, entry ] of Object.entries(readLockfile())) {
    if (selector.startsWith('deepmerge@')) {
      expect(satisfies(entry.version, repair.vulnerableRange), selector).toBe(false);
    }
  }
}

function testExplorerBigintBufferReplacementBehavior(): void {
  const requireConsumer = createRequire(requireInstalled.resolve('@solana/buffer-layout-utils'));
  const converter = v.parse(v.object({
    toBigIntLE: v.function(),
    toBigIntBE: v.function(),
    toBufferLE: v.function(),
    toBufferBE: v.function(),
  }), requireConsumer('bigint-buffer'));
  const value = BigInt('0x0123456789abcdef');
  const littleEndian = Buffer.from('efcdab8967452301', 'hex');
  const bigEndian = Buffer.from('0123456789abcdef', 'hex');
  expect(converter.toBigIntLE(littleEndian)).toBe(value);
  expect(converter.toBigIntBE(bigEndian)).toBe(value);
  expect(converter.toBufferLE(value, 8)).toEqual(littleEndian);
  expect(converter.toBufferBE(value, 8)).toEqual(bigEndian);
  expect(converter.toBigIntLE(Buffer.alloc(0))).toBe(BigInt(0));
  const largeBuffer = Buffer.alloc(1024, 0xff);
  const largeValue = (BigInt(1) << BigInt(8192)) - BigInt(1);
  expect(converter.toBigIntLE(largeBuffer)).toBe(largeValue);
  expect(largeBuffer).toEqual(Buffer.alloc(1024, 0xff));
  expect(littleEndian).toEqual(Buffer.from('efcdab8967452301', 'hex'));
}

function testExplorerDeepmergeReplacementBehavior(): void {
  for (const requiring of ADVISORY_REPAIRS.deepmerge.requiringPackages) {
    const requireConsumer = createRequire(requireInstalled.resolve(requiring));
    const merge = v.parse(v.function(), requireConsumer('deepmerge'));
    const source: unknown = JSON.parse('{"nested":{"__proto__":{"poisoned":true},"right":2},"__proto__":{"poisoned":true}}');
    const merged: unknown = merge({ nested: { left: 1 } }, source);
    const result = v.parse(v.object({ nested: v.object({ left: v.number(), right: v.number() }) }), merged);
    expect(result).toEqual({ nested: { left: 1, right: 2 } });
    expect(Object.getPrototypeOf(merged)).toBe(Object.prototype);
    expect(Object.getPrototypeOf(result.nested)).toBe(Object.prototype);
    expect(merged).not.toHaveProperty('poisoned');
    expect(merged).not.toHaveProperty('nested.poisoned');
    expect(result.nested).not.toHaveProperty('poisoned');
    const mergeAll = v.parse(v.function(), requireConsumer('deepmerge').all);
    expect(mergeAll([ { values: [ 1 ] }, { values: [ 2 ] } ])).toEqual({ values: [ 1, 2 ] });
    expect(merge({ values: [ 1 ] }, { values: [ 2 ] }, { arrayMerge: (_left: unknown, right: unknown) => right }))
      .toEqual({ values: [ 2 ] });
  }
}

test(testExplorerBigintBufferAdvisory, testExplorerBigintBufferAdvisory);
test(testExplorerGlobAdvisory, testExplorerGlobAdvisory);
test(testExplorerOracledbAdvisory, testExplorerOracledbAdvisory);
test(testExplorerDeepmergeAdvisory, testExplorerDeepmergeAdvisory);
test(testExplorerBigintBufferReplacementBehavior, testExplorerBigintBufferReplacementBehavior);
test(testExplorerDeepmergeReplacementBehavior, testExplorerDeepmergeReplacementBehavior);
