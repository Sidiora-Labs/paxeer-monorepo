import { accessSync, constants, statSync } from 'node:fs';
import { delimiter, resolve } from 'node:path';

function isExecutableFile(path: string): boolean {
  try {
    if (!statSync(path).isFile()) return false;
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export function resolveAnvilBinary(
  env: Readonly<{ ANVIL_BIN?: string; PATH?: string; [key: string]: string | undefined }>,
): string {
  if (env.ANVIL_BIN && isExecutableFile(env.ANVIL_BIN)) return env.ANVIL_BIN;

  for (const directory of env.PATH ? env.PATH.split(delimiter) : []) {
    const candidate = resolve(directory, 'anvil');
    if (isExecutableFile(candidate)) return candidate;
  }

  throw new Error('anvil binary unavailable: set ANVIL_BIN or PATH');
}
