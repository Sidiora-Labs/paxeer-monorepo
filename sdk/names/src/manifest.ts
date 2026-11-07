import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';

export type Manifest = {
  version: 1;
  name: string;
  entries: Record<string, { sha256: string; size: number; type: string }>;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function validateManifest(value: unknown): asserts value is Manifest {
  if (
    !isRecord(value) ||
    Object.keys(value).sort().join(',') !== 'entries,name,version' ||
    value.version !== 1 ||
    typeof value.name !== 'string' ||
    !isRecord(value.entries)
  ) {
    throw new TypeError('Invalid manifest shape');
  }
  for (const [path, entry] of Object.entries(value.entries)) {
    if (!path.startsWith('/') || path.split('/').includes('..')) {
      throw new TypeError(`Invalid manifest path: ${path}`);
    }
    if (
      !isRecord(entry) ||
      Object.keys(entry).sort().join(',') !== 'sha256,size,type' ||
      typeof entry.sha256 !== 'string' ||
      !/^[0-9a-fA-F]{64}$/.test(entry.sha256) ||
      typeof entry.size !== 'number' ||
      !Number.isSafeInteger(entry.size) ||
      entry.size < 0 ||
      typeof entry.type !== 'string'
    ) {
      throw new TypeError(`Invalid manifest entry: ${path}`);
    }
  }
}

export function canonicalManifest(manifest: Manifest): string {
  validateManifest(manifest);
  const entries = Object.fromEntries(
    Object.keys(manifest.entries)
      .sort()
      .map((path) => {
        const entry = manifest.entries[path]!;
        return [path, { sha256: entry.sha256, size: entry.size, type: entry.type }];
      })
  );
  return JSON.stringify({ entries, name: manifest.name, version: manifest.version });
}

export function manifestDigest(manifest: Manifest): `0x${string}` {
  return `0x${bytesToHex(sha256(utf8ToBytes(canonicalManifest(manifest))))}`;
}

export function parseManifest(bytes: Uint8Array | string): Manifest {
  const text =
    typeof bytes === 'string' ? bytes : new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  const manifest: unknown = JSON.parse(text);
  validateManifest(manifest);
  return manifest;
}
