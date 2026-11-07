import { createHash } from 'node:crypto';
import { readFileSync, realpathSync, statSync } from 'node:fs';
import { dirname, isAbsolute, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const requiredSources = Object.freeze([
  'apps/launchpad/web/src/hooks/chat/qualify-connected.py',
  'apps/launchpad/web/src/hooks/chat/qualification-sources.mjs',
  'services/launchpad/social/internal/fanout/conn.go',
  'services/launchpad/social/internal/fanout/hub.go',
  'services/launchpad/social/internal/fanout/durable.go',
  'services/launchpad/social/internal/fanout/notification.go',
  'apps/launchpad/web/src/core/clients/chat-ws.ts',
  'apps/launchpad/web/src/hooks/chat/use-chat.ts',
]);

export function repositoryRoot(start = dirname(fileURLToPath(import.meta.url))) {
  let directory = realpathSync(start);
  for (;;) {
    try {
      if (statSync(resolve(directory, 'codify.kvx')).isFile()) return directory;
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
    const parent = dirname(directory);
    if (parent === directory) throw new Error('repository marker codify.kvx not found');
    directory = parent;
  }
}

function sourceDigest(root, name) {
  const path = realpathSync(resolve(root, name));
  const within = relative(realpathSync(root), path);
  if (within === '..' || within.startsWith('../') || isAbsolute(within) || !statSync(path).isFile()) {
    throw new Error('source candidate digest mismatch');
  }
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

export function sourceManifest(root = repositoryRoot()) {
  return Object.fromEntries(requiredSources.map(name => [name, sourceDigest(root, name)]));
}

/** @param {Record<string, string>} sources */
export function validateSourceManifest(sources, root = repositoryRoot()) {
  if (!sources || requiredSources.some(name => !Object.hasOwn(sources, name))) {
    throw new Error('measurement source manifest omits a required actual code path');
  }
  for (const [name, expected] of Object.entries(sources)) {
    if (sourceDigest(root, name) !== expected) throw new Error('source candidate digest mismatch');
  }
}

export function sourceManifestCommand(command, input = '') {
  if (command === '--manifest') return JSON.stringify(sourceManifest()) + '\n';
  if (command !== '--validate') throw new Error('expected --manifest or --validate');
  validateSourceManifest(JSON.parse(input));
  return '';
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.stdout.write(sourceManifestCommand(process.argv[2], process.argv[2] === '--validate' ? readFileSync(0, 'utf8') : ''));
}
